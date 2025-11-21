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

import static java.lang.Math.max;
import static java.lang.Math.min;

import com.google.cloud.run.crema.Advanced.ScalerConfig.Behavior;
import com.google.cloud.run.crema.Advanced.ScalerConfig.Behavior.Scaling;
import com.google.cloud.run.crema.Advanced.ScalerConfig.Behavior.Scaling.Policy;
import com.google.common.flogger.FluentLogger;
import java.math.BigDecimal;
import java.math.MathContext;
import java.math.RoundingMode;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Deque;
import java.util.Iterator;
import java.util.List;
import java.util.Optional;
import java.util.function.BiFunction;

/**
 * Class to apply bounds to scaling up/down based on the provided scaling config.
 *
 * <p>Scaling is stabilized relative to the mins and maxes of the period. This class's consumers are
 * responsible for marking recommendation and instance counts, and this class will look back for the
 * periods specified in the config for mins and maxes to determine bounds.
 */
public class ScalingStabilizer {
  private static final FluentLogger logger = FluentLogger.forEnclosingClass();

  private static class BoundsDebugInfo {

    private final List<String> scaleUp = new ArrayList<>();
    private final List<String> scaleDown = new ArrayList<>();
    private String selectorDebugString = "";

    private void setSelectorDebugString(Scaling.SelectPolicy selector) {
      if (!selectorDebugString.isEmpty()) {
        // This should never happen since it's only set once per scale up or scale down path and
        // only one of those two paths should run, not both.
        throw new IllegalStateException("Selector debug string already set.");
      }

      selectorDebugString = String.format("{Select: %s}", selector);
    }

    public void addScaleUp(String policyType, int limit) {
      scaleUp.add(String.format("{Policy: %s, Limit: %d}", policyType, limit));
    }

    public void addScaleDown(String policyType, int limit) {
      scaleDown.add(String.format("{Policy: %s, Limit: %d}", policyType, limit));
    }

    public String toScaleUpBoundDebugString() {
      return toString("up", scaleUp);
    }

    public String toScaleDownBoundDebugString() {
      return toString("down", scaleDown);
    }

    private String toString(String direction, List<String> debugStrings) {
      if (!selectorDebugString.isEmpty()) {
        debugStrings.add(0, selectorDebugString);
      }

      return String.format("Scale-%s: %s", direction, String.join("; ", debugStrings));
    }
  }

  private record DataPoint(Instant timestamp, int value) {}

  private static final int MIN_INSTANCES = 0;
  private static final int MAX_INSTANCES = Integer.MAX_VALUE;

  private final Deque<DataPoint> scaleUpEvents = new ArrayDeque<>();
  private final Deque<DataPoint> scaleDownEvents = new ArrayDeque<>();
  private final Deque<DataPoint> recommendations = new ArrayDeque<>();

  public ScalingStabilizer(int currentInstanceCount) {
    recordInitialRecommendation(Instant.now(), currentInstanceCount);
  }

  private void recordInitialRecommendation(Instant time, int recommendedInstances) {
    recommendations.add(new DataPoint(time, recommendedInstances));
  }

  private Duration getLongestPolicyPeriod(Scaling scaling) {
    return scaling.getPoliciesList().stream()
        .map(policy -> Duration.ofSeconds(policy.getPeriodSeconds()))
        .max(Duration::compareTo)
        .orElse(Duration.ZERO);
  }

  // This should only be called when the desired instance count is different from the current
  // instance count. No-op otherwise.
  public void markScaleEvent(
      Behavior behavior, Instant time, int currentInstanceCount, int desiredInstanceCount) {
    if (desiredInstanceCount == currentInstanceCount) {
      // Don't keep track of non-changes.
      return;
    }

    if (!behavior.hasScaleUp() && !behavior.hasScaleDown()) {
      // Don't keep track of scaling events since they won't be used.
      return;
    }

    if (desiredInstanceCount > currentInstanceCount) {
      processScaleEvent(
          scaleUpEvents, behavior.getScaleUp(), desiredInstanceCount - currentInstanceCount, time);
    } else {
      processScaleEvent(
          scaleDownEvents,
          behavior.getScaleDown(),
          currentInstanceCount - desiredInstanceCount,
          time);
    }
  }

