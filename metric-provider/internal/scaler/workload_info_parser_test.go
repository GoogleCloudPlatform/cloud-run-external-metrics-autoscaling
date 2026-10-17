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

import "testing"

func TestWorkloadInfoParser(t *testing.T) {
	t.Run("workloadParser_stringForInvalidWorkload_returnsError", func(t *testing.T) {
		_, err := ParseWorkloadInfo("projects/my-project/locations/us-central1/completely/wrong")
		if err == nil {
			t.Fatalf("expected error for invalid workload")
		}
		wantMsg := "Invalid workload name `projects/my-project/locations/us-central1/completely/wrong`. Expected 'projects/[PROJECT]/locations/[LOCATION]/services/[SERVICE]' or 'projects/[PROJECT]/locations/[LOCATION]/workerpools/[WORKERPOOL]'."
		if err.Error() != wantMsg {
			t.Errorf("got error %q, want %q", err.Error(), wantMsg)
		}
		if _, err := ParseWorkloadInfo("projects/my-project/locations/us-central1/services/workerpools/both-workloads"); err == nil {
			t.Errorf("expected error for invalid workload with extra segments")
		}
	})

	t.Run("workloadParser_stringForServiceWorkload_parsesServiceInfo", func(t *testing.T) {
		info, err := ParseWorkloadInfo("projects/my-project/locations/us-central1/services/my-service")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.WorkloadType != WorkloadTypeService || info.ProjectID != "my-project" || info.Location != "us-central1" || info.Name != "my-service" {
			t.Errorf("unexpected WorkloadInfo: %+v", info)
		}
	})

	t.Run("workloadParser_stringForWorkerPoolWorkload_parsesWorkerPoolV1Format", func(t *testing.T) {
		info, err := ParseWorkloadInfo("projects/my-project/locations/us-central1/workerpools/my-workerpool")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.WorkloadType != WorkloadTypeWorkerPool || info.ProjectID != "my-project" || info.Location != "us-central1" || info.Name != "my-workerpool" {
			t.Errorf("unexpected WorkloadInfo: %+v", info)
		}
	})

	t.Run("workloadParser_stringForWorkerPoolWorkload_parsesWorkerPoolV2Format", func(t *testing.T) {
		info, err := ParseWorkloadInfo("projects/my-project/locations/us-central1/workerPools/my-workerpool")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.WorkloadType != WorkloadTypeWorkerPool || info.ProjectID != "my-project" || info.Location != "us-central1" || info.Name != "my-workerpool" {
			t.Errorf("unexpected WorkloadInfo: %+v", info)
		}
	})
}
