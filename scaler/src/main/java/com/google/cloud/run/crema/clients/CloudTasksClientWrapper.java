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

import com.google.cloud.tasks.v2.CloudTasksClient;
import com.google.cloud.tasks.v2.Task;
import java.io.IOException;

/** Wrapper class for CloudTasksClient to allow mocking in tests. */
public class CloudTasksClientWrapper {
  private final CloudTasksClient cloudTasksClient;

  public static CloudTasksClient cloudTasksClient() throws IOException {
    return CloudTasksClient.create();
  }

  /** Constructs a CloudTasksClientWrapper using an underlying client. */
  public CloudTasksClientWrapper(CloudTasksClient cloudTasksClient) {
    this.cloudTasksClient = cloudTasksClient;
  }

  /**
   * Creates a task in the Cloud Tasks queue by passing to the underlying client.
   *
   * @param parent The parent resource name of the queue in the form
   *     projects/{project}/locations/{region}/queues/{queueId}
   * @param task The task to create.
   * @return The created task.
   */
  public Task createTask(String parent, Task task) {
    return cloudTasksClient.createTask(parent, task);
  }
}
