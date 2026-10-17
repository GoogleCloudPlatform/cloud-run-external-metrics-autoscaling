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

package clients

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/api/run/v2"
)

const (
	testProjectID          = "projectId"
	testRegion             = "global"
	testWorkerPoolName     = "workerpool-name"
	testServiceName        = "service-name"
	testWorkerPoolResource = "projects/projectId/locations/global/workerPools/workerpool-name"
	testServiceResource    = "projects/projectId/locations/global/services/service-name"
)

type updateServiceCall struct {
	name       string
	service    *run.GoogleCloudRunV2Service
	updateMask string
}

type mockServicesAPI struct {
	getServiceFunc    func(ctx context.Context, name string) (*run.GoogleCloudRunV2Service, error)
	updateServiceFunc func(ctx context.Context, name string, service *run.GoogleCloudRunV2Service, updateMask string) error
	getCalls          []string
	updateCalls       []updateServiceCall
}

func (m *mockServicesAPI) getService(ctx context.Context, name string) (*run.GoogleCloudRunV2Service, error) {
	m.getCalls = append(m.getCalls, name)
	if m.getServiceFunc != nil {
		return m.getServiceFunc(ctx, name)
	}
	return &run.GoogleCloudRunV2Service{}, nil
}

func (m *mockServicesAPI) updateService(ctx context.Context, name string, service *run.GoogleCloudRunV2Service, updateMask string) error {
	m.updateCalls = append(m.updateCalls, updateServiceCall{name: name, service: service, updateMask: updateMask})
	if m.updateServiceFunc != nil {
		return m.updateServiceFunc(ctx, name, service, updateMask)
	}
	return nil
}

type updateWorkerPoolCall struct {
	name       string
	workerPool *run.GoogleCloudRunV2WorkerPool
	updateMask string
}

type mockWorkerPoolsAPI struct {
	getWorkerPoolFunc    func(ctx context.Context, name string) (*run.GoogleCloudRunV2WorkerPool, error)
	updateWorkerPoolFunc func(ctx context.Context, name string, workerPool *run.GoogleCloudRunV2WorkerPool, updateMask string) error
	getCalls             []string
	updateCalls          []updateWorkerPoolCall
}

func (m *mockWorkerPoolsAPI) getWorkerPool(ctx context.Context, name string) (*run.GoogleCloudRunV2WorkerPool, error) {
	m.getCalls = append(m.getCalls, name)
	if m.getWorkerPoolFunc != nil {
		return m.getWorkerPoolFunc(ctx, name)
	}
	return &run.GoogleCloudRunV2WorkerPool{}, nil
}

func (m *mockWorkerPoolsAPI) updateWorkerPool(ctx context.Context, name string, workerPool *run.GoogleCloudRunV2WorkerPool, updateMask string) error {
	m.updateCalls = append(m.updateCalls, updateWorkerPoolCall{name: name, workerPool: workerPool, updateMask: updateMask})
	if m.updateWorkerPoolFunc != nil {
		return m.updateWorkerPoolFunc(ctx, name, workerPool, updateMask)
	}
	return nil
}

