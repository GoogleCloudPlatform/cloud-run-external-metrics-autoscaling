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

import com.google.cloud.run.crema.clients.CloudMonitoringClientWrapper;
import com.google.cloud.run.crema.clients.CloudRunClientWrapper;
import com.google.common.flogger.FluentLogger;
import java.io.IOException;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

/**
 * Manages scaler instances for all scaled objects.
 *
 * <p>This class is responsible for managing the scalers for all scaled objects and acts as the
 * interface for interacting with those scalers. Each scaled object has one scaler in
 * ScalersManager.
 */
public final class ScalersManager {

  private static final FluentLogger logger = FluentLogger.forEnclosingClass();

  private final Map<String, Scaler> scalers = new HashMap<>();
  private final CloudRunClientWrapper cloudRunClientWrapper;
  private final CloudMonitoringClientWrapper cloudMonitoringClientWrapper;
  private final ExecutorService executorService;

  public ScalersManager(
      CloudRunClientWrapper cloudRunClientWrapper,
      CloudMonitoringClientWrapper cloudMonitoringClientWrapper) {
    this.cloudRunClientWrapper = cloudRunClientWrapper;
    this.cloudMonitoringClientWrapper = cloudMonitoringClientWrapper;
    this.executorService = Executors.newCachedThreadPool();
  }

  /**
   * Starts a scale task for for each of the scaled objects in the input list
   *
   * @param scaledObjectMetricsList A list of scald objects and associated metrics for which to
   *     perform scaling.
   * @return A list of {@link ScalingResult} objects, each representing the result of a scaling
   *     operation.
   * @throws InterruptedException if the scaling operation is interrupted.
   */
  public List<ScalingResult> scale(List<ScaledObjectMetrics> scaledObjectMetricsList)
      throws InterruptedException {
    List<Callable<ScalingResult>> tasks = new ArrayList<>();
    for (ScaledObjectMetrics scaledObjectMetrics : scaledObjectMetricsList) {
      Callable<ScalingResult> task =
          () -> {
            String scaleTargetName =
                scaledObjectMetrics.getScaledObject().getScaleTargetRef().getName();
            try {
              Scaler scaler = getOrCreateScaler(scaledObjectMetrics.getScaledObject());
              ScalingStatus scalingStatus = scaler.scale(scaledObjectMetrics);
              return ScalingResult.newBuilder()
                  .setScaleTargetName(scaleTargetName)
                  .setStatus(scalingStatus)
                  .build();
            } catch (ExecutionException | IOException | InterruptedException e) {
              logger.atWarning().withCause(e).log("Failed to scale %s", scaleTargetName);
              return ScalingResult.newBuilder()
                  .setScaleTargetName(scaleTargetName)
                  .setStatus(ScalingStatus.FAILED)
                  .build();
            }
          };
      tasks.add(task);
    }

    List<ScalingResult> results = new ArrayList<>();
    List<Future<ScalingResult>> futures = executorService.invokeAll(tasks);
    for (Future<ScalingResult> future : futures) {
      try {
        results.add(future.get());
      } catch (ExecutionException e) {
        logger.atWarning().withCause(e).log("Failed to get scaling result");
        results.add(ScalingResult.newBuilder().setStatus(ScalingStatus.FAILED).build());
      }
    }
    return results;
  }

  /**
   * Returns the existing Scaler for the given ScaledObject or a creates a new one if one does not
   * already exists.
   *
   * @param scaledObject The ScaledObject to get the Scaler for.
   * @return The Scaler for the given ScaledObject.
   * @throws IOException If there is an error creating a scaler.
   */
  private Scaler getOrCreateScaler(ScaledObject scaledObject) throws IOException {
    String scaleTargetName = scaledObject.getScaleTargetRef().getName();
    if (scalers.containsKey(scaleTargetName)) {
      return scalers.get(scaleTargetName);
    }

    WorkloadInfoParser.WorkloadInfo workloadInfo = WorkloadInfoParser.parse(scaleTargetName);
    ConfigurationProvider.StaticConfig staticConfig =
        new ConfigurationProvider(new ConfigurationProvider.SystemEnvProvider()).staticConfig();
    MetricsService metricsService =
        new MetricsService(cloudMonitoringClientWrapper, workloadInfo.projectId());
    Scaler scaler =
        new Scaler(cloudRunClientWrapper, metricsService, staticConfig, workloadInfo.projectId());

    scalers.put(scaleTargetName, scaler);
    logger.atInfo().log("Created new scaler for %s", scaleTargetName);
    return scaler;
  }
}
