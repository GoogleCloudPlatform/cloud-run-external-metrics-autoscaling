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
	"testing"
)

type updateInstancesCall struct {
	name      string
	instances int
	projectID string
	region    string
}

type mockCloudRunClient struct {
	getServiceInstanceCountFunc          func(ctx context.Context, serviceName, projectID, region string) (int, error)
	getWorkerPoolInstanceCountFunc       func(ctx context.Context, workerpoolName, projectID, region string) (int, error)
	updateServiceMinInstancesFunc        func(ctx context.Context, serviceName string, instances int, projectID, region string) error
	updateServiceManualInstancesFunc     func(ctx context.Context, serviceName string, instances int, projectID, region string) error
	updateWorkerPoolManualInstancesFunc  func(ctx context.Context, workerpoolName string, instances int, projectID, region string) error
	updateServiceMinInstancesCalls       []updateInstancesCall
	updateServiceManualInstancesCalls    []updateInstancesCall
	updateWorkerPoolManualInstancesCalls []updateInstancesCall
}

func (m *mockCloudRunClient) GetServiceInstanceCount(ctx context.Context, serviceName, projectID, region string) (int, error) {
	if m.getServiceInstanceCountFunc != nil {
		return m.getServiceInstanceCountFunc(ctx, serviceName, projectID, region)
	}
	return 0, nil
}

func (m *mockCloudRunClient) GetWorkerPoolInstanceCount(ctx context.Context, workerpoolName, projectID, region string) (int, error) {
	if m.getWorkerPoolInstanceCountFunc != nil {
		return m.getWorkerPoolInstanceCountFunc(ctx, workerpoolName, projectID, region)
	}
	return 0, nil
}

func (m *mockCloudRunClient) UpdateServiceMinInstances(ctx context.Context, serviceName string, instances int, projectID, region string) error {
	m.updateServiceMinInstancesCalls = append(m.updateServiceMinInstancesCalls, updateInstancesCall{name: serviceName, instances: instances, projectID: projectID, region: region})
	if m.updateServiceMinInstancesFunc != nil {
		return m.updateServiceMinInstancesFunc(ctx, serviceName, instances, projectID, region)
	}
	return nil
}

func (m *mockCloudRunClient) UpdateServiceManualInstances(ctx context.Context, serviceName string, instances int, projectID, region string) error {
	m.updateServiceManualInstancesCalls = append(m.updateServiceManualInstancesCalls, updateInstancesCall{name: serviceName, instances: instances, projectID: projectID, region: region})
	if m.updateServiceManualInstancesFunc != nil {
		return m.updateServiceManualInstancesFunc(ctx, serviceName, instances, projectID, region)
	}
	return nil
}

func (m *mockCloudRunClient) UpdateWorkerPoolManualInstances(ctx context.Context, workerpoolName string, instances int, projectID, region string) error {
	m.updateWorkerPoolManualInstancesCalls = append(m.updateWorkerPoolManualInstancesCalls, updateInstancesCall{name: workerpoolName, instances: instances, projectID: projectID, region: region})
	if m.updateWorkerPoolManualInstancesFunc != nil {
		return m.updateWorkerPoolManualInstancesFunc(ctx, workerpoolName, instances, projectID, region)
	}
	return nil
}

func TestInstanceCountProvider(t *testing.T) {
	ctx := context.Background()
	projectID := "test-project"
	location := "test-location"
	serviceName := "test-service"
	workerpoolName := "test-workerpool"

	t.Run("getInstanceCount_serviceInstances", func(t *testing.T) {
		mockClient := &mockCloudRunClient{
			getServiceInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				if name == serviceName && proj == projectID && loc == location {
					return 2, nil
				}
				return 0, nil
			},
		}
		info := WorkloadInfo{
			WorkloadType: WorkloadTypeService,
			ProjectID:    projectID,
			Location:     location,
			Name:         serviceName,
		}
		got, err := GetInstanceCount(ctx, mockClient, info)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 2 {
			t.Errorf("got %d, want 2", got)
		}
	})

	t.Run("getInstanceCount_workerPoolInstances", func(t *testing.T) {
		mockClient := &mockCloudRunClient{
			getWorkerPoolInstanceCountFunc: func(ctx context.Context, name, proj, loc string) (int, error) {
				if name == workerpoolName && proj == projectID && loc == location {
					return 4, nil
				}
				return 0, nil
			},
		}
		info := WorkloadInfo{
			WorkloadType: WorkloadTypeWorkerPool,
			ProjectID:    projectID,
			Location:     location,
			Name:         workerpoolName,
		}
		got, err := GetInstanceCount(ctx, mockClient, info)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 4 {
			t.Errorf("got %d, want 4", got)
		}
	})
}
