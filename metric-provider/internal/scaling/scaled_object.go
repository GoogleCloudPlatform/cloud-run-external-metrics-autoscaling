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
	"fmt"
	"strings"
)

// SelectPolicy defines how to choose a scaling bound when multiple policies are specified.
type SelectPolicy int32

const (
	SelectPolicyUnspecified SelectPolicy = 0
	SelectPolicyMax         SelectPolicy = 1
	SelectPolicyMin         SelectPolicy = 2
	SelectPolicyDisabled    SelectPolicy = 3
)

func (s SelectPolicy) String() string {
	switch s {
	case SelectPolicyMax:
		return "MAX"
	case SelectPolicyMin:
		return "MIN"
	case SelectPolicyDisabled:
		return "DISABLED"
	default:
		return "SELECT_POLICY_UNSPECIFIED"
	}
}

// PolicyType defines the unit of a scaling policy.
type PolicyType int32

const (
	PolicyTypeUnspecified PolicyType = 0
	PolicyTypePercent     PolicyType = 1
	PolicyTypeInstances   PolicyType = 2
)

func (p PolicyType) String() string {
	switch p {
	case PolicyTypeInstances:
		return "INSTANCES"
	case PolicyTypePercent:
		return "PERCENT"
	default:
		return "TYPE_UNSPECIFIED"
	}
}

// Policy represents a single HPA rate-limiting policy.
type Policy struct {
	Type          PolicyType
	Value         int32
	PeriodSeconds int32
}

func (p *Policy) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{type:%s value:%d period_seconds:%d}", p.Type, p.Value, p.PeriodSeconds)
}

// ScalingRules represents stabilization and rate-limiting rules for scale-up or scale-down.
type ScalingRules struct {
	StabilizationWindowSeconds int32
	SelectPolicy               SelectPolicy
	Policies                   []*Policy
}

func (r *ScalingRules) String() string {
	if r == nil {
		return "<nil>"
	}
	policies := make([]string, len(r.Policies))
	for i, p := range r.Policies {
		policies[i] = p.String()
	}
	return fmt.Sprintf("{stabilization_window_seconds:%d select_policy:%s policies:[%s]}", r.StabilizationWindowSeconds, r.SelectPolicy, strings.Join(policies, " "))
}

// Behavior holds scale-up and scale-down rules.
type Behavior struct {
	ScaleUp   *ScalingRules
	ScaleDown *ScalingRules
}

func (b *Behavior) String() string {
	if b == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{scale_up:%s scale_down:%s}", b.ScaleUp.String(), b.ScaleDown.String())
}

func (b *Behavior) GetScaleUp() *ScalingRules {
	if b != nil {
		return b.ScaleUp
	}
	return nil
}

func (b *Behavior) GetScaleDown() *ScalingRules {
	if b != nil {
		return b.ScaleDown
	}
	return nil
}

// ScalerConfig holds instance bounds and scaling behavior.
type ScalerConfig struct {
	Behavior     *Behavior
	MinInstances int32
	MaxInstances int32
}

func (s *ScalerConfig) GetBehavior() *Behavior {
	if s != nil {
		return s.Behavior
	}
	return nil
}

func (s *ScalerConfig) GetMinInstances() int32 {
	if s != nil {
		return s.MinInstances
	}
	return 0
}

func (s *ScalerConfig) GetMaxInstances() int32 {
	if s != nil {
		return s.MaxInstances
	}
	return 0
}

func (s *ScalerConfig) String() string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{min_instances:%d max_instances:%d behavior:%s}", s.MinInstances, s.MaxInstances, s.Behavior.String())
}

// Advanced holds advanced scaling configuration.
type Advanced struct {
	ScalerConfig *ScalerConfig
}

func (a *Advanced) String() string {
	if a == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{scaler_config:%s}", a.ScalerConfig.String())
}

// ScaleTargetRef identifies the target Cloud Run service or worker pool.
type ScaleTargetRef struct {
	Name string
}

func (s *ScaleTargetRef) String() string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{name:%q}", s.Name)
}

// ScaledObject holds the parsed configuration for a scaled workload.
type ScaledObject struct {
	ScaleTargetRef *ScaleTargetRef
	Advanced       *Advanced
}

func (s *ScaledObject) String() string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{scale_target_ref:%s advanced:%s}", s.ScaleTargetRef.String(), s.Advanced.String())
}
