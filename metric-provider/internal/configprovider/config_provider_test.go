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

package configprovider

import (
	"context"
	"crema/metric-provider/api"
	"crema/metric-provider/internal/clients"
	"errors"
	"testing"

	"crema/metric-provider/internal/logging"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"

	parametermanagerpb "cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
)

func TestGetCremaConfig(t *testing.T) {
	configString := `
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
`
	paramVersion := &parametermanagerpb.ParameterVersion{
		Payload: &parametermanagerpb.ParameterVersionPayload{
			Data: []byte(configString),
		},
	}

	stubClient := &clients.StubParameterManagerClient{
		ParamVersion: paramVersion,
		Err:          nil,
	}

	logger := logging.NewLogger()
	configProvider := New(stubClient, &logger)

	config, err := configProvider.GetCremaConfig(context.Background(), "test-parameter")
	if err != nil {
		t.Errorf("GetCremaConfig() error = %v, wantErr %v", err, false)
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
	trigger1 := scaledObjectSpec1.Triggers[0]
	if trigger1.Type != "foo" {
		t.Errorf("unexpected trigger type: got %v, want %v", trigger1.Type, "foo")
	}
	metadata1 := trigger1.Metadata
	if metadata1["bar"] != "baz" {
		t.Errorf("unexpected bar: got %v, want %v", metadata1["bar"], "baz")
	}
	if metadata1["qux"] != "25" {
		t.Errorf("unexpected qux: got %v, want %v", metadata1["qux"], "25")
	}

	advancedConfig := scaledObjectSpec1.Advanced
	if advancedConfig == nil {
		t.Fatalf("advanced config is nil")
	}
	hpaConfig := advancedConfig.HorizontalPodAutoscalerConfig
	if hpaConfig == nil {
		t.Fatalf("hpa config is nil")
	}
	behavior := hpaConfig.Behavior
	if behavior == nil {
		t.Fatalf("behavior is nil")
	}

	scaleDown := behavior.ScaleDown
	if scaleDown == nil {
		t.Fatalf("scaleDown is nil")
	}
	if *scaleDown.StabilizationWindowSeconds != 90 {
		t.Errorf("unexpected scaleDown stabilizationWindowSeconds: got %v, want %v", *scaleDown.StabilizationWindowSeconds, 90)
	}
	if len(scaleDown.Policies) != 1 {
		t.Fatalf("unexpected number of scaleDown policies: got %v, want %v", len(scaleDown.Policies), 1)
	}
	scaleDownPolicy := scaleDown.Policies[0]
	if scaleDownPolicy.Type != "Pods" {
		t.Errorf("unexpected scaleDown policy type: got %v, want %v", scaleDownPolicy.Type, "Pods")
	}
	if scaleDownPolicy.Value != 2 {
		t.Errorf("unexpected scaleDown policy value: got %v, want %v", scaleDownPolicy.Value, 2)
	}
	if scaleDownPolicy.PeriodSeconds != 30 {
		t.Errorf("unexpected scaleDown policy periodSeconds: got %v, want %v", scaleDownPolicy.PeriodSeconds, 30)
	}

	scaleUp := behavior.ScaleUp
	if scaleUp == nil {
		t.Fatalf("scaleUp is nil")
	}
	if *scaleUp.StabilizationWindowSeconds != 90 {
		t.Errorf("unexpected scaleUp stabilizationWindowSeconds: got %v, want %v", *scaleUp.StabilizationWindowSeconds, 90)
	}
	if len(scaleUp.Policies) != 1 {
		t.Fatalf("unexpected number of scaleUp policies: got %v, want %v", len(scaleUp.Policies), 1)
	}
	scaleUpPolicy := scaleUp.Policies[0]
	if scaleUpPolicy.Type != "Percent" {
		t.Errorf("unexpected scaleUp policy type: got %v, want %v", scaleUpPolicy.Type, "Percent")
	}
	if scaleUpPolicy.Value != 100 {
		t.Errorf("unexpected scaleUp policy value: got %v, want %v", scaleUpPolicy.Value, 100)
	}
	if scaleUpPolicy.PeriodSeconds != 30 {
		t.Errorf("unexpected scaleUp policy periodSeconds: got %v, want %v", scaleUpPolicy.PeriodSeconds, 30)
	}

	// Second scaled object
	scaledObjectSpec2 := config.Spec.ScaledObjects[1].Spec
	if scaledObjectSpec2.ScaleTargetRef.Name != "projects/my-project/locations/us-central1/services/my-service" {
		t.Errorf("unexpected scaleTargetRef name: got %v, want %v", scaledObjectSpec2.ScaleTargetRef.Name, "projects/my-project/locations/us-central1/services/my-service")
	}
	if len(scaledObjectSpec2.Triggers) != 1 {
		t.Errorf("unexpected number of triggers: got %v, want %v", len(scaledObjectSpec2.Triggers), 1)
	}
	trigger2 := scaledObjectSpec2.Triggers[0]
	if trigger2.Type != "quux" {
		t.Errorf("unexpected trigger type: got %v, want %v", trigger2.Type, "quux")
	}
	metadata2 := trigger2.Metadata
	if metadata2["corge"] != "grault" {
		t.Errorf("unexpected corge: got %v, want %v", metadata2["corge"], "grault")
	}
	if metadata2["garply"] != "3" {
		t.Errorf("unexpected garply: got %v, want %v", metadata2["garply"], "3")
	}
}

func TestGetCremaConfig_IgnoresUnrecognizedFields(t *testing.T) {
	configString := `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects:
  - spec:
      triggers:
      - type: foo
        metadata:
          bar: baz
          qux: 25
      unknownTriggerField: shouldBeIgnored
    unrecognizedSpecField: shouldBeIgnored
`
	paramVersion := &parametermanagerpb.ParameterVersion{
		Payload: &parametermanagerpb.ParameterVersionPayload{
			Data: []byte(configString),
		},
	}

	stubClient := &clients.StubParameterManagerClient{
		ParamVersion: paramVersion,
		Err:          nil,
	}

	logger := logging.NewLogger()
	configProvider := New(stubClient, &logger)

	_, err := configProvider.GetCremaConfig(context.Background(), "test-parameter")
	if err != nil {
		t.Errorf("GetCremaConfig() error = %v, wantErr %v", err, nil)
	}
}

func TestGetCremaConfig_ReturnsErrorForInvalidYaml(t *testing.T) {
	configString := `foo`
	paramVersion := &parametermanagerpb.ParameterVersion{
		Payload: &parametermanagerpb.ParameterVersionPayload{
			Data: []byte(configString),
		},
	}

	stubClient := &clients.StubParameterManagerClient{
		ParamVersion: paramVersion,
		Err:          nil,
	}

	logger := logging.NewLogger()
	configProvider := New(stubClient, &logger)

	_, err := configProvider.GetCremaConfig(context.Background(), "test-parameter")
	if err == nil { // We expect an error.
		t.Errorf("GetCremaConfig() expected an error, got nil")
	}
}

func TestGetCremaConfig_ReturnsErrorOnClientError(t *testing.T) {
	expectedErr := errors.New("client error")
	stubClient := &clients.StubParameterManagerClient{
		ParamVersion: nil,
		Err:          expectedErr,
	}

	logger := logging.NewLogger()
	configProvider := New(stubClient, &logger)

	_, err := configProvider.GetCremaConfig(context.Background(), "test-parameter")
	if err == nil {
		t.Errorf("GetCremaConfig() expected an error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("GetCremaConfig() error = %v, want %v", err, expectedErr)
	}
}

func TestGetCremaConfig_WithTriggerAuthentications(t *testing.T) {
	configString := `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects: []
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
`
	paramVersion := &parametermanagerpb.ParameterVersion{
		Payload: &parametermanagerpb.ParameterVersionPayload{
			Data: []byte(configString),
		},
	}

	stubClient := &clients.StubParameterManagerClient{
		ParamVersion: paramVersion,
		Err:          nil,
	}

	logger := logging.NewLogger()
	configProvider := New(stubClient, &logger)

	config, err := configProvider.GetCremaConfig(context.Background(), "test-parameter")
	if err != nil {
		t.Errorf("GetCremaConfig() error = %v, wantErr %v", err, false)
	}

	if len(config.Spec.TriggerAuthentications) != 2 {
		t.Fatalf("unexpected number of trigger authentications: got %v, want %v", len(config.Spec.TriggerAuthentications), 2)
	}

	// First TriggerAuthentication
	ta1 := config.Spec.TriggerAuthentications[0]
	if ta1.Name != "test-trigger-auth-1" {
		t.Errorf("unexpected trigger authentication name: got %v, want %v", ta1.Name, "test-trigger-auth-1")
	}
	if ta1.Spec.GCPSecretManager.Secrets[0].Parameter != "foo" {
		t.Errorf("unexpected secret parameter: got %v, want %v", ta1.Spec.GCPSecretManager.Secrets[0].Parameter, "foo")
	}

	// Second TriggerAuthentication
	ta2 := config.Spec.TriggerAuthentications[1]
	if ta2.Name != "test-trigger-auth-2" {
		t.Errorf("unexpected trigger authentication name: got %v, want %v", ta2.Name, "test-trigger-auth-2")
	}
	if ta2.Spec.GCPSecretManager.Secrets[0].Parameter != "qux" {
		t.Errorf("unexpected secret parameter: got %v, want %v", ta2.Spec.GCPSecretManager.Secrets[0].Parameter, "qux")
	}

}

func TestGetCremaConfig_CallsClientWithParameterName(t *testing.T) {
	stubClient := &clients.StubParameterManagerClient{
		ParamVersion: &parametermanagerpb.ParameterVersion{
			Payload: &parametermanagerpb.ParameterVersionPayload{
				Data: []byte(""),
			},
		},
	}
	logger := logging.NewLogger()
	configProvider := New(stubClient, &logger)

	parameterName := "my-test-parameter"
	configProvider.GetCremaConfig(context.Background(), parameterName)

	if stubClient.LastRequest == nil {
		t.Fatalf("client was not called")
	}
	if stubClient.LastRequest.Name != parameterName {
		t.Errorf("unexpected parameter name: got %v, want %v", stubClient.LastRequest.Name, parameterName)
	}
}

func TestGetCremaConfig_DefaultMaxInstances(t *testing.T) {
	configString := `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: projects/my-project/locations/us-central1/workerPools/my-worker-pool
    - spec:
        scaleTargetRef:
          name: projects/my-project/locations/us-central1/services/my-service
        maxReplicaCount: 5
`
	paramVersion := &parametermanagerpb.ParameterVersion{
		Payload: &parametermanagerpb.ParameterVersionPayload{
			Data: []byte(configString),
		},
	}

	stubClient := &clients.StubParameterManagerClient{
		ParamVersion: paramVersion,
		Err:          nil,
	}

	logger := logging.NewLogger()
	configProvider := New(stubClient, &logger)

	config, err := configProvider.GetCremaConfig(context.Background(), "test-parameter")
	if err != nil {
		t.Errorf("GetCremaConfig() error = %v, wantErr %v", err, false)
	}

	if len(config.Spec.ScaledObjects) != 2 {
		t.Fatalf("unexpected number of scaled objects: got %v, want %v", len(config.Spec.ScaledObjects), 2)
	}

	// First scaled object should have the default max instances
	scaledObjectSpec1 := config.Spec.ScaledObjects[0].Spec
	if scaledObjectSpec1.MaxReplicaCount == nil {
		t.Errorf("MaxReplicaCount should not be nil")
	} else if *scaledObjectSpec1.MaxReplicaCount != 100 {
		t.Errorf("unexpected MaxReplicaCount: got %v, want %v", *scaledObjectSpec1.MaxReplicaCount, 100)
	}

	// Second scaled object should have the specified max instances
	scaledObjectSpec2 := config.Spec.ScaledObjects[1].Spec
	if scaledObjectSpec2.MaxReplicaCount == nil {
		t.Errorf("MaxReplicaCount should not be nil")
	} else if *scaledObjectSpec2.MaxReplicaCount != 5 {
		t.Errorf("unexpected MaxReplicaCount: got %v, want %v", *scaledObjectSpec2.MaxReplicaCount, 5)
	}
}

func TestGetCremaConfig_DefaultScalingStabilization(t *testing.T) {
	configString := `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: projects/my-project/locations/us-central1/workerPools/my-worker-pool
`
	paramVersion := &parametermanagerpb.ParameterVersion{
		Payload: &parametermanagerpb.ParameterVersionPayload{
			Data: []byte(configString),
		},
	}

	stubClient := &clients.StubParameterManagerClient{
		ParamVersion: paramVersion,
		Err:          nil,
	}

	logger := logging.NewLogger()
	configProvider := New(stubClient, &logger)

	config, err := configProvider.GetCremaConfig(context.Background(), "test-parameter")
	if err != nil {
		t.Errorf("GetCremaConfig() error = %v, wantErr %v", err, false)
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
	if *scaleDown.StabilizationWindowSeconds != 300 {
		t.Errorf("unexpected ScaleDown StabilizationWindowSeconds: got %v, want %v", *scaleDown.StabilizationWindowSeconds, 300)
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
}

func TestGetCremaConfig_DefaultScalingStabilization_PartialConfig(t *testing.T) {
	tests := []struct {
		name                           string
		configString                   string
		expectedScaleDownWindowSeconds int32
		expectedScaleDownPolicies      []autoscalingv2.HPAScalingPolicy
		expectedScaleDownSelectPolicy  *autoscalingv2.ScalingPolicySelect
		expectedScaleUpWindowSeconds   int32
		expectedScaleUpPolicies        []autoscalingv2.HPAScalingPolicy
		expectedScaleUpSelectPolicy    *autoscalingv2.ScalingPolicySelect
	}{
		{
			name: "Only ScaleDown policies provided, ScaleUp should have defaults",
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
        advanced:
          horizontalPodAutoscalerConfig:
            behavior:
              scaleDown:
                stabilizationWindowSeconds: 60
                policies:
                  - type: Pods
                    value: 1
                    periodSeconds: 10
`,
			expectedScaleDownWindowSeconds: 60,
			expectedScaleDownPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PodsScalingPolicy, Value: 1, PeriodSeconds: 10},
			},
			expectedScaleDownSelectPolicy: func() *autoscalingv2.ScalingPolicySelect { s := autoscalingv2.MinChangePolicySelect; return &s }(),
			expectedScaleUpWindowSeconds:  0, // Default
			expectedScaleUpPolicies: []autoscalingv2.HPAScalingPolicy{ // Default
				{Type: autoscalingv2.PercentScalingPolicy, Value: 100, PeriodSeconds: 15},
				{Type: autoscalingv2.PodsScalingPolicy, Value: 4, PeriodSeconds: 15},
			},
			expectedScaleUpSelectPolicy: func() *autoscalingv2.ScalingPolicySelect { s := autoscalingv2.MaxChangePolicySelect; return &s }(),
		},
		{
			name: "Only ScaleUp policies provided, ScaleDown should have defaults",
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
        advanced:
          horizontalPodAutoscalerConfig:
            behavior:
              scaleUp:
                stabilizationWindowSeconds: 30
                policies:
                  - type: Pods
                    value: 2
                    periodSeconds: 5
`,
			expectedScaleDownWindowSeconds: 300, // Default
			expectedScaleDownPolicies: []autoscalingv2.HPAScalingPolicy{ // Default
				{Type: autoscalingv2.PercentScalingPolicy, Value: 100, PeriodSeconds: 15},
			},
			expectedScaleDownSelectPolicy: func() *autoscalingv2.ScalingPolicySelect { s := autoscalingv2.MinChangePolicySelect; return &s }(),
			expectedScaleUpWindowSeconds:  30,
			expectedScaleUpPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PodsScalingPolicy, Value: 2, PeriodSeconds: 5},
			},
			expectedScaleUpSelectPolicy: func() *autoscalingv2.ScalingPolicySelect { s := autoscalingv2.MaxChangePolicySelect; return &s }(),
		},
		{
			name: "Both ScaleDown and ScaleUp policies provided, no defaults should be applied",
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
        advanced:
          horizontalPodAutoscalerConfig:
            behavior:
              scaleDown:
                stabilizationWindowSeconds: 120
                policies:
                  - type: Pods
                    value: 3
                    periodSeconds: 60
              scaleUp:
                stabilizationWindowSeconds: 45
                policies:
                  - type: Percent
                    value: 50
                    periodSeconds: 10
`,
			expectedScaleDownWindowSeconds: 120,
			expectedScaleDownPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PodsScalingPolicy, Value: 3, PeriodSeconds: 60},
			},
			expectedScaleDownSelectPolicy: func() *autoscalingv2.ScalingPolicySelect { s := autoscalingv2.MinChangePolicySelect; return &s }(),
			expectedScaleUpWindowSeconds:  45,
			expectedScaleUpPolicies: []autoscalingv2.HPAScalingPolicy{
				{Type: autoscalingv2.PercentScalingPolicy, Value: 50, PeriodSeconds: 10},
			},
			expectedScaleUpSelectPolicy: func() *autoscalingv2.ScalingPolicySelect { s := autoscalingv2.MaxChangePolicySelect; return &s }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paramVersion := &parametermanagerpb.ParameterVersion{
				Payload: &parametermanagerpb.ParameterVersionPayload{
					Data: []byte(tt.configString),
				},
			}

			stubClient := &clients.StubParameterManagerClient{
				ParamVersion: paramVersion,
				Err:          nil,
			}

			logger := logging.NewLogger()
			configProvider := New(stubClient, &logger)

			config, err := configProvider.GetCremaConfig(context.Background(), "test-parameter")
			if err != nil {
				t.Errorf("GetCremaConfig() error = %v, wantErr %v", err, false)
				return
			}

			if len(config.Spec.ScaledObjects) != 1 {
				t.Fatalf("unexpected number of scaled objects: got %v, want %v", len(config.Spec.ScaledObjects), 1)
			}

			scaledObjectSpec := config.Spec.ScaledObjects[0].Spec

			// Verify ScaleDown
			if scaledObjectSpec.Advanced == nil ||
				scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig == nil ||
				scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior == nil ||
				scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown == nil {
				t.Fatalf("ScaleDown behavior should not be nil")
			}

			scaleDown := scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown
			if *scaleDown.StabilizationWindowSeconds != tt.expectedScaleDownWindowSeconds {
				t.Errorf("unexpected ScaleDown StabilizationWindowSeconds: got %v, want %v", *scaleDown.StabilizationWindowSeconds, tt.expectedScaleDownWindowSeconds)
			}
			if len(scaleDown.Policies) != len(tt.expectedScaleDownPolicies) {
				t.Fatalf("unexpected number of ScaleDown Policies: got %v, want %v", len(scaleDown.Policies), len(tt.expectedScaleDownPolicies))
			}
			for i, policy := range scaleDown.Policies {
				if policy.Type != tt.expectedScaleDownPolicies[i].Type ||
					policy.Value != tt.expectedScaleDownPolicies[i].Value ||
					policy.PeriodSeconds != tt.expectedScaleDownPolicies[i].PeriodSeconds {
					t.Errorf("unexpected ScaleDown Policy at index %d: got %+v, want %+v", i, policy, tt.expectedScaleDownPolicies[i])
				}
			}
			if scaleDown.SelectPolicy == nil || *scaleDown.SelectPolicy != *tt.expectedScaleDownSelectPolicy {
				t.Errorf("unexpected ScaleDown SelectPolicy: got %v, want %v", scaleDown.SelectPolicy, tt.expectedScaleDownSelectPolicy)
			}

			// Verify ScaleUp
			if scaledObjectSpec.Advanced == nil ||
				scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig == nil ||
				scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior == nil ||
				scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp == nil {
				t.Fatalf("ScaleUp behavior should not be nil")
			}

			scaleUp := scaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp
			if *scaleUp.StabilizationWindowSeconds != tt.expectedScaleUpWindowSeconds {
				t.Errorf("unexpected ScaleUp StabilizationWindowSeconds: got %v, want %v", *scaleUp.StabilizationWindowSeconds, tt.expectedScaleUpWindowSeconds)
			}
			if len(scaleUp.Policies) != len(tt.expectedScaleUpPolicies) {
				t.Fatalf("unexpected number of ScaleUp Policies: got %v, want %v", len(scaleUp.Policies), len(tt.expectedScaleUpPolicies))
			}
			for i, policy := range scaleUp.Policies {
				if policy.Type != tt.expectedScaleUpPolicies[i].Type ||
					policy.Value != tt.expectedScaleUpPolicies[i].Value ||
					policy.PeriodSeconds != tt.expectedScaleUpPolicies[i].PeriodSeconds {
					t.Errorf("unexpected ScaleUp Policy at index %d: got %+v, want %+v", i, policy, tt.expectedScaleUpPolicies[i])
				}
			}
			if scaleUp.SelectPolicy == nil || *scaleUp.SelectPolicy != *tt.expectedScaleUpSelectPolicy {
				t.Errorf("unexpected ScaleUp SelectPolicy: got %v, want %v", scaleUp.SelectPolicy, tt.expectedScaleUpSelectPolicy)
			}
		})
	}
}

func TestGetCremaConfig_NilScaledObjectSpec(t *testing.T) {
	configString := `
apiVersion: crema/v1
kind: CremaConfig
metadata:
  name: test-crema-config
spec:
  scaledObjects:
    - spec: # This spec is explicitly nil
`
	paramVersion := &parametermanagerpb.ParameterVersion{
		Payload: &parametermanagerpb.ParameterVersionPayload{
			Data: []byte(configString),
		},
	}

	stubClient := &clients.StubParameterManagerClient{
		ParamVersion: paramVersion,
		Err:          nil,
	}

	logger := logging.NewLogger()
	configProvider := New(stubClient, &logger)

	config, err := configProvider.GetCremaConfig(context.Background(), "test-parameter")
	if err != nil {
		t.Errorf("GetCremaConfig() error = %v, wantErr %v", err, false)
	}

	if len(config.Spec.ScaledObjects) != 1 {
		t.Fatalf("unexpected number of scaled objects: got %v, want %v", len(config.Spec.ScaledObjects), 1)
	}

	scaledObject := config.Spec.ScaledObjects[0]
	if scaledObject.Spec.MaxReplicaCount == nil || *scaledObject.Spec.MaxReplicaCount != 100 {
		t.Errorf("unexpected MaxReplicaCount for nil spec: got %v, want %v", scaledObject.Spec.MaxReplicaCount, 100)
	}
}

func TestGetWithDefaultMaxInstances(t *testing.T) {
	// a scaled object with maxReplicaCount set
	soWithMaxReplica := api.CremaScaledObject{
		Spec: kedav1alpha1.ScaledObjectSpec{
			MaxReplicaCount: func(i int32) *int32 { return &i }(5),
		},
	}
	// a scaled object without maxReplicaCount set
	soWithoutMaxReplica := api.CremaScaledObject{
		Spec: kedav1alpha1.ScaledObjectSpec{},
	}

	config := api.CremaConfig{
		Spec: api.CremaConfigSpec{
			ScaledObjects: []api.CremaScaledObject{soWithMaxReplica, soWithoutMaxReplica},
		},
	}

	updatedConfig := getWithDefaultMaxInstances(config)

	// Check first scaled object, it should be unchanged
	if *updatedConfig.Spec.ScaledObjects[0].Spec.MaxReplicaCount != 5 {
		t.Errorf("Expected MaxReplicaCount to be 5, got %d", *updatedConfig.Spec.ScaledObjects[0].Spec.MaxReplicaCount)
	}

	// Check second scaled object, it should have the default
	if *updatedConfig.Spec.ScaledObjects[1].Spec.MaxReplicaCount != 100 {
		t.Errorf("Expected MaxReplicaCount to be 100, got %d", *updatedConfig.Spec.ScaledObjects[1].Spec.MaxReplicaCount)
	}

	// Test with empty scaled objects
	config = api.CremaConfig{
		Spec: api.CremaConfigSpec{
			ScaledObjects: []api.CremaScaledObject{},
		},
	}
	updatedConfig = getWithDefaultMaxInstances(config)
	if len(updatedConfig.Spec.ScaledObjects) != 0 {
		t.Error("Expected ScaledObjects to be empty")
	}
}

func TestGetWithDefaultScalingPolicies(t *testing.T) {
	// Case 1: Empty behavior
	behavior := autoscalingv2.HorizontalPodAutoscalerBehavior{}
	updatedBehavior := getWithDefaultScalingPolicies(behavior)

	if updatedBehavior.ScaleDown == nil {
		t.Fatal("ScaleDown should not be nil")
	}
	if *updatedBehavior.ScaleDown.StabilizationWindowSeconds != 300 {
		t.Errorf("Expected ScaleDown StabilizationWindowSeconds to be 300, got %d", *updatedBehavior.ScaleDown.StabilizationWindowSeconds)
	}
	if *updatedBehavior.ScaleDown.SelectPolicy != autoscalingv2.MinChangePolicySelect {
		t.Errorf("Expected ScaleDown SelectPolicy to be Min, got %s", *updatedBehavior.ScaleDown.SelectPolicy)
	}

	if updatedBehavior.ScaleUp == nil {
		t.Fatal("ScaleUp should not be nil")
	}
	if *updatedBehavior.ScaleUp.StabilizationWindowSeconds != 0 {
		t.Errorf("Expected ScaleUp StabilizationWindowSeconds to be 0, got %d", *updatedBehavior.ScaleUp.StabilizationWindowSeconds)
	}
	if *updatedBehavior.ScaleUp.SelectPolicy != autoscalingv2.MaxChangePolicySelect {
		t.Errorf("Expected ScaleUp SelectPolicy to be Max, got %s", *updatedBehavior.ScaleUp.SelectPolicy)
	}

	// Case 2: ScaleDown policies exist, but SelectPolicy is nil
	behavior = autoscalingv2.HorizontalPodAutoscalerBehavior{
		ScaleDown: &autoscalingv2.HPAScalingRules{
			Policies: []autoscalingv2.HPAScalingPolicy{
				{Type: "Pods", Value: 1, PeriodSeconds: 10},
			},
		},
	}
	updatedBehavior = getWithDefaultScalingPolicies(behavior)
	if *updatedBehavior.ScaleDown.SelectPolicy != autoscalingv2.MinChangePolicySelect {
		t.Errorf("Expected ScaleDown SelectPolicy to be Min, got %s", *updatedBehavior.ScaleDown.SelectPolicy)
	}
	if updatedBehavior.ScaleUp == nil {
		t.Fatal("ScaleUp should still be defaulted when only ScaleDown is present")
	}

	// Case 3: ScaleUp policies exist, but SelectPolicy is nil
	behavior = autoscalingv2.HorizontalPodAutoscalerBehavior{
		ScaleUp: &autoscalingv2.HPAScalingRules{
			Policies: []autoscalingv2.HPAScalingPolicy{
				{Type: "Pods", Value: 1, PeriodSeconds: 10},
			},
		},
	}
	updatedBehavior = getWithDefaultScalingPolicies(behavior)
	if *updatedBehavior.ScaleUp.SelectPolicy != autoscalingv2.MaxChangePolicySelect {
		t.Errorf("Expected ScaleUp SelectPolicy to be Max, got %s", *updatedBehavior.ScaleUp.SelectPolicy)
	}
	if updatedBehavior.ScaleDown == nil {
		t.Fatal("ScaleDown should still be defaulted when only ScaleUp is present")
	}
}

func TestGetWithDefaultScalingStabilizationForScaledObject(t *testing.T) {
	// Case 1: ScaledObject with no Advanced config
	so := api.CremaScaledObject{
		Spec: kedav1alpha1.ScaledObjectSpec{},
	}
	updatedSO := getWithDefaultScalingStabilizationForScaledObject(so)

	if updatedSO.Spec.Advanced == nil {
		t.Fatal("Advanced should not be nil")
	}
	if updatedSO.Spec.Advanced.HorizontalPodAutoscalerConfig == nil {
		t.Fatal("HorizontalPodAutoscalerConfig should not be nil")
	}
	if updatedSO.Spec.Advanced.HorizontalPodAutoscalerConfig.Behavior == nil {
		t.Fatal("Behavior should not be nil")
	}
	// Check a default value
	if *updatedSO.Spec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown.StabilizationWindowSeconds != 300 {
		t.Error("Default scale down stabilization window not applied")
	}

	// Case 2: ScaledObject with partial Advanced config
	stabilizationWindow := int32(60)
	so = api.CremaScaledObject{
		Spec: kedav1alpha1.ScaledObjectSpec{
			Advanced: &kedav1alpha1.AdvancedConfig{
				HorizontalPodAutoscalerConfig: &kedav1alpha1.HorizontalPodAutoscalerConfig{
					Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
						ScaleDown: &autoscalingv2.HPAScalingRules{
							StabilizationWindowSeconds: &stabilizationWindow,
						},
					},
				},
			},
		},
	}
	updatedSO = getWithDefaultScalingStabilizationForScaledObject(so)
	if *updatedSO.Spec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown.StabilizationWindowSeconds != 60 {
		t.Errorf("Expected StabilizationWindowSeconds to be 60, got %d", *updatedSO.Spec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown.StabilizationWindowSeconds)
	}
	if updatedSO.Spec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp == nil {
		t.Fatal("ScaleUp should have been defaulted")
	}
}

func TestGetWithDefaultScalingStabilization(t *testing.T) {
	so := api.CremaScaledObject{
		Spec: kedav1alpha1.ScaledObjectSpec{},
	}
	config := api.CremaConfig{
		Spec: api.CremaConfigSpec{
			ScaledObjects: []api.CremaScaledObject{so},
		},
	}
	updatedConfig := getWithDefaultScalingStabilization(config)
	if updatedConfig.Spec.ScaledObjects[0].Spec.Advanced == nil {
		t.Fatal("Advanced should not be nil")
	}
}
