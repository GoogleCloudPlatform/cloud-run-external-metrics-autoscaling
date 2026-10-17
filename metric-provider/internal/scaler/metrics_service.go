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
	"fmt"
	"reflect"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/go-logr/logr"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CloudMonitoringClient defines the interface for writing custom metrics to Cloud Monitoring.
type CloudMonitoringClient interface {
	CreateTimeSeries(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) error
	Close() error
}

// MetricsService encapsulates the logic of interacting with Cloud Monitoring.
type MetricsService struct {
	cloudMonitoringClient CloudMonitoringClient
	projectName           string
	resourceLabels        map[string]string
	logger                logr.Logger
}

// NewMetricsService creates a new MetricsService.
func NewMetricsService(cloudMonitoringClient CloudMonitoringClient, projectID string, logger logr.Logger) (*MetricsService, error) {
	if cloudMonitoringClient == nil || (reflect.ValueOf(cloudMonitoringClient).Kind() == reflect.Ptr && reflect.ValueOf(cloudMonitoringClient).IsNil()) {
		return nil, fmt.Errorf("cloud monitoring client cannot be nil")
	}
	if projectID == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}
	return &MetricsService{
		cloudMonitoringClient: cloudMonitoringClient,
		projectName:           fmt.Sprintf("projects/%s", projectID),
		resourceLabels:        map[string]string{"project_id": projectID},
		logger:                logger,
	}, nil
}

// WriteMetricIgnoreFailure writes a custom gauge metric to Cloud Monitoring ignoring any failures.
func (m *MetricsService) WriteMetricIgnoreFailure(ctx context.Context, metricName string, value float64, metricLabels map[string]string) error {
	if metricName == "" {
		return fmt.Errorf("metricName cannot be empty")
	}

	now := time.Now()
	labels := make(map[string]string, len(metricLabels))
	for k, v := range metricLabels {
		labels[k] = v
	}

	req := &monitoringpb.CreateTimeSeriesRequest{
		Name: m.projectName,
		TimeSeries: []*monitoringpb.TimeSeries{
			{
				Metric: &metricpb.Metric{
					Type:   fmt.Sprintf("custom.googleapis.com/%s", metricName),
					Labels: labels,
				},
				Resource: &monitoredres.MonitoredResource{
					Type:   "global",
					Labels: m.resourceLabels,
				},
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							EndTime: timestamppb.New(now.Truncate(time.Millisecond)),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_DoubleValue{
								DoubleValue: value,
							},
						},
					},
				},
				MetricKind: metricpb.MetricDescriptor_GAUGE,
				ValueType:  metricpb.MetricDescriptor_DOUBLE,
			},
		},
	}

	if err := m.cloudMonitoringClient.CreateTimeSeries(ctx, req); err != nil {
		// Errors may occur if emitting metrics too frequently (e.g., frequently invoked scaler or on instance restart).
		m.logger.V(1).Info(fmt.Sprintf("Failed to create time series: %v", err))
	}
	return nil
}
