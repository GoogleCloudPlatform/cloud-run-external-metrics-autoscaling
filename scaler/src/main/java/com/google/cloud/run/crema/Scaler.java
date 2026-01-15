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

import static java.lang.Math.clamp;
import static java.lang.Math.max;

import com.google.cloud.run.crema.clients.CloudRunClientWrapper;
import com.google.common.base.Preconditions;
import com.google.common.collect.ImmutableMap;
import com.google.common.flogger.FluentLogger;
import java.io.IOException;
import java.time.Instant;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.ExecutionException;

/**
 * Performs scaling logic for a single ScaledObject.
 *
 * <p>This class takes a configuration of a ScaledObject and its metrics, applies stabilization
 * logic, and updates the number of consumer instances in Cloud Run.
 */
public class Scaler {
  private static final FluentLogger logger = FluentLogger.forEnclosingClass();

  private static final String RECOMMENDED_INSTANCE_COUNT_METRIC_NAME = "recommended_instance_count";
  private static final String REQUESTED_INSTANCE_COUNT_METRIC_NAME = "requested_instance_count";
  private static final String METRIC_VALUE_METRIC_NAME = "metric_value";
  private static final String TARGET_VALUE_METRIC_NAME = "target_value";
  private static final String TARGET_AVERAGE_VALUE_METRIC_NAME = "target_average_value";

  private final Map<String, ScalingStabilizer> scalingStabilizers = new HashMap<>();
  private final CloudRunClientWrapper cloudRunClientWrapper;
  private final MetricsService metricsService;
  private final ConfigurationProvider.StaticConfig staticConfig;
  private final String projectId;

  public Scaler(
      CloudRunClientWrapper cloudRunClientWrapper,
      MetricsService metricsService,
      ConfigurationProvider.StaticConfig config,
      String projectId) {
    this.cloudRunClientWrapper =
        Preconditions.checkNotNull(cloudRunClientWrapper, "Cloud Run client cannot be null.");
    this.metricsService =
        Preconditions.checkNotNull(metricsService, "Metrics service cannot be null.");
    this.staticConfig = Preconditions.checkNotNull(config, "Static config cannot be null.");
    this.projectId = Preconditions.checkNotNull(projectId, "Project ID cannot be null.");
  }

  /**
   * Scales the target Cloud Run service or worker pool based on the provided metrics.
   *
   * <p>A valid trigger is a metric with a non-zero target value. If no valid triggers are found,
   * the scaling for the given workload will be considered failed.
   *
   * @param scaledObjectMetrics The metrics for the scaled object.
   * @throws IOException If an error occurs while communicating with Cloud Run or other services.
   */
  public ScalingStatus scale(ScaledObjectMetrics scaledObjectMetrics)
      throws IOException, ExecutionException, InterruptedException {
    Instant now = Instant.now();

    String workloadName = scaledObjectMetrics.getScaledObject().getScaleTargetRef().getName();
    final WorkloadInfoParser.WorkloadInfo workloadInfo = WorkloadInfoParser.parse(workloadName);

    if (workloadInfo.workloadType() == WorkloadInfoParser.WorkloadType.WORKERPOOL
        && staticConfig.useMinInstances()) {
      throw new IllegalArgumentException(
          "USE_MIN_INSTANCES is not supported for worker pool workloads.");
    }

    int currentInstanceCount =
        InstanceCountProvider.getInstanceCount(cloudRunClientWrapper, workloadInfo);
    logger.atInfo().log("Current instances for %s: %d", workloadName, currentInstanceCount);

    int unboundedRecommendation = 0;
    boolean hasValidTrigger = false;

    if (scaledObjectMetrics.getMetricsCount() == 0) {
      logger.atInfo().log("No metrics configured for %s, scaling down to 0", workloadName);
      updateInstanceCount(0, workloadInfo);
      return ScalingStatus.SUCCEEDED;
    }

    for (Metric metric : scaledObjectMetrics.getMetricsList()) {
      int recommendation;
      if (metric.hasTargetAverageValue() && metric.getTargetAverageValue() > 0) {
        recommendation =
            TargetAverageValueScaling.makeRecommendation(
                metric.getValue(), metric.getTargetAverageValue(), currentInstanceCount);
      } else if (metric.hasTargetValue() && metric.getTargetValue() > 0) {
        recommendation =
            TargetValueScaling.makeRecommendation(
                metric.getValue(), metric.getTargetValue(), currentInstanceCount);
      } else {
        logger.atWarning().log(
            "Target value and target average value for %s are 0. At least one must be a"
                + " non-zero value. Skipping scaling workload %s on the trigger.",
            metric.getTriggerId(), workloadName);
        continue;
      }
      logger.atInfo().log(
          "Recommendation for %s based on scaling trigger %s: %d",
          workloadName, metric.getTriggerId(), recommendation);

      if (staticConfig.outputScalerMetrics()) {
        emitTriggerMetrics(metric, workloadInfo);
      }

      unboundedRecommendation = max(unboundedRecommendation, recommendation);
      hasValidTrigger = true;
    }

    if (!hasValidTrigger) {
      logger.atWarning().log(
          "No valid triggers found for %s. Skipping scaling workload.", workloadName);
      return ScalingStatus.FAILED;
    }

    Advanced.ScalerConfig scalerConfig =
        scaledObjectMetrics.getScaledObject().getAdvanced().getScalerConfig();

    ScalingStabilizer scalingStabilizer =
        scalingStabilizers.computeIfAbsent(
            workloadInfo.name(), (String k) -> new ScalingStabilizer(currentInstanceCount));

    int newInstanceCount =
        getBoundedRecommendation(
            currentInstanceCount,
            unboundedRecommendation,
            scalingStabilizer,
            scalerConfig,
            now,
            workloadName);

    logger.atInfo().log("Recommended instances for %s: %d", workloadName, newInstanceCount);
    if (newInstanceCount != currentInstanceCount) {
      updateInstanceCount(newInstanceCount, workloadInfo);
      scalingStabilizer.markScaleEvent(
          scalerConfig.getBehavior(), now, currentInstanceCount, newInstanceCount);
    } else {
      logger.atInfo().log("Recommended instances for %s is unchanged.", workloadName);
    }

    if (staticConfig.outputScalerMetrics()) {
      emitInstanceCountMetrics(unboundedRecommendation, newInstanceCount, workloadInfo);
    }

    return ScalingStatus.SUCCEEDED;
  }

