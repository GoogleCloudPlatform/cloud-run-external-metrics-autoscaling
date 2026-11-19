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
	"fmt"
	"math"
	"strings"
	"time"

	"crema/metric-provider/internal/scaling"

	"github.com/go-logr/logr"
)

const (
	minInstances = 0
	maxInstances = math.MaxInt32
)

type boundsDebugInfo struct {
	scaleUp             []string
	scaleDown           []string
	selectorDebugString string
}

func (b *boundsDebugInfo) setSelectorDebugString(selector scaling.SelectPolicy) {
	b.selectorDebugString = fmt.Sprintf("{Select: %s}", selector.String())
}

func (b *boundsDebugInfo) addScaleUp(policyType string, limit int) {
	b.scaleUp = append(b.scaleUp, fmt.Sprintf("{Policy: %s, Limit: %d}", policyType, limit))
}

func (b *boundsDebugInfo) addScaleDown(policyType string, limit int) {
	b.scaleDown = append(b.scaleDown, fmt.Sprintf("{Policy: %s, Limit: %d}", policyType, limit))
}

func (b *boundsDebugInfo) toScaleUpBoundDebugString() string {
	return b.format("up", b.scaleUp)
}

func (b *boundsDebugInfo) toScaleDownBoundDebugString() string {
	return b.format("down", b.scaleDown)
}

func (b *boundsDebugInfo) format(direction string, debugStrings []string) string {
	parts := make([]string, 0, len(debugStrings)+1)
	if b.selectorDebugString != "" {
		parts = append(parts, b.selectorDebugString)
	}
	parts = append(parts, debugStrings...)
	return fmt.Sprintf("Scale-%s: %s", direction, strings.Join(parts, "; "))
}

type dataPoint struct {
	timestamp time.Time
	value     int
}

// ScalingStabilizer applies bounds to scaling up/down based on the provided scaling config.
// Scaling is stabilized relative to the mins and maxes of the period.
type ScalingStabilizer struct {
	scaleUpEvents   []dataPoint
	scaleDownEvents []dataPoint
	recommendations []dataPoint
	logger          logr.Logger
}

// NewScalingStabilizer creates a new ScalingStabilizer with an initial recommendation recorded at the current time.
func NewScalingStabilizer(currentInstanceCount int, logger logr.Logger) *ScalingStabilizer {
	s := &ScalingStabilizer{
		logger: logger,
	}
	s.recordInitialRecommendation(time.Now(), currentInstanceCount)
	return s
}

func (s *ScalingStabilizer) recordInitialRecommendation(t time.Time, recommendedInstances int) {
	s.recommendations = append(s.recommendations, dataPoint{timestamp: t, value: recommendedInstances})
}

func getLongestPolicyPeriod(rules *scaling.ScalingRules) time.Duration {
	if rules == nil {
		return 0
	}
	var maxPeriod time.Duration
	for _, policy := range rules.Policies {
		if policy == nil {
			continue
		}
		d := time.Duration(policy.PeriodSeconds) * time.Second
		if d > maxPeriod {
			maxPeriod = d
		}
	}
	return maxPeriod
}

// MarkScaleEvent records a scaling event when the desired instance count differs from the current instance count.
func (s *ScalingStabilizer) MarkScaleEvent(behavior *scaling.Behavior, t time.Time, currentInstanceCount, desiredInstanceCount int) {
	if desiredInstanceCount == currentInstanceCount {
		return
	}
	if behavior == nil || (behavior.ScaleUp == nil && behavior.ScaleDown == nil) {
		return
	}

	if desiredInstanceCount > currentInstanceCount {
		s.scaleUpEvents = processScaleEvent(s.scaleUpEvents, behavior.GetScaleUp(), desiredInstanceCount-currentInstanceCount, t)
	} else {
		s.scaleDownEvents = processScaleEvent(s.scaleDownEvents, behavior.GetScaleDown(), currentInstanceCount-desiredInstanceCount, t)
	}
}

