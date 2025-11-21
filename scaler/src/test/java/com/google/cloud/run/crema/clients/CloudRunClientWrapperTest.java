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

package com.google.cloud.run.crema.clients;

import static com.google.common.truth.Truth.assertThat;
import static org.junit.Assert.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import com.google.api.gax.longrunning.OperationFuture;
import com.google.cloud.run.v2.GetServiceRequest;
import com.google.cloud.run.v2.GetWorkerPoolRequest;
import com.google.cloud.run.v2.RevisionScaling;
import com.google.cloud.run.v2.RevisionTemplate;
import com.google.cloud.run.v2.Service;
import com.google.cloud.run.v2.ServiceScaling;
import com.google.cloud.run.v2.ServicesClient;
import com.google.cloud.run.v2.UpdateServiceRequest;
import com.google.cloud.run.v2.UpdateWorkerPoolRequest;
import com.google.cloud.run.v2.WorkerPool;
import com.google.cloud.run.v2.WorkerPoolsClient;
import com.google.cloud.run.v2.WorkerPoolScaling;
import com.google.protobuf.Timestamp;
import java.io.IOException;
import java.time.Instant;
import java.util.concurrent.ExecutionException;
import org.junit.Before;
import org.junit.Rule;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;
import org.mockito.ArgumentCaptor;
import org.mockito.Captor;
import org.mockito.Mock;
import org.mockito.junit.MockitoJUnit;
import org.mockito.junit.MockitoRule;

@RunWith(JUnit4.class)
public final class CloudRunClientWrapperTest {

  @Rule public final MockitoRule mockito = MockitoJUnit.rule();

  @Mock private ServicesClient servicesClient;
  @Mock private WorkerPoolsClient workerPoolsClient;

  @Captor private ArgumentCaptor<GetServiceRequest> getServiceRequestCaptor;
  @Captor private ArgumentCaptor<GetWorkerPoolRequest> getWorkerPoolRequestCaptor;
  @Captor private ArgumentCaptor<UpdateServiceRequest> updateServiceRequestCaptor;
  @Captor private ArgumentCaptor<UpdateWorkerPoolRequest> updateWorkerPoolRequestCaptor;

  private CloudRunClientWrapper cloudRunClientWrapper;

  private static final String PROJECT_ID = "projectId";
  private static final String REGION = "global";
  private static final String WORKERPOOL_NAME = "workerpool-name";
  private static final String SERVICE_NAME = "service-name";

  private static final String WORKERPOOL_RESOURCE_NAME =
      String.format("projects/%s/locations/%s/workerPools/%s", PROJECT_ID, REGION, WORKERPOOL_NAME);
  private static final String SERVICE_RESOURCE_NAME =
      String.format("projects/%s/locations/%s/services/%s", PROJECT_ID, REGION, SERVICE_NAME);

  @Before
  public void setUp() {
    cloudRunClientWrapper = new CloudRunClientWrapper(servicesClient, workerPoolsClient);
  }

  @Test
  public void getWorkerPoolInstanceCount_withNullScaling_returnsZero() {
    when(workerPoolsClient.getWorkerPool(getWorkerPoolRequestCaptor.capture()))
        .thenReturn(WorkerPool.newBuilder().build());

    assertThat(
            cloudRunClientWrapper.getWorkerPoolInstanceCount(
                WORKERPOOL_NAME, PROJECT_ID, REGION))
        .isEqualTo(0);
  }

  @Test
  public void getWorkerPoolInstanceCount_withManualScaling_returnsManualInstances() {
    int numInstances = 20;
    WorkerPool workerPool =
        WorkerPool.newBuilder()
            .setScaling(WorkerPoolScaling.newBuilder().setManualInstanceCount(numInstances))
            .build();
    when(workerPoolsClient.getWorkerPool(getWorkerPoolRequestCaptor.capture()))
        .thenReturn(workerPool);

    assertThat(
            cloudRunClientWrapper.getWorkerPoolInstanceCount(
                WORKERPOOL_NAME, PROJECT_ID, REGION))
        .isEqualTo(numInstances);
  }

  @Test
  public void updateWorkerPoolManualInstances_succeeds() throws ExecutionException, InterruptedException {
    WorkerPool workerPool = WorkerPool.newBuilder().build();
    when(workerPoolsClient.getWorkerPool(any(GetWorkerPoolRequest.class)))
        .thenReturn(workerPool);

    OperationFuture<WorkerPool, WorkerPool> operationFuture = mock(OperationFuture.class);
    when(workerPoolsClient.updateWorkerPoolAsync(updateWorkerPoolRequestCaptor.capture()))
        .thenReturn(operationFuture);
    when(operationFuture.get()).thenReturn(workerPool);

    cloudRunClientWrapper.updateWorkerPoolManualInstances(WORKERPOOL_NAME, 10, PROJECT_ID, REGION);

    UpdateWorkerPoolRequest actual = updateWorkerPoolRequestCaptor.getValue();
    assertThat(actual.getWorkerPool().getScaling().getManualInstanceCount()).isEqualTo(10);
    assertThat(actual.getUpdateMask().getPaths(0)).isEqualTo("scaling.manual_instance_count");
  }

