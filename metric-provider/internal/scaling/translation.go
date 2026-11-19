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
	"crema/metric-provider/api"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	v2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ToKedaScaledObjects converts from CREMA config to a collection of KEDA scaled objects
func ToKedaScaledObjects(cremaConfig *api.CremaConfig) []kedav1alpha1.ScaledObject {
	var kedaScaledObjects []kedav1alpha1.ScaledObject
	for _, cremaScaledObject := range cremaConfig.Spec.ScaledObjects {
		if cremaScaledObject.Spec.ScaleTargetRef == nil {
			continue
		}

		scaleTargetName := cremaScaledObject.Spec.ScaleTargetRef.Name
		if scaleTargetName == "" {
			continue
		}

		kedaScaledObject := kedav1alpha1.ScaledObject{
			ObjectMeta: metav1.ObjectMeta{
				Name: scaleTargetName,
			},
			Spec: cremaScaledObject.Spec,
		}
		kedaScaledObjects = append(kedaScaledObjects, kedaScaledObject)
	}

	return kedaScaledObjects
}

// PopulateTargetValue populates the appropriate target in cremaMetric based on the provided KEDA target
func PopulateTargetValue(kedaTarget v2.MetricTarget, cremaMetric *Metric) {
	if kedaTarget.Type == v2.AverageValueMetricType {
		targetValue := kedaTarget.AverageValue.AsApproximateFloat64()
		cremaMetric.TargetAverageValue = &targetValue
		cremaMetric.TargetValue = nil
		return
	}

	targetValue := kedaTarget.Value.AsApproximateFloat64()
	cremaMetric.TargetValue = &targetValue
	cremaMetric.TargetAverageValue = nil
}

// ToScaledObject converts from KEDA scaled object spec to internal ScaledObject struct
func ToScaledObject(kedaScaledObjectSpec kedav1alpha1.ScaledObjectSpec) *ScaledObject {
	scaledObject := &ScaledObject{
		ScaleTargetRef: &ScaleTargetRef{
			Name: kedaScaledObjectSpec.ScaleTargetRef.Name,
		},
	}

	if kedaScaledObjectSpec.Advanced != nil && kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig != nil && kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior != nil {
		scaledObject.Advanced = &Advanced{
			ScalerConfig: &ScalerConfig{
				Behavior: &Behavior{},
			},
		}

		if kedaScaledObjectSpec.MinReplicaCount != nil {
			scaledObject.Advanced.ScalerConfig.MinInstances = *kedaScaledObjectSpec.MinReplicaCount
		}
		if kedaScaledObjectSpec.MaxReplicaCount != nil {
			scaledObject.Advanced.ScalerConfig.MaxInstances = *kedaScaledObjectSpec.MaxReplicaCount
		}

		if kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown != nil {
			scaledObject.Advanced.ScalerConfig.Behavior.ScaleDown = &ScalingRules{
				SelectPolicy: *toSelectPolicy(kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown.SelectPolicy),
			}
			if kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown.StabilizationWindowSeconds != nil {
				scaledObject.Advanced.ScalerConfig.Behavior.ScaleDown.StabilizationWindowSeconds = *kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown.StabilizationWindowSeconds
			}
			for _, policy := range kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown.Policies {
				p := &Policy{
					Type:          *toScalingPolicyType(policy.Type),
					Value:         policy.Value,
					PeriodSeconds: policy.PeriodSeconds,
				}
				scaledObject.Advanced.ScalerConfig.Behavior.ScaleDown.Policies = append(scaledObject.Advanced.ScalerConfig.Behavior.ScaleDown.Policies, p)
			}
		}

		if kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp != nil {
			scaledObject.Advanced.ScalerConfig.Behavior.ScaleUp = &ScalingRules{
				SelectPolicy: *toSelectPolicy(kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp.SelectPolicy),
			}
			if kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp.StabilizationWindowSeconds != nil {
				scaledObject.Advanced.ScalerConfig.Behavior.ScaleUp.StabilizationWindowSeconds = *kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp.StabilizationWindowSeconds
			}
			for _, policy := range kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp.Policies {
				p := &Policy{
					Type:          *toScalingPolicyType(policy.Type),
					Value:         policy.Value,
					PeriodSeconds: policy.PeriodSeconds,
				}
				scaledObject.Advanced.ScalerConfig.Behavior.ScaleUp.Policies = append(scaledObject.Advanced.ScalerConfig.Behavior.ScaleUp.Policies, p)
			}
		}
	}

	return scaledObject
}

func toScalingPolicyType(kedaPolicyType v2.HPAScalingPolicyType) *PolicyType {
	var policyType PolicyType
	switch kedaPolicyType {
	case v2.PodsScalingPolicy:
		policyType = PolicyTypeInstances
	case v2.PercentScalingPolicy:
		policyType = PolicyTypePercent
	default:
		policyType = PolicyTypeUnspecified
	}
	return &policyType
}

func toSelectPolicy(kedaSelectPolicy *v2.ScalingPolicySelect) *SelectPolicy {
	selectPolicy := SelectPolicyMax
	if kedaSelectPolicy != nil {
		switch *kedaSelectPolicy {
		case v2.MinChangePolicySelect:
			selectPolicy = SelectPolicyMin
		case v2.DisabledPolicySelect:
			selectPolicy = SelectPolicyDisabled
		default:
			// Do nothing. An Unspecified select policy is MAX by default for consistency with k8s
		}
	}
	return &selectPolicy
}
