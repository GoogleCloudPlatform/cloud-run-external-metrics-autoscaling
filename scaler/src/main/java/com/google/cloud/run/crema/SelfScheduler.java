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

import com.google.cloud.run.crema.clients.CloudRunMetadataClient;
import com.google.cloud.run.crema.clients.CloudTasksClientWrapper;
import com.google.cloud.tasks.v2.HttpMethod;
import com.google.cloud.tasks.v2.HttpRequest;
import com.google.cloud.tasks.v2.OidcToken;
import com.google.cloud.tasks.v2.Task;
import com.google.common.base.Ascii;
import com.google.common.base.Preconditions;
import com.google.common.flogger.FluentLogger;
import com.google.protobuf.Timestamp;
import java.io.IOException;
import java.time.Duration;
import java.time.Instant;
import java.util.Map;

/**
 * Encapsulates the logic for scheduling tasks which invoke the Kafka Scaler on Cloud Run.
 *
 * <p>Consumer of this class needs cloudtasks.tasks.create permission e.g. from Cloud Tasks
 * Enqueuer.
 */
public final class SelfScheduler {
  private static final FluentLogger logger = FluentLogger.forEnclosingClass();

  private static final String SELF_SCHEDULED_HEADER_KEY = "Kafka-Scaler-Self-Scheduled";

  private final CloudTasksClientWrapper cloudTasks;
  private final CloudRunMetadataClient cloudRunMetadataClient;
  private final ConfigurationProvider configProvider;
  private final ConfigurationProvider.SchedulingConfig config;
  private String scalerUrl;

  public SelfScheduler(
      CloudTasksClientWrapper cloudTasksClientWrapper,
      CloudRunMetadataClient cloudRunMetadataClient,
      ConfigurationProvider configProvider) {
    this.cloudTasks =
        Preconditions.checkNotNull(cloudTasksClientWrapper, "Cloud Tasks client cannot be null.");
    this.cloudRunMetadataClient =
        Preconditions.checkNotNull(
            cloudRunMetadataClient, "Cloud Run metadata connection provider cannot be null.");
    this.configProvider =
        Preconditions.checkNotNull(configProvider, "Config Provider cannot be null.");
    this.config = configProvider.selfSchedulingConfig();
  }

  /**
   * Schedules tasks to invoke the scaler based on the configured cycle duration.
   *
   * <p>The tasks are scheduled in the future, with the interval between tasks determined by the
   * cycle duration. If the cycle duration is greater than or equal to MAX_CYCLE_DURATION, no tasks
   * are scheduled. If the request headers indicate that the request originated from a scheduled
   * task, no additional tasks are scheduled.
   *
   * @param now The current instant.
   * @param requestHeaders The request headers.
   */
  public void scheduleTasks(Instant now, Map<String, String> requestHeaders) {
    if (config.cycleDuration().compareTo(ConfigurationProvider.MAX_CYCLE_DURATION) >= 0) {
      logger.atInfo().log(
          "Cycle duration is greater than or equal to max cycle duration; skipping additional task"
              + " scheduling.");
      return;
    }

    if (scalerUrl == null) {
      try {
        scalerUrl = configProvider.scalerUrl(cloudRunMetadataClient.projectNumberRegion());
      } catch (RuntimeException | IOException e) {
        logger.atSevere().withCause(e).log("[SCHEDULER] Failed to schedule additional tasks");
        // It's safe to continue here, we just can't schedule additional tasks. We'll try again on
        // the next non-self-scheduled request.
        return;
      }
    }

    // Compare ignoring case to make this resilient to header formatting.
    if (requestHeaders != null
        && requestHeaders.keySet().stream()
            .anyMatch(s -> Ascii.equalsIgnoreCase(s, SELF_SCHEDULED_HEADER_KEY))) {
      // This request originated from a Kafka Scaler. Skip scheduling additional tasks.
      return;
    }

    if (config.cycleDuration().compareTo(Duration.ZERO) > 0) {
      // Do integer division because we want to floor the result.
      int followUpTasks = (int) (60 / config.cycleDuration().toSeconds());
      if (followUpTasks > 0) {
        int taskIntervalSeconds = 60 / followUpTasks;
        for (int i = taskIntervalSeconds; i < 60; i += taskIntervalSeconds) {
          scheduleTask(now.plus(Duration.ofSeconds(i)), scalerUrl);
        }
      }
    }
  }

  /**
   * Schedules a task to invoke the scaler at the specified `time`.
   *
   * <p>The task scheduled will contain an http body that indicates that the request is from Cloud
   * Task. This should be used as signal that the request should not cause additional tasks to be
   * enqueued.
   *
   * @param time The time at which the scaler should be invoked.
   * @param scalerUrl The URL of the scaler service.
   */
  private void scheduleTask(Instant time, String scalerUrl) {
    OidcToken oidcToken =
        OidcToken.newBuilder()
            .setServiceAccountEmail(config.invokerServiceAccountEmail())
            .setAudience(scalerUrl)
            .build();
    HttpRequest httpRequest =
        HttpRequest.newBuilder()
            .setUrl(scalerUrl)
            .setHttpMethod(HttpMethod.POST)
            .setOidcToken(oidcToken)
            .putHeaders(SELF_SCHEDULED_HEADER_KEY, "true")
            .build();
    Timestamp scheduleTime =
        Timestamp.newBuilder().setSeconds(time.getEpochSecond()).setNanos(time.getNano()).build();
    Task task = Task.newBuilder().setHttpRequest(httpRequest).setScheduleTime(scheduleTime).build();

    try {
      var unused = cloudTasks.createTask(config.fullyQualifiedCloudTaskQueueName(), task);
    } catch (RuntimeException ex) {
      logger.atSevere().withCause(ex).log("Failed to create task");
    }
  }
}
