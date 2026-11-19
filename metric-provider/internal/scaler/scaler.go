// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scaler

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "crema/metric-provider/proto"

	"github.com/go-logr/logr"
)

const (
	recommendedInstanceCountMetricName = "recommended_instance_count"
	requestedInstanceCountMetricName   = "requested_instance_count"
	metricValueMetricName              = "metric_value"
	targetValueMetricName              = "target_value"
	targetAverageValueMetricName       = "target_average_value"
)

// MetricWriter defines the interface for writing custom metrics.
type MetricWriter interface {
	WriteMetricIgnoreFailure(ctx context.Context, metricName string, value float64, metricLabels map[string]string) error
}

// Scaler performs scaling logic for a single ScaledObject.
type Scaler struct {
	mu                 sync.Mutex
	scalingStabilizers map[string]*ScalingStabilizer
	cloudRunClient     CloudRunClient
	metricsService     MetricWriter
	staticConfig       StaticConfig
	projectID          string
	logger             logr.Logger
}

// NewScaler creates a new Scaler for a ScaledObject.
func NewScaler(
	cloudRunClient CloudRunClient,
	metricsService MetricWriter,
	config StaticConfig,
	projectID string,
	logger logr.Logger,
) *Scaler {
	return &Scaler{
		scalingStabilizers: make(map[string]*ScalingStabilizer),
		cloudRunClient:     cloudRunClient,
		metricsService:     metricsService,
		staticConfig:       config,
		projectID:          projectID,
		logger:             logger,
	}
}

