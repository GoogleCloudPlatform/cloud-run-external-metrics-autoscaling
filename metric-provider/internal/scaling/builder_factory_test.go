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
	"crema/metric-provider/api"
	"testing"
	"time"

	"crema/metric-provider/internal/resolvers"
	"crema/metric-provider/internal/clients"
	"crema/metric-provider/internal/logging"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuilderFactory_MakeBuilders(t *testing.T) {
	t.Run("successful scaler creation", func(t *testing.T) {
		logger := logging.NewLogger()
		secretClient := &clients.StubSecretManagerClient{
			ProjectID: "test-project",
		}
		authResolver := resolvers.NewAuthResolver(secretClient)
		factory := NewBuilderFactory(authResolver, 1*time.Second, &logger)
		scaledObject := &kedav1alpha1.ScaledObject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-scaledobject",
				Namespace: "test-namespace",
			},
			Spec: kedav1alpha1.ScaledObjectSpec{
				ScaleTargetRef: &kedav1alpha1.ScaleTarget{
					Name: "test-deployment",
				},
				Triggers: []kedav1alpha1.ScaleTriggers{
					{
						Type: "external",
						Metadata: map[string]string{
							"scalerAddress": "localhost:9090",
						},
					},
				},
			},
		}

		builders, err := factory.MakeBuilders(context.Background(), scaledObject, nil, false)
		require.NoError(t, err)
		require.NotNil(t, builders)
		require.Len(t, builders, 1)

		scaler := builders[0].Scaler
		assert.NotNil(t, scaler)
	})

	t.Run("unknown scaler type", func(t *testing.T) {
		logger := logging.NewLogger()
		secretClient := &clients.StubSecretManagerClient{
			ProjectID: "test-project",
		}
		authResolver := resolvers.NewAuthResolver(secretClient)
		factory := NewBuilderFactory(authResolver, 1*time.Second, &logger)
		scaledObjectWithUnknownScaler := &kedav1alpha1.ScaledObject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-scaledobject-unknown",
				Namespace: "test-namespace",
			},
			Spec: kedav1alpha1.ScaledObjectSpec{
				ScaleTargetRef: &kedav1alpha1.ScaleTarget{
					Name: "test-deployment",
				},
				Triggers: []kedav1alpha1.ScaleTriggers{
					{
						Type: "unknown-scaler",
					},
				},
			},
		}

		builders, err := factory.MakeBuilders(context.Background(), scaledObjectWithUnknownScaler, nil, false)
		assert.Error(t, err)
		assert.NotNil(t, builders)
		assert.Len(t, builders, 0)
	})

	t.Run("successful scaler creation with auth", func(t *testing.T) {
		logger := logging.NewLogger()
		secretClient := &clients.StubSecretManagerClient{
			Secrets: map[string]string{
				"projects/test-project/secrets/test-secret/versions/latest": "test-secret-value",
			},
			ProjectID: "test-project",
		}
		authResolver := resolvers.NewAuthResolver(secretClient)
		factory := NewBuilderFactory(authResolver, 1*time.Second, &logger)
		scaledObject := &kedav1alpha1.ScaledObject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-scaledobject",
				Namespace: "test-namespace",
			},
			Spec: kedav1alpha1.ScaledObjectSpec{
				ScaleTargetRef: &kedav1alpha1.ScaleTarget{
					Name: "test-deployment",
				},
				Triggers: []kedav1alpha1.ScaleTriggers{
					{
						Type: "external",
						Metadata: map[string]string{
							"scalerAddress": "localhost:9090",
						},
						AuthenticationRef: &kedav1alpha1.AuthenticationRef{
							Name: "test-auth",
						},
					},
				},
			},
		}
		triggerAuths := []api.TriggerAuthentication{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-auth",
				},
				Spec: api.TriggerAuthenticationSpec{
					GCPSecretManager: &kedav1alpha1.GCPSecretManager{
						Secrets: []kedav1alpha1.GCPSecretManagerSecret{
							{
								Parameter: "test-param",
								ID:        "test-secret",
								Version:   "latest",
							},
						},
					},
				},
			},
		}

		builders, err := factory.MakeBuilders(context.Background(), scaledObject, triggerAuths, false)
		require.NoError(t, err)
		require.NotNil(t, builders)
		require.Len(t, builders, 1)

		scaler := builders[0].Scaler
		assert.NotNil(t, scaler)
		assert.Equal(t, "test-secret-value", builders[0].ScalerConfig.AuthParams["test-param"])
	})
}