  private void processScaleEvent(
      Deque<DataPoint> scaleEvents, Scaling scaling, int instanceCountChange, Instant time) {
    Duration longestPolicyPeriod = getLongestPolicyPeriod(scaling);
    Instant cutoff = Instant.now().minus(longestPolicyPeriod);

    scaleEvents.removeIf((DataPoint dataPoint) -> dataPoint.timestamp().isBefore(cutoff));
    DataPoint newSample = new DataPoint(time, instanceCountChange);
    scaleEvents.add(newSample);
  }

  private int getInstanceCountChangeForPeriod(
      Instant time, Duration lookbackPeriod, Deque<DataPoint> scaleEvents) {
    Instant cutoff = time.minus(lookbackPeriod);
    return scaleEvents.stream()
        .filter(scaleEvent -> scaleEvent.timestamp().isAfter(cutoff))
        .mapToInt(DataPoint::value)
        .sum();
  }

  // Returns a recommendation for scaling that's bounded by the stabilization window and rate
  // policies.
  //
  // Lookback cutoffs are determined using the specified `time` and are exclusive.
  //
  // IMPORTANT: This function only works for 1->N and N->1 scaling.
  public int getStabilizedRecommendation(
      Behavior behavior,
      Instant time,
      int currentInstanceCount,
      int recommendedInstances,
      String workloadName) {
    if (currentInstanceCount == 0 && recommendedInstances > 0) {
      return 1;
    }
    BoundsDebugInfo bounds = new BoundsDebugInfo();
    int boundedRecommendation = recommendedInstances;
    if (recommendedInstances > currentInstanceCount) {
      boundedRecommendation =
          getBoundedRecommendationForScaleUp(
              behavior, time, currentInstanceCount, recommendedInstances, bounds);
      if (boundedRecommendation < recommendedInstances) {
        logger.atInfo().log(
            "[SCALER] For workload %s, scale up is bounded down to %d from the recommendation of"
                + " %d",
            workloadName, boundedRecommendation, recommendedInstances);
      }
    } else if (recommendedInstances < currentInstanceCount) {
      boundedRecommendation =
          getBoundedRecommendationForScaleDown(
              behavior, time, currentInstanceCount, recommendedInstances, bounds);
      if (boundedRecommendation > recommendedInstances) {
        logger.atInfo().log(
            "[SCALER] For workload %s, scale down is bounded up to %d from the recommendation of"
                + " %d",
            workloadName, boundedRecommendation, recommendedInstances);
      }
    }

    // Always record the recommendation, even if it's unchanged.
    recommendations.add(new DataPoint(time, recommendedInstances));

    if (recommendedInstances > currentInstanceCount) {
      logger.atInfo().log(
          "[SCALER] Scaling is bounded for %s: %s",
          workloadName, bounds.toScaleUpBoundDebugString());
    } else if (recommendedInstances < currentInstanceCount) {
      logger.atInfo().log(
          "[SCALER] Scaling is bounded for %s: %s",
          workloadName, bounds.toScaleDownBoundDebugString());
    }

    return boundedRecommendation;
  }

  // Applies stabilization and rate limiting scale up policies to a newly recommended instance
  // count.
  //
  // Returns an instance count that respects the bounds set by scaling config.
  private int getBoundedRecommendationForScaleUp(
      Behavior behavior,
      Instant time,
      int currentInstanceCount,
      int recommendedInstances,
      BoundsDebugInfo debug) {
    if (!behavior.hasScaleUp()) {
      return recommendedInstances;
    }

    Scaling scaleUp = behavior.getScaleUp();
    Optional<Integer> stabilizationBound =
        stabilizeRecommendation(behavior, time, currentInstanceCount, recommendedInstances, debug);
    Optional<Integer> rateLimitBound =
        rateLimitScaleUpRecommendation(time, currentInstanceCount, scaleUp, debug);

    int scaleUpBound = recommendedInstances;
    if (stabilizationBound.isPresent()) {
      scaleUpBound = min(stabilizationBound.get(), scaleUpBound);
    }
    if (rateLimitBound.isPresent()) {
      scaleUpBound = min(rateLimitBound.get(), scaleUpBound);
    }

    return scaleUpBound;
  }

