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

import com.google.api.core.ApiFutureCallback;
import com.google.api.core.ApiFutures;
import com.google.api.gax.longrunning.OperationFuture;
import com.google.cloud.run.v2.GetServiceRequest;
import com.google.cloud.run.v2.GetWorkerPoolRequest;
import com.google.cloud.run.v2.RevisionScaling;
import com.google.cloud.run.v2.RevisionTemplate;
import com.google.cloud.run.v2.Service;
import com.google.cloud.run.v2.ServiceName;
import com.google.cloud.run.v2.ServiceScaling;
import com.google.cloud.run.v2.ServicesClient;
import com.google.cloud.run.v2.UpdateServiceRequest;
import com.google.cloud.run.v2.UpdateWorkerPoolRequest;
import com.google.cloud.run.v2.WorkerPool;
import com.google.cloud.run.v2.WorkerPoolName;
import com.google.cloud.run.v2.WorkerPoolScaling;
import com.google.cloud.run.v2.WorkerPoolsClient;
import com.google.common.flogger.FluentLogger;
import com.google.common.flogger.MetadataKey;
import com.google.common.util.concurrent.MoreExecutors;
import com.google.protobuf.FieldMask;
import com.google.protobuf.Timestamp;
import java.io.IOException;
import java.time.Instant;
import java.util.concurrent.ExecutionException;

/** Thin wrapper around Cloud Run API */
public class CloudRunClientWrapper {
  private static final FluentLogger logger = FluentLogger.forEnclosingClass();

  private static final MetadataKey<String> RESOURCE = MetadataKey.single("resource", String.class);

  private final ServicesClient servicesClient;
  private final WorkerPoolsClient workerPoolsClient;

  public CloudRunClientWrapper() throws IOException {
    this.servicesClient = ServicesClient.create();
    this.workerPoolsClient = WorkerPoolsClient.create();
  }

  public CloudRunClientWrapper(ServicesClient servicesClient, WorkerPoolsClient workerPoolsClient) {
    this.servicesClient = servicesClient;
    this.workerPoolsClient = workerPoolsClient;
  }

  /**
   * Returns the instance count for a given worker pool.
   *
   * <p>Cloud Run Admin API will return null if min instances is 0 with autoscaling. In this case,
   * translate the null to 0.
   *
   * @param workerpoolName The name of the worker pool.
   * @return The instance count.
   */
  public int getWorkerPoolInstanceCount(String workerpoolName, String projectId, String region) {
    WorkerPool workerpool = getWorkerPool(workerpoolName, projectId, region);
    if (workerpool.hasScaling()) {
      return workerpool.getScaling().getManualInstanceCount();
    }
    return 0;
  }

  /**
   * Returns the last deployment time for a given worker pool.
   *
   * @param workerpoolName The name of the worker pool.
   * @return The last deployment time or EPOCH if not set.
   */
  public Instant getWorkerPoolLastDeploymentTime(
      String workerpoolName, String projectId, String region) {
    WorkerPool workerpool = getWorkerPool(workerpoolName, projectId, region);
    if (workerpool.hasUpdateTime()) {
      Timestamp updateTime = workerpool.getUpdateTime();
      return Instant.ofEpochSecond(updateTime.getSeconds(), updateTime.getNanos());
    }
    return Instant.EPOCH;
  }

  /**
   * Updates the number of manual instances for a given worker pool.
   *
   * <p>Waits only until Cloud Run accepts the update; the operation completes asynchronously.
   *
   * @param workerpoolName The name of the worker pool to update.
   * @param instances The desired number of instances.
   * @throws ExecutionException If the update request is rejected.
   * @throws InterruptedException If an error occurs during the API call.
   */
  public void updateWorkerPoolManualInstances(
      String workerpoolName, int instances, String projectId, String region)
      throws ExecutionException, InterruptedException {
    String resourceName = WorkerPoolName.of(projectId, region, workerpoolName).toString();
    WorkerPool currentWorkerPool = getWorkerPool(workerpoolName, projectId, region);

    WorkerPoolScaling newScaling =
        WorkerPoolScaling.newBuilder().setManualInstanceCount(instances).build();

    WorkerPool workerPoolToUpdate = currentWorkerPool.toBuilder().setScaling(newScaling).build();

    FieldMask updateMask = FieldMask.newBuilder().addPaths("scaling.manual_instance_count").build();

    UpdateWorkerPoolRequest updateRequest =
        UpdateWorkerPoolRequest.newBuilder()
            .setWorkerPool(workerPoolToUpdate)
            .setUpdateMask(updateMask)
            .build();

    OperationFuture<WorkerPool, WorkerPool> operation =
        workerPoolsClient.updateWorkerPoolAsync(updateRequest);
    logCompletionAsync(resourceName, operation);
    operation.getInitialFuture().get(); // Wait for acceptance only, not completion
  }