  // Output a recommendation according to stabilization and min and max instances
  private int getBoundedRecommendation(
      int currentInstanceCount,
      int unboundedRecommendation,
      ScalingStabilizer scalingStabilizer,
      Advanced.ScalerConfig scalerConfig,
      Instant now,
      String workloadName) {

    int stabilizedInstanceCount =
        scalingStabilizer.getStabilizedRecommendation(
            scalerConfig.getBehavior(),
            now,
            currentInstanceCount,
            unboundedRecommendation,
            workloadName);

    int newInstanceCount =
        clamp(
            stabilizedInstanceCount,
            scalerConfig.getMinInstances(),
            scalerConfig.getMaxInstances());

    if (newInstanceCount != stabilizedInstanceCount) {
      logger.atInfo().log(
          "Recommendation for %s was clamped to range [MinReplicaCount=%d,"
              + " MaxReplicaCount=%d]",
          workloadName, scalerConfig.getMinInstances(), scalerConfig.getMaxInstances());
    }

    return newInstanceCount;
  }

  private void updateInstanceCount(
      int newInstanceCount, WorkloadInfoParser.WorkloadInfo workloadInfo)
      throws ExecutionException, IOException, InterruptedException {
    if (staticConfig.useMinInstances()) {
      if (workloadInfo.workloadType() == WorkloadInfoParser.WorkloadType.SERVICE) {
        try {
          cloudRunClientWrapper.updateServiceMinInstances(
              workloadInfo.name(),
              newInstanceCount,
              workloadInfo.projectId(),
              workloadInfo.location());
        } catch (ExecutionException | InterruptedException e) {
          logger.atWarning().withCause(e).log(
              "Failed to update min instances for %s", workloadInfo.name());
          throw new IOException(e);
        }
      } else {
        // We should never realistically get here because we should have checked against this in
        // the constructor.
        throw new IllegalArgumentException("Min instances are not supported for worker pools.");
      }
    } else {
      if (workloadInfo.workloadType() == WorkloadInfoParser.WorkloadType.SERVICE) {
        try {
          cloudRunClientWrapper.updateServiceManualInstances(
              workloadInfo.name(),
              newInstanceCount,
              workloadInfo.projectId(),
              workloadInfo.location());
        } catch (UnsupportedOperationException e) {
          throw new IOException(e);
        }
        logger.atInfo().log(
            "Sent update request for service %s to set instances to %d.",
            workloadInfo.name(), newInstanceCount);
      } else {
        cloudRunClientWrapper.updateWorkerPoolManualInstances(
            workloadInfo.name(),
            newInstanceCount,
            workloadInfo.projectId(),
            workloadInfo.location());
        logger.atInfo().log(
            "Sent update request for workerpool %s to set instances to %d.",
            workloadInfo.name(), newInstanceCount);
      }
    }
  }

  private void emitInstanceCountMetrics(
      int recommendedInstanceCount,
      int newInstanceCount,
      WorkloadInfoParser.WorkloadInfo workloadInfo) {
    ImmutableMap<String, String> metricLabels =
        ImmutableMap.of("project_id", projectId, "consumer_service", workloadInfo.name());
    try {
      metricsService.writeMetricIgnoreFailure(
          RECOMMENDED_INSTANCE_COUNT_METRIC_NAME, (double) recommendedInstanceCount, metricLabels);
      metricsService.writeMetricIgnoreFailure(
          REQUESTED_INSTANCE_COUNT_METRIC_NAME, (double) newInstanceCount, metricLabels);
    } catch (RuntimeException ex) {
      // An exception here is not critical to scaling. Log the exception and continue.
      logger.atWarning().withCause(ex).log("Failed to write metrics to Cloud Monitoring.");
    }
  }

  private void emitTriggerMetrics(Metric metric, WorkloadInfoParser.WorkloadInfo workloadInfo) {
    ImmutableMap<String, String> metricLabels =
        ImmutableMap.of(
            "project_id",
            projectId,
            "consumer_service",
            workloadInfo.name(),
            "trigger_id",
            metric.getTriggerId());
    try {
      metricsService.writeMetricIgnoreFailure(
          metric.getTriggerType() + "/" + METRIC_VALUE_METRIC_NAME,
          metric.getValue(),
          metricLabels);

      // metric either hasTargetAverageValue or hasTargetValue because of the one-of proto
      // definition
      if (metric.hasTargetAverageValue()) {
        metricsService.writeMetricIgnoreFailure(
            metric.getTriggerType() + "/" + TARGET_AVERAGE_VALUE_METRIC_NAME,
            metric.getTargetAverageValue(),
            metricLabels);
      } else {
        metricsService.writeMetricIgnoreFailure(
            metric.getTriggerType() + "/" + TARGET_VALUE_METRIC_NAME,
            metric.getTargetValue(),
            metricLabels);
      }
    } catch (RuntimeException ex) {
      // An exception here is not critical to scaling. Log the exception and continue.
      logger.atWarning().withCause(ex).log("Failed to write metrics to Cloud Monitoring.");
    }
  }
}