  // Applies Instances and Percent rate limiting policies to a scale-up recommendation.
  private Optional<Integer> rateLimitScaleUpRecommendation(
      Instant time, int currentInstanceCount, Scaling scaleUp, BoundsDebugInfo debug) {
    // Allow the largest change by default.
    BiFunction<Integer, Integer, Integer> comparator = Math::max;
    int scaleUpBound = 0;

    if (scaleUp.getSelectPolicy() == Scaling.SelectPolicy.DISABLED) {
      // Check disabled because this takes priority over empty policies.
      debug.setSelectorDebugString(Scaling.SelectPolicy.DISABLED);
      return Optional.of(currentInstanceCount);
    } else if (scaleUp.getPoliciesList().isEmpty()) {
      return Optional.empty();
    } else if (scaleUp.getSelectPolicy() == Scaling.SelectPolicy.MIN) {
      debug.setSelectorDebugString(Scaling.SelectPolicy.MIN);
      comparator = Math::min;
      scaleUpBound = MAX_INSTANCES;
    } else if (scaleUp.getSelectPolicy() == Scaling.SelectPolicy.MAX) {
      debug.setSelectorDebugString(Scaling.SelectPolicy.MAX);
      comparator = Math::max;
      scaleUpBound = 0;
    }

    for (Policy policy : scaleUp.getPoliciesList()) {
      int bound;
      int instancesAddedInPeriod =
          getInstanceCountChangeForPeriod(
              time, Duration.ofSeconds(policy.getPeriodSeconds()), scaleUpEvents);
      int instancesRemovedInPeriod =
          getInstanceCountChangeForPeriod(
              time, Duration.ofSeconds(policy.getPeriodSeconds()), scaleDownEvents);
      int periodStartInstances =
          currentInstanceCount - instancesAddedInPeriod + instancesRemovedInPeriod;

      switch (policy.getType()) {
        case PERCENT -> {
          // Round up to ensure we're able to scale up.
          MathContext context = new MathContext(16, RoundingMode.UP);
          BigDecimal startInstances = new BigDecimal(periodStartInstances);
          BigDecimal policyValue = new BigDecimal(policy.getValue());
          bound =
              startInstances
                  .multiply(
                      BigDecimal.ONE.add(policyValue.divide(new BigDecimal(100), context), context),
                      context)
                  .setScale(0, RoundingMode.UP)
                  .intValue();
        }
        case INSTANCES -> bound = periodStartInstances + policy.getValue();
        default ->
            throw new IllegalArgumentException("Unsupported policy type: " + policy.getType());
      }

      debug.addScaleUp(policy.getType().toString(), bound);
      scaleUpBound = comparator.apply(scaleUpBound, bound);
    }

    return Optional.of(scaleUpBound);
  }

  // Applies stabilization and rate limiting scale down policies to a newly recommended instance
  // count.
  //
  // Returns an instance count that respects the bounds set by scaling config.
  private int getBoundedRecommendationForScaleDown(
      Behavior behavior,
      Instant time,
      int currentInstanceCount,
      int recommendedInstances,
      BoundsDebugInfo debug) {
    if (!behavior.hasScaleDown()) {
      return recommendedInstances;
    }

    Scaling scaleDown = behavior.getScaleDown();
    Optional<Integer> stabilizationBound =
        stabilizeRecommendation(behavior, time, currentInstanceCount, recommendedInstances, debug);
    Optional<Integer> rateLimitBound =
        rateLimitScaleDownRecommendation(time, currentInstanceCount, scaleDown, debug);

    int scaleDownBound = recommendedInstances;
    if (stabilizationBound.isPresent()) {
      scaleDownBound = max(stabilizationBound.get(), scaleDownBound);
    }
    if (rateLimitBound.isPresent()) {
      scaleDownBound = max(rateLimitBound.get(), scaleDownBound);
    }

    return scaleDownBound;
  }

