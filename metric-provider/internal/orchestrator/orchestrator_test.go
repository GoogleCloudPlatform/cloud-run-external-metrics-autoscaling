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
	"testing"

	"crema/metric-provider/api"
	"crema/metric-provider/internal/logging"
	"crema/metric-provider/internal/scaling"
	pb "crema/metric-provider/proto"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/kedacore/keda/v2/pkg/scaling/cache"
	"github.com/stretchr/testify/mock"
	v2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/resource"
)

type MockScalerServerClient struct {
	mock.Mock
}

func (m *MockScalerServerClient) Scale(ctx context.Context, req *pb.ScaleRequest) (*pb.ScaleResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.ScaleResponse), args.Error(1)
}

func (m *MockScalerServerClient) Close() error {
	return nil
}

type MockBuilderFactory struct {
	mock.Mock
}

func (m *MockBuilderFactory) MakeBuilders(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, triggerAuths []api.TriggerAuthentication, isGRPC bool) ([]cache.ScalerBuilder, error) {
	args := m.Called(ctx, scaledObject, triggerAuths, isGRPC)
	return args.Get(0).([]cache.ScalerBuilder), args.Error(1)
}

type MockStateProvider struct {
	mock.Mock
}

func (m *MockStateProvider) GetScaledObjectState(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, builders []cache.ScalerBuilder) (scaling.ScaledObjectState, error) {
	args := m.Called(ctx, scaledObject, builders)
	if args.Get(0) == nil {
		return scaling.ScaledObjectState{}, args.Error(1)
	}
	return args.Get(0).(scaling.ScaledObjectState), args.Error(1)
}

