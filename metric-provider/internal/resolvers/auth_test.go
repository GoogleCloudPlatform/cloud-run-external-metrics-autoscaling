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
	"crema/metric-provider/internal/clients"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAuthResolver_ResolveAuthRefAndPodIdentity(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name                string
		client              *clients.StubSecretManagerClient
		triggerAuths        []api.TriggerAuthentication
		triggerAuthName     string
		expected            map[string]string
		expectedPodIdentity kedav1alpha1.AuthPodIdentity
		expectedErr         error
	}{
		{
			name:            "nil trigger auths",
			triggerAuths:    nil,
			triggerAuthName: "test-auth",
			expected:        nil,
			expectedPodIdentity: kedav1alpha1.AuthPodIdentity{Provider: kedav1alpha1.PodIdentityProviderNone},
			expectedErr:     errors.New(""),
		},
		{
			name:            "no matching trigger auth",
			triggerAuths:    []api.TriggerAuthentication{},
			triggerAuthName: "test-auth",
			expected:        nil,
			expectedPodIdentity: kedav1alpha1.AuthPodIdentity{Provider: kedav1alpha1.PodIdentityProviderNone},
			expectedErr:     errors.New(""),
		},
		{
			name: "matching trigger auth found",
			client: &clients.StubSecretManagerClient{
				Secrets: map[string]string{
					"projects/test-project/secrets/secret1/versions/latest": "value1",
				},
				ProjectID: "test-project",
			},
			triggerAuths: []api.TriggerAuthentication{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-auth"},
					Spec: api.TriggerAuthenticationSpec{
						GCPSecretManager: &kedav1alpha1.GCPSecretManager{
							Secrets: []kedav1alpha1.GCPSecretManagerSecret{
								{Parameter: "param1", ID: "secret1", Version: "latest"},
							},
						},
					},
				},
			},
			triggerAuthName: "test-auth",
			expected: map[string]string{
				"param1": "value1",
			},
			expectedPodIdentity: kedav1alpha1.AuthPodIdentity{Provider: kedav1alpha1.PodIdentityProviderNone},
			expectedErr: nil,
		},
		{
			name: "matching trigger auth with pod identity",
			client: &clients.StubSecretManagerClient{
				ProjectID: "test-project",
			},
			triggerAuths: []api.TriggerAuthentication{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-auth"},
					Spec: api.TriggerAuthenticationSpec{
						PodIdentity: &api.AuthPodIdentity{
							Provider: api.PodIdentityProviderGCP,
						},
					},
				},
			},
			triggerAuthName: "test-auth",
			expected:        map[string]string{},
			expectedPodIdentity: kedav1alpha1.AuthPodIdentity{Provider: kedav1alpha1.PodIdentityProviderGCP},
			expectedErr:     nil,
		},
		{
			name: "one secret fails to resolve",
			client: &clients.StubSecretManagerClient{
				Secrets: map[string]string{
					"projects/test-project/secrets/secret1/versions/latest": "value1",
				},
				Error:     errors.New(""),
				ProjectID: "test-project",
			},
			triggerAuths: []api.TriggerAuthentication{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-auth"},
					Spec: api.TriggerAuthenticationSpec{
						GCPSecretManager: &kedav1alpha1.GCPSecretManager{
							Secrets: []kedav1alpha1.GCPSecretManagerSecret{
								{Parameter: "param1", ID: "secret1", Version: "latest"},
								{Parameter: "param2", ID: "secret-not-found", Version: "latest"},
							},
						},
					},
				},
			},
			triggerAuthName: "test-auth",
			expected:        nil,
			expectedPodIdentity: kedav1alpha1.AuthPodIdentity{Provider: kedav1alpha1.PodIdentityProviderNone},
			expectedErr:     errors.New(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := NewAuthResolver(tc.client)
			result, podIdentity, err := resolver.ResolveAuthRefAndPodIdentity(ctx, tc.triggerAuths, tc.triggerAuthName)

			if tc.expectedErr != nil {
				if err == nil {
					t.Errorf("expected error '%v', got '%v'", tc.expectedErr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.expectedPodIdentity, podIdentity); diff != "" {
				t.Errorf("unexpected pod identity (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAuthResolver_resolveAuthParams(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name        string
		client      *clients.StubSecretManagerClient
		spec        api.TriggerAuthenticationSpec
		expected    map[string]string
		expectedErr error
	}{
		{
			name:        "no secret manager config",
			spec:        api.TriggerAuthenticationSpec{},
			expected:    map[string]string{},
			expectedErr: nil,
		},
		{
			name: "secrets resolved successfully",
			client: &clients.StubSecretManagerClient{
				Secrets: map[string]string{
					"projects/test-project/secrets/secret1/versions/latest": "value1",
					"projects/test-project/secrets/secret2/versions/1":      "value2",
				},
				ProjectID: "test-project",
			},
			spec: api.TriggerAuthenticationSpec{
				GCPSecretManager: &kedav1alpha1.GCPSecretManager{
					Secrets: []kedav1alpha1.GCPSecretManagerSecret{
						{Parameter: "param1", ID: "secret1", Version: "latest"},
						{Parameter: "param2", ID: "secret2", Version: "1"},
					},
				},
			},
			expected: map[string]string{
				"param1": "value1",
				"param2": "value2",
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := NewAuthResolver(tc.client)
			result, err := resolver.resolveAuthParams(ctx, tc.spec)

			if tc.expectedErr != nil {
				if err == nil {
					t.Errorf("expected error '%v', got '%v'", tc.expectedErr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}
