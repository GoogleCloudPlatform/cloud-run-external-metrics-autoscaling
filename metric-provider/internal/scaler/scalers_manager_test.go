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
	"sync"
	"testing"

	"crema/metric-provider/internal/scaling"

	"github.com/go-logr/logr"
)

func TestScalersManager(t *testing.T) {
	ctx := context.Background()
	logger := logr.Discard()
	targetName := "projects/test-project/locations/test-location/services/test-service"

	t.Run("scale_returnsSuccess", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 1, nil
			},
		}
		sm := NewScalersManager(crClient, &mockCloudMonitoringClient{}, &logger)
		som := &scaling.ScaledObjectMetrics{
			ScaledObject: &scaling.ScaledObject{
				ScaleTargetRef: &scaling.ScaleTargetRef{Name: targetName},
			},
		}

		resp, err := sm.Scale(ctx, &scaling.ScaleRequest{ScaledObjectMetrics: []*scaling.ScaledObjectMetrics{som}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(resp.Results))
		}
		if resp.Results[0].Status != scaling.ScalingStatusSucceeded || resp.Results[0].ScaleTargetName != targetName {
			t.Errorf("unexpected result: %+v", resp.Results[0])
		}
		if len(sm.scalers) != 1 {
			t.Errorf("expected 1 scaler in manager, got %d", len(sm.scalers))
		}
		if _, ok := sm.scalers[targetName]; !ok {
			t.Errorf("expected scalers map to contain key %q", targetName)
		}
	})

	t.Run("scale_concurrentBatchForSameScaledObject_allSucceed", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 1, nil
			},
		}
		sm := NewScalersManager(crClient, &mockCloudMonitoringClient{}, &logger)
		som := &scaling.ScaledObjectMetrics{
			ScaledObject: &scaling.ScaledObject{
				ScaleTargetRef: &scaling.ScaleTargetRef{Name: targetName},
			},
		}

		const concurrentTasks = 20
		metricsList := make([]*scaling.ScaledObjectMetrics, concurrentTasks)
		for i := 0; i < concurrentTasks; i++ {
			metricsList[i] = som
		}

		resp, err := sm.Scale(ctx, &scaling.ScaleRequest{ScaledObjectMetrics: metricsList})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Results) != concurrentTasks {
			t.Fatalf("expected %d results, got %d", concurrentTasks, len(resp.Results))
		}
		for _, res := range resp.Results {
			if res.Status != scaling.ScalingStatusSucceeded || res.ScaleTargetName != targetName {
				t.Errorf("unexpected result: %+v", res)
			}
		}
		if len(sm.scalers) != 1 {
			t.Errorf("expected 1 scaler in manager, got %d", len(sm.scalers))
		}
		if _, ok := sm.scalers[targetName]; !ok {
			t.Errorf("expected scalers map to contain key %q", targetName)
		}
	})

	t.Run("scale_simultaneousGoroutinesForSameScaledObject_allSucceed", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 1, nil
			},
		}
		sm := NewScalersManager(crClient, &mockCloudMonitoringClient{}, &logger)
		som := &scaling.ScaledObjectMetrics{
			ScaledObject: &scaling.ScaledObject{
				ScaleTargetRef: &scaling.ScaleTargetRef{Name: targetName},
			},
		}

		const threadCount = 10
		var readyWg sync.WaitGroup
		var doneWg sync.WaitGroup
		readyWg.Add(threadCount)
		doneWg.Add(threadCount)
		startCh := make(chan struct{})
		responses := make([]*scaling.ScaleResponse, threadCount)

		for i := 0; i < threadCount; i++ {
			go func(idx int) {
				defer doneWg.Done()
				readyWg.Done()
				<-startCh
				resp, err := sm.Scale(ctx, &scaling.ScaleRequest{ScaledObjectMetrics: []*scaling.ScaledObjectMetrics{som}})
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				responses[idx] = resp
			}(i)
		}

		readyWg.Wait()
		close(startCh)
		doneWg.Wait()

		for _, resp := range responses {
			if len(resp.Results) != 1 || resp.Results[0].Status != scaling.ScalingStatusSucceeded || resp.Results[0].ScaleTargetName != targetName {
				t.Errorf("unexpected response: %+v", resp)
			}
		}
		if len(sm.scalers) != 1 {
			t.Errorf("expected 1 scaler in manager, got %d", len(sm.scalers))
		}
		if _, ok := sm.scalers[targetName]; !ok {
			t.Errorf("expected scalers map to contain key %q", targetName)
		}
	})

	t.Run("scale_unsupportedPolicyType_recoversAndReturnsFailedStatus", func(t *testing.T) {
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				return 1, nil
			},
		}
		sm := NewScalersManager(crClient, &mockCloudMonitoringClient{}, &logger)
		badSom := &scaling.ScaledObjectMetrics{
			ScaledObject: &scaling.ScaledObject{
				ScaleTargetRef: &scaling.ScaleTargetRef{Name: targetName},
				Advanced: &scaling.Advanced{
					ScalerConfig: &scaling.ScalerConfig{
						MinInstances: 0,
						MaxInstances: 10,
						Behavior: &scaling.Behavior{
							ScaleUp: &scaling.ScalingRules{
								Policies: []*scaling.Policy{
									{Type: scaling.PolicyTypeUnspecified, Value: 10, PeriodSeconds: 60},
								},
							},
						},
					},
				},
			},
			Metrics: []*scaling.Metric{{Value: 500.0, TargetValue: floatPtr(100.0)}},
		}
		goodTarget := "projects/test-project/locations/test-location/services/good-service"
		goodSom := &scaling.ScaledObjectMetrics{
			ScaledObject: &scaling.ScaledObject{
				ScaleTargetRef: &scaling.ScaleTargetRef{Name: goodTarget},
			},
		}

		resp, err := sm.Scale(ctx, &scaling.ScaleRequest{ScaledObjectMetrics: []*scaling.ScaledObjectMetrics{badSom, goodSom}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(resp.Results))
		}
		if resp.Results[0].Status != scaling.ScalingStatusFailed || resp.Results[0].ScaleTargetName != targetName {
			t.Errorf("expected first workload to fail gracefully, got %+v", resp.Results[0])
		}
		if resp.Results[1].Status != scaling.ScalingStatusSucceeded || resp.Results[1].ScaleTargetName != goodTarget {
			t.Errorf("expected second workload to succeed, got %+v", resp.Results[1])
		}
	})

	t.Run("scale_callerContextCancelled_stillCompletesWithTimeoutBound", func(t *testing.T) {
		var observedHasDeadline bool
		var observedErr error
		crClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(callCtx context.Context, name, proj, loc string) (int, error) {
				_, observedHasDeadline = callCtx.Deadline()
				observedErr = callCtx.Err()
				return 1, nil
			},
		}
		sm := NewScalersManager(crClient, &mockCloudMonitoringClient{}, &logger)
		som := &scaling.ScaledObjectMetrics{
			ScaledObject: &scaling.ScaledObject{
				ScaleTargetRef: &scaling.ScaleTargetRef{Name: targetName},
			},
		}

		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		resp, err := sm.Scale(cancelledCtx, &scaling.ScaleRequest{ScaledObjectMetrics: []*scaling.ScaledObjectMetrics{som}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Results) != 1 || resp.Results[0].Status != scaling.ScalingStatusSucceeded {
			t.Errorf("expected scaling to succeed with detached context, got %+v", resp.Results)
		}
		if !observedHasDeadline {
			t.Errorf("expected detached scaleCtx to carry a timeout deadline")
		}
		if observedErr != nil {
			t.Errorf("expected detached scaleCtx not to be cancelled when caller ctx is cancelled, got %v", observedErr)
		}
	})
}