func TestOrchestrator_RefreshMetrics(t *testing.T) {
	logger := logging.NewLogger()

	t.Run("should send scale request when metrics are available", func(t *testing.T) {

		mockScalerClient := new(MockScalerServerClient)
		mockBuilderFactory := new(MockBuilderFactory)
		mockStateProvider := new(MockStateProvider)

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

		builders := []cache.ScalerBuilder{}
		mockBuilderFactory.On("MakeBuilders", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(builders, nil)

		scaledObjectState := &scaling.ScaledObjectState{
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
		mockStateProvider.On("GetScaledObjectState", mock.Anything, mock.Anything, mock.Anything).Return(*scaledObjectState, nil)

		expectedScaleRequest := &pb.ScaleRequest{
			ScaledObjectMetrics: []*pb.ScaledObjectMetrics{
				{
					ScaledObject: &pb.ScaledObject{
						ScaleTargetRef: &pb.ScaleTargetRef{
							Name: scaleTargetRefName,
						},
					},
					Metrics: []*pb.Metric{
						{
							Value: metricValue,
							Target: &pb.Metric_TargetAverageValue{
								TargetAverageValue: 15.0,
							},
							TriggerId:   triggerName,
							TriggerType: internalTriggerType,
						},
					},
				},
			},
		}
		mockScalerClient.On("Scale", mock.Anything, expectedScaleRequest).Return(&pb.ScaleResponse{}, nil)

		orchestrator.RefreshMetrics(context.Background())

		mockScalerClient.AssertExpectations(t)
	})

	t.Run("should not send scale request when there are no metrics", func(t *testing.T) {
		mockScalerClient := new(MockScalerServerClient)
		mockBuilderFactory := new(MockBuilderFactory)
		mockStateProvider := new(MockStateProvider)

		cremaConfig := &api.CremaConfig{}

		orchestrator := New(mockScalerClient, cremaConfig, mockBuilderFactory, mockStateProvider, &logger)
		orchestrator.RefreshMetrics(context.Background())

		mockScalerClient.AssertNotCalled(t, "Scale", mock.Anything, mock.Anything)
	})

	t.Run("should continue processing when MakeBuilders fails for one scaled object", func(t *testing.T) {
		mockScalerClient := new(MockScalerServerClient)
		mockBuilderFactory := new(MockBuilderFactory)
		mockStateProvider := new(MockStateProvider)

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

		mockBuilderFactory.On("MakeBuilders", mock.Anything, mock.MatchedBy(func(so *kedav1alpha1.ScaledObject) bool {
			return so.Name == failingScaleTargetRefName
		}), mock.Anything, mock.Anything).Return([]cache.ScalerBuilder{}, errors.New("MakeBuilders error"))
		builders := []cache.ScalerBuilder{}
		mockBuilderFactory.On("MakeBuilders", mock.Anything, mock.MatchedBy(func(so *kedav1alpha1.ScaledObject) bool {
			return so.Name == successfulScaleTargetRefName
		}), mock.Anything, mock.Anything).Return(builders, nil)

		scaledObjectState := &scaling.ScaledObjectState{
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
		mockStateProvider.On("GetScaledObjectState", mock.Anything, mock.Anything, mock.Anything).Return(*scaledObjectState, nil)
		expectedScaleRequest := &pb.ScaleRequest{
			ScaledObjectMetrics: []*pb.ScaledObjectMetrics{
				{
					ScaledObject: &pb.ScaledObject{
						ScaleTargetRef: &pb.ScaleTargetRef{
							Name: successfulScaleTargetRefName,
						},
					},
					Metrics: []*pb.Metric{
						{
							Value: metricValue,
							Target: &pb.Metric_TargetAverageValue{
								TargetAverageValue: 15.0,
							},
							TriggerId:   triggerName,
							TriggerType: triggerType,
						},
					},
				},
			},
		}
		mockScalerClient.On("Scale", mock.Anything, expectedScaleRequest).Return(&pb.ScaleResponse{}, nil)

		orchestrator.RefreshMetrics(context.Background())

		mockScalerClient.AssertExpectations(t)
	})

	t.Run("should continue processing when GetScaledObjectState fails for one scaled object", func(t *testing.T) {
		mockScalerClient := new(MockScalerServerClient)
		mockBuilderFactory := new(MockBuilderFactory)
		mockStateProvider := new(MockStateProvider)

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

		builders := []cache.ScalerBuilder{}
		mockBuilderFactory.On("MakeBuilders", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(builders, nil)

		mockStateProvider.On("GetScaledObjectState", mock.Anything, mock.MatchedBy(func(so *kedav1alpha1.ScaledObject) bool {
			return so.Name == failingScaleTargetRefName
		}), mock.Anything).Return(nil, errors.New("GetScaledObjectState error"))

		scaledObjectState := &scaling.ScaledObjectState{
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
		mockStateProvider.On("GetScaledObjectState", mock.Anything, mock.MatchedBy(func(so *kedav1alpha1.ScaledObject) bool {
			return so.Name == successfulScaleTargetRefName
		}), mock.Anything).Return(*scaledObjectState, nil)

		expectedScaleRequest := &pb.ScaleRequest{
			ScaledObjectMetrics: []*pb.ScaledObjectMetrics{
				{
					ScaledObject: &pb.ScaledObject{
						ScaleTargetRef: &pb.ScaleTargetRef{
							Name: successfulScaleTargetRefName,
						},
					},
					Metrics: []*pb.Metric{
						{
							Value: metricValue,
							Target: &pb.Metric_TargetAverageValue{
								TargetAverageValue: 15.0,
							},
							TriggerId:   triggerName,
							TriggerType: triggerType,
						},
					},
				},
			},
		}
		mockScalerClient.On("Scale", mock.Anything, expectedScaleRequest).Return(&pb.ScaleResponse{}, nil)

		orchestrator.RefreshMetrics(context.Background())

		mockScalerClient.AssertExpectations(t)
	})

	t.Run("should log error when scale request fails", func(t *testing.T) {
		mockScalerClient := new(MockScalerServerClient)
		mockBuilderFactory := new(MockBuilderFactory)
		mockStateProvider := new(MockStateProvider)

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
		builders := []cache.ScalerBuilder{}
		mockBuilderFactory.On("MakeBuilders", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(builders, nil)

		scaledObjectState := &scaling.ScaledObjectState{
			MetricAndTargetValues: []scaling.MetricAndTargetValue{
				{
					MetricValue: 10.0, TargetValue: v2.MetricTarget{
						Type:         v2.AverageValueMetricType,
						AverageValue: resource.NewQuantity(15, resource.DecimalSI),
					},
					TriggerName: "test-trigger",
					TriggerType: "test-trigger-type",
				},
			},
		}
		mockStateProvider.On("GetScaledObjectState", mock.Anything, mock.Anything, mock.Anything).Return(*scaledObjectState, nil)
		mockScalerClient.On("Scale", mock.Anything, mock.Anything).Return(nil, errors.New("Scale error"))

		err := orchestrator.RefreshMetrics(context.Background())
		if err == nil {
			t.Errorf("Expected an error from RefreshMetrics, got nil")
		}

		mockScalerClient.AssertExpectations(t)
	})
}
