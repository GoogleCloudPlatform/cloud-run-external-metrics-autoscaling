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

package scaling

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/kedacore/keda/v2/pkg/scalers"
	"github.com/kedacore/keda/v2/pkg/scalers/scalersconfig"
	"github.com/kedacore/keda/v2/pkg/scaling/cache"
	v2 "k8s.io/api/autoscaling/v2"
)

// Provides metrics and activity for scaled objects
type StateProvider struct {
	logger *logr.Logger
}

// A scaled object's state according to the state of all its scalers
type ScaledObjectState struct {
	MetricAndTargetValues []MetricAndTargetValue
	IsActive              bool // True if any scalers are active
}

// A scalerState corresponds to a single metric source
type scalerState struct {
	// We assume that each scaler only produces one metric value.
	// TOOD: Handle multiple metric values from for Solace and External scalers.
	metricAndTargetValue MetricAndTargetValue
	isActive             bool
}

// Wrap state and error in a single struct for channel compatibility
type scalerChanResult struct {
	triggerIndex int
	scalerState  scalerState
	err          error
}

type MetricAndTargetValue struct {
	TriggerName string
	TriggerType string
	MetricValue float64
	TargetValue v2.MetricTarget
}

func NewStateProvider(logger *logr.Logger) *StateProvider {
	return &StateProvider{logger: logger}
}

// GetScaledObjectState returns the state for the scaledObject
// Returns an error if unable to retrieve metrics from any of the configured metric sources
func (sp *StateProvider) GetScaledObjectState(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, scalerBuilders []cache.ScalerBuilder) (ScaledObjectState, error) {
	logger := sp.logger.WithValues("scaleTargetRef", scaledObject.Spec.ScaleTargetRef.Name)

	resultsChan := make(chan scalerChanResult, len(scalerBuilders))
	var wg sync.WaitGroup

	for _, builder := range scalerBuilders {
		wg.Add(1)
		go func(builder cache.ScalerBuilder) {
			defer wg.Done()
			triggerIndex := builder.ScalerConfig.TriggerIndex
			triggerType := scaledObject.Spec.Triggers[triggerIndex].Type
			scalerLogger := logger.WithValues("triggerIndex", triggerIndex)
			resultsChan <- getScalerState(ctx, builder.Scaler, builder.ScalerConfig, triggerType, triggerIndex, scalerLogger)
		}(builder)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	isAnyScalerActive := false
	var metricAndTargetValues []MetricAndTargetValue

	for result := range resultsChan {
		if result.err != nil {
			logger.Error(result.err, "failed to read metrics", "triggerIndex", result.triggerIndex)
			continue
		} else {
			scalerState := result.scalerState
			metricAndTargetValues = append(metricAndTargetValues, scalerState.metricAndTargetValue)
			if scalerState.isActive {
				isAnyScalerActive = true
			}
		}
	}

	if len(metricAndTargetValues) == 0 {
		return ScaledObjectState{}, fmt.Errorf("failed to retrieve any metrics for scaling")
	}

	state := ScaledObjectState{
		MetricAndTargetValues: metricAndTargetValues,
		IsActive:              isAnyScalerActive,
	}

	return state, nil
}

// TODO: Update to handle https://keda.sh/docs/2.18/concepts/scaling-deployments/#scaling-modifiers
func getScalerState(ctx context.Context, scaler scalers.Scaler, config scalersconfig.ScalerConfig, triggerType string, triggerIndex int, logger logr.Logger) scalerChanResult {
	triggerName := strings.Replace(fmt.Sprintf("%T", scaler), "*scalers.", "", 1)
	if config.TriggerName != "" {
		triggerName = config.TriggerName
	}

	var scalerErr error

	metricSpecs := scaler.GetMetricSpecForScaling(ctx)
	if len(metricSpecs) == 0 {
		return scalerChanResult{}
	}

	if len(metricSpecs) > 1 {
		logger.Info("Scaler returned multiple metric specs but only one is expected.")
	}

	spec := metricSpecs[0]
	metricName := "unused" // KEDA only uses this to include in the return value to K8s
	metrics, isActive, err := scaler.GetMetricsAndActivity(ctx, metricName)
	metricAndTargetValue := MetricAndTargetValue{}
	if err != nil {
		scalerErr = fmt.Errorf("failed to get metrics and activity from scaler: %w", err)
	} else {
		if len(metrics) > 1 {
			logger.Info("Scaler returned multiple metrics but only one is expected.")
		}
		metric := metrics[0]
		metricValue := metric.Value.AsApproximateFloat64()
		metricAndTargetValue = MetricAndTargetValue{
			TriggerName: triggerName,
			TriggerType: triggerType,
			MetricValue: metricValue,
			TargetValue: spec.External.Target,
		}

		targetValue := 0.0
		switch spec.External.Target.Type {
		case v2.AverageValueMetricType:
			targetValue = spec.External.Target.AverageValue.AsApproximateFloat64()
		case v2.ValueMetricType:
			targetValue = spec.External.Target.Value.AsApproximateFloat64()
		default:
			scalerErr = fmt.Errorf("unsupported target type %s", spec.External.Target.Type)
		}

		if scalerErr == nil {
			logger.Info("Successfully fetched metric and target values", "metric", metricValue, spec.External.Target.Type, targetValue)
		}
	}

	return scalerChanResult{
		triggerIndex: triggerIndex,
		scalerState: scalerState{
			isActive:             isActive,
			metricAndTargetValue: metricAndTargetValue,
		},
		err: scalerErr,
	}
}
