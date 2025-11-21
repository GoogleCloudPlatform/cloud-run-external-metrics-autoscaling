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

import com.google.cloud.run.crema.clients.CloudRunClientWrapper;
import java.io.IOException;

/** Utility class for reading the instance count for a given workload. */
final class InstanceCountProvider {

  private InstanceCountProvider() {}

  /**
   * Gets the last requested instance count for a given workload.
   *
   * @param cloudRunClient The Cloud Run client.
   * @param workloadInfo The workload info.
   * @return The instance count.
   * @throws IOException If there is an error getting the instance count.
   */
  public static int getInstanceCount(
      CloudRunClientWrapper cloudRunClient, WorkloadInfoParser.WorkloadInfo workloadInfo)
      throws IOException {
    int instanceCount;
    if (workloadInfo.workloadType() == WorkloadInfoParser.WorkloadType.SERVICE) {
      instanceCount =
          cloudRunClient.getServiceInstanceCount(
              workloadInfo.name(), workloadInfo.projectId(), workloadInfo.location());
    } else {
      instanceCount =
          cloudRunClient.getWorkerPoolInstanceCount(
              workloadInfo.name(), workloadInfo.projectId(), workloadInfo.location());
    }
    return instanceCount;
  }
}
