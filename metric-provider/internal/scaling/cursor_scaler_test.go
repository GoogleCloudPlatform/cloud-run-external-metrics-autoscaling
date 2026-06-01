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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/kedacore/keda/v2/pkg/scalers/scalersconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestNewCursorScaler(t *testing.T) {
	t.Run("valid configuration in trigger metadata", func(t *testing.T) {
		config := &scalersconfig.ScalerConfig{
			TriggerMetadata: map[string]string{
				"apiKey": "test-api-key",
			},
			AuthParams:        map[string]string{},
			GlobalHTTPTimeout: 1 * time.Second,
		}

		scaler, err := NewCursorScaler(config)
		require.NoError(t, err)
		require.NotNil(t, scaler)
	})

	t.Run("valid configuration in auth params", func(t *testing.T) {
		config := &scalersconfig.ScalerConfig{
			TriggerMetadata: map[string]string{},
			AuthParams: map[string]string{
				"apiKey": "test-api-key-auth",
			},
			GlobalHTTPTimeout: 1 * time.Second,
		}

		scaler, err := NewCursorScaler(config)
		require.NoError(t, err)
		require.NotNil(t, scaler)
	})

	t.Run("missing API key", func(t *testing.T) {
		config := &scalersconfig.ScalerConfig{
			TriggerMetadata:   map[string]string{},
			AuthParams:        map[string]string{},
			GlobalHTTPTimeout: 1 * time.Second,
		}

		scaler, err := NewCursorScaler(config)
		assert.Error(t, err)
		assert.Nil(t, scaler)
	})
}

func TestCursorScaler_GetMetricsAndActive(t *testing.T) {
	t.Run("team private workers connected and in use", func(t *testing.T) {
		mockResponse := cursorSummaryResponse{
			TeamSummary: &cursorSummary{
				InUse:          3,
				TotalConnected: 5,
			},
		}

		config := &scalersconfig.ScalerConfig{
			TriggerMetadata: map[string]string{
				"apiKey": "my-test-key",
			},
			AuthParams:        map[string]string{},
			GlobalHTTPTimeout: 1 * time.Second,
		}

		scalerInterface, err := NewCursorScaler(config)
		require.NoError(t, err)

		scaler := scalerInterface.(*cursorScaler)
		scaler.httpClient.Transport = &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "https://api.cursor.com/v0/private-workers/summary", req.URL.String())
				assert.Equal(t, "application/json", req.Header.Get("Accept"))

				authHeader := req.Header.Get("Authorization")
				expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("my-test-key:"))
				assert.Equal(t, expectedAuth, authHeader)

				body, _ := json.Marshal(mockResponse)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			},
		}

		metrics, active, err := scaler.GetMetricsAndActivity(context.Background(), "cursor_in_use")
		require.NoError(t, err)
		assert.True(t, active)
		require.Len(t, metrics, 1)
		assert.Equal(t, int64(3), metrics[0].Value.Value())
	})

	t.Run("user private workers connected but idle", func(t *testing.T) {
		mockResponse := cursorSummaryResponse{
			UserSummary: &cursorSummary{
				InUse:          0,
				TotalConnected: 2,
			},
		}

		config := &scalersconfig.ScalerConfig{
			TriggerMetadata: map[string]string{
				"apiKey": "user-key",
			},
			AuthParams:        map[string]string{},
			GlobalHTTPTimeout: 1 * time.Second,
		}

		scalerInterface, err := NewCursorScaler(config)
		require.NoError(t, err)

		scaler := scalerInterface.(*cursorScaler)
		scaler.httpClient.Transport = &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				body, _ := json.Marshal(mockResponse)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			},
		}

		metrics, active, err := scaler.GetMetricsAndActivity(context.Background(), "cursor_in_use")
		require.NoError(t, err)
		assert.False(t, active) // Active should be false since InUse is 0
		require.Len(t, metrics, 1)
		assert.Equal(t, int64(0), metrics[0].Value.Value())
	})

	t.Run("no private workers connected", func(t *testing.T) {
		mockResponse := cursorSummaryResponse{}

		config := &scalersconfig.ScalerConfig{
			TriggerMetadata: map[string]string{
				"apiKey": "empty-key",
			},
			AuthParams:        map[string]string{},
			GlobalHTTPTimeout: 1 * time.Second,
		}

		scalerInterface, err := NewCursorScaler(config)
		require.NoError(t, err)

		scaler := scalerInterface.(*cursorScaler)
		scaler.httpClient.Transport = &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				body, _ := json.Marshal(mockResponse)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			},
		}

		metrics, active, err := scaler.GetMetricsAndActivity(context.Background(), "cursor_in_use")
		require.NoError(t, err)
		assert.False(t, active)
		require.Len(t, metrics, 1)
		assert.Equal(t, int64(0), metrics[0].Value.Value())
	})
}
