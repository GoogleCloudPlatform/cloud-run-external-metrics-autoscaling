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
)

type StubSecretManagerClient struct {
	Secrets   map[string]string
	Error     error
	ProjectID string
}

func (c *StubSecretManagerClient) ReadSecret(ctx context.Context, secretID, version string) (string, error) {
	key := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", c.ProjectID, secretID, version)
	if val, ok := c.Secrets[key]; ok {
		return val, nil
	}
	if c.Error != nil {
		return "", c.Error
	}
	return "", fmt.Errorf("secret not found: %s", key)
}
