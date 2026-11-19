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
	"errors"
	"reflect"
	"sort"
	"testing"

	"crema/metric-provider/internal/logging"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/kedacore/keda/v2/pkg/scalers/scalersconfig"
	v2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/apis/external_metrics"
)

type mockScaler struct {
	isActive bool
	metrics  []external_metrics.ExternalMetricValue
	err      error
}

func (m *mockScaler) GetMetricSpecForScaling(context.Context) []v2.MetricSpec {
	if len(m.metrics) > 0 {
		var specs []v2.MetricSpec
		for _, metric := range m.metrics {
			specs = append(specs, v2.MetricSpec{
				Type: v2.ExternalMetricSourceType,
				External: &v2.ExternalMetricSource{
					Metric: v2.MetricIdentifier{
						Name: metric.MetricName,
					},
					Target: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(10, resource.DecimalSI),
					},
				},
			})
		}
		return specs
	}

	return []v2.MetricSpec{
		{
			Type: v2.ExternalMetricSourceType,
			External: &v2.ExternalMetricSource{
				Metric: v2.MetricIdentifier{
					Name: "default-mock-metric",
				},
				Target: v2.MetricTarget{
					Type:         v2.AverageValueMetricType,
					AverageValue: resource.NewQuantity(10, resource.DecimalSI),
				},
			},
		},
	}
}

func (m *mockScaler) GetMetricsAndActivity(ctx context.Context, metricName string) ([]external_metrics.ExternalMetricValue, bool, error) {
	if m.err != nil {
		return nil, false, m.err
	}

	return m.metrics, m.isActive, nil
}

func (m *mockScaler) Close(context.Context) error {
	return nil
}