  // Applies Instances and Percent rate limiting policies to a scale-down recommendation.
  private Optional<Integer> rateLimitScaleDownRecommendation(
      Instant time, int currentInstanceCount, Scaling scaleDown, BoundsDebugInfo debug) {
    BiFunction<Integer, Integer, Integer> comparator = Math::min;
    int scaleDownBound = MAX_INSTANCES;

    if (scaleDown.getSelectPolicy() == Scaling.SelectPolicy.DISABLED) {
      // Check disabled because this takes priority over empty policies.
      debug.setSelectorDebugString(Scaling.SelectPolicy.DISABLED);
      return Optional.of(currentInstanceCount);
    } else if (scaleDown.getPoliciesList().isEmpty()) {
      return Optional.empty();
    } else if (scaleDown.getSelectPolicy() == Scaling.SelectPolicy.MIN) {
      debug.setSelectorDebugString(Scaling.SelectPolicy.MIN);
      comparator = Math::max;
      scaleDownBound = MIN_INSTANCES;
    } else if (scaleDown.getSelectPolicy() == Scaling.SelectPolicy.MAX) {
      debug.setSelectorDebugString(Scaling.SelectPolicy.MAX);
      comparator = Math::min;
      scaleDownBound = MAX_INSTANCES;
    }

    for (Policy policy : scaleDown.getPoliciesList()) {
      int bound;
      int instancesAddedInPeriod =
          getInstanceCountChangeForPeriod(
              time, Duration.ofSeconds(policy.getPeriodSeconds()), scaleUpEvents);
      int instancesRemovedInPeriod =
          getInstanceCountChangeForPeriod(
              time, Duration.ofSeconds(policy.getPeriodSeconds()), scaleDownEvents);
      int periodStartInstances =
          currentInstanceCount - instancesAddedInPeriod + instancesRemovedInPeriod;

      bound =
          switch (policy.getType()) {
            case PERCENT -> {
              // Round down to ensure we're able to scale down.
              MathContext context = new MathContext(16, RoundingMode.DOWN);
              BigDecimal startInstances = new BigDecimal(periodStartInstances);
              BigDecimal policyValue = new BigDecimal(policy.getValue());
              yield startInstances
                  .multiply(
                      BigDecimal.ONE.subtract(
                          policyValue.divide(new BigDecimal(100), context), context),
                      context)
                  .intValue();
            }
            case INSTANCES -> periodStartInstances - policy.getValue();
            default ->
                throw new IllegalArgumentException("Unsupported policy type: " + policy.getType());
          };
      debug.addScaleDown(policy.getType().toString(), bound);
      scaleDownBound = comparator.apply(scaleDownBound, bound);
    }

    return Optional.of(scaleDownBound);
  }

  // Prevents scaling up/down until all signals in the lookback period are consistent.
  private Optional<Integer> stabilizeRecommendation(
      Behavior behavior,
      Instant time,
      int currentInstanceCount,
      int recommendedInstances,
      BoundsDebugInfo debug) {
    Instant scaleUpStabilizationCutoff = getStabilizationCutoff(time, behavior.getScaleUp());
    Instant scaleDownStabilizationCutoff = getStabilizationCutoff(time, behavior.getScaleDown());

    int minInUpStabilizationWindow = recommendedInstances;
    int maxInDownStabilizationWindow = recommendedInstances;

    Iterator<DataPoint> iterator = recommendations.iterator();
    while (iterator.hasNext()) {
      DataPoint recommendation = iterator.next();
      if (recommendation.timestamp().isAfter(time)) {
        // We're done searching once we reach this because the list is sorted by time.
        break;
      }
      if (recommendation.timestamp().isAfter(scaleUpStabilizationCutoff)) {
        minInUpStabilizationWindow = min(minInUpStabilizationWindow, recommendation.value());
      }
      if (recommendation.timestamp().isAfter(scaleDownStabilizationCutoff)) {
        maxInDownStabilizationWindow = max(maxInDownStabilizationWindow, recommendation.value());
      }
      if (recommendation.timestamp().isBefore(scaleUpStabilizationCutoff)
          && recommendation.timestamp().isBefore(scaleDownStabilizationCutoff)) {
        iterator.remove();
      }
    }

    int stabilizedRecommendation = currentInstanceCount;
    if (stabilizedRecommendation < minInUpStabilizationWindow) {
      stabilizedRecommendation = minInUpStabilizationWindow;
    }
    if (stabilizedRecommendation > maxInDownStabilizationWindow) {
      stabilizedRecommendation = maxInDownStabilizationWindow;
    }

    if (recommendedInstances > currentInstanceCount) {
      debug.addScaleUp("STABILIZATION", minInUpStabilizationWindow);
    } else if (recommendedInstances < currentInstanceCount) {
      debug.addScaleDown("STABILIZATION", maxInDownStabilizationWindow);
    }

    return Optional.of(stabilizedRecommendation);
  }

  private static Instant getStabilizationCutoff(Instant time, Scaling scaling) {
    if (scaling == null || scaling.getStabilizationWindowSeconds() == 0) {
      return time;
    }
    return time.minus(Duration.ofSeconds(scaling.getStabilizationWindowSeconds()));
  }
}
