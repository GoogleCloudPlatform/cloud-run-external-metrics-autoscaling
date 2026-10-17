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

// MakeTargetAverageValueRecommendation makes a scaling recommendation based on the current value,
// the configured target value, and the current instance count.
// In the case that targetValue is 0, the current instance count is returned for consistency with HPA's behavior.
func MakeTargetAverageValueRecommendation(currentValue, targetValue float64, currentInstanceCount int) int {
	if targetValue == 0 {
		return currentInstanceCount
	}
	return ceilToSaturatedInt(currentValue / targetValue)
}