  @Test
  public void updateWorkerPoolManualInstances_operationError_throwsExecutionException()
      throws ExecutionException, InterruptedException {
    WorkerPool workerPool = WorkerPool.newBuilder().build();
    when(workerPoolsClient.getWorkerPool(any(GetWorkerPoolRequest.class)))
        .thenReturn(workerPool);

    OperationFuture<WorkerPool, WorkerPool> operationFuture = mock(OperationFuture.class);
    when(workerPoolsClient.updateWorkerPoolAsync(any(UpdateWorkerPoolRequest.class)))
        .thenReturn(operationFuture);
    when(operationFuture.get()).thenThrow(new ExecutionException(new IOException("error")));

    assertThrows(
        ExecutionException.class,
        () ->
            cloudRunClientWrapper.updateWorkerPoolManualInstances(
                WORKERPOOL_NAME, 10, PROJECT_ID, REGION));
  }

  @Test
  public void getServiceInstanceCount_withServiceLevelManualScaling_returnsManualInstances() {
    int numInstances = 25;
    Service service =
        Service.newBuilder()
            .setScaling(
                ServiceScaling.newBuilder()
                    .setScalingMode(ServiceScaling.ScalingMode.MANUAL)
                    .setManualInstanceCount(numInstances))
            .build();
    when(servicesClient.getService(getServiceRequestCaptor.capture())).thenReturn(service);

    assertThat(cloudRunClientWrapper.getServiceInstanceCount(SERVICE_NAME, PROJECT_ID, REGION))
        .isEqualTo(numInstances);
  }

  @Test
  public void getServiceInstanceCount_withRevisionTemplateScaling_returnsMinInstances() {
    int numInstances = 20;
    Service service =
        Service.newBuilder()
            .setTemplate(
                RevisionTemplate.newBuilder()
                    .setScaling(RevisionScaling.newBuilder().setMinInstanceCount(numInstances)))
            .build();
    when(servicesClient.getService(getServiceRequestCaptor.capture())).thenReturn(service);

    assertThat(cloudRunClientWrapper.getServiceInstanceCount(SERVICE_NAME, PROJECT_ID, REGION))
        .isEqualTo(numInstances);
  }

  @Test
  public void getServiceInstanceCount_withRevisionTemplateScalingAndZeroMinInstances_returnsZero() {
    Service service =
        Service.newBuilder()
            .setTemplate(
                RevisionTemplate.newBuilder()
                    .setScaling(RevisionScaling.newBuilder().setMinInstanceCount(0)))
            .build();
    when(servicesClient.getService(getServiceRequestCaptor.capture())).thenReturn(service);

    assertThat(cloudRunClientWrapper.getServiceInstanceCount(SERVICE_NAME, PROJECT_ID, REGION))
        .isEqualTo(0);
  }

  @Test
  public void getServiceInstanceCount_withNoScalingConfigured_returnsZero() {
    Service service = Service.newBuilder().build();
    when(servicesClient.getService(getServiceRequestCaptor.capture()))
        .thenReturn(service);

    assertThat(cloudRunClientWrapper.getServiceInstanceCount(SERVICE_NAME, PROJECT_ID, REGION))
        .isEqualTo(0);
  }

  @Test
  public void getServiceLastDeploymentTime_withUpdateTime_returnsInstant() {
    Instant updateTime = Instant.parse("2025-04-01T18:05:07.879813Z");

    Service service =
        Service.newBuilder()
            .setUpdateTime(
                Timestamp.newBuilder()
                    .setSeconds(updateTime.getEpochSecond())
                    .setNanos(updateTime.getNano()))
            .build();
    when(servicesClient.getService(getServiceRequestCaptor.capture())).thenReturn(service);

    assertThat(cloudRunClientWrapper.getServiceLastDeploymentTime(SERVICE_NAME, PROJECT_ID, REGION))
        .isEqualTo(updateTime);
  }

  @Test
  public void getServiceLastDeploymentTime_withNullUpdateTime_returnsEpoch() {
    Service service = Service.newBuilder().build();
    when(servicesClient.getService(getServiceRequestCaptor.capture())).thenReturn(service);

    assertThat(cloudRunClientWrapper.getServiceLastDeploymentTime(SERVICE_NAME, PROJECT_ID, REGION))
        .isEqualTo(Instant.EPOCH);
  }

