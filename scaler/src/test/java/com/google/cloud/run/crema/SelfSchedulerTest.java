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
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.google.cloud.run.crema.clients.CloudRunMetadataClient;
import com.google.cloud.run.crema.clients.CloudTasksClientWrapper;
import com.google.cloud.tasks.v2.HttpMethod;
import com.google.cloud.tasks.v2.HttpRequest;
import com.google.cloud.tasks.v2.Task;
import com.google.common.collect.ImmutableMap;
import com.google.protobuf.Timestamp;
import java.io.IOException;
import java.time.Duration;
import java.time.Instant;
import org.junit.Rule;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.MockitoJUnit;
import org.mockito.junit.MockitoRule;

@RunWith(JUnit4.class)
public final class SelfSchedulerTest {

  private static final String QUEUE_NAME =
      "projects/my-project/locations/us-central1/queues/my-queue-name";
  private static final String INVOKER_SERVICE_ACCOUNT_EMAIL = "invoker-service-account-email";
  private static final String SCALER_URL = "scaler-url";

  @Rule public final MockitoRule mockito = MockitoJUnit.rule();

  @Mock private CloudTasksClientWrapper cloudTasksClientWrapper;
  @Mock private CloudRunMetadataClient cloudRunMetadataClient;
  @Mock private ConfigurationProvider configurationProvider;

  @Test
  public void scheduleAdditionalTasks_cycleTimeTooLong_doesNotScheduleTasks() {
    ConfigurationProvider.SchedulingConfig schedulingConfig =
        new ConfigurationProvider.SchedulingConfig(
            QUEUE_NAME, INVOKER_SERVICE_ACCOUNT_EMAIL, Duration.ofMinutes(1));

    when(configurationProvider.selfSchedulingConfig()).thenReturn(schedulingConfig);

    SelfScheduler selfScheduler =
        new SelfScheduler(cloudTasksClientWrapper, cloudRunMetadataClient, configurationProvider);

    Instant now = Instant.now();
    selfScheduler.scheduleTasks(now, ImmutableMap.of());

    verify(cloudTasksClientWrapper, never()).createTask(any(), any());
  }

  @Test
  public void scheduleAdditionalTasks_headerWithAnyCase_doesNotScheduleTasksForSelfScheduledTask() {
    ConfigurationProvider.SchedulingConfig schedulingConfig =
        new ConfigurationProvider.SchedulingConfig(
            QUEUE_NAME, INVOKER_SERVICE_ACCOUNT_EMAIL, Duration.ofSeconds(30));

    when(configurationProvider.selfSchedulingConfig()).thenReturn(schedulingConfig);
    SelfScheduler selfScheduler =
        new SelfScheduler(cloudTasksClientWrapper, cloudRunMetadataClient, configurationProvider);

    Instant now = Instant.now();
    selfScheduler.scheduleTasks(now, ImmutableMap.of("Kafka-Scaler-Self-Scheduled", "true"));
    selfScheduler.scheduleTasks(now, ImmutableMap.of("Kafka-scaler-self-scheduled", "true"));

    verify(cloudTasksClientWrapper, never()).createTask(any(), any());
  }

  @Test
  public void scheduleAdditionalTasks_nullMap_schedulesTheRightNumberOfTasks() throws IOException {
    ConfigurationProvider.SchedulingConfig schedulingConfig =
        new ConfigurationProvider.SchedulingConfig(
            QUEUE_NAME, INVOKER_SERVICE_ACCOUNT_EMAIL, Duration.ofSeconds(20));

    when(configurationProvider.selfSchedulingConfig()).thenReturn(schedulingConfig);

    when(configurationProvider.scalerUrl(any())).thenReturn(SCALER_URL);

    SelfScheduler selfScheduler =
        new SelfScheduler(cloudTasksClientWrapper, cloudRunMetadataClient, configurationProvider);

    ArgumentCaptor<Task> taskCaptor = ArgumentCaptor.forClass(Task.class);
    Task task = Task.newBuilder().setName("task-name").build();
    when(cloudTasksClientWrapper.createTask(any(), any())).thenReturn(task);

    Instant now = Instant.now();
    selfScheduler.scheduleTasks(now, null);

    verify(cloudTasksClientWrapper, times(2)).createTask(eq(QUEUE_NAME), taskCaptor.capture());
  }