// Scale scales the target Cloud Run service or worker pool based on the provided metrics.
func (s *Scaler) Scale(ctx context.Context, scaledObjectMetrics *pb.ScaledObjectMetrics) (pb.ScalingStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	var workloadName string
	if scaledObjectMetrics != nil && scaledObjectMetrics.ScaledObject != nil && scaledObjectMetrics.ScaledObject.ScaleTargetRef != nil {
		workloadName = scaledObjectMetrics.ScaledObject.ScaleTargetRef.Name
	}
	workloadInfo, err := ParseWorkloadInfo(workloadName)
	if err != nil {
		return pb.ScalingStatus_FAILED, err
	}

	if workloadInfo.WorkloadType == WorkloadTypeWorkerPool && s.staticConfig.UseMinInstances {
		return pb.ScalingStatus_FAILED, fmt.Errorf("USE_MIN_INSTANCES is not supported for worker pool workloads")
	}

	currentInstanceCount, err := GetInstanceCount(ctx, s.cloudRunClient, workloadInfo)
	if err != nil {
		return pb.ScalingStatus_FAILED, err
	}
	s.logger.Info(
		fmt.Sprintf("Current instances for %s: %d", workloadName, currentInstanceCount),
		"resource", workloadName,
		"currentInstanceCount", currentInstanceCount,
	)

	unboundedRecommendation := 0
	hasValidTrigger := false

	if len(scaledObjectMetrics.Metrics) == 0 {
		s.logger.Info(
			fmt.Sprintf("No metrics configured for %s, scaling down to 0", workloadName),
			"resource", workloadName,
		)
		if err := s.updateInstanceCount(ctx, 0, workloadInfo); err != nil {
			return pb.ScalingStatus_FAILED, err
		}
		return pb.ScalingStatus_SUCCEEDED, nil
	}

	for _, metric := range scaledObjectMetrics.Metrics {
		var recommendation int
		if hasTargetAverageValue(metric) && metric.GetTargetAverageValue() > 0 {
			recommendation = MakeTargetAverageValueRecommendation(metric.Value, metric.GetTargetAverageValue(), currentInstanceCount)
		} else if hasTargetValue(metric) && metric.GetTargetValue() > 0 {
			recommendation = MakeTargetValueRecommendation(metric.Value, metric.GetTargetValue(), currentInstanceCount)
		} else {
			s.logger.Info(
				fmt.Sprintf("Target value and target average value for %s are 0. At least one must be a non-zero value. Skipping scaling workload %s on the trigger.", metric.TriggerId, workloadName),
			)
			continue
		}
		s.logger.Info(
			fmt.Sprintf("Recommendation for %s based on scaling trigger %s: %d", workloadName, metric.TriggerId, recommendation),
		)

		if s.staticConfig.OutputScalerMetrics {
			s.emitTriggerMetrics(ctx, metric, workloadInfo)
		}

		unboundedRecommendation = max(unboundedRecommendation, recommendation)
		hasValidTrigger = true
	}

	if !hasValidTrigger {
		s.logger.Info(
			fmt.Sprintf("No valid triggers found for %s. Skipping scaling workload.", workloadName),
		)
		return pb.ScalingStatus_FAILED, nil
	}

	var scalerConfig *pb.Advanced_ScalerConfig
	if scaledObjectMetrics.ScaledObject != nil && scaledObjectMetrics.ScaledObject.Advanced != nil {
		scalerConfig = scaledObjectMetrics.ScaledObject.Advanced.ScalerConfig
	}

	scalingStabilizer, ok := s.scalingStabilizers[workloadInfo.Name]
	if !ok {
		scalingStabilizer = NewScalingStabilizer(currentInstanceCount, s.logger)
		s.scalingStabilizers[workloadInfo.Name] = scalingStabilizer
	}

	newInstanceCount, err := s.getBoundedRecommendation(
		currentInstanceCount,
		unboundedRecommendation,
		scalingStabilizer,
		scalerConfig,
		now,
		workloadName,
	)
	if err != nil {
		return pb.ScalingStatus_FAILED, err
	}

	s.logger.Info(
		fmt.Sprintf("Recommended instances for %s: %d", workloadName, newInstanceCount),
		"resource", workloadName,
		"recommendedInstanceCount", newInstanceCount,
	)
	if newInstanceCount != currentInstanceCount {
		if err := s.updateInstanceCount(ctx, newInstanceCount, workloadInfo); err != nil {
			return pb.ScalingStatus_FAILED, err
		}
		scalingStabilizer.MarkScaleEvent(scalerConfig.GetBehavior(), now, currentInstanceCount, newInstanceCount)
	} else {
		s.logger.Info(
			fmt.Sprintf("Recommended instances for %s is unchanged.", workloadName),
			"resource", workloadName,
		)
	}

	if s.staticConfig.OutputScalerMetrics {
		s.emitInstanceCountMetrics(ctx, unboundedRecommendation, newInstanceCount, workloadInfo)
	}

	return pb.ScalingStatus_SUCCEEDED, nil
}

func (s *Scaler) getBoundedRecommendation(
	currentInstanceCount int,
	unboundedRecommendation int,
	scalingStabilizer *ScalingStabilizer,
	scalerConfig *pb.Advanced_ScalerConfig,
	now time.Time,
	workloadName string,
) (int, error) {
	stabilizedInstanceCount := scalingStabilizer.GetStabilizedRecommendation(
		scalerConfig.GetBehavior(),
		now,
		currentInstanceCount,
		unboundedRecommendation,
		workloadName,
	)

	minInst := int(scalerConfig.GetMinInstances())
	maxInst := int(scalerConfig.GetMaxInstances())
	if minInst > maxInst {
		return 0, fmt.Errorf("minInstances (%d) cannot be greater than maxInstances (%d)", minInst, maxInst)
	}
	newInstanceCount := min(max(stabilizedInstanceCount, minInst), maxInst)

	if newInstanceCount != stabilizedInstanceCount {
		s.logger.Info(
			fmt.Sprintf("Recommendation for %s was clamped to range", workloadName),
			"resource", workloadName,
			"minReplicaCount", minInst,
			"maxReplicaCount", maxInst,
		)
	}

	return newInstanceCount, nil
}

