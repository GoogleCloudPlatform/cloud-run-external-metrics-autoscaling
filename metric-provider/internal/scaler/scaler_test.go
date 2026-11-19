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
	"errors"
	"testing"

	pb "crema/metric-provider/proto"

	"github.com/go-logr/logr"
)

type mockMetricWriter struct {
	calls int
}

func (m *mockMetricWriter) WriteMetricIgnoreFailure(ctx context.Context, metricName string, value float64, metricLabels map[string]string) error {
	m.calls++
	return nil
}

func defaultAdvanced() *pb.Advanced {
	return &pb.Advanced{
		ScalerConfig: &pb.Advanced_ScalerConfig{
			MinInstances: 0,
			MaxInstances: 100,
			Behavior: &pb.Advanced_ScalerConfig_Behavior{
				ScaleUp:   &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 0},
				ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 0},
			},
		},
	}
}

func TestScaler(t *testing.T) {
	ctx := context.Background()
	logger := logr.Discard()

	const (
		serviceName            = "test-service"
		workerpoolName         = "test-workerpool"
		projectID              = "test-project"
		location               = "test-location"
		serviceWorkloadName    = "projects/test-project/locations/test-location/services/test-service"
		workerpoolWorkloadName = "projects/test-project/locations/test-location/workerpools/test-workerpool"
	)

	manualScalingConfig := StaticConfig{UseMinInstances: false, OutputScalerMetrics: false}
	autoScalingConfig := StaticConfig{UseMinInstances: true, OutputScalerMetrics: false}

	t.Run("scale_minInstancesForWorkerPool_returnsError", func(t *testing.T) {
		s := NewScaler(&mockCloudRunClient{}, &mockMetricWriter{}, autoScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: workerpoolWorkloadName},
			},
		}
		if _, err := s.Scale(ctx, som); err == nil {
			t.Errorf("expected error for USE_MIN_INSTANCES with worker pool")
		}
	})

	t.Run("scale_getServiceInstanceCountError_doesNotUpdateInstances", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 0, errors.New("test exception")
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, manualScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
			},
		}
		if _, err := s.Scale(ctx, som); err == nil {
			t.Errorf("expected error when GetServiceInstanceCount fails")
		}
		if len(crClient.updateServiceManualInstancesCalls) != 0 || len(crClient.updateServiceMinInstancesCalls) != 0 {
			t.Errorf("expected no update calls")
		}
	})

	t.Run("scale_getWorkerPoolInstanceCountError_doesNotUpdateInstances", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getWorkerPoolInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 0, errors.New("test exception")
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, manualScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: workerpoolWorkloadName},
			},
		}
		if _, err := s.Scale(ctx, som); err == nil {
			t.Errorf("expected error when GetWorkerPoolInstanceCount fails")
		}
		if len(crClient.updateWorkerPoolManualInstancesCalls) != 0 {
			t.Errorf("expected no update calls")
		}
	})

	t.Run("scale_noMetrics_scalesDownToZero", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 5, nil
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, manualScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
				Advanced:       defaultAdvanced(),
			},
		}
		status, err := s.Scale(ctx, som)
		if err != nil || status != pb.ScalingStatus_SUCCEEDED {
			t.Fatalf("unexpected status=%v, err=%v", status, err)
		}
		if len(crClient.updateServiceManualInstancesCalls) != 1 || crClient.updateServiceManualInstancesCalls[0].instances != 0 {
			t.Errorf("expected UpdateServiceManualInstances to 0, got %+v", crClient.updateServiceManualInstancesCalls)
		}
	})

	t.Run("scale_toIncreaseInstances_updatesServiceManualInstanceCount", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 5, nil
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, manualScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
				Advanced:       defaultAdvanced(),
			},
			Metrics: []*pb.Metric{{Value: 2000.0, Target: &pb.Metric_TargetValue{TargetValue: 1000.0}}},
		}
		status, err := s.Scale(ctx, som)
		if err != nil || status != pb.ScalingStatus_SUCCEEDED {
			t.Fatalf("unexpected status=%v, err=%v", status, err)
		}
		if len(crClient.updateServiceManualInstancesCalls) != 1 || crClient.updateServiceManualInstancesCalls[0].instances != 10 {
			t.Errorf("expected UpdateServiceManualInstances to 10, got %+v", crClient.updateServiceManualInstancesCalls)
		}
	})

	t.Run("scale_toIncreaseInstances_updatesWorkerPoolManualInstanceCount", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getWorkerPoolInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 5, nil
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, manualScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: workerpoolWorkloadName},
				Advanced:       defaultAdvanced(),
			},
			Metrics: []*pb.Metric{{Value: 2000.0, Target: &pb.Metric_TargetValue{TargetValue: 1000.0}}},
		}
		status, err := s.Scale(ctx, som)
		if err != nil || status != pb.ScalingStatus_SUCCEEDED {
			t.Fatalf("unexpected status=%v, err=%v", status, err)
		}
		if len(crClient.updateWorkerPoolManualInstancesCalls) != 1 || crClient.updateWorkerPoolManualInstancesCalls[0].instances != 10 {
			t.Errorf("expected UpdateWorkerPoolManualInstances to 10, got %+v", crClient.updateWorkerPoolManualInstancesCalls)
		}
	})

	t.Run("scale_toIncreaseInstances_updatesServiceMinInstanceCount", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 5, nil
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, autoScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
				Advanced:       defaultAdvanced(),
			},
			Metrics: []*pb.Metric{{Value: 2000.0, Target: &pb.Metric_TargetValue{TargetValue: 1000.0}}},
		}
		status, err := s.Scale(ctx, som)
		if err != nil || status != pb.ScalingStatus_SUCCEEDED {
			t.Fatalf("unexpected status=%v, err=%v", status, err)
		}
		if len(crClient.updateServiceMinInstancesCalls) != 1 || crClient.updateServiceMinInstancesCalls[0].instances != 10 {
			t.Errorf("expected UpdateServiceMinInstances to 10, got %+v", crClient.updateServiceMinInstancesCalls)
		}
	})

	t.Run("scale_withOutputMetricsEnabled_outputsMetrics", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 5, nil
			},
		}
		mw := &mockMetricWriter{}
		s := NewScaler(crClient, mw, StaticConfig{UseMinInstances: false, OutputScalerMetrics: true}, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
				Advanced:       defaultAdvanced(),
			},
			Metrics: []*pb.Metric{{Value: 2000.0, Target: &pb.Metric_TargetValue{TargetValue: 1000.0}}},
		}
		_, err := s.Scale(ctx, som)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mw.calls != 4 {
			t.Errorf("got %d metric calls, want 4", mw.calls)
		}
	})

	t.Run("scale_usesTargetAverageValue_whenTargetValueIsZero", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 1, nil
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, autoScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
				Advanced:       defaultAdvanced(),
			},
			Metrics: []*pb.Metric{{Value: 200.0, Target: &pb.Metric_TargetAverageValue{TargetAverageValue: 100.0}}},
		}
		status, err := s.Scale(ctx, som)
		if err != nil || status != pb.ScalingStatus_SUCCEEDED {
			t.Fatalf("unexpected status=%v, err=%v", status, err)
		}
		if len(crClient.updateServiceMinInstancesCalls) != 1 || crClient.updateServiceMinInstancesCalls[0].instances != 2 {
			t.Errorf("expected UpdateServiceMinInstances to 2, got %+v", crClient.updateServiceMinInstancesCalls)
		}
	})

	t.Run("scale_usesTargetAverageValue_whenBothAreSet", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 1, nil
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, autoScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
				Advanced:       defaultAdvanced(),
			},
			Metrics: []*pb.Metric{{Value: 200.0, Target: &pb.Metric_TargetAverageValue{TargetAverageValue: 100.0}}},
		}
		status, err := s.Scale(ctx, som)
		if err != nil || status != pb.ScalingStatus_SUCCEEDED {
			t.Fatalf("unexpected status=%v, err=%v", status, err)
		}
		if len(crClient.updateServiceMinInstancesCalls) != 1 || crClient.updateServiceMinInstancesCalls[0].instances != 2 {
			t.Errorf("expected UpdateServiceMinInstances to 2, got %+v", crClient.updateServiceMinInstancesCalls)
		}
	})

	t.Run("scale_skipsMetric_whenBothTargetsAreZero", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 1, nil
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, autoScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
			},
			Metrics: []*pb.Metric{{Value: 200.0, Target: &pb.Metric_TargetAverageValue{TargetAverageValue: 0}}},
		}
		status, err := s.Scale(ctx, som)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != pb.ScalingStatus_FAILED {
			t.Errorf("got status %v, want %v", status, pb.ScalingStatus_FAILED)
		}
		if len(crClient.updateServiceMinInstancesCalls) != 0 {
			t.Errorf("expected no update calls")
		}
	})

	t.Run("scale_newInstanceCountMatchesCurrent_doesNotUpdateInstances", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 5, nil
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, manualScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
				Advanced:       defaultAdvanced(),
			},
			Metrics: []*pb.Metric{{Value: 1000.0, Target: &pb.Metric_TargetValue{TargetValue: 1000.0}}},
		}
		status, err := s.Scale(ctx, som)
		if err != nil || status != pb.ScalingStatus_SUCCEEDED {
			t.Fatalf("unexpected status=%v, err=%v", status, err)
		}
		if len(crClient.updateServiceManualInstancesCalls) != 0 || len(crClient.updateServiceMinInstancesCalls) != 0 || len(crClient.updateWorkerPoolManualInstancesCalls) != 0 {
			t.Errorf("expected no update calls")
		}
	})

	t.Run("scale_appliesMaxInstances", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 1, nil
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, autoScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
				Advanced: &pb.Advanced{
					ScalerConfig: &pb.Advanced_ScalerConfig{
						MinInstances: 5,
						MaxInstances: 10,
						Behavior: &pb.Advanced_ScalerConfig_Behavior{
							ScaleUp:   &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 0},
							ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 0},
						},
					},
				},
			},
			Metrics: []*pb.Metric{{Value: 3000.0, Target: &pb.Metric_TargetValue{TargetValue: 200.0}}},
		}
		status, err := s.Scale(ctx, som)
		if err != nil || status != pb.ScalingStatus_SUCCEEDED {
			t.Fatalf("unexpected status=%v, err=%v", status, err)
		}
		if len(crClient.updateServiceMinInstancesCalls) != 1 || crClient.updateServiceMinInstancesCalls[0].instances != 10 {
			t.Errorf("expected UpdateServiceMinInstances to 10, got %+v", crClient.updateServiceMinInstancesCalls)
		}
	})

	t.Run("scale_withServiceApiException_returnsError", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 5, nil
			},
			updateServiceManualInstancesFunc: func(ctx context.Context, serviceName string, instances int, projectID, region string) error {
				return errors.New("api exception")
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, manualScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
				Advanced:       defaultAdvanced(),
			},
			Metrics: []*pb.Metric{{Value: 2000.0, Target: &pb.Metric_TargetValue{TargetValue: 1000.0}}},
		}
		if _, err := s.Scale(ctx, som); err == nil {
			t.Errorf("expected error when UpdateServiceManualInstances fails")
		}
	})

	t.Run("scale_minInstancesGreaterThanMaxInstances_returnsError", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 5, nil
			},
		}
		s := NewScaler(crClient, &mockMetricWriter{}, manualScalingConfig, projectID, logger)
		som := &pb.ScaledObjectMetrics{
			ScaledObject: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{Name: serviceWorkloadName},
				Advanced: &pb.Advanced{
					ScalerConfig: &pb.Advanced_ScalerConfig{
						MinInstances: 20,
						MaxInstances: 10,
					},
				},
			},
			Metrics: []*pb.Metric{{Value: 2000.0, Target: &pb.Metric_TargetValue{TargetValue: 1000.0}}},
		}
		status, err := s.Scale(ctx, som)
		if err == nil || status != pb.ScalingStatus_FAILED {
			t.Errorf("expected error and ScalingStatusFailed when minInstances > maxInstances, got status=%v, err=%v", status, err)
		}
	})
}
