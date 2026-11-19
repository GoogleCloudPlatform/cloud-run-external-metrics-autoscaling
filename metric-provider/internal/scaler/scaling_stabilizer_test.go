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

package scaler

import (
	"testing"
	"time"

	pb "crema/metric-provider/proto"

	"github.com/go-logr/logr"
)

func TestScalingStabilizer(t *testing.T) {
	logger := logr.Discard()

	assertRec := func(t *testing.T, got, want int) {
		t.Helper()
		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	}

	t.Run("scalingStabilizer_withEmptyConfig", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{}
		ss := NewScalingStabilizer(100, logger)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, time.Now(), 5, 1000, "test-workload"), 1000)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, time.Now(), 1000, 1, "test-workload"), 1)
	})

	t.Run("getStabilizedRecommendation_scaleUpStabilizationOnly_returnsStabilizationWindowBound", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 300},
		}
		now := time.Now()
		ss := NewScalingStabilizer(100, logger)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 100, 110, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(120*time.Second), 100, 110, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(180*time.Second), 100, 110, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(240*time.Second), 100, 110, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(301*time.Second), 100, 110, "test-workload"), 110)
	})

	t.Run("getStabilizedRecommendation_scaleDownStabilizationOnly_returnsStabilizationWindowBound", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 300},
		}
		now := time.Now()
		ss := NewScalingStabilizer(100, logger)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 100, 90, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(120*time.Second), 100, 90, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(180*time.Second), 100, 90, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(240*time.Second), 100, 90, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(301*time.Second), 100, 90, "test-workload"), 90)
	})

	t.Run("getStabilizedRecommendation_unchangedRecommendation_countsTowardScaleUpStabilization", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 300},
		}
		now := time.Now()
		ss := NewScalingStabilizer(100, logger)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(4*time.Minute), 100, 100, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(6*time.Minute), 100, 110, "test-workload"), 100)
	})

	t.Run("getStabilizedRecommendation_unchangedRecommendation_countsTowardScaleDownStabilization", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 300},
		}
		now := time.Now()
		ss := NewScalingStabilizer(100, logger)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(4*time.Minute), 100, 100, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(6*time.Minute), 100, 90, "test-workload"), 100)
	})

	t.Run("getStabilizedRecommendation_scaleDownPercentPolicy_returnsPercentPolicyBound", func(t *testing.T) {
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{percentPolicy}},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 10, "test-workload"), 50)
		ss.MarkScaleEvent(behavior, now, 100, 50)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 50, 10, "test-workload"), 50)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 50, 10, "test-workload"), 25)
		ss.MarkScaleEvent(behavior, now.Add(60*time.Second), 50, 25)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(140*time.Second), 25, 0, "test-workload"), 12)
	})

	t.Run("getStabilizedRecommendation_scaleDownInstancesPolicy_returnsInstancesPolicyBound", func(t *testing.T) {
		instancesPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES, Value: 1, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{instancesPolicy}},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 90, "test-workload"), 99)
		ss.MarkScaleEvent(behavior, now, 100, 99)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 99, 90, "test-workload"), 99)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 99, 90, "test-workload"), 98)
		ss.MarkScaleEvent(behavior, now.Add(60*time.Second), 99, 98)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(140*time.Second), 98, 0, "test-workload"), 97)
	})

	t.Run("getStabilizedRecommendation_multipleScaleDownPoliciesUnsetSelectPolicy_returnsMinBound", func(t *testing.T) {
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}
		instancesPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES, Value: 1, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{instancesPolicy, percentPolicy}},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 40, "test-workload"), 50)
		ss.MarkScaleEvent(behavior, now, 100, 50)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 50, 0, "test-workload"), 50)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 50, 0, "test-workload"), 25)
		ss.MarkScaleEvent(behavior, now.Add(61*time.Second), 50, 25)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(140*time.Second), 25, 0, "test-workload"), 12)
	})

	t.Run("getStabilizedRecommendation_multipleScaleDownPoliciesMaxSelectPolicy_returnsMinBound", func(t *testing.T) {
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}
		instancesPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES, Value: 1, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				Policies:     []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{instancesPolicy, percentPolicy},
				SelectPolicy: pb.Advanced_ScalerConfig_Behavior_Scaling_MAX,
			},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 40, "test-workload"), 50)
		ss.MarkScaleEvent(behavior, now, 100, 50)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 50, 0, "test-workload"), 50)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 50, 0, "test-workload"), 25)
		ss.MarkScaleEvent(behavior, now.Add(61*time.Second), 50, 25)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(140*time.Second), 25, 0, "test-workload"), 12)
	})

	t.Run("getStabilizedRecommendation_multipleScaleDownPoliciesMinSelectPolicy_returnsMaxBound", func(t *testing.T) {
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}
		instancesPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES, Value: 1, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				Policies:     []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{instancesPolicy, percentPolicy},
				SelectPolicy: pb.Advanced_ScalerConfig_Behavior_Scaling_MIN,
			},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 40, "test-workload"), 99)
		ss.MarkScaleEvent(behavior, now, 100, 99)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 99, 98, "test-workload"), 99)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 99, 90, "test-workload"), 98)
		ss.MarkScaleEvent(behavior, now.Add(61*time.Second), 99, 98)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(140*time.Second), 98, 0, "test-workload"), 97)
	})

	t.Run("getStabilizedRecommendation_scaleUpPercentPolicy_returnsPercentPolicyBound", func(t *testing.T) {
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{percentPolicy}},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 200, "test-workload"), 150)
		ss.MarkScaleEvent(behavior, now, 100, 150)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 150, 200, "test-workload"), 150)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 150, 250, "test-workload"), 225)
		ss.MarkScaleEvent(behavior, now.Add(61*time.Second), 150, 250)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(140*time.Second), 225, 500, "test-workload"), 338)
	})

	t.Run("getStabilizedRecommendation_scaleUpFromZero_isAlwaysAllowed", func(t *testing.T) {
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				Policies:                   []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{percentPolicy},
				StabilizationWindowSeconds: 300,
			},
		}
		ss := NewScalingStabilizer(0, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 0, 1, "test-workload"), 1)
	})

	t.Run("getStabilizedRecommendation_scaleUpFromZero_countsTowardScaleDownStabilization", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 300},
		}
		ss := NewScalingStabilizer(0, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(10*time.Second), 0, 1, "test-workload"), 1)
		// The workload took its work off the queue, so the metric drops back to 0.
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(20*time.Second), 1, 0, "test-workload"), 1)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(311*time.Second), 1, 0, "test-workload"), 0)
	})

	t.Run("getStabilizedRecommendation_scaleUpInstancesPolicy_returnsInstancesPolicyBound", func(t *testing.T) {
		instancesPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES, Value: 5, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{instancesPolicy}},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 200, "test-workload"), 105)
		ss.MarkScaleEvent(behavior, now, 100, 105)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 105, 200, "test-workload"), 105)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 105, 200, "test-workload"), 110)
		ss.MarkScaleEvent(behavior, now.Add(60*time.Second), 105, 110)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(140*time.Second), 110, 500, "test-workload"), 115)
	})

	t.Run("getStabilizedRecommendation_scaleUpMultiplePoliciesUnsetSelectPolicy_returnsMaxBound", func(t *testing.T) {
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}
		instancesPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES, Value: 5, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{instancesPolicy, percentPolicy}},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 200, "test-workload"), 150)
		ss.MarkScaleEvent(behavior, now, 100, 150)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 150, 200, "test-workload"), 150)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 150, 250, "test-workload"), 225)
		ss.MarkScaleEvent(behavior, now.Add(61*time.Second), 150, 250)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(140*time.Second), 225, 500, "test-workload"), 338)
	})

	t.Run("getStabilizedRecommendation_scaleUpMultiplePoliciesMaxSelectPolicy_returnsMaxBound", func(t *testing.T) {
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}
		instancesPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES, Value: 5, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				Policies:     []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{instancesPolicy, percentPolicy},
				SelectPolicy: pb.Advanced_ScalerConfig_Behavior_Scaling_MAX,
			},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 200, "test-workload"), 150)
		ss.MarkScaleEvent(behavior, now, 100, 150)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 150, 200, "test-workload"), 150)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 150, 250, "test-workload"), 225)
		ss.MarkScaleEvent(behavior, now.Add(61*time.Second), 150, 250)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(140*time.Second), 225, 500, "test-workload"), 338)
	})

	t.Run("getStabilizedRecommendation_scaleUpMultiplePoliciesWithMinSelectPolicy_returnsMinBound", func(t *testing.T) {
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 100, PeriodSeconds: 90}
		instancesPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_INSTANCES, Value: 5, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				Policies:     []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{instancesPolicy, percentPolicy},
				SelectPolicy: pb.Advanced_ScalerConfig_Behavior_Scaling_MIN,
			},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 200, "test-workload"), 105)
		ss.MarkScaleEvent(behavior, now, 100, 105)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 105, 200, "test-workload"), 105)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(75*time.Second), 105, 200, "test-workload"), 110)
		ss.MarkScaleEvent(behavior, now.Add(60*time.Second), 105, 110)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(140*time.Second), 110, 500, "test-workload"), 115)
	})

	t.Run("getStabilizedRecommendation_scaleUpDisabled_returnsCurrentInstances", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{SelectPolicy: pb.Advanced_ScalerConfig_Behavior_Scaling_DISABLED},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 200, "test-workload"), 100)
		ss.MarkScaleEvent(behavior, now, 100, 200)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 200, 400, "test-workload"), 200)
	})

	t.Run("getStabilizedRecommendation_scaleDownDisabled_returnsCurrentInstances", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{SelectPolicy: pb.Advanced_ScalerConfig_Behavior_Scaling_DISABLED},
		}
		ss := NewScalingStabilizer(200, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 200, 100, "test-workload"), 200)
		ss.MarkScaleEvent(behavior, now, 200, 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 50, "test-workload"), 100)
	})

	t.Run("getStabilizedRecommendation_scaleUpMultiplePoliciesMaxSelectPolicyAndStabilizationWindow_returnsStabilizationBound", func(t *testing.T) {
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				StabilizationWindowSeconds: 60,
				Policies:                   []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{percentPolicy},
				SelectPolicy:               pb.Advanced_ScalerConfig_Behavior_Scaling_MAX,
			},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 100, 200, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 100, 200, "test-workload"), 150)
	})

	t.Run("getStabilizedRecommendation_scaleDownMultiplePoliciesMaxSelectPolicyAndStabilizationWindow_returnsStabilizationBound", func(t *testing.T) {
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				StabilizationWindowSeconds: 60,
				Policies:                   []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{percentPolicy},
				SelectPolicy:               pb.Advanced_ScalerConfig_Behavior_Scaling_MAX,
			},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 100, 50, "test-workload"), 100)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 100, 50, "test-workload"), 50)
	})

	t.Run("getStabilizedRecommendation_decreasingWindowSize_returnsCorrectBound", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 300},
		}
		now := time.Now()
		ss := NewScalingStabilizer(100, logger)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 100, 90, "test-workload"), 100)

		decreasedStabilizationWindow := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 60},
		}
		assertRec(t, ss.GetStabilizedRecommendation(decreasedStabilizationWindow, now.Add(119*time.Second), 100, 80, "test-workload"), 90)
	})

	t.Run("getStabilizedRecommendation_increasingWindowSize_returnsCorrectBound", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 300},
		}
		now := time.Now()
		ss := NewScalingStabilizer(100, logger)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 100, 90, "test-workload"), 100)

		increasedStabilizationWindow := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{StabilizationWindowSeconds: 360},
		}
		assertRec(t, ss.GetStabilizedRecommendation(increasedStabilizationWindow, now.Add(359*time.Second), 100, 80, "test-workload"), 100)
	})

	t.Run("getStabilizedRecommendation_percentIncreases_returnsCorrectBound", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}},
			},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 10, "test-workload"), 50)
		ss.MarkScaleEvent(behavior, now, 100, 50)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 50, 10, "test-workload"), 50)

		increasedScaledownPercent := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 80, PeriodSeconds: 60}},
			},
		}
		assertRec(t, ss.GetStabilizedRecommendation(increasedScaledownPercent, now.Add(60*time.Second), 50, 0, "test-workload"), 10)
	})

	t.Run("getStabilizedRecommendation_percentDecreases_returnsCorrectBound", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}},
			},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 10, "test-workload"), 50)
		ss.MarkScaleEvent(behavior, now, 100, 50)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(59*time.Second), 50, 10, "test-workload"), 50)

		decreasedScaledownPercent := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 20, PeriodSeconds: 60}},
			},
		}
		assertRec(t, ss.GetStabilizedRecommendation(decreasedScaledownPercent, now.Add(60*time.Second), 50, 0, "test-workload"), 40)
	})

	t.Run("getStabilizedRecommendation_rateWindowIncreases_returnsCorrectBound", func(t *testing.T) {
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}},
			},
		}
		ss := NewScalingStabilizer(100, logger)
		now := time.Now()

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now, 100, 10, "test-workload"), 50)
		ss.MarkScaleEvent(behavior, now, 100, 50)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(60*time.Second), 50, 45, "test-workload"), 45)
		ss.MarkScaleEvent(behavior, now.Add(60*time.Second), 50, 45)

		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(90*time.Second), 45, 40, "test-workload"), 40)
		ss.MarkScaleEvent(behavior, now.Add(90*time.Second), 45, 40)

		decreasedScaleDownWindow := &pb.Advanced_ScalerConfig_Behavior{
			ScaleDown: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				Policies: []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 30}},
			},
		}
		assertRec(t, ss.GetStabilizedRecommendation(decreasedScaleDownWindow, now.Add(120*time.Second), 40, 0, "test-workload"), 20)
	})

	t.Run("getStabilizedRecommendation_negativePeriodStartInstancesScaleUpPercent_roundsAwayFromZero", func(t *testing.T) {
		// If periodStartInstances becomes negative (e.g. -5) and scaleUp percent is 50%,
		// Java BigDecimal.valueOf(-5 * 150.0 / 100.0).setScale(0, RoundingMode.UP) = -8.
		percentPolicy := &pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{Type: pb.Advanced_ScalerConfig_Behavior_Scaling_Policy_PERCENT, Value: 50, PeriodSeconds: 60}
		behavior := &pb.Advanced_ScalerConfig_Behavior{
			ScaleUp: &pb.Advanced_ScalerConfig_Behavior_Scaling{
				SelectPolicy: pb.Advanced_ScalerConfig_Behavior_Scaling_MIN,
				Policies:     []*pb.Advanced_ScalerConfig_Behavior_Scaling_Policy{percentPolicy},
			},
		}
		ss := NewScalingStabilizer(10, logger)
		now := time.Now()
		// Record a large scale-up event of +15 in the window, while currentInstanceCount is 10 -> periodStartInstances = -5.
		ss.MarkScaleEvent(behavior, now, 0, 15)
		assertRec(t, ss.GetStabilizedRecommendation(behavior, now.Add(10*time.Second), 10, 20, "test-workload"), -8)
	})
}