func processScaleEvent(scaleEvents []dataPoint, rules *scaling.ScalingRules, instanceCountChange int, t time.Time) []dataPoint {
	longestPolicyPeriod := getLongestPolicyPeriod(rules)
	cutoff := time.Now().Add(-longestPolicyPeriod)

	filtered := scaleEvents[:0]
	for _, dp := range scaleEvents {
		if !dp.timestamp.Before(cutoff) {
			filtered = append(filtered, dp)
		}
	}
	return append(filtered, dataPoint{timestamp: t, value: instanceCountChange})
}

func getInstanceCountChangeForPeriod(t time.Time, lookbackPeriod time.Duration, scaleEvents []dataPoint) int {
	cutoff := t.Add(-lookbackPeriod)
	sum := 0
	for _, ev := range scaleEvents {
		if ev.timestamp.After(cutoff) {
			sum += ev.value
		}
	}
	return sum
}

// GetStabilizedRecommendation returns a recommendation for scaling bounded by the stabilization window and rate policies.
// Lookback cutoffs are determined using the specified time and are exclusive.
func (s *ScalingStabilizer) GetStabilizedRecommendation(
	behavior *scaling.Behavior,
	t time.Time,
	currentInstanceCount int,
	recommendedInstances int,
	workloadName string,
) int {
	if currentInstanceCount == 0 && recommendedInstances > 0 {
		// Record it like any other recommendation: the scale-down stabilization window has to
		// cover the instance started here, or the next lower recommendation removes it at once.
		s.recommendations = append(s.recommendations, dataPoint{timestamp: t, value: recommendedInstances})
		return 1
	}
	bounds := &boundsDebugInfo{}
	boundedRecommendation := recommendedInstances
	if recommendedInstances > currentInstanceCount {
		boundedRecommendation = s.getBoundedRecommendationForScaleUp(behavior, t, currentInstanceCount, recommendedInstances, bounds)
		if boundedRecommendation < recommendedInstances {
			s.logger.Info(fmt.Sprintf("For workload %s, scale up is bounded down to %d from the recommendation of %d", workloadName, boundedRecommendation, recommendedInstances))
		}
	} else if recommendedInstances < currentInstanceCount {
		boundedRecommendation = s.getBoundedRecommendationForScaleDown(behavior, t, currentInstanceCount, recommendedInstances, bounds)
		if boundedRecommendation > recommendedInstances {
			s.logger.Info(fmt.Sprintf("For workload %s, scale down is bounded up to %d from the recommendation of %d", workloadName, boundedRecommendation, recommendedInstances))
		}
	}

	// Always record the recommendation, even if it's unchanged.
	s.recommendations = append(s.recommendations, dataPoint{timestamp: t, value: recommendedInstances})

	if recommendedInstances > currentInstanceCount {
		s.logger.Info(fmt.Sprintf("Scaling is bounded for %s: %s", workloadName, bounds.toScaleUpBoundDebugString()))
	} else if recommendedInstances < currentInstanceCount {
		s.logger.Info(fmt.Sprintf("Scaling is bounded for %s: %s", workloadName, bounds.toScaleDownBoundDebugString()))
	}

	return boundedRecommendation
}

func (s *ScalingStabilizer) getBoundedRecommendationForScaleUp(
	behavior *scaling.Behavior,
	t time.Time,
	currentInstanceCount int,
	recommendedInstances int,
	debug *boundsDebugInfo,
) int {
	if behavior == nil || behavior.ScaleUp == nil {
		return recommendedInstances
	}

	scaleUp := behavior.ScaleUp
	stabilizationBound := s.stabilizeRecommendation(behavior, t, currentInstanceCount, recommendedInstances, debug)
	rateLimitBound, hasRateLimit := s.rateLimitScaleUpRecommendation(t, currentInstanceCount, scaleUp, debug)

	scaleUpBound := min(stabilizationBound, recommendedInstances)
	if hasRateLimit {
		scaleUpBound = min(rateLimitBound, scaleUpBound)
	}
	return scaleUpBound
}

