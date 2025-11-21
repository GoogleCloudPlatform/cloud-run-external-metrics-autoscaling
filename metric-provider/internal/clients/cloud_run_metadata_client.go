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
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	projectIDMetadataURL = "http://metadata.google.internal/computeMetadata/v1/project/project-id"
	metadataFlavorHeader = "Metadata-Flavor"
	metadataFlavorValue  = "Google"
)

// CloudRunMetadataClient is a client for retrieving Cloud Run metadata from the Cloud Run metadata server.
type CloudRunMetadataClient struct {
	client *http.Client
}

func CloudRunMetadata() *CloudRunMetadataClient {
	return &CloudRunMetadataClient{
		client: &http.Client{},
	}
}

// GetProjectID retrieves the project id.
func (c *CloudRunMetadataClient) GetProjectID() (string, error) {
	req, err := http.NewRequest(http.MethodGet, projectIDMetadataURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create metadata request: %w", err)
	}
	req.Header.Set(metadataFlavorHeader, metadataFlavorValue)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make metadata request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to retrieve project id, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read metadata response body: %w", err)
	}

	projectID := strings.TrimSpace(string(body))
	if projectID == "" {
		return "", fmt.Errorf("project id not found in metadata response")
	}

	return projectID, nil
}