func TestStateProvider_GetScaledObjectState(t *testing.T) {
	log := logging.NewLogger()
	scaledObject := &kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "test-so", Namespace: "test-ns"},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				Name: "test-deployment",
			},
			Triggers: []kedav1alpha1.ScaleTriggers{
				{Type: "type1"},
				{Type: "type2"},
			},
		},
	}

	metric1 := external_metrics.ExternalMetricValue{
		MetricName: "metric1",
		Value:      *resource.NewQuantity(10, resource.DecimalSI),
	}
	metric2 := external_metrics.ExternalMetricValue{
		MetricName: "metric2",
		Value:      *resource.NewQuantity(20, resource.DecimalSI),
	}

	testCases := []struct {
		name                    string
		builders                []ScalerBuilder
		expectedIsActive        bool
		expectedMetricAndTarget []MetricAndTargetValue
		expectedError           bool
	}{
		{
			name: "one active scaler",
			builders: []ScalerBuilder{
				{
					Scaler:       &mockScaler{isActive: true, metrics: []external_metrics.ExternalMetricValue{metric1}},
					ScalerConfig: scalersconfig.ScalerConfig{TriggerName: "trigger1", TriggerIndex: 0},
				},
			},
			expectedIsActive: true,
			expectedMetricAndTarget: []MetricAndTargetValue{
				{
					TriggerName: "trigger1",
					TriggerType: "type1",
					MetricValue: float64(metric1.Value.Value()),
					TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(10, resource.DecimalSI),
					},
				},
			},
			expectedError: false,
		},
		{
			name: "one inactive scaler",
			builders: []ScalerBuilder{
				{
					Scaler:       &mockScaler{isActive: false, metrics: []external_metrics.ExternalMetricValue{metric1}},
					ScalerConfig: scalersconfig.ScalerConfig{TriggerName: "trigger1", TriggerIndex: 0},
				},
			},
			expectedIsActive: false,
			expectedMetricAndTarget: []MetricAndTargetValue{
				{
					TriggerName: "trigger1",
					TriggerType: "type1",
					MetricValue: float64(metric1.Value.Value()),
					TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(10, resource.DecimalSI),
					},
				},
			},
			expectedError: false,
		},
		{
			name: "one scaler with error",
			builders: []ScalerBuilder{
				{
					Scaler:       &mockScaler{err: errors.New("scaler error")},
					ScalerConfig: scalersconfig.ScalerConfig{TriggerIndex: 0},
				},
			},
			expectedIsActive:        false,
			expectedMetricAndTarget: []MetricAndTargetValue{},
			expectedError:           true,
		},
		{
			name: "multiple scalers, one active",
			builders: []ScalerBuilder{
				{
					Scaler:       &mockScaler{isActive: false, metrics: []external_metrics.ExternalMetricValue{metric1}},
					ScalerConfig: scalersconfig.ScalerConfig{TriggerName: "trigger1", TriggerIndex: 0},
				},
				{
					Scaler:       &mockScaler{isActive: true, metrics: []external_metrics.ExternalMetricValue{metric2}},
					ScalerConfig: scalersconfig.ScalerConfig{TriggerName: "trigger2", TriggerIndex: 1},
				},
			},
			expectedIsActive: true,
			expectedMetricAndTarget: []MetricAndTargetValue{
				{
					TriggerName: "trigger1",
					TriggerType: "type1",
					MetricValue: float64(metric1.Value.Value()),
					TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(10, resource.DecimalSI),
					},
				},
				{
					TriggerName: "trigger2",
					TriggerType: "type2",
					MetricValue: float64(metric2.Value.Value()),
					TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(10, resource.DecimalSI),
					},
				},
			},
			expectedError: false,
		},
		{
			name: "multiple scalers, one with error",
			builders: []ScalerBuilder{
				{
					Scaler:       &mockScaler{isActive: true, metrics: []external_metrics.ExternalMetricValue{metric1}},
					ScalerConfig: scalersconfig.ScalerConfig{TriggerName: "trigger1", TriggerIndex: 0},
				},
				{
					Scaler:       &mockScaler{err: errors.New("scaler error")},
					ScalerConfig: scalersconfig.ScalerConfig{TriggerIndex: 1},
				},
			},
			expectedIsActive: true,
			expectedMetricAndTarget: []MetricAndTargetValue{
				{
					TriggerName: "trigger1",
					TriggerType: "type1",
					MetricValue: float64(metric1.Value.Value()),
					TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(10, resource.DecimalSI),
					},
				},
			},
			expectedError: false,
		},
		{
			name: "multiple scalers, all inactive",
			builders: []ScalerBuilder{
				{
					Scaler:       &mockScaler{isActive: false, metrics: []external_metrics.ExternalMetricValue{metric1}},
					ScalerConfig: scalersconfig.ScalerConfig{TriggerName: "trigger1", TriggerIndex: 0},
				},
				{
					Scaler:       &mockScaler{isActive: false, metrics: []external_metrics.ExternalMetricValue{metric2}},
					ScalerConfig: scalersconfig.ScalerConfig{TriggerName: "trigger2", TriggerIndex: 1},
				},
			},
			expectedIsActive: false,
			expectedMetricAndTarget: []MetricAndTargetValue{
				{
					TriggerName: "trigger1",
					TriggerType: "type1",
					MetricValue: float64(metric1.Value.Value()),
					TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(10, resource.DecimalSI),
					},
				},
				{
					TriggerName: "trigger2",
					TriggerType: "type2",
					MetricValue: float64(metric2.Value.Value()),
					TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(10, resource.DecimalSI),
					},
				},
			},
			expectedError: false,
		},
		{
			name: "multiple scalers, multiple errors",
			builders: []ScalerBuilder{
				{
					Scaler:       &mockScaler{err: errors.New("scaler error 1")},
					ScalerConfig: scalersconfig.ScalerConfig{TriggerIndex: 0},
				},
				{
					Scaler:       &mockScaler{err: errors.New("scaler error 2")},
					ScalerConfig: scalersconfig.ScalerConfig{TriggerIndex: 1},
				},
			},
			expectedIsActive:        false,
			expectedMetricAndTarget: []MetricAndTargetValue{},
			expectedError:           true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewStateProvider(&log)
			state, err := handler.GetScaledObjectState(context.Background(), scaledObject, tc.builders)

			if state.IsActive != tc.expectedIsActive {
				t.Errorf("IsActive = %v, want %v", state.IsActive, tc.expectedIsActive)
			}

			gotMetrics := append([]MetricAndTargetValue(nil), state.MetricAndTargetValues...)
			wantMetrics := append([]MetricAndTargetValue(nil), tc.expectedMetricAndTarget...)
			sort.Slice(gotMetrics, func(i, j int) bool { return gotMetrics[i].TriggerName < gotMetrics[j].TriggerName })
			sort.Slice(wantMetrics, func(i, j int) bool { return wantMetrics[i].TriggerName < wantMetrics[j].TriggerName })
			if len(gotMetrics) != len(wantMetrics) || (len(gotMetrics) > 0 && !reflect.DeepEqual(gotMetrics, wantMetrics)) {
				t.Errorf("MetricAndTargetValues = %v, want %v", gotMetrics, wantMetrics)
			}

			if tc.expectedError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