  @Test
  public void scheduleAdditionalTasks_emptyHeadersMap_schedulesTheRightNumberOfTasks()
      throws IOException {
    ConfigurationProvider.SchedulingConfig schedulingConfig =
        new ConfigurationProvider.SchedulingConfig(
            QUEUE_NAME, INVOKER_SERVICE_ACCOUNT_EMAIL, Duration.ofSeconds(13));

    when(configurationProvider.selfSchedulingConfig()).thenReturn(schedulingConfig);
    when(configurationProvider.scalerUrl(any())).thenReturn(SCALER_URL);

    SelfScheduler selfScheduler =
        new SelfScheduler(cloudTasksClientWrapper, cloudRunMetadataClient, configurationProvider);

    ArgumentCaptor<Task> taskCaptor = ArgumentCaptor.forClass(Task.class);
    Task task = Task.newBuilder().setName("task-name").build();
    when(cloudTasksClientWrapper.createTask(any(), any())).thenReturn(task);

    Instant now = Instant.now();
    selfScheduler.scheduleTasks(now, ImmutableMap.of());

    verify(cloudTasksClientWrapper, times(3)).createTask(eq(QUEUE_NAME), taskCaptor.capture());
  }

  @Test
  public void scheduleAdditionalTasks_schedulesTasks() throws IOException {
    ConfigurationProvider.SchedulingConfig schedulingConfig =
        new ConfigurationProvider.SchedulingConfig(
            QUEUE_NAME, INVOKER_SERVICE_ACCOUNT_EMAIL, Duration.ofSeconds(30));

    when(configurationProvider.selfSchedulingConfig()).thenReturn(schedulingConfig);
    when(configurationProvider.scalerUrl(any())).thenReturn(SCALER_URL);

    SelfScheduler selfScheduler =
        new SelfScheduler(cloudTasksClientWrapper, cloudRunMetadataClient, configurationProvider);

    ArgumentCaptor<Task> taskCaptor = ArgumentCaptor.forClass(Task.class);
    Task task = Task.newBuilder().setName("task-name").build();
    when(cloudTasksClientWrapper.createTask(any(), any())).thenReturn(task);

    Instant now = Instant.now();
    Instant expectedScheduleTime = now.plusSeconds(30);
    selfScheduler.scheduleTasks(now, ImmutableMap.of("Another-Header", "true"));

    verify(cloudTasksClientWrapper).createTask(eq(QUEUE_NAME), taskCaptor.capture());

    Timestamp actualScheduleTime =
        Timestamp.newBuilder()
            .setSeconds(expectedScheduleTime.getEpochSecond())
            .setNanos(expectedScheduleTime.getNano())
            .build();
    assertThat(taskCaptor.getValue().getScheduleTime()).isEqualTo(actualScheduleTime);

    HttpRequest request = taskCaptor.getValue().getHttpRequest();
    assertThat(request.getOidcToken().getServiceAccountEmail())
        .isEqualTo(INVOKER_SERVICE_ACCOUNT_EMAIL);
    assertThat(request.getOidcToken().getAudience()).isEqualTo(SCALER_URL);
    assertThat(request.getUrl()).isEqualTo(SCALER_URL);
    assertThat(request.getHttpMethod()).isEqualTo(HttpMethod.POST);
    assertThat(request.getHeadersMap()).containsEntry("Kafka-Scaler-Self-Scheduled", "true");
  }

  @Test
  public void scheduleAdditionalTasks_configProviderThrowsException_doesNotschedulesTasks()
      throws IOException {
    ConfigurationProvider.SchedulingConfig schedulingConfig =
        new ConfigurationProvider.SchedulingConfig(
            QUEUE_NAME, INVOKER_SERVICE_ACCOUNT_EMAIL, Duration.ofSeconds(30));

    when(configurationProvider.selfSchedulingConfig()).thenReturn(schedulingConfig);
    when(configurationProvider.scalerUrl(any())).thenThrow(new IllegalArgumentException());

    SelfScheduler selfScheduler =
        new SelfScheduler(cloudTasksClientWrapper, cloudRunMetadataClient, configurationProvider);

    Task task = Task.newBuilder().setName("task-name").build();
    when(cloudTasksClientWrapper.createTask(any(), any())).thenReturn(task);

    Instant now = Instant.now();
    selfScheduler.scheduleTasks(now, ImmutableMap.of("Another-Header", "true"));

    verify(cloudTasksClientWrapper, never()).createTask(any(), any());
  }
}