  /**
   * Returns the instance count for a given service.
   *
   * <p>Cloud Run Admin API will return null if min instances is 0 with autoscaling. In this case,
   * translate the null to 0.
   *
   * @param serviceName The name of the service.
   * @return The instance count.
   */
  public int getServiceInstanceCount(String serviceName, String projectId, String region) {
    Service service = getService(serviceName, projectId, region);
    if (service.hasScaling()
        && service.getScaling().getScalingMode() == ServiceScaling.ScalingMode.MANUAL) {
      return service.getScaling().getManualInstanceCount();
    }
    if (service.hasTemplate() && service.getTemplate().hasScaling()) {
      return service.getTemplate().getScaling().getMinInstanceCount();
    }
    return 0;
  }

  /**
   * Returns the last deployment time for a given service.
   *
   * @param serviceName The name of the service.
   * @return The last deployment time or EPOCH if not set.
   */
  public Instant getServiceLastDeploymentTime(String serviceName, String projectId, String region) {
    Service service = getService(serviceName, projectId, region);
    if (service.hasUpdateTime()) {
      Timestamp updateTime = service.getUpdateTime();
      return Instant.ofEpochSecond(updateTime.getSeconds(), updateTime.getNanos());
    }
    return Instant.EPOCH;
  }

  /**
   * Updates the number of min instances for a given service.
   *
   * <p>Waits only until Cloud Run accepts the update; the operation completes asynchronously.
   *
   * @param serviceName The name of the service to update.
   * @param instances The desired number of instances.
   * @throws ExecutionException If the update request is rejected.
   * @throws InterruptedException If an error occurs during the API call.
   */
  public void updateServiceMinInstances(
      String serviceName, int instances, String projectId, String region)
      throws ExecutionException, InterruptedException {
    String resourceName = ServiceName.of(projectId, region, serviceName).toString();
    Service currentService = getService(serviceName, projectId, region);

    RevisionScaling newScaling =
        currentService.getTemplate().getScaling().toBuilder()
            .setMinInstanceCount(instances)
            .build();

    RevisionTemplate newTemplate =
        currentService.getTemplate().toBuilder().setScaling(newScaling).build();

    Service serviceToUpdate = currentService.toBuilder().setTemplate(newTemplate).build();

    FieldMask updateMask =
        FieldMask.newBuilder().addPaths("template.scaling.min_instance_count").build();

    UpdateServiceRequest updateRequest =
        UpdateServiceRequest.newBuilder()
            .setService(serviceToUpdate)
            .setUpdateMask(updateMask)
            .build();

    OperationFuture<Service, Service> operation = servicesClient.updateServiceAsync(updateRequest);
    logCompletionAsync(resourceName, operation);
    operation.getInitialFuture().get(); // Wait for acceptance only, not completion
  }

  /**
   * Updates the number of manual instances for a given service.
   *
   * <p>Waits only until Cloud Run accepts the update; the operation completes asynchronously.
   *
   * @param serviceName The name of the service to update.
   * @param instances The desired number of instances.
   */
  public void updateServiceManualInstances(
      String serviceName, int instances, String projectId, String region)
      throws ExecutionException, InterruptedException {
    String resourceName = ServiceName.of(projectId, region, serviceName).toString();
    Service currentService = getService(serviceName, projectId, region);

    ServiceScaling newScaling =
        ServiceScaling.newBuilder().setManualInstanceCount(instances).build();

    Service serviceToUpdate = currentService.toBuilder().setScaling(newScaling).build();

    FieldMask updateMask = FieldMask.newBuilder().addPaths("scaling.manual_instance_count").build();

    UpdateServiceRequest updateRequest =
        UpdateServiceRequest.newBuilder()
            .setService(serviceToUpdate)
            .setUpdateMask(updateMask)
            .build();

    OperationFuture<Service, Service> operation = servicesClient.updateServiceAsync(updateRequest);
    logCompletionAsync(resourceName, operation);
    operation.getInitialFuture().get(); // Wait for acceptance only, not completion
  }

  private <T> void logCompletionAsync(String resourceName, OperationFuture<T, ?> operation) {
    ApiFutures.addCallback(
        operation,
        new ApiFutureCallback<T>() {
          @Override
          public void onSuccess(T result) {
            logger.atInfo().with(RESOURCE, resourceName)
            .log("Update operation for %s completed", resourceName);
          }

          @Override
          public void onFailure(Throwable t) {
            logger.atWarning().withCause(t).with(RESOURCE, resourceName)
            .log("Update operation for %s failed", resourceName);
          }
        },
        MoreExecutors.directExecutor());
  }

  private WorkerPool getWorkerPool(String workerpoolName, String projectId, String region) {
    WorkerPoolName name = WorkerPoolName.of(projectId, region, workerpoolName);
    GetWorkerPoolRequest request =
        GetWorkerPoolRequest.newBuilder().setName(name.toString()).build();
    WorkerPool workerPool = workerPoolsClient.getWorkerPool(request);
    return workerPool;
  }

  private Service getService(String serviceName, String projectId, String region) {
    ServiceName name = ServiceName.of(projectId, region, serviceName);
    GetServiceRequest getRequest = GetServiceRequest.newBuilder().setName(name.toString()).build();
    Service service = servicesClient.getService(getRequest);
    return service;
  }
}
