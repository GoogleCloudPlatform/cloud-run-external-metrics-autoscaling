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
	"fmt"

	parametermanagerpb "cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

const (
	defaultMaxInstances                        = 100
	defaultScaledownStabilizationWindowSeconds = 300
)

type parameterManagerClient interface {
	GetParameterVersion(context.Context, *parametermanagerpb.GetParameterVersionRequest) (*parametermanagerpb.ParameterVersion, error)
}

// ConfigProvider provides access to configuration.
type ConfigProvider struct {
	client parameterManagerClient
	logger *logr.Logger
}

// Create a new ConfigurationProvider instance.
func New(parameterManagerClient parameterManagerClient, logger *logr.Logger) *ConfigProvider {
	return &ConfigProvider{
		client: parameterManagerClient,
		logger: logger,
	}
}

// GetCremaConfig returns CREMA config parsed from the specified parameter version
func (cp *ConfigProvider) GetCremaConfig(ctx context.Context, parameterVersionName string) (api.CremaConfig, error) {
	pv, err := cp.readParameterVersion(ctx, parameterVersionName)
	if err != nil {
		return api.CremaConfig{}, fmt.Errorf("failed to get parameter version %s: %w", parameterVersionName, err)
	}

	cp.logger.Info("Successfully retrieved", "parameter", parameterVersionName, "parameterVersion", pv)

	if pv.Payload == nil || pv.Payload.Data == nil {
		return api.CremaConfig{}, fmt.Errorf("parameter payload or data is nil for parameter version %s", parameterVersionName)
	}

	config, err := unmarshalCremaConfig(pv.Payload.Data)
	if err != nil {
		return api.CremaConfig{}, err
	}

	return applyDefaults(config), nil
}

func (cp *ConfigProvider) readParameterVersion(ctx context.Context, parameterVersionName string) (*parametermanagerpb.ParameterVersion, error) {
	if cp == nil || cp.client == nil {
		return nil, fmt.Errorf("ConfigProvider is not initialized; use New()")
	}

	req := &parametermanagerpb.GetParameterVersionRequest{
		Name: parameterVersionName,
	}

	return cp.client.GetParameterVersion(ctx, req)
}

func unmarshalCremaConfig(data []byte) (api.CremaConfig, error) {
	var config api.CremaConfig
	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return config, fmt.Errorf("failed to unmarshal data to CremaConfig: %w", err)
	}

	return config, nil
}

func applyDefaults(cremaConfig api.CremaConfig) api.CremaConfig {
	cremaConfig = applyDefaultMaxInstances(cremaConfig)
	return applyDefaultScalingStabilization(cremaConfig)
}

func applyDefaultMaxInstances(cremaConfig api.CremaConfig) api.CremaConfig {
	for i := range cremaConfig.Spec.ScaledObjects {
		scaledObject := &cremaConfig.Spec.ScaledObjects[i]
		if scaledObject.Spec.MaxReplicaCount == nil {
			defaultValue := int32(defaultMaxInstances)
			scaledObject.Spec.MaxReplicaCount = &defaultValue
		}
	}
	return cremaConfig
}

func applyDefaultScalingStabilization(cremaConfig api.CremaConfig) api.CremaConfig {
	for i, scaledObject := range cremaConfig.Spec.ScaledObjects {
		cremaConfig.Spec.ScaledObjects[i] = applyDefaultScalingStabilizationForScaledObject(scaledObject)
	}
	return cremaConfig
}

func applyDefaultScalingStabilizationForScaledObject(scaledObject api.CremaScaledObject) api.CremaScaledObject {
	if scaledObject.Spec.Advanced == nil {
		scaledObject.Spec.Advanced = &kedav1alpha1.AdvancedConfig{}
	}
	if scaledObject.Spec.Advanced.HorizontalPodAutoscalerConfig == nil {
		scaledObject.Spec.Advanced.HorizontalPodAutoscalerConfig = &kedav1alpha1.HorizontalPodAutoscalerConfig{}
	}
	if scaledObject.Spec.Advanced.HorizontalPodAutoscalerConfig.Behavior == nil {
		scaledObject.Spec.Advanced.HorizontalPodAutoscalerConfig.Behavior = &autoscalingv2.HorizontalPodAutoscalerBehavior{}
	}

	behavior := applyDefaultScalingPolicies(*scaledObject.Spec.Advanced.HorizontalPodAutoscalerConfig.Behavior)
	scaledObject.Spec.Advanced.HorizontalPodAutoscalerConfig.Behavior = &behavior

	return scaledObject
}

func applyDefaultScalingPolicies(behavior autoscalingv2.HorizontalPodAutoscalerBehavior) autoscalingv2.HorizontalPodAutoscalerBehavior {
	if behavior.ScaleDown == nil {
		scaleDown := &autoscalingv2.HPAScalingRules{}

		defaultValue := int32(defaultScaledownStabilizationWindowSeconds)
		scaleDown.StabilizationWindowSeconds = &defaultValue

		scaleDown.Policies = []autoscalingv2.HPAScalingPolicy{
			{Type: autoscalingv2.PercentScalingPolicy, Value: 100, PeriodSeconds: 15},
		}

		behavior.ScaleDown = scaleDown
	}

	if behavior.ScaleDown.SelectPolicy == nil {
		// If ScaleDown exists but SelectPolicy is not set, apply the default SelectPolicy
		defaultSelectPolicy := autoscalingv2.MinChangePolicySelect
		behavior.ScaleDown.SelectPolicy = &defaultSelectPolicy
	}

	if behavior.ScaleUp == nil {
		scaleUp := &autoscalingv2.HPAScalingRules{}

		defaultValue := int32(0)
		scaleUp.StabilizationWindowSeconds = &defaultValue

		scaleUp.Policies = []autoscalingv2.HPAScalingPolicy{
			{Type: autoscalingv2.PercentScalingPolicy, Value: 100, PeriodSeconds: 15},
			{Type: autoscalingv2.PodsScalingPolicy, Value: 4, PeriodSeconds: 15},
		}

		behavior.ScaleUp = scaleUp
	}

	if behavior.ScaleUp.SelectPolicy == nil {
		// If ScaleUp exists but SelectPolicy is not set, apply the default SelectPolicy
		defaultSelectPolicy := autoscalingv2.MaxChangePolicySelect
		behavior.ScaleUp.SelectPolicy = &defaultSelectPolicy
	}

	return behavior
}
