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

package scaling

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers"
	"github.com/kedacore/keda/v2/pkg/scalers/scalersconfig"
	kedautil "github.com/kedacore/keda/v2/pkg/util"
	v2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/apis/external_metrics"
)

type cursorScaler struct {
	metricType v2.MetricTargetType
	metadata   *cursorScalerMetadata
	httpClient *http.Client
	logger     logr.Logger
}

type cursorScalerMetadata struct {
	apiKey string
}

type cursorSummaryResponse struct {
	TeamSummary *cursorSummary `json:"teamSummary"`
	UserSummary *cursorSummary `json:"userSummary"`
}

type cursorSummary struct {
	InUse          int `json:"inUse"`
	TotalConnected int `json:"totalConnected"`
}

// NewCursorScaler creates a new Cursor scaler
func NewCursorScaler(config *scalersconfig.ScalerConfig) (scalers.Scaler, error) {
	metricType, err := scalers.GetMetricTargetType(config)
	if err != nil {
		return nil, fmt.Errorf("error getting scaler metric type: %w", err)
	}

	meta, err := parseCursorMetadata(config)
	if err != nil {
		return nil, fmt.Errorf("error parsing cursor metadata: %w", err)
	}

	httpClient := kedautil.CreateHTTPClient(config.GlobalHTTPTimeout, false)

	return &cursorScaler{
		metricType: metricType,
		metadata:   meta,
		httpClient: httpClient,
		logger:     scalers.InitializeLogger(config, "cursor_scaler"),
	}, nil
}

func parseCursorMetadata(config *scalersconfig.ScalerConfig) (*cursorScalerMetadata, error) {
	meta := &cursorScalerMetadata{}

	// Resolve the API key from authentication params or trigger metadata
	if val, ok := config.AuthParams["apiKey"]; ok && val != "" {
		meta.apiKey = val
	} else if val, ok := config.TriggerMetadata["apiKey"]; ok && val != "" {
		meta.apiKey = val
	} else {
		return nil, fmt.Errorf("no apiKey provided in metadata or auth params")
	}

	return meta, nil
}

// GetMetricSpecForScaling returns the metric spec for the scaler
func (s *cursorScaler) GetMetricSpecForScaling(ctx context.Context) []v2.MetricSpec {
	metricName := kedautil.ModifyMetricName("cursor_in_use")
	
	externalMetric := &v2.ExternalMetricSource{
		Metric: v2.MetricIdentifier{
			Name: metricName,
		},
		Target: v2.MetricTarget{
			Type: s.metricType,
		},
	}

	metricSpec := v2.MetricSpec{
		Type:     v2.ExternalMetricSourceType,
		External: externalMetric,
	}
	return []v2.MetricSpec{metricSpec}
}

// GetMetricsAndActivity retrieves the current metric value from Cursor API
func (s *cursorScaler) GetMetricsAndActivity(ctx context.Context, metricName string) ([]external_metrics.ExternalMetricValue, bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.cursor.com/v0/private-workers/summary", nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add authorization header: "Authorization: Basic <Base64(apiKey:)>"
	authStr := s.metadata.apiKey + ":"
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authStr))
	req.Header.Set("Authorization", "Basic "+encodedAuth)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("Cursor API returned status code %d", resp.StatusCode)
	}

	var summary cursorSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, false, fmt.Errorf("failed to decode Cursor API response: %w", err)
	}

	var inUse int
	var totalConnected int
	var active bool

	if summary.TeamSummary != nil && summary.TeamSummary.TotalConnected > 0 {
		inUse = summary.TeamSummary.InUse
		totalConnected = summary.TeamSummary.TotalConnected
	} else if summary.UserSummary != nil && summary.UserSummary.TotalConnected > 0 {
		inUse = summary.UserSummary.InUse
		totalConnected = summary.UserSummary.TotalConnected
	} else {
		s.logger.Info("No private workers connected to Cursor")
		active = false
	}

	if totalConnected > 0 {
		active = inUse > 0
	}

	s.logger.Info(fmt.Sprintf("Retrieved Cursor private workers summary: inUse=%d, totalConnected=%d", inUse, totalConnected))

	metricValue := external_metrics.ExternalMetricValue{
		MetricName: metricName,
		Value:      *resource.NewQuantity(int64(inUse), resource.DecimalSI),
		Timestamp:  metav1.Now(),
	}

	return []external_metrics.ExternalMetricValue{metricValue}, active, nil
}

// Close closes the scaler
func (s *cursorScaler) Close(ctx context.Context) error {
	return nil
}
