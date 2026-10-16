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

import static com.google.common.truth.Truth.assertThat;

import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class TargetValueScalingTest {

  @Test
  public void makeRecommendation_currentValueLessThanTarget_scalesDown() {
    // currentInstanceCount * currentValue / targetValue
    // 10 * 50 / 100 = 5
    int actual =
        TargetValueScaling.makeRecommendation(
            /* currentValue= */ 50.0, /* targetValue= */ 100.0, /* currentInstanceCount= */ 10);
    assertThat(actual).isEqualTo(5);
  }

  @Test
  public void makeRecommendation_currentValueGreaterThanTarget_scalesUp() {
    // 20 * 200 / 100 = 40
    int actual =
        TargetValueScaling.makeRecommendation(
            /* currentValue= */ 200.0, /* targetValue= */ 100.0, /* currentInstanceCount= */ 20);
    assertThat(actual).isEqualTo(40);
  }

  @Test
  public void makeRecommendation_currentValueEqualsTarget_noChange() {
    // 15 * 100 / 100 = 15
    int actual = TargetValueScaling.makeRecommendation(/*currentValue=*/100.0, /*targetValue=*/100.0, /*currentInstanceCount=*/15);
    assertThat(actual).isEqualTo(15);
  }

  @Test
  public void makeRecommendation_usesCeilingForScalingUp() {
    // 10 * 101 / 100 = 10.1 -> 11
    int actual =
        TargetValueScaling.makeRecommendation(
            /* currentValue= */ 101.0, /* targetValue= */ 100.0, /* currentInstanceCount= */ 10);
    assertThat(actual).isEqualTo(11);
  }

  @Test
  public void makeRecommendation_usesCeilingForScalingDown() {
    // 20 * 99 / 100 = 19.8 -> 20
    int actual =
        TargetValueScaling.makeRecommendation(
            /* currentValue= */ 99.0, /* targetValue= */ 100.0, /* currentInstanceCount= */ 20);
    assertThat(actual).isEqualTo(20);
  }

  @Test
  public void makeRecommendation_withZeroInstances_scalesUpToOne() {
    // max(0, 1) * 100 / 100 = 1
    int actual =
        TargetValueScaling.makeRecommendation(
            /* currentValue= */ 100.0, /* targetValue= */ 100.0, /* currentInstanceCount= */ 0);
    assertThat(actual).isEqualTo(1);
  }

  @Test
  public void makeRecommendation_withZeroCurrentValue_returnsZero() {
    // 10 * 0 / 100 = 0
    int actual =
        TargetValueScaling.makeRecommendation(
            /* currentValue= */ 0.0, /* targetValue= */ 100.0, /* currentInstanceCount= */ 10);
    assertThat(actual).isEqualTo(0);
  }

  @Test
  public void makeRecommendation_withZeroTargetValue_returnsCurrentInstanceCount() {
    int currentInstanceCount = 10;
    int actual =
        TargetValueScaling.makeRecommendation(
            /* currentValue= */ 100.0,
            /* targetValue= */ 0.0,
            /* currentInstanceCount= */ currentInstanceCount);
    assertThat(actual).isEqualTo(currentInstanceCount);
  }
}
