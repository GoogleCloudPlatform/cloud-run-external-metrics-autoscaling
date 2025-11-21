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
	pb "crema/metric-provider/proto"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	v2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// There are 3 different data types involved in CREMA:
// - CREMA config: The configuration API that CREMA reads; contains a collection of KEDA's ScaledObject
// - KEDA scaled object: the configuration that's passed to KEDA's scalers
// - Scaler protobuf: the data communication between metric provider and scaler containers

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
func PopulateTargetValue(kedaTarget v2.MetricTarget, cremaMetric *pb.Metric) {
	if kedaTarget.Type == v2.AverageValueMetricType {
		targetValue := float64(kedaTarget.AverageValue.Value())
		cremaMetric.Target = &pb.Metric_TargetAverageValue{
			TargetAverageValue: targetValue,
		}
		return
	}

	targetValue := float64(kedaTarget.Value.Value())
	cremaMetric.Target = &pb.Metric_TargetValue{
		TargetValue: targetValue,
	}
}

// ToPbScaledObject Converts from KEDA scaled object to scaled object protobuf
func ToPbScaledObject(kedaScaledObjectSpec kedav1alpha1.ScaledObjectSpec) *pb.ScaledObject {
	pbScaledObject := &pb.ScaledObject{
		ScaleTargetRef: &pb.ScaleTargetRef{
			Name: kedaScaledObjectSpec.ScaleTargetRef.Name,
		},
	}

	if kedaScaledObjectSpec.Advanced != nil && kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig != nil && kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior != nil {
		pbScaledObject.Advanced = &pb.Advanced{
			ScalerConfig: &pb.Advanced_ScalerConfig{
				Behavior: &pb.Advanced_ScalerConfig_Behavior{},
			},
		}

		if kedaScaledObjectSpec.MinReplicaCount != nil {
			pbScaledObject.Advanced.ScalerConfig.MinInstances = *kedaScaledObjectSpec.MinReplicaCount
		}
		if kedaScaledObjectSpec.MaxReplicaCount != nil {
			pbScaledObject.Advanced.ScalerConfig.MaxInstances = *kedaScaledObjectSpec.MaxReplicaCount
		}

		if kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown != nil {
			pbScaledObject.Advanced.ScalerConfig.Behavior.ScaleDown = &pb.Advanced_ScalerConfig_Behavior_Scaling{
				SelectPolicy: *toPbSelectPolicy(kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown.SelectPolicy),
			}
			if kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown.StabilizationWindowSeconds != nil {
				pbScaledObject.Advanced.ScalerConfig.Behavior.ScaleDown.StabilizationWindowSeconds = *kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown.StabilizationWindowSeconds
			}
			for _, policy := range kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleDown.Policies {
				pbPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{
					Type:          *toPbScalingPolicyType(policy.Type),
					Value:         policy.Value,
					PeriodSeconds: policy.PeriodSeconds,
				}
				pbScaledObject.Advanced.ScalerConfig.Behavior.ScaleDown.Policies = append(pbScaledObject.Advanced.ScalerConfig.Behavior.ScaleDown.Policies, pbPolicy)
			}
		}

		if kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp != nil {
			pbScaledObject.Advanced.ScalerConfig.Behavior.ScaleUp = &pb.Advanced_ScalerConfig_Behavior_Scaling{
				SelectPolicy: *toPbSelectPolicy(kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp.SelectPolicy),
			}
			if kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp.StabilizationWindowSeconds != nil {
				pbScaledObject.Advanced.ScalerConfig.Behavior.ScaleUp.StabilizationWindowSeconds = *kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp.StabilizationWindowSeconds
			}
			for _, policy := range kedaScaledObjectSpec.Advanced.HorizontalPodAutoscalerConfig.Behavior.ScaleUp.Policies {
				pbPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{
					Type:          *toPbScalingPolicyType(policy.Type),
					Value:         policy.Value,
					PeriodSeconds: policy.PeriodSeconds,
				}
				pbScaledObject.Advanced.ScalerConfig.Behavior.ScaleUp.Policies = append(pbScaledObject.Advanced.ScalerConfig.Behavior.ScaleUp.Policies, pbPolicy)
			}
		}
	}

	return pbScaledObject
}

func toPbScalingPolicyType(kedaPolicyType v2.HPAScalingPolicyType) *pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_Type {
	var pbPolicyType pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_Type
	switch kedaPolicyType {
	case v2.PodsScalingPolicy:
		pbPolicyType = pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES
	case v2.PercentScalingPolicy:
		pbPolicyType = pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT
	default:
		pbPolicyType = pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_TYPE_UNSPECIFIED
	}
	return &pbPolicyType
}

func toPbSelectPolicy(kedaSelectPolicy *v2.ScalingPolicySelect) *pb.Advanced_ScalerConfig_Behavior_Scaling_SelectPolicy {
	pbSelectPolicy := pb.Advanced_ScalerConfig_Behavior_Scaling_MAX
	if kedaSelectPolicy != nil {
		switch *kedaSelectPolicy {
		case v2.MinChangePolicySelect:
			pbSelectPolicy = pb.Advanced_ScalerConfig_Behavior_Scaling_MIN
		case v2.DisabledPolicySelect:
			pbSelectPolicy = pb.Advanced_ScalerConfig_Behavior_Scaling_DISABLED
		default:
			// Do nothing. An Unspecified select policy is MAX by default for consistency with k8s
		}
	}
	return &pbSelectPolicy
}