func (s *Scaler) updateInstanceCount(ctx context.Context, newInstanceCount int, workloadInfo WorkloadInfo) error {
	if s.staticConfig.UseMinInstances {
		if workloadInfo.WorkloadType == WorkloadTypeService {
			if err := s.cloudRunClient.UpdateServiceMinInstances(ctx, workloadInfo.Name, newInstanceCount, workloadInfo.ProjectID, workloadInfo.Location); err != nil {
				s.logger.Error(err, fmt.Sprintf("Failed to update min instances for %s", workloadInfo.Name))
				return err
			}
			return nil
		}
		return fmt.Errorf("min instances are not supported for worker pools")
	}

	if workloadInfo.WorkloadType == WorkloadTypeService {
		if err := s.cloudRunClient.UpdateServiceManualInstances(ctx, workloadInfo.Name, newInstanceCount, workloadInfo.ProjectID, workloadInfo.Location); err != nil {
			return err
		}
		s.logger.Info(fmt.Sprintf("Sent update request for service %s to set instances to %d.", workloadInfo.Name, newInstanceCount))
		return nil
	}

	if err := s.cloudRunClient.UpdateWorkerPoolManualInstances(ctx, workloadInfo.Name, newInstanceCount, workloadInfo.ProjectID, workloadInfo.Location); err != nil {
		return err
	}
	s.logger.Info(fmt.Sprintf("Sent update request for workerpool %s to set instances to %d.", workloadInfo.Name, newInstanceCount))
	return nil
}

func (s *Scaler) emitInstanceCountMetrics(ctx context.Context, recommendedInstanceCount, newInstanceCount int, workloadInfo WorkloadInfo) {
	metricLabels := map[string]string{
		"project_id":       s.projectID,
		"consumer_service": workloadInfo.Name,
	}
	if err := s.metricsService.WriteMetricIgnoreFailure(ctx, recommendedInstanceCountMetricName, float64(recommendedInstanceCount), metricLabels); err != nil {
		s.logger.Error(err, "Failed to write metrics to Cloud Monitoring.")
		return
	}
	if err := s.metricsService.WriteMetricIgnoreFailure(ctx, requestedInstanceCountMetricName, float64(newInstanceCount), metricLabels); err != nil {
		s.logger.Error(err, "Failed to write metrics to Cloud Monitoring.")
	}
}

func (s *Scaler) emitTriggerMetrics(ctx context.Context, metric *pb.Metric, workloadInfo WorkloadInfo) {
	metricLabels := map[string]string{
		"project_id":       s.projectID,
		"consumer_service": workloadInfo.Name,
		"trigger_id":       metric.TriggerId,
	}
	if err := s.metricsService.WriteMetricIgnoreFailure(ctx, metric.TriggerType+"/"+metricValueMetricName, metric.Value, metricLabels); err != nil {
		s.logger.Error(err, "Failed to write metrics to Cloud Monitoring.")
		return
	}
	if hasTargetAverageValue(metric) {
		if err := s.metricsService.WriteMetricIgnoreFailure(ctx, metric.TriggerType+"/"+targetAverageValueMetricName, metric.GetTargetAverageValue(), metricLabels); err != nil {
			s.logger.Error(err, "Failed to write metrics to Cloud Monitoring.")
		}
	} else {
		if err := s.metricsService.WriteMetricIgnoreFailure(ctx, metric.TriggerType+"/"+targetValueMetricName, metric.GetTargetValue(), metricLabels); err != nil {
			s.logger.Error(err, "Failed to write metrics to Cloud Monitoring.")
		}
	}
}

// hasTargetAverageValue reports whether the metric's target is a per-instance target average value.
func hasTargetAverageValue(metric *pb.Metric) bool {
	_, ok := metric.GetTarget().(*pb.Metric_TargetAverageValue)
	return ok
}

// hasTargetValue reports whether the metric's target is a total target value.
func hasTargetValue(metric *pb.Metric) bool {
	_, ok := metric.GetTarget().(*pb.Metric_TargetValue)
	return ok
}