func TestCloudRunClient(t *testing.T) {
	ctx := context.Background()

	t.Run("GetWorkerPoolInstanceCount_withNullScaling_returnsZero", func(t *testing.T) {
		wpAPI := &mockWorkerPoolsAPI{
			getWorkerPoolFunc: func(ctx context.Context, name string) (*run.GoogleCloudRunV2WorkerPool, error) {
				return &run.GoogleCloudRunV2WorkerPool{}, nil
			},
		}
		client := &CloudRunClient{workerPools: wpAPI}

		got, err := client.GetWorkerPoolInstanceCount(ctx, testWorkerPoolName, testProjectID, testRegion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
		if len(wpAPI.getCalls) != 1 || wpAPI.getCalls[0] != testWorkerPoolResource {
			t.Errorf("unexpected getCalls: %v", wpAPI.getCalls)
		}
	})

	t.Run("GetWorkerPoolInstanceCount_withManualScaling_returnsManualInstances", func(t *testing.T) {
		wpAPI := &mockWorkerPoolsAPI{
			getWorkerPoolFunc: func(ctx context.Context, name string) (*run.GoogleCloudRunV2WorkerPool, error) {
				return &run.GoogleCloudRunV2WorkerPool{
					Scaling: &run.GoogleCloudRunV2WorkerPoolScaling{
						ManualInstanceCount: 20,
					},
				}, nil
			},
		}
		client := &CloudRunClient{workerPools: wpAPI}

		got, err := client.GetWorkerPoolInstanceCount(ctx, testWorkerPoolName, testProjectID, testRegion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 20 {
			t.Errorf("got %d, want 20", got)
		}
	})

	t.Run("UpdateWorkerPoolManualInstances_succeeds", func(t *testing.T) {
		wpAPI := &mockWorkerPoolsAPI{}
		client := &CloudRunClient{workerPools: wpAPI}

		err := client.UpdateWorkerPoolManualInstances(ctx, testWorkerPoolName, 10, testProjectID, testRegion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(wpAPI.updateCalls) != 1 {
			t.Fatalf("expected 1 update call, got %d", len(wpAPI.updateCalls))
		}
		call := wpAPI.updateCalls[0]
		if call.name != testWorkerPoolResource {
			t.Errorf("got name %q, want %q", call.name, testWorkerPoolResource)
		}
		if call.workerPool.Scaling.ManualInstanceCount != 10 {
			t.Errorf("got manual instance count %d, want 10", call.workerPool.Scaling.ManualInstanceCount)
		}
		if !reflect.DeepEqual(call.workerPool.Scaling.ForceSendFields, []string{"ManualInstanceCount"}) {
			t.Errorf("got ForceSendFields %v, want [ManualInstanceCount]", call.workerPool.Scaling.ForceSendFields)
		}
		if call.updateMask != "scaling.manual_instance_count" {
			t.Errorf("got updateMask %q, want %q", call.updateMask, "scaling.manual_instance_count")
		}
	})

	t.Run("UpdateWorkerPoolManualInstances_zeroInstances_serializesZero", func(t *testing.T) {
		wpAPI := &mockWorkerPoolsAPI{}
		client := &CloudRunClient{workerPools: wpAPI}

		if err := client.UpdateWorkerPoolManualInstances(ctx, testWorkerPoolName, 0, testProjectID, testRegion); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		call := wpAPI.updateCalls[0]
		jsonBytes, err := call.workerPool.Scaling.MarshalJSON()
		if err != nil {
			t.Fatalf("unexpected MarshalJSON error: %v", err)
		}
		if !strings.Contains(string(jsonBytes), `"manualInstanceCount":0`) {
			t.Errorf("expected serialized JSON to contain manualInstanceCount:0, got %s", string(jsonBytes))
		}
	})

	t.Run("UpdateWorkerPoolManualInstances_requestRejected_returnsError", func(t *testing.T) {
		wpAPI := &mockWorkerPoolsAPI{
			updateWorkerPoolFunc: func(ctx context.Context, name string, workerPool *run.GoogleCloudRunV2WorkerPool, updateMask string) error {
				return errors.New("rejected")
			},
		}
		client := &CloudRunClient{workerPools: wpAPI}

		err := client.UpdateWorkerPoolManualInstances(ctx, testWorkerPoolName, 10, testProjectID, testRegion)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("UpdateWorkerPoolManualInstances_previousOperationPending_dispatchesAnyway", func(t *testing.T) {
		wpAPI := &mockWorkerPoolsAPI{}
		client := &CloudRunClient{workerPools: wpAPI}

		if err := client.UpdateWorkerPoolManualInstances(ctx, testWorkerPoolName, 10, testProjectID, testRegion); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := client.UpdateWorkerPoolManualInstances(ctx, testWorkerPoolName, 20, testProjectID, testRegion); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(wpAPI.updateCalls) != 2 {
			t.Errorf("got %d update calls, want 2", len(wpAPI.updateCalls))
		}
	})

	t.Run("GetServiceInstanceCount_withServiceLevelManualScaling_returnsManualInstances", func(t *testing.T) {
		svcAPI := &mockServicesAPI{
			getServiceFunc: func(ctx context.Context, name string) (*run.GoogleCloudRunV2Service, error) {
				return &run.GoogleCloudRunV2Service{
					Scaling: &run.GoogleCloudRunV2ServiceScaling{
						ScalingMode:         "MANUAL",
						ManualInstanceCount: 25,
					},
				}, nil
			},
		}
		client := &CloudRunClient{services: svcAPI}

		got, err := client.GetServiceInstanceCount(ctx, testServiceName, testProjectID, testRegion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 25 {
			t.Errorf("got %d, want 25", got)
		}
	})

	t.Run("GetServiceInstanceCount_withRevisionTemplateScaling_returnsMinInstances", func(t *testing.T) {
		svcAPI := &mockServicesAPI{
			getServiceFunc: func(ctx context.Context, name string) (*run.GoogleCloudRunV2Service, error) {
				return &run.GoogleCloudRunV2Service{
					Template: &run.GoogleCloudRunV2RevisionTemplate{
						Scaling: &run.GoogleCloudRunV2RevisionScaling{
							MinInstanceCount: 20,
						},
					},
				}, nil
			},
		}
		client := &CloudRunClient{services: svcAPI}

		got, err := client.GetServiceInstanceCount(ctx, testServiceName, testProjectID, testRegion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 20 {
			t.Errorf("got %d, want 20", got)
		}
	})

	t.Run("GetServiceInstanceCount_withRevisionTemplateScalingAndZeroMinInstances_returnsZero", func(t *testing.T) {
		svcAPI := &mockServicesAPI{
			getServiceFunc: func(ctx context.Context, name string) (*run.GoogleCloudRunV2Service, error) {
				return &run.GoogleCloudRunV2Service{
					Template: &run.GoogleCloudRunV2RevisionTemplate{
						Scaling: &run.GoogleCloudRunV2RevisionScaling{
							MinInstanceCount: 0,
						},
					},
				}, nil
			},
		}
		client := &CloudRunClient{services: svcAPI}

		got, err := client.GetServiceInstanceCount(ctx, testServiceName, testProjectID, testRegion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("GetServiceInstanceCount_withNoScalingConfigured_returnsZero", func(t *testing.T) {
		svcAPI := &mockServicesAPI{
			getServiceFunc: func(ctx context.Context, name string) (*run.GoogleCloudRunV2Service, error) {
				return &run.GoogleCloudRunV2Service{}, nil
			},
		}
		client := &CloudRunClient{services: svcAPI}

		got, err := client.GetServiceInstanceCount(ctx, testServiceName, testProjectID, testRegion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("UpdateServiceMinInstances_succeeds", func(t *testing.T) {
		svcAPI := &mockServicesAPI{
			getServiceFunc: func(ctx context.Context, name string) (*run.GoogleCloudRunV2Service, error) {
				return &run.GoogleCloudRunV2Service{
					Template: &run.GoogleCloudRunV2RevisionTemplate{
						Scaling: &run.GoogleCloudRunV2RevisionScaling{
							MinInstanceCount: 5,
							MaxInstanceCount: 222,
						},
					},
				}, nil
			},
		}
		client := &CloudRunClient{services: svcAPI}

		err := client.UpdateServiceMinInstances(ctx, testServiceName, 10, testProjectID, testRegion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(svcAPI.updateCalls) != 1 {
			t.Fatalf("expected 1 update call, got %d", len(svcAPI.updateCalls))
		}
		call := svcAPI.updateCalls[0]
		if call.service.Template.Scaling.MinInstanceCount != 10 {
			t.Errorf("got min instance count %d, want 10", call.service.Template.Scaling.MinInstanceCount)
		}
		if !reflect.DeepEqual(call.service.Template.Scaling.ForceSendFields, []string{"MinInstanceCount"}) {
			t.Errorf("got ForceSendFields %v, want [MinInstanceCount]", call.service.Template.Scaling.ForceSendFields)
		}
		if call.service.Template.Scaling.MaxInstanceCount != 222 {
			t.Errorf("got max instance count %d, want 222", call.service.Template.Scaling.MaxInstanceCount)
		}
		if call.updateMask != "template.scaling.min_instance_count" {
			t.Errorf("got updateMask %q, want %q", call.updateMask, "template.scaling.min_instance_count")
		}
	})

	t.Run("UpdateServiceMinInstances_zeroInstances_serializesZero", func(t *testing.T) {
		svcAPI := &mockServicesAPI{}
		client := &CloudRunClient{services: svcAPI}

		if err := client.UpdateServiceMinInstances(ctx, testServiceName, 0, testProjectID, testRegion); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		call := svcAPI.updateCalls[0]
		jsonBytes, err := call.service.Template.Scaling.MarshalJSON()
		if err != nil {
			t.Fatalf("unexpected MarshalJSON error: %v", err)
		}
		if !strings.Contains(string(jsonBytes), `"minInstanceCount":0`) {
			t.Errorf("expected serialized JSON to contain minInstanceCount:0, got %s", string(jsonBytes))
		}
	})

	t.Run("UpdateServiceMinInstances_requestRejected_returnsError", func(t *testing.T) {
		svcAPI := &mockServicesAPI{
			updateServiceFunc: func(ctx context.Context, name string, service *run.GoogleCloudRunV2Service, updateMask string) error {
				return errors.New("rejected")
			},
		}
		client := &CloudRunClient{services: svcAPI}

		err := client.UpdateServiceMinInstances(ctx, testServiceName, 10, testProjectID, testRegion)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("UpdateServiceManualInstances_succeeds", func(t *testing.T) {
		svcAPI := &mockServicesAPI{
			getServiceFunc: func(ctx context.Context, name string) (*run.GoogleCloudRunV2Service, error) {
				return &run.GoogleCloudRunV2Service{
					Scaling: &run.GoogleCloudRunV2ServiceScaling{
						ManualInstanceCount: 5,
					},
				}, nil
			},
		}
		client := &CloudRunClient{services: svcAPI}

		err := client.UpdateServiceManualInstances(ctx, testServiceName, 10, testProjectID, testRegion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(svcAPI.updateCalls) != 1 {
			t.Fatalf("expected 1 update call, got %d", len(svcAPI.updateCalls))
		}
		call := svcAPI.updateCalls[0]
		if call.service.Scaling.ManualInstanceCount != 10 {
			t.Errorf("got manual instance count %d, want 10", call.service.Scaling.ManualInstanceCount)
		}
		if !reflect.DeepEqual(call.service.Scaling.ForceSendFields, []string{"ManualInstanceCount"}) {
			t.Errorf("got ForceSendFields %v, want [ManualInstanceCount]", call.service.Scaling.ForceSendFields)
		}
		if call.updateMask != "scaling.manual_instance_count" {
			t.Errorf("got updateMask %q, want %q", call.updateMask, "scaling.manual_instance_count")
		}
	})

	t.Run("UpdateServiceManualInstances_zeroInstances_serializesZero", func(t *testing.T) {
		svcAPI := &mockServicesAPI{}
		client := &CloudRunClient{services: svcAPI}

		if err := client.UpdateServiceManualInstances(ctx, testServiceName, 0, testProjectID, testRegion); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		call := svcAPI.updateCalls[0]
		jsonBytes, err := call.service.Scaling.MarshalJSON()
		if err != nil {
			t.Fatalf("unexpected MarshalJSON error: %v", err)
		}
		if !strings.Contains(string(jsonBytes), `"manualInstanceCount":0`) {
			t.Errorf("expected serialized JSON to contain manualInstanceCount:0, got %s", string(jsonBytes))
		}
	})

	t.Run("UpdateServiceManualInstances_requestRejected_returnsError", func(t *testing.T) {
		svcAPI := &mockServicesAPI{
			updateServiceFunc: func(ctx context.Context, name string, service *run.GoogleCloudRunV2Service, updateMask string) error {
				return errors.New("rejected")
			},
		}
		client := &CloudRunClient{services: svcAPI}

		err := client.UpdateServiceManualInstances(ctx, testServiceName, 10, testProjectID, testRegion)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("UpdateServiceManualInstances_previousOperationPending_dispatchesAnyway", func(t *testing.T) {
		svcAPI := &mockServicesAPI{}
		client := &CloudRunClient{services: svcAPI}

		if err := client.UpdateServiceManualInstances(ctx, testServiceName, 10, testProjectID, testRegion); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := client.UpdateServiceManualInstances(ctx, testServiceName, 20, testProjectID, testRegion); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(svcAPI.updateCalls) != 2 {
			t.Errorf("got %d update calls, want 2", len(svcAPI.updateCalls))
		}
	})
}
