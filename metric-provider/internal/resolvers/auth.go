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

package resolvers

import (
	"context"
	"crema/metric-provider/api"
	"fmt"
)

type secretManagerClient interface {
	ReadSecret(ctx context.Context, secretID, version string) (string, error)
}

type AuthResolver struct {
	client secretManagerClient
}

// Create a new AuthResolver instance. The zero value is not usable.
func NewAuthResolver(client secretManagerClient) *AuthResolver {
	return &AuthResolver{
		client: client,
	}
}

// ResolveAuthRef returns the auth params for the trigger authentication object with the given name
func (r *AuthResolver) ResolveAuthRef(ctx context.Context, triggerAuths []api.TriggerAuthentication, triggerAuthName string) (map[string]string, error) {
	if triggerAuths == nil {
		return nil, fmt.Errorf("no trigger authentication provided")
	}

	for _, triggerAuth := range triggerAuths {
		if triggerAuth.ObjectMeta.Name == triggerAuthName {
			return r.resolveAuthParams(ctx, triggerAuth.Spec)
		}
	}

	return nil, fmt.Errorf("no matching trigger authentication for ref `%s`", triggerAuthName)
}

func (r *AuthResolver) resolveAuthParams(ctx context.Context, triggerAuthSpec api.TriggerAuthenticationSpec) (map[string]string, error) {
	result := make(map[string]string)

	if triggerAuthSpec.GCPSecretManager != nil && len(triggerAuthSpec.GCPSecretManager.Secrets) > 0 {
		for _, secret := range triggerAuthSpec.GCPSecretManager.Secrets {
			secretValue, err := r.client.ReadSecret(ctx, secret.ID, secret.Version)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve secret `%s`: %w", secret.ID, err)
			}
			result[secret.Parameter] = secretValue
		}
	}

	return result, nil
}
