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
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
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
	updateServiceFunc func(ctx context.Context, name string, service *run.GoogleCloudRunV2Service, updateMask string) (*run.GoogleLongrunningOperation, error)
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

func (m *mockServicesAPI) updateService(ctx context.Context, name string, service *run.GoogleCloudRunV2Service, updateMask string) (*run.GoogleLongrunningOperation, error) {
	m.updateCalls = append(m.updateCalls, updateServiceCall{name: name, service: service, updateMask: updateMask})
	if m.updateServiceFunc != nil {
		return m.updateServiceFunc(ctx, name, service, updateMask)
	}
	return &run.GoogleLongrunningOperation{Name: "operations/test", Done: true}, nil
}

type updateWorkerPoolCall struct {
	name       string
	workerPool *run.GoogleCloudRunV2WorkerPool
	updateMask string
}

type mockWorkerPoolsAPI struct {
	getWorkerPoolFunc    func(ctx context.Context, name string) (*run.GoogleCloudRunV2WorkerPool, error)
	updateWorkerPoolFunc func(ctx context.Context, name string, workerPool *run.GoogleCloudRunV2WorkerPool, updateMask string) (*run.GoogleLongrunningOperation, error)
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

func (m *mockWorkerPoolsAPI) updateWorkerPool(ctx context.Context, name string, workerPool *run.GoogleCloudRunV2WorkerPool, updateMask string) (*run.GoogleLongrunningOperation, error) {
	m.updateCalls = append(m.updateCalls, updateWorkerPoolCall{name: name, workerPool: workerPool, updateMask: updateMask})
	if m.updateWorkerPoolFunc != nil {
		return m.updateWorkerPoolFunc(ctx, name, workerPool, updateMask)
	}
	return &run.GoogleLongrunningOperation{Name: "operations/test", Done: true}, nil
}

// newLogCapture returns a logger that sends every formatted log line to the returned channel.
func newLogCapture() (logr.Logger, <-chan string) {
	lines := make(chan string, 10)
	logger := funcr.New(func(prefix, args string) { lines <- args }, funcr.Options{})
	return logger, lines
}

func waitForLogLine(t *testing.T, lines <-chan string) string {
	t.Helper()
	select {
	case line := <-lines:
		return line
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a log line")
		return ""
	}
}

func newTestOpClient(svc *mockServicesAPI, wp *mockWorkerPoolsAPI, getOp func(context.Context, string) (*run.GoogleLongrunningOperation, error), logger logr.Logger, timeout time.Duration) *CloudRunClient {
	return &CloudRunClient{
		services:       svc,
		workerPools:    wp,
		getOperation:   getOp,
		logger:         logger,
		opPollInterval: time.Millisecond,
		opPollTimeout:  timeout,
	}
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
			updateWorkerPoolFunc: func(ctx context.Context, name string, workerPool *run.GoogleCloudRunV2WorkerPool, updateMask string) (*run.GoogleLongrunningOperation, error) {
				return nil, errors.New("rejected")
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
		if len(svcAPI.getCalls) != 1 || svcAPI.getCalls[0] != testServiceResource {
			t.Errorf("unexpected getCalls: %v", svcAPI.getCalls)
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
		if call.name != testServiceResource {
			t.Errorf("got name %q, want %q", call.name, testServiceResource)
		}
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
			updateServiceFunc: func(ctx context.Context, name string, service *run.GoogleCloudRunV2Service, updateMask string) (*run.GoogleLongrunningOperation, error) {
				return nil, errors.New("rejected")
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
		if call.name != testServiceResource {
			t.Errorf("got name %q, want %q", call.name, testServiceResource)
		}
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
			updateServiceFunc: func(ctx context.Context, name string, service *run.GoogleCloudRunV2Service, updateMask string) (*run.GoogleLongrunningOperation, error) {
				return nil, errors.New("rejected")
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

func TestCloudRunClient_logsHowUpdateOperationsEnd(t *testing.T) {
	ctx := context.Background()
	pending := &run.GoogleLongrunningOperation{Name: "operations/op-1"}

	tests := []struct {
		name          string
		updateOp      *run.GoogleLongrunningOperation
		updateErr     error
		workerPool    bool
		pollTimeout   time.Duration
		pollResponses []*run.GoogleLongrunningOperation
		pollErr       error
		wantErr       bool
		wantLogParts  []string
		wantPollCount int
		wantZeroLogs  bool
	}{
		{
			name:          "operationCompletes_logsCompletion",
			updateOp:      pending,
			pollResponses: []*run.GoogleLongrunningOperation{{Name: "operations/op-1"}, {Name: "operations/op-1", Done: true}},
			wantLogParts:  []string{"Update operation completed", `"resource"="` + testServiceResource + `"`},
			wantPollCount: 2,
		},
		{
			name:          "operationFails_logsFailureWithCause",
			updateOp:      pending,
			workerPool:    true,
			pollResponses: []*run.GoogleLongrunningOperation{{Name: "operations/op-1", Done: true, Error: &run.GoogleRpcStatus{Code: 9, Message: "revision is not ready"}}},
			wantLogParts:  []string{"Update operation failed", `"resource"="` + testWorkerPoolResource + `"`, "revision is not ready (code 9)"},
			wantPollCount: 1,
		},
		{
			name:          "operationAlreadyDone_logsWithoutPolling",
			updateOp:      &run.GoogleLongrunningOperation{Name: "operations/done", Done: true},
			wantLogParts:  []string{"Update operation completed", `"resource"="` + testServiceResource + `"`},
			wantPollCount: 0,
		},
		{
			name:          "pollingFails_logsThatTheOutcomeIsUnknown",
			updateOp:      pending,
			pollErr:       errors.New("permission denied"),
			wantLogParts:  []string{"Update operation failed", `"resource"="` + testServiceResource + `"`, "failed to check operation", "permission denied"},
			wantPollCount: 1,
		},
		{
			name:          "pollingTransportDeadlineExceeded_logsPollingFailure",
			updateOp:      pending,
			pollErr:       context.DeadlineExceeded,
			wantLogParts:  []string{"Update operation failed", `"resource"="` + testServiceResource + `"`, "failed to check operation", context.DeadlineExceeded.Error()},
			wantPollCount: 1,
		},
		{
			name:          "operationOutlivesPollingTimeout_logsTimeout",
			updateOp:      pending,
			pollTimeout:   20 * time.Millisecond,
			pollResponses: []*run.GoogleLongrunningOperation{pending},
			wantLogParts:  []string{"Update operation failed", `"resource"="` + testServiceResource + `"`, "timed out waiting for operation", context.DeadlineExceeded.Error()},
			wantPollCount: -1, // varies with scheduling within 20ms
		},
		{
			name:         "updateRejected_logsNothing",
			updateErr:    errors.New("rejected"),
			wantErr:      true,
			wantZeroLogs: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var polled []string
			idx := 0
			getOp := func(ctx context.Context, name string) (*run.GoogleLongrunningOperation, error) {
				mu.Lock()
				defer mu.Unlock()
				polled = append(polled, name)
				if tc.pollErr != nil {
					return nil, tc.pollErr
				}
				op := tc.pollResponses[min(idx, len(tc.pollResponses)-1)]
				idx++
				return op, nil
			}

			timeout := tc.pollTimeout
			if timeout == 0 {
				timeout = 5 * time.Second
			}
			logger, lines := newLogCapture()

			var svcAPI *mockServicesAPI
			var wpAPI *mockWorkerPoolsAPI
			if tc.workerPool {
				wpAPI = &mockWorkerPoolsAPI{
					updateWorkerPoolFunc: func(context.Context, string, *run.GoogleCloudRunV2WorkerPool, string) (*run.GoogleLongrunningOperation, error) {
						return tc.updateOp, tc.updateErr
					},
				}
			} else {
				svcAPI = &mockServicesAPI{
					updateServiceFunc: func(context.Context, string, *run.GoogleCloudRunV2Service, string) (*run.GoogleLongrunningOperation, error) {
						return tc.updateOp, tc.updateErr
					},
				}
			}

			client := newTestOpClient(svcAPI, wpAPI, getOp, logger, timeout)
			var err error
			if tc.workerPool {
				err = client.UpdateWorkerPoolManualInstances(ctx, testWorkerPoolName, 10, testProjectID, testRegion)
			} else {
				err = client.UpdateServiceManualInstances(ctx, testServiceName, 10, testProjectID, testRegion)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tc.wantErr)
			}

			if tc.wantZeroLogs {
				select {
				case line := <-lines:
					t.Errorf("got unexpected log line %q", line)
				case <-time.After(30 * time.Millisecond):
				}
				return
			}

			line := waitForLogLine(t, lines)
			for _, want := range tc.wantLogParts {
				if !strings.Contains(line, want) {
					t.Errorf("got log line %q, want substring %q", line, want)
				}
			}
			if tc.wantPollCount >= 0 {
				mu.Lock()
				got := append([]string(nil), polled...)
				mu.Unlock()
				if len(got) != tc.wantPollCount {
					t.Errorf("got %d polls (%v), want %d", len(got), got, tc.wantPollCount)
				}
			}
		})
	}
}
