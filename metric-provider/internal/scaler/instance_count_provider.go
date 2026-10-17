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

import "context"

// CloudRunClient defines the Cloud Run operations needed by the scaler.
type CloudRunClient interface {
	GetServiceInstanceCount(ctx context.Context, serviceName, projectID, region string) (int, error)
	GetWorkerPoolInstanceCount(ctx context.Context, workerpoolName, projectID, region string) (int, error)
	UpdateServiceMinInstances(ctx context.Context, serviceName string, instances int, projectID, region string) error
	UpdateServiceManualInstances(ctx context.Context, serviceName string, instances int, projectID, region string) error
	UpdateWorkerPoolManualInstances(ctx context.Context, workerpoolName string, instances int, projectID, region string) error
}

// GetInstanceCount gets the last requested instance count for a given workload.
func GetInstanceCount(ctx context.Context, cloudRunClient CloudRunClient, workloadInfo WorkloadInfo) (int, error) {
	if workloadInfo.WorkloadType == WorkloadTypeService {
		return cloudRunClient.GetServiceInstanceCount(ctx, workloadInfo.Name, workloadInfo.ProjectID, workloadInfo.Location)
	}
	return cloudRunClient.GetWorkerPoolInstanceCount(ctx, workloadInfo.Name, workloadInfo.ProjectID, workloadInfo.Location)
}
