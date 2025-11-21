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
	IsActive              bool
}

// Each scaler corresponds to a metric source
type scalerState struct {
	// We assume that each scaler only produces one metric value.
	// TOOD: Handle multiple metric values from for Solace and External scalers.
	metricAndTargetValue MetricAndTargetValue
	isActive             bool
	err                  error
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

func (sp *StateProvider) GetScaledObjectState(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, scalerBuilders []cache.ScalerBuilder) (ScaledObjectState, error) {
	logger := sp.logger.WithValues("scaledObject.Name", scaledObject.Name)

	resultsChan := make(chan scalerState, len(scalerBuilders))
	var wg sync.WaitGroup

	for i, builder := range scalerBuilders {
		wg.Add(1)
		go func(builder cache.ScalerBuilder, triggerType string) {
			defer wg.Done()
			scalerLogger := logger.WithValues("scaler", fmt.Sprintf("%T", builder.Scaler))
			resultsChan <- getScalerState(ctx, builder.Scaler, builder.ScalerConfig, triggerType, scalerLogger)
		}(builder, scaledObject.Spec.Triggers[i].Type)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	isAnyScalerActive := false
	var allMetricAndTargetValues []MetricAndTargetValue
	var scalerErrors []string

	for result := range resultsChan {
		if result.err != nil {
			scalerErrors = append(scalerErrors, result.err.Error())
		} else {
			allMetricAndTargetValues = append(allMetricAndTargetValues, result.metricAndTargetValue)
		}

		if result.isActive {
			isAnyScalerActive = true
		}
	}

	var consolidatedError error
	if len(scalerErrors) > 0 {
		consolidatedError = fmt.Errorf("encountered %d error(s) while getting metrics: %s", len(scalerErrors), strings.Join(scalerErrors, "; "))
	}

	state := ScaledObjectState{
		MetricAndTargetValues: allMetricAndTargetValues,
		IsActive:              isAnyScalerActive,
	}

	return state, consolidatedError
}

// TODO: Update to handle https://keda.sh/docs/2.18/concepts/scaling-deployments/#scaling-modifiers
func getScalerState(ctx context.Context, scaler scalers.Scaler, config scalersconfig.ScalerConfig, triggerType string, logger logr.Logger) scalerState {
	triggerName := strings.Replace(fmt.Sprintf("%T", scaler), "*scalers.", "", 1)
	if config.TriggerName != "" {
		triggerName = config.TriggerName
	}

	var scalerErr error

	metricSpecs := scaler.GetMetricSpecForScaling(ctx)
	if len(metricSpecs) == 0 {
		return scalerState{isActive: false}
	}

	if len(metricSpecs) > 1 {
		logger.Info("Scaler returned multiple metric specs but only one is expected.")
	}

	spec := metricSpecs[0]
	metricName := "unused"
	metrics, isActive, err := scaler.GetMetricsAndActivity(ctx, metricName)
	metricAndTargetValue := MetricAndTargetValue{}
	if err != nil {
		logger.Error(err, "error reading metrics")
		scalerErr = err
	} else {
		if len(metrics) > 1 {
			logger.Info("Scaler returned multiple metrics but only one is expected.")
		}
		metric := metrics[0]
		metricAndTargetValue = MetricAndTargetValue{
			TriggerName: triggerName,
			TriggerType: triggerType,
			MetricValue: float64(metric.Value.Value()),
			TargetValue: spec.External.Target,
		}

	}

	return scalerState{
		isActive:             isActive,
		metricAndTargetValue: metricAndTargetValue,
		err:                  scalerErr,
	}
}
