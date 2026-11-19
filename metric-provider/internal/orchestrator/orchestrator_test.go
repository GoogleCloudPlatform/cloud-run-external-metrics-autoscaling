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

package orchestrator

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"crema/metric-provider/api"
	"crema/metric-provider/internal/logging"
	"crema/metric-provider/internal/scaling"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	v2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/metrics/pkg/apis/external_metrics"
)

type MockScalerServerClient struct {
	scaleFunc   func(ctx context.Context, req *scaling.ScaleRequest) (*scaling.ScaleResponse, error)
	scaleCalls  []*scaling.ScaleRequest
	scaleCalled bool
}

func (m *MockScalerServerClient) Scale(ctx context.Context, req *scaling.ScaleRequest) (*scaling.ScaleResponse, error) {
	m.scaleCalled = true
	m.scaleCalls = append(m.scaleCalls, req)
	if m.scaleFunc != nil {
		return m.scaleFunc(ctx, req)
	}
	return &scaling.ScaleResponse{}, nil
}

func (m *MockScalerServerClient) Close() error {
	return nil
}

type MockBuilderFactory struct {
	makeBuildersFunc func(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, triggerAuths []api.TriggerAuthentication, isGRPC bool) ([]scaling.ScalerBuilder, error)
}

func (m *MockBuilderFactory) MakeBuilders(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, triggerAuths []api.TriggerAuthentication, isGRPC bool) ([]scaling.ScalerBuilder, error) {
	if m.makeBuildersFunc != nil {
		return m.makeBuildersFunc(ctx, scaledObject, triggerAuths, isGRPC)
	}
	return nil, nil
}

type MockStateProvider struct {
	getScaledObjectStateFunc func(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, builders []scaling.ScalerBuilder) (scaling.ScaledObjectState, error)
}

func (m *MockStateProvider) GetScaledObjectState(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, builders []scaling.ScalerBuilder) (scaling.ScaledObjectState, error) {
	if m.getScaledObjectStateFunc != nil {
		return m.getScaledObjectStateFunc(ctx, scaledObject, builders)
	}
	return scaling.ScaledObjectState{}, nil
}