func (s *ScalingStabilizer) rateLimitScaleUpRecommendation(
	t time.Time,
	currentInstanceCount int,
	scaleUp *scaling.ScalingRules,
	debug *boundsDebugInfo,
) (int, bool) {
	comparator := func(a, b int) int { return max(a, b) }
	scaleUpBound := 0

	if scaleUp.SelectPolicy == scaling.SelectPolicyDisabled {
		debug.setSelectorDebugString(scaling.SelectPolicyDisabled)
		return currentInstanceCount, true
	} else if len(scaleUp.Policies) == 0 {
		return 0, false
	} else if scaleUp.SelectPolicy == scaling.SelectPolicyMin {
		debug.setSelectorDebugString(scaling.SelectPolicyMin)
		comparator = func(a, b int) int { return min(a, b) }
		scaleUpBound = maxInstances
	} else if scaleUp.SelectPolicy == scaling.SelectPolicyMax {
		debug.setSelectorDebugString(scaling.SelectPolicyMax)
		comparator = func(a, b int) int { return max(a, b) }
		scaleUpBound = 0
	}

	for _, policy := range scaleUp.Policies {
		period := time.Duration(policy.PeriodSeconds) * time.Second
		instancesAddedInPeriod := getInstanceCountChangeForPeriod(t, period, s.scaleUpEvents)
		instancesRemovedInPeriod := getInstanceCountChangeForPeriod(t, period, s.scaleDownEvents)
		periodStartInstances := currentInstanceCount - instancesAddedInPeriod + instancesRemovedInPeriod

		var bound int
		switch policy.Type {
		case scaling.PolicyTypePercent:
			// Round up away from zero to match Java BigDecimal.setScale(0, RoundingMode.UP)
			bound = roundUpAwayFromZeroToSaturatedInt(float64(periodStartInstances) * (100.0 + float64(policy.Value)) / 100.0)
		case scaling.PolicyTypeInstances:
			bound = clampToInt32(int64(periodStartInstances) + int64(policy.Value))
		default:
			panic(fmt.Sprintf("unsupported policy type: %v", policy.Type))
		}

		debug.addScaleUp(policy.Type.String(), bound)
		scaleUpBound = comparator(scaleUpBound, bound)
	}

	return scaleUpBound, true
}

func (s *ScalingStabilizer) getBoundedRecommendationForScaleDown(
	behavior *scaling.Behavior,
	t time.Time,
	currentInstanceCount int,
	recommendedInstances int,
	debug *boundsDebugInfo,
) int {
	if behavior == nil || behavior.ScaleDown == nil {
		return recommendedInstances
	}

	scaleDown := behavior.ScaleDown
	stabilizationBound := s.stabilizeRecommendation(behavior, t, currentInstanceCount, recommendedInstances, debug)
	rateLimitBound, hasRateLimit := s.rateLimitScaleDownRecommendation(t, currentInstanceCount, scaleDown, debug)

	scaleDownBound := max(stabilizationBound, recommendedInstances)
	if hasRateLimit {
		scaleDownBound = max(rateLimitBound, scaleDownBound)
	}
	return scaleDownBound
}

func (s *ScalingStabilizer) rateLimitScaleDownRecommendation(
	t time.Time,
	currentInstanceCount int,
	scaleDown *scaling.ScalingRules,
	debug *boundsDebugInfo,
) (int, bool) {
	comparator := func(a, b int) int { return min(a, b) }
	scaleDownBound := maxInstances

	if scaleDown.SelectPolicy == scaling.SelectPolicyDisabled {
		debug.setSelectorDebugString(scaling.SelectPolicyDisabled)
		return currentInstanceCount, true
	} else if len(scaleDown.Policies) == 0 {
		return 0, false
	} else if scaleDown.SelectPolicy == scaling.SelectPolicyMin {
		debug.setSelectorDebugString(scaling.SelectPolicyMin)
		comparator = func(a, b int) int { return max(a, b) }
		scaleDownBound = minInstances
	} else if scaleDown.SelectPolicy == scaling.SelectPolicyMax {
		debug.setSelectorDebugString(scaling.SelectPolicyMax)
		comparator = func(a, b int) int { return min(a, b) }
		scaleDownBound = maxInstances
	}

	for _, policy := range scaleDown.Policies {
		period := time.Duration(policy.PeriodSeconds) * time.Second
		instancesAddedInPeriod := getInstanceCountChangeForPeriod(t, period, s.scaleUpEvents)
		instancesRemovedInPeriod := getInstanceCountChangeForPeriod(t, period, s.scaleDownEvents)
		periodStartInstances := currentInstanceCount - instancesAddedInPeriod + instancesRemovedInPeriod

		var bound int
		switch policy.Type {
		case scaling.PolicyTypePercent:
			// Round down to ensure we're able to scale down: trunc(periodStartInstances * (100 - value) / 100)
			bound = truncToSaturatedInt(float64(periodStartInstances) * (100.0 - float64(policy.Value)) / 100.0)
		case scaling.PolicyTypeInstances:
			bound = clampToInt32(int64(periodStartInstances) - int64(policy.Value))
		default:
			panic(fmt.Sprintf("unsupported policy type: %v", policy.Type))
		}

		debug.addScaleDown(policy.Type.String(), bound)
		scaleDownBound = comparator(scaleDownBound, bound)
	}

	return scaleDownBound, true
}

