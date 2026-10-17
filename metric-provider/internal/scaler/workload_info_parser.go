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
	"fmt"
	"regexp"
)

// WorkloadType represents the type of Cloud Run workload.
type WorkloadType int

const (
	// WorkloadTypeService represents a Cloud Run Service.
	WorkloadTypeService WorkloadType = iota
	// WorkloadTypeWorkerPool represents a Cloud Run WorkerPool.
	WorkloadTypeWorkerPool
)

var (
	servicePattern = regexp.MustCompile(`^projects/([^/]+)/locations/([^/]+)/services/([^/]+)$`)
	// `workerpools` and `workerPools` are both valid depending on Cloud Run API version
	workerpoolPattern = regexp.MustCompile(`^projects/([^/]+)/locations/([^/]+)/worker[Pp]ools/([^/]+)$`)
)

// WorkloadInfo holds the parsed information from a workload resource name.
type WorkloadInfo struct {
	WorkloadType WorkloadType
	ProjectID    string
	Location     string
	Name         string
}

// ParseWorkloadInfo parses a workload string into a WorkloadInfo struct.
func ParseWorkloadInfo(input string) (WorkloadInfo, error) {
	if matches := servicePattern.FindStringSubmatch(input); matches != nil {
		return WorkloadInfo{
			WorkloadType: WorkloadTypeService,
			ProjectID:    matches[1],
			Location:     matches[2],
			Name:         matches[3],
		}, nil
	}
	if matches := workerpoolPattern.FindStringSubmatch(input); matches != nil {
		return WorkloadInfo{
			WorkloadType: WorkloadTypeWorkerPool,
			ProjectID:    matches[1],
			Location:     matches[2],
			Name:         matches[3],
		}, nil
	}
	return WorkloadInfo{}, fmt.Errorf(
		"Invalid workload name `%s`. Expected 'projects/[PROJECT]/locations/[LOCATION]/services/[SERVICE]' or 'projects/[PROJECT]/locations/[LOCATION]/workerpools/[WORKERPOOL]'.",
		input,
	)
}
