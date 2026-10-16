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

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

type mockOrchestrator struct {
	refreshMetricsCalled chan bool
}

func (m *mockOrchestrator) RefreshMetrics(ctx context.Context) error {
	m.refreshMetricsCalled <- true
	return nil
}

func newMockOrchestrator() *mockOrchestrator {
	return &mockOrchestrator{
		refreshMetricsCalled: make(chan bool, 1),
	}
}

type mockScalerServerClient struct{}

func (m *mockScalerServerClient) Close() error {
	return nil
}

func TestHandleHealthCheck(t *testing.T) {
	logger := logr.Discard()
	s := &Server{
		logger: &logger,
	}
	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(s.handleHealthCheck)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	expected := `ok`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestHandleRequest(t *testing.T) {
	mockOrchestrator := newMockOrchestrator()
	logger := logr.Discard()
	s := &Server{
		scalingOrchestrator: mockOrchestrator,
		logger:              &logger,
	}
	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(s.handleRequest)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	select {
	case <-mockOrchestrator.refreshMetricsCalled:
		// success
	default:
		t.Error("expected RefreshMetrics to be called, but it was not")
	}
}

func TestHandleRequest_MethodNotAllowed(t *testing.T) {
	logger := logr.Discard()
	s := &Server{
		logger: &logger,
	}
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(s.handleRequest)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}
}

func TestRefreshMetricsPolling(t *testing.T) {
	orchestrator := newMockOrchestrator()
	pollingInterval := 10 * time.Millisecond
	logger := logr.Discard()
	s := &Server{
		scalingOrchestrator: orchestrator,
		pollingInterval:     &pollingInterval,
		logger:              &logger,
		scalerServerClient:  &mockScalerServerClient{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := s.Start(ctx); err != nil && err.Error() != "http: Server closed" {
			t.Errorf("Server.Start() returned an unexpected error: %v", err)
		}
	}()

	select {
	case <-orchestrator.refreshMetricsCalled:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for RefreshMetrics to be called")
	}
}