func TestOrchestrator_RefreshMetrics(t *testing.T) {
	logger := logging.NewLogger()

	t.Run("should send scale request when metrics are available", func(t *testing.T) {
		mockScalerClient := &MockScalerServerClient{}
		mockBuilderFactory := &MockBuilderFactory{}
		mockStateProvider := &MockStateProvider{}

		scaleTargetRefName := "my-workerpool"
		triggerName := "my-trigger-name"
		internalTriggerType := "internal-trigger-type"
		metricValue := 10.0

		cremaConfig := &api.CremaConfig{
			Spec: api.CremaConfigSpec{
				ScaledObjects: []api.CremaScaledObject{
					{
						Spec: kedav1alpha1.ScaledObjectSpec{
							ScaleTargetRef: &kedav1alpha1.ScaleTarget{
								Name: scaleTargetRefName,
							},
							Triggers: []kedav1alpha1.ScaleTriggers{
								{
									Type: "foo",
								},
							},
						},
					},
				},
			},
		}

		orchestrator := New(
			mockScalerClient,
			cremaConfig,
			mockBuilderFactory,
			mockStateProvider,
			&logger,
		)

		builders := []scaling.ScalerBuilder{}
		mockBuilderFactory.makeBuildersFunc = func(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, triggerAuths []api.TriggerAuthentication, isGRPC bool) ([]scaling.ScalerBuilder, error) {
			return builders, nil
		}

		scaledObjectState := scaling.ScaledObjectState{
			MetricAndTargetValues: []scaling.MetricAndTargetValue{
				{
					MetricValue: metricValue,
					TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(15, resource.DecimalSI),
					},
					TriggerName: triggerName,
					TriggerType: internalTriggerType,
				},
			},
		}
		mockStateProvider.getScaledObjectStateFunc = func(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, builders []scaling.ScalerBuilder) (scaling.ScaledObjectState, error) {
			return scaledObjectState, nil
		}

		targetAvg := 15.0
		expectedScaleRequest := &scaling.ScaleRequest{
			ScaledObjectMetrics: []*scaling.ScaledObjectMetrics{
				{
					ScaledObject: &scaling.ScaledObject{
						ScaleTargetRef: &scaling.ScaleTargetRef{
							Name: scaleTargetRefName,
						},
					},
					Metrics: []*scaling.Metric{
						{
							Value:              metricValue,
							TargetAverageValue: &targetAvg,
							TriggerID:          triggerName,
							TriggerType:        internalTriggerType,
						},
					},
				},
			},
		}

		_ = orchestrator.RefreshMetrics(context.Background())

		if len(mockScalerClient.scaleCalls) != 1 {
			t.Fatalf("expected Scale to be called once, got %d", len(mockScalerClient.scaleCalls))
		}
		if !reflect.DeepEqual(mockScalerClient.scaleCalls[0], expectedScaleRequest) {
			t.Errorf("unexpected ScaleRequest:\ngot:  %+v\nwant: %+v", mockScalerClient.scaleCalls[0], expectedScaleRequest)
		}
	})

	t.Run("should not send scale request when there are no metrics", func(t *testing.T) {
		mockScalerClient := &MockScalerServerClient{}
		mockBuilderFactory := &MockBuilderFactory{}
		mockStateProvider := &MockStateProvider{}

		cremaConfig := &api.CremaConfig{}

		orchestrator := New(mockScalerClient, cremaConfig, mockBuilderFactory, mockStateProvider, &logger)
		_ = orchestrator.RefreshMetrics(context.Background())

		if mockScalerClient.scaleCalled {
			t.Errorf("expected Scale not to be called")
		}
	})

	t.Run("should continue processing when MakeBuilders fails for one scaled object", func(t *testing.T) {
		mockScalerClient := &MockScalerServerClient{}
		mockBuilderFactory := &MockBuilderFactory{}
		mockStateProvider := &MockStateProvider{}

		successfulScaleTargetRefName := "successful-scaled-object"
		failingScaleTargetRefName := "failing-scaled-object"
		metricValue := 10.0
		triggerName := "test-trigger"
		triggerType := "test-trigger-type"

		cremaConfig := &api.CremaConfig{
			Spec: api.CremaConfigSpec{
				ScaledObjects: []api.CremaScaledObject{
					{
						Spec: kedav1alpha1.ScaledObjectSpec{
							ScaleTargetRef: &kedav1alpha1.ScaleTarget{
								Name: failingScaleTargetRefName,
							},
						},
					},
					{
						Spec: kedav1alpha1.ScaledObjectSpec{
							ScaleTargetRef: &kedav1alpha1.ScaleTarget{
								Name: successfulScaleTargetRefName,
							},
							Triggers: []kedav1alpha1.ScaleTriggers{
								{
									Type: "foo",
								},
							},
						},
					},
				},
			},
		}

		orchestrator := New(mockScalerClient, cremaConfig, mockBuilderFactory, mockStateProvider, &logger)

		mockBuilderFactory.makeBuildersFunc = func(ctx context.Context, so *kedav1alpha1.ScaledObject, triggerAuths []api.TriggerAuthentication, isGRPC bool) ([]scaling.ScalerBuilder, error) {
			if so.Name == failingScaleTargetRefName {
				return []scaling.ScalerBuilder{}, errors.New("MakeBuilders error")
			}
			return []scaling.ScalerBuilder{}, nil
		}

		scaledObjectState := scaling.ScaledObjectState{
			MetricAndTargetValues: []scaling.MetricAndTargetValue{
				{
					MetricValue: metricValue,
					TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(15, resource.DecimalSI),
					},
					TriggerName: triggerName,
					TriggerType: triggerType,
				},
			},
		}
		mockStateProvider.getScaledObjectStateFunc = func(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, builders []scaling.ScalerBuilder) (scaling.ScaledObjectState, error) {
			return scaledObjectState, nil
		}

		targetAvg := 15.0
		expectedScaleRequest := &scaling.ScaleRequest{
			ScaledObjectMetrics: []*scaling.ScaledObjectMetrics{
				{
					ScaledObject: &scaling.ScaledObject{
						ScaleTargetRef: &scaling.ScaleTargetRef{
							Name: successfulScaleTargetRefName,
						},
					},
					Metrics: []*scaling.Metric{
						{
							Value:              metricValue,
							TargetAverageValue: &targetAvg,
							TriggerID:          triggerName,
							TriggerType:        triggerType,
						},
					},
				},
			},
		}

		_ = orchestrator.RefreshMetrics(context.Background())

		if len(mockScalerClient.scaleCalls) != 1 {
			t.Fatalf("expected Scale to be called once, got %d", len(mockScalerClient.scaleCalls))
		}
		if !reflect.DeepEqual(mockScalerClient.scaleCalls[0], expectedScaleRequest) {
			t.Errorf("unexpected ScaleRequest:\ngot:  %+v\nwant: %+v", mockScalerClient.scaleCalls[0], expectedScaleRequest)
		}
	})

	t.Run("should continue processing when GetScaledObjectState fails for one scaled object", func(t *testing.T) {
		mockScalerClient := &MockScalerServerClient{}
		mockBuilderFactory := &MockBuilderFactory{}
		mockStateProvider := &MockStateProvider{}

		successfulScaleTargetRefName := "successful-scaled-object"
		failingScaleTargetRefName := "failing-scaled-object"
		metricValue := 10.0
		triggerName := "test-trigger"
		triggerType := "test-trigger-type"

		cremaConfig := &api.CremaConfig{
			Spec: api.CremaConfigSpec{
				ScaledObjects: []api.CremaScaledObject{
					{
						Spec: kedav1alpha1.ScaledObjectSpec{
							ScaleTargetRef: &kedav1alpha1.ScaleTarget{
								Name: failingScaleTargetRefName,
							},
						},
					},
					{
						Spec: kedav1alpha1.ScaledObjectSpec{
							ScaleTargetRef: &kedav1alpha1.ScaleTarget{
								Name: successfulScaleTargetRefName,
							},
							Triggers: []kedav1alpha1.ScaleTriggers{
								{
									Type: "foo",
								},
							},
						},
					},
				},
			},
		}

		orchestrator := New(mockScalerClient, cremaConfig, mockBuilderFactory, mockStateProvider, &logger)

		mockBuilderFactory.makeBuildersFunc = func(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, triggerAuths []api.TriggerAuthentication, isGRPC bool) ([]scaling.ScalerBuilder, error) {
			return []scaling.ScalerBuilder{}, nil
		}

		scaledObjectState := scaling.ScaledObjectState{
			MetricAndTargetValues: []scaling.MetricAndTargetValue{
				{
					MetricValue: float64(10),
					TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(15, resource.DecimalSI),
					},
					TriggerName: triggerName,
					TriggerType: triggerType,
				},
			},
		}
		mockStateProvider.getScaledObjectStateFunc = func(ctx context.Context, so *kedav1alpha1.ScaledObject, builders []scaling.ScalerBuilder) (scaling.ScaledObjectState, error) {
			if so.Name == failingScaleTargetRefName {
				return scaling.ScaledObjectState{}, errors.New("GetScaledObjectState error")
			}
			return scaledObjectState, nil
		}

		targetAvg := 15.0
		expectedScaleRequest := &scaling.ScaleRequest{
			ScaledObjectMetrics: []*scaling.ScaledObjectMetrics{
				{
					ScaledObject: &scaling.ScaledObject{
						ScaleTargetRef: &scaling.ScaleTargetRef{
							Name: successfulScaleTargetRefName,
						},
					},
					Metrics: []*scaling.Metric{
						{
							Value:              metricValue,
							TargetAverageValue: &targetAvg,
							TriggerID:          triggerName,
							TriggerType:        triggerType,
						},
					},
				},
			},
		}

		_ = orchestrator.RefreshMetrics(context.Background())

		if len(mockScalerClient.scaleCalls) != 1 {
			t.Fatalf("expected Scale to be called once, got %d", len(mockScalerClient.scaleCalls))
		}
		if !reflect.DeepEqual(mockScalerClient.scaleCalls[0], expectedScaleRequest) {
			t.Errorf("unexpected ScaleRequest:\ngot:  %+v\nwant: %+v", mockScalerClient.scaleCalls[0], expectedScaleRequest)
		}
	})

	t.Run("should log error when scale request fails", func(t *testing.T) {
		mockScalerClient := &MockScalerServerClient{
			scaleFunc: func(ctx context.Context, req *scaling.ScaleRequest) (*scaling.ScaleResponse, error) {
				return nil, errors.New("Scale error")
			},
		}
		mockBuilderFactory := &MockBuilderFactory{
			makeBuildersFunc: func(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, triggerAuths []api.TriggerAuthentication, isGRPC bool) ([]scaling.ScalerBuilder, error) {
				return []scaling.ScalerBuilder{}, nil
			},
		}
		mockStateProvider := &MockStateProvider{
			getScaledObjectStateFunc: func(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, builders []scaling.ScalerBuilder) (scaling.ScaledObjectState, error) {
				return scaling.ScaledObjectState{
					MetricAndTargetValues: []scaling.MetricAndTargetValue{
						{
							MetricValue: 10.0,
							TargetValue: v2.MetricTarget{
								Type:         v2.AverageValueMetricType,
								AverageValue: resource.NewQuantity(15, resource.DecimalSI),
							},
							TriggerName: "test-trigger",
							TriggerType: "test-trigger-type",
						},
					},
				}, nil
			},
		}

		cremaConfig := &api.CremaConfig{
			Spec: api.CremaConfigSpec{
				ScaledObjects: []api.CremaScaledObject{
					{
						Spec: kedav1alpha1.ScaledObjectSpec{
							ScaleTargetRef: &kedav1alpha1.ScaleTarget{
								Name: "test-scaled-object",
							},
							Triggers: []kedav1alpha1.ScaleTriggers{
								{
									Type: "test-trigger",
								},
							},
						},
					},
				},
			},
		}

		orchestrator := New(mockScalerClient, cremaConfig, mockBuilderFactory, mockStateProvider, &logger)

		err := orchestrator.RefreshMetrics(context.Background())
		if err == nil {
			t.Errorf("Expected an error from RefreshMetrics, got nil")
		}
		if !mockScalerClient.scaleCalled {
			t.Errorf("Expected Scale to be called")
		}
	})

	t.Run("should close all generated scalers upon cycle completion", func(t *testing.T) {
		mockScalerClient := &MockScalerServerClient{}
		mockBuilderFactory := &MockBuilderFactory{}
		mockStateProvider := &MockStateProvider{}

		cremaConfig := &api.CremaConfig{
			Spec: api.CremaConfigSpec{
				ScaledObjects: []api.CremaScaledObject{
					{
						Spec: kedav1alpha1.ScaledObjectSpec{
							ScaleTargetRef: &kedav1alpha1.ScaleTarget{
								Name: "my-workerpool",
							},
							Triggers: []kedav1alpha1.ScaleTriggers{
								{
									Type: "foo",
								},
							},
						},
					},
				},
			},
		}

		orchestrator := New(
			mockScalerClient,
			cremaConfig,
			mockBuilderFactory,
			mockStateProvider,
			&logger,
		)

		mockScaler := &MockLifecycleScaler{}
		builders := []scaling.ScalerBuilder{
			{
				Scaler: mockScaler,
			},
		}
		mockBuilderFactory.makeBuildersFunc = func(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, triggerAuths []api.TriggerAuthentication, isGRPC bool) ([]scaling.ScalerBuilder, error) {
			return builders, nil
		}

		scaledObjectState := scaling.ScaledObjectState{
			MetricAndTargetValues: []scaling.MetricAndTargetValue{
				{
					MetricValue: 10.0,
					TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(15, resource.DecimalSI),
					},
					TriggerName: "test-trigger",
					TriggerType: "test-trigger-type",
				},
			},
		}
		mockStateProvider.getScaledObjectStateFunc = func(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, builders []scaling.ScalerBuilder) (scaling.ScaledObjectState, error) {
			return scaledObjectState, nil
		}

		_ = orchestrator.RefreshMetrics(context.Background())

		if !mockScaler.closeCalled {
			t.Errorf("expected scaler.Close to be called")
		}
	})
}

type MockLifecycleScaler struct {
	closeCalled bool
}

func (m *MockLifecycleScaler) GetMetricsAndActivity(ctx context.Context, metricName string) ([]external_metrics.ExternalMetricValue, bool, error) {
	return nil, false, nil
}

func (m *MockLifecycleScaler) GetMetricSpecForScaling(ctx context.Context) []v2.MetricSpec {
	return nil
}

func (m *MockLifecycleScaler) Close(ctx context.Context) error {
	m.closeCalled = true
	return nil
}
