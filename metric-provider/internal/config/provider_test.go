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

package config

import (
	"context"
	"crema/metric-provider/api"
	"crema/metric-provider/internal/clients"
	"errors"
	"testing"

	"crema/metric-provider/internal/logging"

	autoscalingv2 "k8s.io/api/autoscaling/v2"

	parametermanagerpb "cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
)

// Asserter is a function that performs assertions on the result of GetCremaConfig.
type Asserter func(t *testing.T, config api.CremaConfig, stubClient *clients.StubParameterManagerClient, err error)

func TestGetCremaConfig(t *testing.T) {
	const defaultTestParameterVersionName = "projects/p/parameters/p/versions/v"

	tests := []struct {
		name                     string
		parameterVersionName     string
		configString             string
		wantErr                  bool
		expectedConfigAssertions Asserter // Custom assertion logic for the config
	}{
		{
			name:                 "Returns error for invalid YAML",
			parameterVersionName: defaultTestParameterVersionName,
			configString:         `foo`,
			wantErr:              true,
		},
		{
			name:                 "Successfully retrieves config with default values",
			parameterVersionName: defaultTestParameterVersionName,
			configString: `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: projects/my-project/locations/us-central1/workerPools/my-worker-pool
        triggers:
          - type: foo
            metadata:
              bar: baz
              qux: 25
        advanced:
          horizontalPodAutoscalerConfig:
            behavior:
              scaleDown:
                stabilizationWindowSeconds: 90
                policies:
                  - type: Pods
                    value: 2
                    periodSeconds: 30
              scaleUp:
                stabilizationWindowSeconds: 90
                policies:
                  - type: Percent
                    value: 100
                    periodSeconds: 30
    - spec:
        scaleTargetRef:
          name: projects/my-project/locations/us-central1/services/my-service
        triggers:
          - type: quux
            metadata:
              corge: grault
              garply: 3
`,
			wantErr: false,
			expectedConfigAssertions: func(t *testing.T, config api.CremaConfig, _ *clients.StubParameterManagerClient, err error) {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if len(config.Spec.ScaledObjects) != 2 {
					t.Fatalf("unexpected number of scaled objects: got %v, want %v", len(config.Spec.ScaledObjects), 2)
				}
				// First scaled object
				scaledObjectSpec1 := config.Spec.ScaledObjects[0].Spec
				if scaledObjectSpec1.ScaleTargetRef.Name != "projects/my-project/locations/us-central1/workerPools/my-worker-pool" {
					t.Errorf("unexpected scaleTargetRef name: got %v, want %v", scaledObjectSpec1.ScaleTargetRef.Name, "projects/my-project/locations/us-central1/workerPools/my-worker-pool")
				}
				if len(scaledObjectSpec1.Triggers) != 1 {
					t.Errorf("unexpected number of triggers: got %v, want %v", len(scaledObjectSpec1.Triggers), 1)
				}

				// Second scaled object
				scaledObjectSpec2 := config.Spec.ScaledObjects[1].Spec
				if scaledObjectSpec2.ScaleTargetRef.Name != "projects/my-project/locations/us-central1/services/my-service" {
					t.Errorf("unexpected scaleTargetRef name: got %v, want %v", scaledObjectSpec2.ScaleTargetRef.Name, "projects/my-project/locations/us-central1/services/my-service")
				}
				if len(scaledObjectSpec2.Triggers) != 1 {
					t.Errorf("unexpected number of triggers: got %v, want %v", len(scaledObjectSpec2.Triggers), 1)
				}
			},
		},
		{
			name:                 "Fails on unrecognized fields",
			parameterVersionName: defaultTestParameterVersionName,
			configString: `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects:
  - spec:
      scaleTargetRef:
        name: projects/my-project/locations/us-central1/services/my-service
      triggers:
      - type: foo
        metadata:
          value: 25
      unknownTriggerField: bar
    unrecognizedSpecField: baz
`,
			wantErr: true,
		},
		{
			name:                 "Retrieves config with trigger authentications",
			parameterVersionName: defaultTestParameterVersionName,
			configString: `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: projects/my-project/locations/us-central1/services/my-service
        triggers:
          - type: foo
            metadata:
              bar: baz
  triggerAuthentications:
    - metadata:
        name: test-trigger-auth-1
      spec:
        gcpSecretManager:
          secrets:
            - parameter: foo
              id: bar
              version: baz
    - metadata:
        name: test-trigger-auth-2
      spec:
        gcpSecretManager:
          secrets:
            - parameter: qux
              id: quz
              version: corge
`,
			wantErr: false,
			expectedConfigAssertions: func(t *testing.T, config api.CremaConfig, _ *clients.StubParameterManagerClient, err error) {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if len(config.Spec.TriggerAuthentications) != 2 {
					t.Fatalf("unexpected number of trigger authentications: got %v, want %v", len(config.Spec.TriggerAuthentications), 2)
				}
				// Assert specific fields for trigger authentications
				ta1 := config.Spec.TriggerAuthentications[0]
				if ta1.Name != "test-trigger-auth-1" {
					t.Errorf("unexpected trigger authentication name: got %v, want %v", ta1.Name, "test-trigger-auth-1")
				}
			},
		},
		{
			name:                 "Calls client with parameter name",
			parameterVersionName: "projects/my-project/parameters/my-param/versions/123",
			configString: `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: projects/my-project/locations/us-central1/services/my-service
        triggers:
          - type: foo
            metadata:
              bar: baz
`,
			wantErr: false,
			expectedConfigAssertions: func(t *testing.T, _ api.CremaConfig, stubClient *clients.StubParameterManagerClient, err error) {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if stubClient.LastRequest == nil {
					t.Fatalf("client was not called")
				}
				if stubClient.LastRequest.Name != "projects/my-project/parameters/my-param/versions/123" {
					t.Errorf("unexpected parameter name: got %v, want %v", stubClient.LastRequest.Name, "projects/my-project/parameters/my-param/versions/123")
				}
			},
		},
		{
			name:                 "Applies default max instances",
			parameterVersionName: defaultTestParameterVersionName,
			configString: `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: projects/my-project/locations/us-central1/workerPools/my-worker-pool
        triggers:
          - type: foo
            metadata:
              bar: baz
    - spec:
        scaleTargetRef:
          name: projects/my-project/locations/us-central1/services/my-service
        triggers:
          - type: foo
            metadata:
              bar: baz
        maxReplicaCount: 5
`,
			wantErr: false,
			expectedConfigAssertions: func(t *testing.T, config api.CremaConfig, _ *clients.StubParameterManagerClient, err error) {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if len(config.Spec.ScaledObjects) != 2 {
					t.Fatalf("unexpected number of scaled objects: got %v, want %v", len(config.Spec.ScaledObjects), 2)
				}
				// First scaled object should have the default max instances
				scaledObjectSpec1 := config.Spec.ScaledObjects[0].Spec
				if scaledObjectSpec1.MaxReplicaCount == nil || *scaledObjectSpec1.MaxReplicaCount != defaultMaxInstances {
					t.Errorf("unexpected MaxReplicaCount: got %v, want %v", scaledObjectSpec1.MaxReplicaCount, defaultMaxInstances)
				}
				// Second scaled object should have the specified max instances
				scaledObjectSpec2 := config.Spec.ScaledObjects[1].Spec
				if scaledObjectSpec2.MaxReplicaCount == nil || *scaledObjectSpec2.MaxReplicaCount != 5 {
					t.Errorf("unexpected MaxReplicaCount: got %v, want %v", scaledObjectSpec2.MaxReplicaCount, 5)
				}
			},
		},
		{
			name:                 "Applies default scaling stabilization",
			parameterVersionName: defaultTestParameterVersionName,
			configString: `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: projects/my-project/locations/us-central1/workerPools/my-worker-pool
        triggers:
          - type: foo
            metadata:
              bar: baz
`,
			wantErr: false,
			expectedConfigAssertions: func(t *testing.T, config api.CremaConfig, _ *clients.StubParameterManagerClient, err error) {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if len(config.Spec.ScaledObjects) != 1 {
					t.Fatalf("unexpected number of scaled objects: got %v, want %v", len(config.Spec.ScaledObjects), 1)
				}

				scaledObjectSpec := config.Spec.ScaledObjects[0].Spec

				// Verify ScaleDown defaults
				if scaledObjectSpec.Advanced == nil ||
					scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig == nil ||
					scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior == nil ||
					scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown == nil {
					t.Fatalf("ScaleDown behavior should not be nil")
				}

				scaleDown := scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown
				if *scaleDown.StabilizationWindowSeconds != defaultScaledownStabilizationWindowSeconds {
					t.Errorf("unexpected ScaleDown StabilizationWindowSeconds: got %v, want %v", *scaleDown.StabilizationWindowSeconds, defaultScaledownStabilizationWindowSeconds)
				}
				if len(scaleDown.Policies) != 1 ||
					scaleDown.Policies[0].Type != autoscalingv2.PercentScalingPolicy ||
					scaleDown.Policies[0].Value != 100 ||
					scaleDown.Policies[0].PeriodSeconds != 15 {
					t.Errorf("unexpected ScaleDown Policies: got %+v, want [{%s 100 15}]", scaleDown.Policies, autoscalingv2.PercentScalingPolicy)
				}
				if *scaleDown.SelectPolicy != autoscalingv2.MinChangePolicySelect {
					t.Errorf("unexpected ScaleDown SelectPolicy: got %v, want %v", *scaleDown.SelectPolicy, autoscalingv2.MinChangePolicySelect)
				}

				// Verify ScaleUp defaults
				if scaledObjectSpec.Advanced == nil ||
					scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig == nil ||
					scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior == nil ||
					scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp == nil {
					t.Fatalf("ScaleUp behavior should not be nil")
				}

				scaleUp := scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp
				if *scaleUp.StabilizationWindowSeconds != 0 {
					t.Errorf("unexpected ScaleUp StabilizationWindowSeconds: got %v, want %v", *scaleUp.StabilizationWindowSeconds, 0)
				}
				if len(scaleUp.Policies) != 2 ||
					scaleUp.Policies[0].Type != autoscalingv2.PercentScalingPolicy || scaleUp.Policies[0].Value != 100 || scaleUp.Policies[0].PeriodSeconds != 15 ||
					scaleUp.Policies[1].Type != autoscalingv2.PodsScalingPolicy || scaleUp.Policies[1].Value != 4 || scaleUp.Policies[1].PeriodSeconds != 15 {
					t.Errorf("unexpected ScaleUp Policies: got %+v, want [{%s 100 15} {%s 4 15}]", scaleUp.Policies, autoscalingv2.PercentScalingPolicy, autoscalingv2.PodsScalingPolicy)
				}
				if *scaleUp.SelectPolicy != autoscalingv2.MaxChangePolicySelect {
					t.Errorf("unexpected ScaleUp SelectPolicy: got %v, want %v", *scaleUp.SelectPolicy, autoscalingv2.MaxChangePolicySelect)
				}
			},
		},
		{
			name:                 "Applies default scaledown when only scaleup is provided",
			parameterVersionName: defaultTestParameterVersionName,
			configString: `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: projects/my-project/locations/us-central1/workerPools/my-worker-pool
        triggers:
          - type: foo
            metadata:
              bar: baz
        advanced:
          horizontalPodAutoscalerConfig:
            behavior:
              scaleUp:
                stabilizationWindowSeconds: 60
                policies:
                  - type: Percent
                    value: 50
                    periodSeconds: 20
`,
			wantErr: false,
			expectedConfigAssertions: func(t *testing.T, config api.CremaConfig, _ *clients.StubParameterManagerClient, err error) {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if len(config.Spec.ScaledObjects) != 1 {
					t.Fatalf("unexpected number of scaled objects: got %v, want %v", len(config.Spec.ScaledObjects), 1)
				}

				scaledObjectSpec := config.Spec.ScaledObjects[0].Spec

				// Verify ScaleDown defaults are applied
				if scaledObjectSpec.Advanced == nil ||
					scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig == nil ||
					scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior == nil ||
					scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown == nil {
					t.Fatalf("ScaleDown behavior should not be nil")
				}

				scaleDown := scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown
				if *scaleDown.StabilizationWindowSeconds != defaultScaledownStabilizationWindowSeconds {
					t.Errorf("unexpected ScaleDown StabilizationWindowSeconds: got %v, want %v", *scaleDown.StabilizationWindowSeconds, defaultScaledownStabilizationWindowSeconds)
				}
				if len(scaleDown.Policies) != 1 ||
					scaleDown.Policies[0].Type != autoscalingv2.PercentScalingPolicy ||
					scaleDown.Policies[0].Value != 100 ||
					scaleDown.Policies[0].PeriodSeconds != 15 {
					t.Errorf("unexpected ScaleDown Policies: got %+v, want [{%s 100 15}]", scaleDown.Policies, autoscalingv2.PercentScalingPolicy)
				}

				// Verify ScaleUp provided values are preserved
				scaleUp := scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp
				if *scaleUp.StabilizationWindowSeconds != 60 {
					t.Errorf("unexpected ScaleUp StabilizationWindowSeconds: got %v, want %v", *scaleUp.StabilizationWindowSeconds, 60)
				}
				if len(scaleUp.Policies) != 1 ||
					scaleUp.Policies[0].Type != autoscalingv2.PercentScalingPolicy ||
					scaleUp.Policies[0].Value != 50 ||
					scaleUp.Policies[0].PeriodSeconds != 20 {
					t.Errorf("unexpected ScaleUp Policies: got %+v, want [{%s 50 20}]", scaleUp.Policies, autoscalingv2.PercentScalingPolicy)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup stub client
			stubClient := &clients.StubParameterManagerClient{
				ParamVersion: &parametermanagerpb.ParameterVersion{
					Payload: &parametermanagerpb.ParameterVersionPayload{
						Data: []byte(tt.configString),
					},
				},
			}

			// Create ConfigProvider
			logger := logging.NewLogger()
			configProvider := NewProvider(stubClient, tt.parameterVersionName, &logger)

			// Get Crema Config
			config, err := configProvider.GetCremaConfig(context.Background())

			// Assert error if expected
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCremaConfig() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Perform custom assertions
			if tt.expectedConfigAssertions != nil {
				t.Run("custom_assertions", func(t *testing.T) {
					t.Helper()
					t.Log("Running custom assertions for", tt.name)
					tt.expectedConfigAssertions(t, config, stubClient, err)
				})
			}
		})
	}
}

func TestGetCremaConfig_ClientError(t *testing.T) {
	const parameterVersionName = "projects/p/parameters/p/versions/v"
	clientErr := errors.New("client error")

	// Setup stub client
	stubClient := &clients.StubParameterManagerClient{
		Err: clientErr,
	}

	// Create ConfigProvider
	logger := logging.NewLogger()
	configProvider := NewProvider(stubClient, parameterVersionName, &logger)

	// Get Crema Config
	_, err := configProvider.GetCremaConfig(context.Background())

	// Assert error
	if err == nil {
		t.Error("expected an error, got nil")
	}
	if !errors.Is(err, clientErr) && err.Error() != "failed to get parameter version "+parameterVersionName+": client error" {
		t.Errorf("expected error to wrap %v, got %v", clientErr, err)
	}
}
