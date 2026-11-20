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
	"fmt"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
)

// createTimeSeriesTimeout is the client library's own default for CreateTimeSeries. The library
// only applies its default when the context has no deadline, and scale calls always have one.
const createTimeSeriesTimeout = 12 * time.Second

// CloudMonitoringClient is a wrapper around the Cloud Monitoring MetricClient.
type CloudMonitoringClient struct {
	client *monitoring.MetricClient
}

// NewCloudMonitoringClient creates a new CloudMonitoringClient.
func NewCloudMonitoringClient(ctx context.Context) (*CloudMonitoringClient, error) {
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud monitoring metric client: %w", err)
	}
	return &CloudMonitoringClient{client: client}, nil
}

// CreateTimeSeries writes time series data to Cloud Monitoring.
func (c *CloudMonitoringClient) CreateTimeSeries(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) error {
	ctx, cancel := context.WithTimeout(ctx, createTimeSeriesTimeout)
	defer cancel()
	return c.client.CreateTimeSeries(ctx, req)
}

// Close closes the underlying Cloud Monitoring MetricClient.
func (c *CloudMonitoringClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
