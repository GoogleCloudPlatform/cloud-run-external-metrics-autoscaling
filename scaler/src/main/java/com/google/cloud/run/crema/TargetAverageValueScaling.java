/*
 * Copyright 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package com.google.cloud.run.crema;

/** Scaling based on current average value and a target value. */
public final class TargetAverageValueScaling {
  /**
   * Make a scaling recommendation based on the current value, the configured target value, and the
   * current instance count. This scaling method assumes the current value is an aggregate across
   * all instances and the target value is a per-instance target.
   *
   * <p>In the case that the targetValue is 0, the current instance count is returned for
   * consistency with HPA's behavior.
   *
   * @param currentValue The current value of the metric.
   * @param targetValue The configured target value for the metric. Must be greater than 0.
   * @param currentInstanceCount The current number of instances. This is not used in the main
   *     calculation for average value scaling, but is kept for API consistency and for the case
   *     where targetValue is 0.
   * @return A recommendation for the number of instances.
   */
  public static int makeRecommendation(
      double currentValue, double targetValue, int currentInstanceCount) {
    if (targetValue == 0) {
      return currentInstanceCount;
    }
    return (int) Math.ceil(currentValue / targetValue);
  }

  private TargetAverageValueScaling() {}
}
