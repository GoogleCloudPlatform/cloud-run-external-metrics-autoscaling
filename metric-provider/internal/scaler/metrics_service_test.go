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
	"errors"
	"reflect"
	"testing"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/go-logr/logr"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
)

type mockCloudMonitoringClient struct {
	createTimeSeriesFunc func(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) error
	calls                []*monitoringpb.CreateTimeSeriesRequest
}

func (m *mockCloudMonitoringClient) CreateTimeSeries(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) error {
	m.calls = append(m.calls, req)
	if m.createTimeSeriesFunc != nil {
		return m.createTimeSeriesFunc(ctx, req)
	}
	return nil
}

func (m *mockCloudMonitoringClient) Close() error {
	return nil
}

func TestMetricsService(t *testing.T) {
	ctx := context.Background()
	logger := logr.Discard()
	projectID := "projectId"

	t.Run("nilCloudMonitoringClient_returnsError", func(t *testing.T) {
		if _, err := NewMetricsService(nil, projectID, logger); err == nil {
			t.Errorf("expected error for nil CloudMonitoringClient")
		}
		var typedNil *mockCloudMonitoringClient
		if _, err := NewMetricsService(typedNil, projectID, logger); err == nil {
			t.Errorf("expected error for typed-nil CloudMonitoringClient")
		}
	})

	t.Run("emptyProjectID_returnsError", func(t *testing.T) {
		if _, err := NewMetricsService(&mockCloudMonitoringClient{}, "", logger); err == nil {
			t.Errorf("expected error for empty projectID")
		}
	})

	t.Run("writeMetricIgnoreFailure_withoutMetricName_returnsError", func(t *testing.T) {
		ms, err := NewMetricsService(&mockCloudMonitoringClient{}, projectID, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := ms.WriteMetricIgnoreFailure(ctx, "", 1.0, map[string]string{}); err == nil {
			t.Errorf("expected error for empty metricName")
		}
	})

	t.Run("writeMetricIgnoreFailure_createTimeSeriesError_doesNotReturnError", func(t *testing.T) {
		mockClient := &mockCloudMonitoringClient{
			createTimeSeriesFunc: func(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) error {
				return errors.New("monitoring error")
			},
		}
		ms, err := NewMetricsService(mockClient, projectID, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := ms.WriteMetricIgnoreFailure(ctx, "test_metric", 1.0, map[string]string{"test_label": "test_value"}); err != nil {
			t.Errorf("expected nil error when CreateTimeSeries fails, got %v", err)
		}
	})

	t.Run("writeMetricIgnoreFailure_writesGaugeMetric", func(t *testing.T) {
		mockClient := &mockCloudMonitoringClient{}
		ms, err := NewMetricsService(mockClient, projectID, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		labels := map[string]string{"test_label": "test_value"}
		if err := ms.WriteMetricIgnoreFailure(ctx, "test_metric", 1.0, labels); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mockClient.calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mockClient.calls))
		}
		actual := mockClient.calls[0]
		if len(actual.TimeSeries) != 1 {
			t.Fatalf("expected 1 TimeSeries, got %d", len(actual.TimeSeries))
		}
		ts := actual.TimeSeries[0]
		if ts.Metric.Type != "custom.googleapis.com/test_metric" {
			t.Errorf("got metric type %q, want %q", ts.Metric.Type, "custom.googleapis.com/test_metric")
		}
		if !reflect.DeepEqual(ts.Metric.Labels, labels) {
			t.Errorf("got labels %v, want %v", ts.Metric.Labels, labels)
		}
		if ts.MetricKind != metricpb.MetricDescriptor_GAUGE {
			t.Errorf("got MetricKind %v, want GAUGE", ts.MetricKind)
		}
		if ts.ValueType != metricpb.MetricDescriptor_DOUBLE {
			t.Errorf("got ValueType %v, want DOUBLE", ts.ValueType)
		}
		if len(ts.Points) != 1 || ts.Points[0].Value.GetDoubleValue() != 1.0 {
			t.Errorf("unexpected points: %+v", ts.Points)
		}
		if ts.Points[0].Interval.GetEndTime().GetNanos()%1_000_000 != 0 {
			t.Errorf("expected EndTime truncated to milliseconds, got nanos=%d", ts.Points[0].Interval.GetEndTime().GetNanos())
		}
		if ts.Resource.Type != "global" {
			t.Errorf("got resource type %q, want global", ts.Resource.Type)
		}
		if !reflect.DeepEqual(ts.Resource.Labels, map[string]string{"project_id": projectID}) {
			t.Errorf("unexpected resource labels: %v", ts.Resource.Labels)
		}
	})

	t.Run("writeMetricIgnoreFailure_withNilLabels_writesGaugeMetric", func(t *testing.T) {
		mockClient := &mockCloudMonitoringClient{}
		ms, err := NewMetricsService(mockClient, projectID, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := ms.WriteMetricIgnoreFailure(ctx, "test_metric", 1.0, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mockClient.calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mockClient.calls))
		}
		if len(mockClient.calls[0].TimeSeries[0].Metric.Labels) != 0 {
			t.Errorf("expected empty labels, got %v", mockClient.calls[0].TimeSeries[0].Metric.Labels)
		}
	})
}