  @Test
  public void getWorkerPoolLastDeploymentTime_withUpdateTime_returnsInstant() {
    Instant updateTime = Instant.parse("2025-04-01T18:05:07.879813Z");

    WorkerPool workerPool =
        WorkerPool.newBuilder()
            .setUpdateTime(
                Timestamp.newBuilder()
                    .setSeconds(updateTime.getEpochSecond())
                    .setNanos(updateTime.getNano()))
            .build();
    when(workerPoolsClient.getWorkerPool(getWorkerPoolRequestCaptor.capture()))
        .thenReturn(workerPool);

    assertThat(
            cloudRunClientWrapper.getWorkerPoolLastDeploymentTime(
                WORKERPOOL_NAME, PROJECT_ID, REGION))
        .isEqualTo(updateTime);
  }

  @Test
  public void getWorkerPoolLastDeploymentTime_withNullUpdateTime_returnsEpoch() {
    WorkerPool workerPool = WorkerPool.newBuilder().build();
    when(workerPoolsClient.getWorkerPool(getWorkerPoolRequestCaptor.capture()))
        .thenReturn(workerPool);

    assertThat(
            cloudRunClientWrapper.getWorkerPoolLastDeploymentTime(
                WORKERPOOL_NAME, PROJECT_ID, REGION))
        .isEqualTo(Instant.EPOCH);
  }

  @Test
  public void updateServiceMinInstances_succeeds() throws ExecutionException, InterruptedException {
    Service service =
        Service.newBuilder()
            .setTemplate(
                RevisionTemplate.newBuilder()
                    .setScaling(RevisionScaling.newBuilder().setMinInstanceCount(5).setMaxInstanceCount(222)))
            .build();
    when(servicesClient.getService(any(GetServiceRequest.class))).thenReturn(service);

    OperationFuture<Service, Service> operationFuture = mock(OperationFuture.class);
    when(servicesClient.updateServiceAsync(updateServiceRequestCaptor.capture()))
        .thenReturn(operationFuture);
    when(operationFuture.get()).thenReturn(service);

    cloudRunClientWrapper.updateServiceMinInstances(SERVICE_NAME, 10, PROJECT_ID, REGION);

    UpdateServiceRequest actual = updateServiceRequestCaptor.getValue();
    assertThat(actual.getService().getTemplate().getScaling().getMinInstanceCount()).isEqualTo(10);
    assertThat(actual.getService().getTemplate().getScaling().getMaxInstanceCount()).isEqualTo(222);
    assertThat(actual.getUpdateMask().getPaths(0)).isEqualTo("template.scaling.min_instance_count");
  }

  @Test
  public void updateServiceMinInstances_operationError_throwsExecutionException()
      throws ExecutionException, InterruptedException {
    Service service = Service.newBuilder().build();
    when(servicesClient.getService(any(GetServiceRequest.class))).thenReturn(service);

    OperationFuture<Service, Service> operationFuture = mock(OperationFuture.class);
    when(servicesClient.updateServiceAsync(any(UpdateServiceRequest.class)))
        .thenReturn(operationFuture);
    when(operationFuture.get()).thenThrow(new ExecutionException(new IOException("error")));

    assertThrows(
        ExecutionException.class,
        () -> cloudRunClientWrapper.updateServiceMinInstances(SERVICE_NAME, 10, PROJECT_ID, REGION));
  }

  @Test
  public void updateServiceManualInstances_succeeds() throws ExecutionException, InterruptedException {
    Service service =
        Service.newBuilder()
            .setScaling(ServiceScaling.newBuilder().setManualInstanceCount(5))
            .build();
    when(servicesClient.getService(any(GetServiceRequest.class))).thenReturn(service);

    OperationFuture<Service, Service> operationFuture = mock(OperationFuture.class);
    when(servicesClient.updateServiceAsync(updateServiceRequestCaptor.capture()))
        .thenReturn(operationFuture);
    when(operationFuture.get()).thenReturn(service);

    cloudRunClientWrapper.updateServiceManualInstances(SERVICE_NAME, 10, PROJECT_ID, REGION);

    UpdateServiceRequest actual = updateServiceRequestCaptor.getValue();
    assertThat(actual.getService().getScaling().getManualInstanceCount()).isEqualTo(10);
    assertThat(actual.getUpdateMask().getPathsList())
        .containsExactly("scaling.manual_instance_count");
  }

  @Test
  public void updateServiceManualInstances_operationError_throwsExecutionException()
      throws ExecutionException, InterruptedException {
    Service service =
        Service.newBuilder()
            .setScaling(ServiceScaling.newBuilder().setManualInstanceCount(5))
            .build();
    when(servicesClient.getService(any(GetServiceRequest.class))).thenReturn(service);

    OperationFuture<Service, Service> operationFuture = mock(OperationFuture.class);
    when(servicesClient.updateServiceAsync(any(UpdateServiceRequest.class)))
        .thenReturn(operationFuture);
    when(operationFuture.get()).thenThrow(new ExecutionException(new IOException("error")));

    assertThrows(
        ExecutionException.class,
        () ->
            cloudRunClientWrapper.updateServiceManualInstances(
                SERVICE_NAME, 10, PROJECT_ID, REGION));
  }
}