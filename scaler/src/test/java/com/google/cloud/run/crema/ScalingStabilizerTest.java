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

import com.google.cloud.run.crema.Advanced.ScalerConfig.Behavior;
import com.google.cloud.run.crema.Advanced.ScalerConfig.Behavior.Scaling;
import com.google.cloud.run.crema.Advanced.ScalerConfig.Behavior.Scaling.Policy;
import java.time.Duration;
import java.time.Instant;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** */
@RunWith(JUnit4.class)
public final class ScalingStabilizerTest {

  @Test
  public void scalingStablizer_withEmptyConfig() {
    Behavior behavior = Behavior.getDefaultInstance();
    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, Instant.now(), 5, 1000, "test-workload"))
        .isEqualTo(1000);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, Instant.now(), 1000, 1, "test-workload"))
        .isEqualTo(1);
  }

  @Test
  public void
      getStabilizedRecommendation_scaleUpStabilizationOnly_returnsStabilizationWindowBound() {
    Scaling scaling = Scaling.newBuilder().setStabilizationWindowSeconds(300).build();
    Behavior behavior = Behavior.newBuilder().setScaleUp(scaling).build();

    Instant now = Instant.now();
    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 100, 110, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(120), 100, 110, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(180), 100, 110, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(240), 100, 110, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(301), 100, 110, "test-workload"))
        .isEqualTo(110);
  }

  @Test
  public void
      getStabilizedRecommendation_scaleDownStabilizationOnly_returnsStabilizationWindowBound() {
    Scaling scaling = Scaling.newBuilder().setStabilizationWindowSeconds(300).build();
    Behavior behavior = Behavior.newBuilder().setScaleDown(scaling).build();

    Instant now = Instant.now();
    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 100, 90, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(120), 100, 90, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(180), 100, 90, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(240), 100, 90, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(301), 100, 90, "test-workload"))
        .isEqualTo(90);
  }

  @Test
  public void
      getStabilizedRecommendation_unchangedRecommendation_countsTowardScaleUpStabilization() {
    Behavior behavior =
        Behavior.newBuilder()
            .setScaleUp(Scaling.newBuilder().setStabilizationWindowSeconds(300).build())
            .build();
    Instant now = Instant.now();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plus(Duration.ofMinutes(4)), 100, 100, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plus(Duration.ofMinutes(6)), 100, 110, "test-workload"))
        .isEqualTo(100);
  }

  @Test
  public void
      getStabilizedRecommendation_unchangedRecommendation_countsTowardScaleDownStabilization() {
    Behavior behavior =
        Behavior.newBuilder()
            .setScaleDown(Scaling.newBuilder().setStabilizationWindowSeconds(300).build())
            .build();
    Instant now = Instant.now();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plus(Duration.ofMinutes(4)), 100, 100, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plus(Duration.ofMinutes(6)), 100, 90, "test-workload"))
        .isEqualTo(100);
  }

  @Test
  public void getStabilizedRecommendation_scaleDownPercentPolicy_returnsPercentPolicyBound() {
    Policy percentPolicy =
        Policy.newBuilder().setType(Policy.Type.PERCENT).setValue(50).setPeriodSeconds(60).build();
    Behavior behavior =
        Behavior.newBuilder()
            .setScaleDown(Scaling.newBuilder().addPolicies(percentPolicy).build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 10, "test-workload"))
        .isEqualTo(50);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 50);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 50, 10, "test-workload"))
        .isEqualTo(50);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 50, 10, "test-workload"))
        .isEqualTo(25);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(60), 50, 25);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(140), 25, 0, "test-workload"))
        .isEqualTo(12);
  }

  @Test
  public void getStabilizedRecommendation_scaleDownInstancesPolicy_returnsInstancesPolicyBound() {
    Policy instancesPolicy =
        Policy.newBuilder().setType(Policy.Type.INSTANCES).setValue(1).setPeriodSeconds(60).build();
    Behavior behavior =
        Behavior.newBuilder()
            .setScaleDown(Scaling.newBuilder().addPolicies(instancesPolicy).build())
            .build();
    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 90, "test-workload"))
        .isEqualTo(99);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 99);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 99, 90, "test-workload"))
        .isEqualTo(99);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 99, 90, "test-workload"))
        .isEqualTo(98);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(60), 99, 98);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(140), 98, 0, "test-workload"))
        .isEqualTo(97);
  }

  @Test
  public void
      getStabilizedRecommendation_multipleScaleDownPoliciesUnsetSelectPolicy_returnsMinBound() {
    Policy percentPolicy =
        Policy.newBuilder().setType(Policy.Type.PERCENT).setValue(50).setPeriodSeconds(60).build();
    Policy instancesPolicy =
        Policy.newBuilder().setType(Policy.Type.INSTANCES).setValue(1).setPeriodSeconds(60).build();

    Behavior behavior =
        Behavior.newBuilder()
            .setScaleDown(
                Scaling.newBuilder()
                    .addPolicies(instancesPolicy)
                    .addPolicies(percentPolicy)
                    .build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 40, "test-workload"))
        .isEqualTo(50);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 50);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 50, 0, "test-workload"))
        .isEqualTo(50);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 50, 0, "test-workload"))
        .isEqualTo(25);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(61), 50, 25);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(140), 25, 0, "test-workload"))
        .isEqualTo(12);
  }

  @Test
  public void
      getStabilizedRecommendation_multipleScaleDownPoliciesMaxSelectPolicy_returnsMinBound() {
    Policy percentPolicy =
        Policy.newBuilder().setType(Policy.Type.PERCENT).setValue(50).setPeriodSeconds(60).build();
    Policy instancesPolicy =
        Policy.newBuilder().setType(Policy.Type.INSTANCES).setValue(1).setPeriodSeconds(60).build();

    Behavior behavior =
        Behavior.newBuilder()
            .setScaleDown(
                Scaling.newBuilder()
                    .addPolicies(instancesPolicy)
                    .addPolicies(percentPolicy)
                    .setSelectPolicy(Scaling.SelectPolicy.MAX)
                    .build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 40, "test-workload"))
        .isEqualTo(50);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 50);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 50, 0, "test-workload"))
        .isEqualTo(50);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 50, 0, "test-workload"))
        .isEqualTo(25);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(61), 50, 25);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(140), 25, 0, "test-workload"))
        .isEqualTo(12);
  }

  @Test
  public void
      getStabilizedRecommendation_multipleScaleDownPoliciesMinSelectPolicy_returnsMaxBound() {
    Policy percentPolicy =
        Policy.newBuilder().setType(Policy.Type.PERCENT).setValue(50).setPeriodSeconds(60).build();
    Policy instancesPolicy =
        Policy.newBuilder().setType(Policy.Type.INSTANCES).setValue(1).setPeriodSeconds(60).build();

    Behavior behavior =
        Behavior.newBuilder()
            .setScaleDown(
                Scaling.newBuilder()
                    .addPolicies(instancesPolicy)
                    .addPolicies(percentPolicy)
                    .setSelectPolicy(Scaling.SelectPolicy.MIN)
                    .build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 40, "test-workload"))
        .isEqualTo(99);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 99);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 99, 98, "test-workload"))
        .isEqualTo(99);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 99, 90, "test-workload"))
        .isEqualTo(98);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(61), 99, 98);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(140), 98, 0, "test-workload"))
        .isEqualTo(97);
  }

  @Test
  public void getStabilizedRecommendation_scaleUpPercentPolicy_returnsPercentPolicyBound() {
    Policy percentPolicy =
        Policy.newBuilder().setType(Policy.Type.PERCENT).setValue(50).setPeriodSeconds(60).build();

    Behavior behavior =
        Behavior.newBuilder()
            .setScaleUp(Scaling.newBuilder().addPolicies(percentPolicy).build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 200, "test-workload"))
        .isEqualTo(150);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 150);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 150, 200, "test-workload"))
        .isEqualTo(150);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 150, 250, "test-workload"))
        .isEqualTo(225);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(61), 150, 250);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(140), 225, 500, "test-workload"))
        .isEqualTo(338);
  }

  @Test
  public void getStabilizedRecommendation_scaleUpFromZero_isAlwaysAllowed() {
    Policy percentPolicy =
        Policy.newBuilder().setType(Policy.Type.PERCENT).setValue(50).setPeriodSeconds(60).build();

    Behavior behavior =
        Behavior.newBuilder()
            .setScaleUp(
                Scaling.newBuilder()
                    .addPolicies(percentPolicy)
                    .setStabilizationWindowSeconds(300)
                    .build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(0);
    Instant now = Instant.now();

    assertThat(scalingStabilizer.getStabilizedRecommendation(behavior, now, 0, 1, "test-workload"))
        .isEqualTo(1);
  }

  @Test
  public void getStabilizedRecommendation_scaleUpInstancesPolicy_returnsInstancesPolicyBound() {
    Policy instancesPolicy =
        Policy.newBuilder().setType(Policy.Type.INSTANCES).setValue(5).setPeriodSeconds(60).build();

    Behavior behavior =
        Behavior.newBuilder()
            .setScaleUp(Scaling.newBuilder().addPolicies(instancesPolicy).build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 200, "test-workload"))
        .isEqualTo(105);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 105);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 105, 200, "test-workload"))
        .isEqualTo(105);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 105, 200, "test-workload"))
        .isEqualTo(110);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(60), 105, 110);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(140), 110, 500, "test-workload"))
        .isEqualTo(115);
  }

  @Test
  public void
      getStabilizedRecommendation_scaleUpMultiplePoliciesUnsetSelectPolicy_returnsMaxBound() {
    Policy percentPolicy =
        Policy.newBuilder().setType(Policy.Type.PERCENT).setValue(50).setPeriodSeconds(60).build();
    Policy instancesPolicy =
        Policy.newBuilder().setType(Policy.Type.INSTANCES).setValue(5).setPeriodSeconds(60).build();

    Behavior behavior =
        Behavior.newBuilder()
            .setScaleUp(
                Scaling.newBuilder()
                    .addPolicies(instancesPolicy)
                    .addPolicies(percentPolicy)
                    .build())
            .build();
    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 200, "test-workload"))
        .isEqualTo(150);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 150);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 150, 200, "test-workload"))
        .isEqualTo(150);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 150, 250, "test-workload"))
        .isEqualTo(225);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(61), 150, 250);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(140), 225, 500, "test-workload"))
        .isEqualTo(338);
  }

  @Test
  public void getStabilizedRecommendation_scaleUpMultiplePoliciesMaxSelectPolicy_returnsMaxBound() {
    Policy percentPolicy =
        Policy.newBuilder().setType(Policy.Type.PERCENT).setValue(50).setPeriodSeconds(60).build();
    Policy instancesPolicy =
        Policy.newBuilder().setType(Policy.Type.INSTANCES).setValue(5).setPeriodSeconds(60).build();

    Behavior behavior =
        Behavior.newBuilder()
            .setScaleUp(
                Scaling.newBuilder()
                    .addPolicies(instancesPolicy)
                    .addPolicies(percentPolicy)
                    .setSelectPolicy(Scaling.SelectPolicy.MAX)
                    .build())
            .build();
    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 200, "test-workload"))
        .isEqualTo(150);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 150);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 150, 200, "test-workload"))
        .isEqualTo(150);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 150, 250, "test-workload"))
        .isEqualTo(225);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(61), 150, 250);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(140), 225, 500, "test-workload"))
        .isEqualTo(338);
  }

  @Test
  public void
      getStabilizedRecommendation_scaleUpMultiplePoliciesWithMinSelectPolicy_returnsMinBound() {
    Policy percentPolicy =
        Policy.newBuilder().setType(Policy.Type.PERCENT).setValue(100).setPeriodSeconds(90).build();
    Policy instancesPolicy =
        Policy.newBuilder().setType(Policy.Type.INSTANCES).setValue(5).setPeriodSeconds(60).build();

    Behavior behavior =
        Behavior.newBuilder()
            .setScaleUp(
                Scaling.newBuilder()
                    .addPolicies(instancesPolicy)
                    .addPolicies(percentPolicy)
                    .setSelectPolicy(Scaling.SelectPolicy.MIN)
                    .build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 200, "test-workload"))
        .isEqualTo(105);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 105);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 105, 200, "test-workload"))
        .isEqualTo(105);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(75), 105, 200, "test-workload"))
        .isEqualTo(110);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(60), 105, 110);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(140), 110, 500, "test-workload"))
        .isEqualTo(115);
  }

  @Test
  public void getStabilizedRecommendation_scaleUpDisabled_returnsCurrentInstances() {
    Behavior behavior =
        Behavior.newBuilder()
            .setScaleUp(Scaling.newBuilder().setSelectPolicy(Scaling.SelectPolicy.DISABLED).build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 200, "test-workload"))
        .isEqualTo(100);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 200);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 200, 400, "test-workload"))
        .isEqualTo(200);
  }

  @Test
  public void getStabilizedRecommendation_scaleDownDisabled_returnsCurrentInstances() {
    Behavior behavior =
        Behavior.newBuilder()
            .setScaleDown(
                Scaling.newBuilder().setSelectPolicy(Scaling.SelectPolicy.DISABLED).build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(200);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 200, 100, "test-workload"))
        .isEqualTo(200);
    scalingStabilizer.markScaleEvent(behavior, now, 200, 100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 50, "test-workload"))
        .isEqualTo(100);
  }

  @Test
  public void
      getStabilizedRecommendation_scaleUpMultiplePoliciesMaxSelectPolicyAndStabilizationWindow_returnsStabilizationBound() {
    Policy percentPolicy =
        Policy.newBuilder().setType(Policy.Type.PERCENT).setValue(50).setPeriodSeconds(60).build();

    Behavior behavior =
        Behavior.newBuilder()
            .setScaleUp(
                Scaling.newBuilder()
                    .setStabilizationWindowSeconds(60)
                    .addPolicies(percentPolicy)
                    .setSelectPolicy(Scaling.SelectPolicy.MAX)
                    .build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    // stabilizationWindow dominates for configured seconds after startup
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 100, 200, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 100, 200, "test-workload"))
        .isEqualTo(150);
  }

  @Test
  public void
      getStabilizedRecommendation_scaleDownMultiplePoliciesMaxSelectPolicyAndStabilizationWindow_returnsStabilizationBound() {
    Policy percentPolicy =
        Policy.newBuilder().setType(Policy.Type.PERCENT).setValue(50).setPeriodSeconds(60).build();

    Behavior behavior =
        Behavior.newBuilder()
            .setScaleDown(
                Scaling.newBuilder()
                    .setStabilizationWindowSeconds(60)
                    .addPolicies(percentPolicy)
                    .setSelectPolicy(Scaling.SelectPolicy.MAX)
                    .build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    // stabilizationWindow dominates for configured seconds after startup
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 100, 50, "test-workload"))
        .isEqualTo(100);
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 100, 50, "test-workload"))
        .isEqualTo(50);
  }

  @Test
  public void getStabilizedRecommendation_decreasingWindowSize_returnsCorrectBound() {
    Scaling scaling = Scaling.newBuilder().setStabilizationWindowSeconds(300).build();
    Behavior behavior = Behavior.newBuilder().setScaleDown(scaling).build();

    Instant now = Instant.now();
    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 100, 90, "test-workload"))
        .isEqualTo(100);

    Behavior decreasedStabilizationWindow =
        Behavior.newBuilder()
            .setScaleDown(Scaling.newBuilder().setStabilizationWindowSeconds(60).build())
            .build();

    // This original recommendation of 100 is no longer in the window; only the recent
    // recommendation of 90 is.
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                decreasedStabilizationWindow, now.plusSeconds(119), 100, 80, "test-workload"))
        .isEqualTo(90);
  }

  @Test
  public void getStabilizedRecommendation_increasingWindowSize_returnsCorrectBound() {
    Scaling scaling = Scaling.newBuilder().setStabilizationWindowSeconds(300).build();
    Behavior behavior = Behavior.newBuilder().setScaleDown(scaling).build();

    Instant now = Instant.now();
    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 100, 90, "test-workload"))
        .isEqualTo(100);

    Behavior increasedStabilizationWindow =
        Behavior.newBuilder()
            .setScaleDown(Scaling.newBuilder().setStabilizationWindowSeconds(360).build())
            .build();

    // This original recommendation of 100 should no longer be in the window but it's still
    // considered here because of the increased window size.
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                increasedStabilizationWindow, now.plusSeconds(359), 100, 80, "test-workload"))
        .isEqualTo(100);
  }

  @Test
  public void getStabilizedRecommendation_percentIncreases_returnsCorrectBound() {
    Behavior behavior =
        Behavior.newBuilder()
            .setScaleDown(
                Scaling.newBuilder()
                    .addPolicies(
                        Policy.newBuilder()
                            .setType(Policy.Type.PERCENT)
                            .setValue(50)
                            .setPeriodSeconds(60)
                            .build())
                    .build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 10, "test-workload"))
        .isEqualTo(50);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 50);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 50, 10, "test-workload"))
        .isEqualTo(50);

    Behavior increasedScaledownPercent =
        Behavior.newBuilder()
            .setScaleDown(
                Scaling.newBuilder()
                    .addPolicies(
                        Policy.newBuilder()
                            .setType(Policy.Type.PERCENT)
                            .setValue(80)
                            .setPeriodSeconds(60)
                            .build())
                    .build())
            .build();

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                increasedScaledownPercent, now.plusSeconds(60), 50, 0, "test-workload"))
        .isEqualTo(10);
  }

  @Test
  public void getStabilizedRecommendation_percentDecreases_returnsCorrectBound() {
    Behavior behavior =
        Behavior.newBuilder()
            .setScaleDown(
                Scaling.newBuilder()
                    .addPolicies(
                        Policy.newBuilder()
                            .setType(Policy.Type.PERCENT)
                            .setValue(50)
                            .setPeriodSeconds(60)
                            .build())
                    .build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 10, "test-workload"))
        .isEqualTo(50);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 50);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(59), 50, 10, "test-workload"))
        .isEqualTo(50);

    Behavior increasedScaledownPercent =
        Behavior.newBuilder()
            .setScaleDown(
                Scaling.newBuilder()
                    .addPolicies(
                        Policy.newBuilder()
                            .setType(Policy.Type.PERCENT)
                            .setValue(20)
                            .setPeriodSeconds(60)
                            .build())
                    .build())
            .build();

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                increasedScaledownPercent, now.plusSeconds(60), 50, 0, "test-workload"))
        .isEqualTo(40);
  }

  @Test
  public void getStabilizedRecommendation_rateWindowIncreases_returnsCorrectBound() {
    Behavior behavior =
        Behavior.newBuilder()
            .setScaleDown(
                Scaling.newBuilder()
                    .addPolicies(
                        Policy.newBuilder()
                            .setType(Policy.Type.PERCENT)
                            .setValue(50)
                            .setPeriodSeconds(60)
                            .build())
                    .build())
            .build();

    ScalingStabilizer scalingStabilizer = new ScalingStabilizer(100);

    Instant now = Instant.now();
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(behavior, now, 100, 10, "test-workload"))
        .isEqualTo(50);
    scalingStabilizer.markScaleEvent(behavior, now, 100, 50);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(60), 50, 45, "test-workload"))
        .isEqualTo(45);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(60), 50, 45);

    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                behavior, now.plusSeconds(90), 45, 40, "test-workload"))
        .isEqualTo(40);
    scalingStabilizer.markScaleEvent(behavior, now.plusSeconds(90), 45, 40);

    Behavior decreasedScaleDownWindow =
        Behavior.newBuilder()
            .setScaleDown(
                Scaling.newBuilder()
                    .addPolicies(
                        Policy.newBuilder()
                            .setType(Policy.Type.PERCENT)
                            .setValue(50)
                            .setPeriodSeconds(30)
                            .build())
                    .build())
            .build();

    // The window decreased to only include `now + 90`; if it included `now + 60`, the bound would
    // have been 22.
    assertThat(
            scalingStabilizer.getStabilizedRecommendation(
                decreasedScaleDownWindow, now.plusSeconds(120), 40, 0, "test-workload"))
        .isEqualTo(20);
  }
}
