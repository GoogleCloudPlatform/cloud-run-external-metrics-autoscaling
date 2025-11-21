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
	"reflect"
	"testing"

	"crema/metric-provider/api"
	pb "crema/metric-provider/proto"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	v2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToKedaScaledObjects(t *testing.T) {
	tests := []struct {
		name        string
		cremaConfig api.CremaConfig
		want        []kedav1alpha1.ScaledObject
	}{
		{
			name: "valid config",
			cremaConfig: api.CremaConfig{
				Spec: api.CremaConfigSpec{
					ScaledObjects: []api.CremaScaledObject{
						{
							Spec: kedav1alpha1.ScaledObjectSpec{
								ScaleTargetRef: &kedav1alpha1.ScaleTarget{
									Name: "test-target",
								},
							},
						},
					},
				},
			},
			want: []kedav1alpha1.ScaledObject{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-target",
					},
					Spec: kedav1alpha1.ScaledObjectSpec{
						ScaleTargetRef: &kedav1alpha1.ScaleTarget{
							Name: "test-target",
						},
					},
				},
			},
		},
		{
			name: "scaled object with no scale target ref",
			cremaConfig: api.CremaConfig{Spec: api.CremaConfigSpec{
				ScaledObjects: []api.CremaScaledObject{
					{
						Spec: kedav1alpha1.ScaledObjectSpec{},
					},
				},
			},
			},
			want: nil,
		},
		{
			name: "scaled object with empty scale target name",
			cremaConfig: api.CremaConfig{
				Spec: api.CremaConfigSpec{
					ScaledObjects: []api.CremaScaledObject{
						{
							Spec: kedav1alpha1.ScaledObjectSpec{
								ScaleTargetRef: &kedav1alpha1.ScaleTarget{
									Name: "",
								},
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "full config",
			cremaConfig: api.CremaConfig{
				Spec: api.CremaConfigSpec{
					ScaledObjects: []api.CremaScaledObject{
						{
							Spec: kedav1alpha1.ScaledObjectSpec{
								ScaleTargetRef: &kedav1alpha1.ScaleTarget{
									Name: "projects/my-project/locations/us-central1/workerPools/my-worker-pool",
								},
								Triggers: []kedav1alpha1.ScaleTriggers{
									{
										Type: "kafka",
										Metadata: map[string]string{
											"bootstrapServers": "10.10.0.25:9092",
											"consumerGroup":    "my-consumer-group",
											"topic":            "my-crema-topic",
											"lagThreshold":     "5",
										},
									},
								},
							},
						},
					},
				},
			},
			want: []kedav1alpha1.ScaledObject{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "projects/my-project/locations/us-central1/workerPools/my-worker-pool",
					},
					Spec: kedav1alpha1.ScaledObjectSpec{
						ScaleTargetRef: &kedav1alpha1.ScaleTarget{
							Name: "projects/my-project/locations/us-central1/workerPools/my-worker-pool",
						},
						Triggers: []kedav1alpha1.ScaleTriggers{
							{
								Type: "kafka",
								Metadata: map[string]string{
									"bootstrapServers": "10.10.0.25:9092",
									"consumerGroup":    "my-consumer-group",
									"topic":            "my-crema-topic",
									"lagThreshold":     "5",
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToKedaScaledObjects(&tt.cremaConfig); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToKedaScaledObjects() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPopulateTargetValue(t *testing.T) {
	tests := []struct {
		name       string
		kedaTarget v2.MetricTarget
		want       *pb.Metric
	}{
		{
			name: "average value",
			kedaTarget: v2.MetricTarget{
				Type:         v2.AverageValueMetricType,
				AverageValue: resource.NewQuantity(10, resource.DecimalSI),
			},
			want: &pb.Metric{
				Target: &pb.Metric_TargetAverageValue{
					TargetAverageValue: 10,
				},
			},
		},
		{
			name: "value",
			kedaTarget: v2.MetricTarget{
				Type:  v2.ValueMetricType,
				Value: resource.NewQuantity(20, resource.DecimalSI),
			},
			want: &pb.Metric{
				Target: &pb.Metric_TargetValue{
					TargetValue: 20,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &pb.Metric{}
			PopulateTargetValue(tt.kedaTarget, got)
			if !reflect.DeepEqual(got.Target, tt.want.Target) {
				t.Errorf("ToCremaMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToPbScaledObject(t *testing.T) {
	type args struct {
		kedaScaledObjectSpec kedav1alpha1.ScaledObjectSpec
	}

	minReplicaCount := int32(1)
	maxReplicaCount := int32(10)

	scaleDownStabilizationWindowSeconds := int32(300)
	scaleUpStabilizationWindowSeconds := int32(0)

	scaleDownSelectPolicy := v2.MinChangePolicySelect
	scaleUpSelectPolicy := v2.MaxChangePolicySelect

	tests := []struct {
		name string
		args args
		want *pb.ScaledObject
	}{
		{
			name: "Test with advanced config and replica counts",
			args: args{
				kedaScaledObjectSpec: kedav1alpha1.ScaledObjectSpec{
					ScaleTargetRef: &kedav1alpha1.ScaleTarget{
						Name: "test-target",
					},
					MinReplicaCount: &minReplicaCount,
					MaxReplicaCount: &maxReplicaCount,
					Advanced: &kedav1alpha1.AdvancedConfig{
						HorizontalPodAutoscalerConfig: &kedav1alpha1.HorizontalPodAutoscalerConfig{
							Behavior: &v2.HorizontalPodAutoscalerBehavior{
								ScaleDown: &v2.HPAScalingRules{
									StabilizationWindowSeconds: &scaleDownStabilizationWindowSeconds,
									SelectPolicy:               &scaleDownSelectPolicy,
									Policies: []v2.HPAScalingPolicy{
										{
											Type:          v2.PodsScalingPolicy,
											Value:         1,
											PeriodSeconds: 60,
										},
									},
								},
								ScaleUp: &v2.HPAScalingRules{
									StabilizationWindowSeconds: &scaleUpStabilizationWindowSeconds,
									SelectPolicy:               &scaleUpSelectPolicy,
									Policies: []v2.HPAScalingPolicy{
										{
											Type:          v2.PercentScalingPolicy,
											Value:         10,
											PeriodSeconds: 30,
										},
									},
								},
							},
						},
					},
				},
			},
			want: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{
					Name: "test-target",
				},
				Advanced: &pb.Advanced{
					ScalerConfig: &pb.Advanced_ScalerConfig{
						Behavior: &pb.Advanced_ScalerConfig_Behavior{
							ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
								StabilizationWindowSeconds: 300,
								SelectPolicy:               pb.Advanced_ScalerConfig_Behavior_Scaling_MIN,
								Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{
									{
										Type:          pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES,
										Value:         1,
										PeriodSeconds: 60,
									},
								},
							},
							ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{
								StabilizationWindowSeconds: 0,
								SelectPolicy:               pb.Advanced_ScalerConfig_Behavior_Scaling_MAX,
								Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{
									{
										Type:          pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT,
										Value:         10,
										PeriodSeconds: 30,
									},
								},
							},
						},
						MinInstances: minReplicaCount,
						MaxInstances: maxReplicaCount,
					},
				},
			},
		},
		{
			name: "Test with advanced config but no replica counts",
			args: args{
				kedaScaledObjectSpec: kedav1alpha1.ScaledObjectSpec{
					ScaleTargetRef: &kedav1alpha1.ScaleTarget{
						Name: "test-target",
					},
					Advanced: &kedav1alpha1.AdvancedConfig{
						HorizontalPodAutoscalerConfig: &kedav1alpha1.HorizontalPodAutoscalerConfig{
							Behavior: &v2.HorizontalPodAutoscalerBehavior{
								ScaleDown: &v2.HPAScalingRules{
									StabilizationWindowSeconds: &scaleDownStabilizationWindowSeconds,
									SelectPolicy:               &scaleDownSelectPolicy,
									Policies: []v2.HPAScalingPolicy{
										{
											Type:          v2.PodsScalingPolicy,
											Value:         1,
											PeriodSeconds: 60,
										},
									},
								},
								ScaleUp: &v2.HPAScalingRules{
									StabilizationWindowSeconds: &scaleUpStabilizationWindowSeconds,
									SelectPolicy:               &scaleUpSelectPolicy,
									Policies: []v2.HPAScalingPolicy{
										{
											Type:          v2.PercentScalingPolicy,
											Value:         10,
											PeriodSeconds: 30,
										},
									},
								},
							},
						},
					},
				},
			},
			want: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{
					Name: "test-target",
				},
				Advanced: &pb.Advanced{
					ScalerConfig: &pb.Advanced_ScalerConfig{
						Behavior: &pb.Advanced_ScalerConfig_Behavior{
							ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
								StabilizationWindowSeconds: 300,
								SelectPolicy:               pb.Advanced_ScalerConfig_Behavior_Scaling_MIN,
								Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{
									{
										Type:          pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES,
										Value:         1,
										PeriodSeconds: 60,
									},
								},
							},
							ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{
								StabilizationWindowSeconds: 0,
								SelectPolicy:               pb.Advanced_ScalerConfig_Behavior_Scaling_MAX,
								Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{
									{
										Type:          pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT,
										Value:         10,
										PeriodSeconds: 30,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Test with nil replica counts and stabilization windows",
			args: args{
				kedaScaledObjectSpec: kedav1alpha1.ScaledObjectSpec{
					ScaleTargetRef: &kedav1alpha1.ScaleTarget{
						Name: "test-target-nil",
					},
					MinReplicaCount: nil,
					MaxReplicaCount: nil,
					Advanced: &kedav1alpha1.AdvancedConfig{
						HorizontalPodAutoscalerConfig: &kedav1alpha1.HorizontalPodAutoscalerConfig{
							Behavior: &v2.HorizontalPodAutoscalerBehavior{
								ScaleDown: &v2.HPAScalingRules{
									StabilizationWindowSeconds: nil,
								},
								ScaleUp: &v2.HPAScalingRules{
									StabilizationWindowSeconds: nil,
								},
							},
						},
					},
				},
			},
			want: &pb.ScaledObject{
				ScaleTargetRef: &pb.ScaleTargetRef{
					Name: "test-target-nil",
				},
				Advanced: &pb.Advanced{
					ScalerConfig: &pb.Advanced_ScalerConfig{
						Behavior: &pb.Advanced_ScalerConfig_Behavior{
							ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
								SelectPolicy: pb.Advanced_ScalerConfig_Behavior_Scaling_MAX,
							},
							ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{
								SelectPolicy: pb.Advanced_ScalerConfig_Behavior_Scaling_MAX,
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToPbScaledObject(tt.args.kedaScaledObjectSpec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToPbScaledObject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToPbScalingPolicyType(t *testing.T) {
	tests := []struct {
		name string
		arg  v2.HPAScalingPolicyType
		want *pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_Type
	}{
		{
			name: "pods scaling policy",
			arg:  v2.PodsScalingPolicy,
			want: func() *pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_Type {
				p := pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES
				return &p
			}(),
		},
		{
			name: "percent scaling policy",
			arg:  v2.PercentScalingPolicy,
			want: func() *pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_Type {
				p := pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT
				return &p
			}(),
		},
		{
			name: "unspecified scaling policy",
			arg:  "unknown",
			want: func() *pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_Type {
				p := pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_TYPE_UNSPECIFIED
				return &p
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toPbScalingPolicyType(tt.arg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toPbScalingPolicyType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToPbSelectPolicy(t *testing.T) {
	tests := []struct {
		name string
		arg  *v2.ScalingPolicySelect
		want *pb.Advanced_ScalerConfig_Behavior_Scaling_SelectPolicy
	}{
		{
			name: "min change policy",
			arg:  func() *v2.ScalingPolicySelect { s := v2.MinChangePolicySelect; return &s }(),
			want: func() *pb.Advanced_ScalerConfig_Behavior_Scaling_SelectPolicy {
				p := pb.Advanced_ScalerConfig_Behavior_Scaling_MIN
				return &p
			}(),
		},
		{
			name: "max change policy",
			arg:  func() *v2.ScalingPolicySelect { s := v2.MaxChangePolicySelect; return &s }(),
			want: func() *pb.Advanced_ScalerConfig_Behavior_Scaling_SelectPolicy {
				p := pb.Advanced_ScalerConfig_Behavior_Scaling_MAX
				return &p
			}(),
		},
		{
			name: "disabled policy",
			arg:  func() *v2.ScalingPolicySelect { s := v2.DisabledPolicySelect; return &s }(),
			want: func() *pb.Advanced_ScalerConfig_Behavior_Scaling_SelectPolicy {
				p := pb.Advanced_ScalerConfig_Behavior_Scaling_DISABLED
				return &p
			}(),
		},
		{
			name: "nil policy",
			arg:  nil,
			want: func() *pb.Advanced_ScalerConfig_Behavior_Scaling_SelectPolicy {
				p := pb.Advanced_ScalerConfig_Behavior_Scaling_MAX
				return &p
			}(),
		},
		{
			name: "unspecified policy",
			arg:  func() *v2.ScalingPolicySelect { s := v2.ScalingPolicySelect("unknown"); return &s }(),
			want: func() *pb.Advanced_ScalerConfig_Behavior_Scaling_SelectPolicy {
				p := pb.Advanced_ScalerConfig_Behavior_Scaling_MAX
				return &p
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toPbSelectPolicy(tt.arg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toPbSelectPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}
