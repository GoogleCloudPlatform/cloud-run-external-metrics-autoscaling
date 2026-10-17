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
	"math"
	"testing"
)

func TestTargetValueScaling(t *testing.T) {
	t.Run("makeRecommendation_currentValueLessThanTarget_scalesDown", func(t *testing.T) {
		if got := MakeTargetValueRecommendation(50.0, 100.0, 10); got != 5 {
			t.Errorf("got %d, want 5", got)
		}
	})

	t.Run("makeRecommendation_currentValueGreaterThanTarget_scalesUp", func(t *testing.T) {
		if got := MakeTargetValueRecommendation(200.0, 100.0, 20); got != 40 {
			t.Errorf("got %d, want 40", got)
		}
	})

	t.Run("makeRecommendation_currentValueEqualsTarget_noChange", func(t *testing.T) {
		if got := MakeTargetValueRecommendation(100.0, 100.0, 15); got != 15 {
			t.Errorf("got %d, want 15", got)
		}
	})

	t.Run("makeRecommendation_usesCeilingForScalingUp", func(t *testing.T) {
		if got := MakeTargetValueRecommendation(101.0, 100.0, 10); got != 11 {
			t.Errorf("got %d, want 11", got)
		}
	})

	t.Run("makeRecommendation_usesCeilingForScalingDown", func(t *testing.T) {
		if got := MakeTargetValueRecommendation(99.0, 100.0, 20); got != 20 {
			t.Errorf("got %d, want 20", got)
		}
	})

	t.Run("makeRecommendation_withZeroInstances_scalesUpToOne", func(t *testing.T) {
		if got := MakeTargetValueRecommendation(100.0, 100.0, 0); got != 1 {
			t.Errorf("got %d, want 1", got)
		}
	})

	t.Run("makeRecommendation_withZeroCurrentValue_returnsZero", func(t *testing.T) {
		if got := MakeTargetValueRecommendation(0.0, 100.0, 10); got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("makeRecommendation_withZeroTargetValue_returnsCurrentInstanceCount", func(t *testing.T) {
		if got := MakeTargetValueRecommendation(100.0, 0.0, 10); got != 10 {
			t.Errorf("got %d, want 10", got)
		}
	})

	t.Run("makeRecommendation_withOverflowOrInfinity_saturatesToMaxInt32", func(t *testing.T) {
		if got := MakeTargetValueRecommendation(1e300, 1e-300, 10); got != math.MaxInt32 {
			t.Errorf("got %d, want %d", got, math.MaxInt32)
		}
		if got := MakeTargetValueRecommendation(math.Inf(1), 100.0, 10); got != math.MaxInt32 {
			t.Errorf("got %d, want %d", got, math.MaxInt32)
		}
	})

	t.Run("makeRecommendation_withNaN_returnsZero", func(t *testing.T) {
		if got := MakeTargetValueRecommendation(math.NaN(), 100.0, 10); got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})
}