func (s *ScalingStabilizer) stabilizeRecommendation(
	behavior *scaling.Behavior,
	t time.Time,
	currentInstanceCount int,
	recommendedInstances int,
	debug *boundsDebugInfo,
) int {
	scaleUpStabilizationCutoff := getStabilizationCutoff(t, behavior.GetScaleUp())
	scaleDownStabilizationCutoff := getStabilizationCutoff(t, behavior.GetScaleDown())

	minInUpStabilizationWindow := recommendedInstances
	maxInDownStabilizationWindow := recommendedInstances

	filtered := s.recommendations[:0]
	for i, recommendation := range s.recommendations {
		if recommendation.timestamp.After(t) {
			filtered = append(filtered, s.recommendations[i:]...)
			break
		}
		if recommendation.timestamp.After(scaleUpStabilizationCutoff) {
			minInUpStabilizationWindow = min(minInUpStabilizationWindow, recommendation.value)
		}
		if recommendation.timestamp.After(scaleDownStabilizationCutoff) {
			maxInDownStabilizationWindow = max(maxInDownStabilizationWindow, recommendation.value)
		}
		if !(recommendation.timestamp.Before(scaleUpStabilizationCutoff) && recommendation.timestamp.Before(scaleDownStabilizationCutoff)) {
			filtered = append(filtered, recommendation)
		}
	}
	s.recommendations = filtered

	stabilizedRecommendation := currentInstanceCount
	if stabilizedRecommendation < minInUpStabilizationWindow {
		stabilizedRecommendation = minInUpStabilizationWindow
	}
	if stabilizedRecommendation > maxInDownStabilizationWindow {
		stabilizedRecommendation = maxInDownStabilizationWindow
	}

	if recommendedInstances > currentInstanceCount {
		debug.addScaleUp("STABILIZATION", minInUpStabilizationWindow)
	} else if recommendedInstances < currentInstanceCount {
		debug.addScaleDown("STABILIZATION", maxInDownStabilizationWindow)
	}

	return stabilizedRecommendation
}

func getStabilizationCutoff(t time.Time, rules *scaling.ScalingRules) time.Time {
	if rules == nil || rules.StabilizationWindowSeconds == 0 {
		return t
	}
	return t.Add(-time.Duration(rules.StabilizationWindowSeconds) * time.Second)
}

func truncToSaturatedInt(v float64) int {
	t := math.Trunc(v)
	if math.IsNaN(t) {
		return 0
	}
	if t >= float64(math.MaxInt32) {
		return math.MaxInt32
	}
	if t <= float64(math.MinInt32) {
		return math.MinInt32
	}
	return int(t)
}

func roundUpAwayFromZeroToSaturatedInt(v float64) int {
	if v < 0 {
		f := math.Floor(v)
		if f <= float64(math.MinInt32) {
			return math.MinInt32
		}
		return int(f)
	}
	return ceilToSaturatedInt(v)
}

func clampToInt32(v int64) int {
	if v >= math.MaxInt32 {
		return math.MaxInt32
	}
	if v <= math.MinInt32 {
		return math.MinInt32
	}
	return int(v)
}
