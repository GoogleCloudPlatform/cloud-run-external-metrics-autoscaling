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

	apiv1client "cloud.google.com/go/parametermanager/apiv1"
	pb "cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
)

// ParameterManagerClient is a client for interacting with the Google Cloud Parameter Manager.
type ParameterManagerClient struct {
	client *apiv1client.Client
}

// ParameterManager creates a new ParameterManagerClient.
func ParameterManager(ctx context.Context) (*ParameterManagerClient, error) {
	client, err := apiv1client.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create parametermanager client: %w", err)
	}
	return &ParameterManagerClient{client: client}, nil
}

// GetParameterVersion retrieves a parameter version from Google Cloud Parameter Manager.
func (c *ParameterManagerClient) GetParameterVersion(ctx context.Context, req *pb.GetParameterVersionRequest) (*pb.ParameterVersion, error) {
	result, err := c.client.GetParameterVersion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get parameter version: %w", err)
	}

	return result, nil
}
