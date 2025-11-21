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
	"fmt"
	"hash/crc32"
	"net/url"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// SecretManagerClient is a client for interacting with GCP Secret Manager.
type SecretManagerClient struct {
	client    *secretmanager.Client
	projectID string
}

// SecretManager creates a new SecretManagerClient. The zero value is not usable.
func SecretManager(ctx context.Context, projectID string) (*SecretManagerClient, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secretmanager client: %w", err)
	}
	return &SecretManagerClient{client: client, projectID: projectID}, nil
}

// GetSecret retrieves a secret from Secret Manager.
// Defaults to the "latest" version if version is empty.
func (smc *SecretManagerClient) ReadSecret(ctx context.Context, secretID, version string) (string, error) {
	if secretID == "" {
		return "", fmt.Errorf("secretID must be provided")
	}
	if version == "" {
		version = "latest"
	}

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", url.PathEscape(smc.projectID), url.PathEscape(secretID), url.PathEscape(version)),
	}

	result, err := smc.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", err
	}

	if result == nil || result.Payload == nil {
		return "", errors.New("received empty result payload upon fetching the secret version")
	}

	crc32c := crc32.MakeTable(crc32.Castagnoli)
	checksum := int64(crc32.Checksum(result.Payload.Data, crc32c))
	if result.Payload.DataCrc32C != nil && checksum != *result.Payload.DataCrc32C {
		return "", errors.New("secret payload data corruption detected")
	}

	return string(result.Payload.Data), nil
}
