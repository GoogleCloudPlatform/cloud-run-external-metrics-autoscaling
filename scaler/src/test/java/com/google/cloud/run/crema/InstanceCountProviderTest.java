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
import static org.mockito.Mockito.when;

import com.google.cloud.run.crema.clients.CloudRunClientWrapper;
import java.io.IOException;
import org.junit.Rule;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;
import org.mockito.Mock;
import org.mockito.junit.MockitoJUnit;
import org.mockito.junit.MockitoRule;

@RunWith(JUnit4.class)
public final class InstanceCountProviderTest {

  @Rule public final MockitoRule mockito = MockitoJUnit.rule();

  @Mock private CloudRunClientWrapper cloudRunClient;

  private static final String PROJECT_ID = "test-project";
  private static final String LOCATION = "test-location";
  private static final String SERVICE_NAME = "test-service";
  private static final String WORKERPOOL_NAME = "test-workerpool";
  private static final WorkloadInfoParser.WorkloadInfo SERVICE_WORKLOAD_INFO =
      new WorkloadInfoParser.WorkloadInfo(
          WorkloadInfoParser.WorkloadType.SERVICE, PROJECT_ID, LOCATION, SERVICE_NAME);
  private static final WorkloadInfoParser.WorkloadInfo WORKERPOOL_WORKLOAD_INFO =
      new WorkloadInfoParser.WorkloadInfo(
          WorkloadInfoParser.WorkloadType.WORKERPOOL, PROJECT_ID, LOCATION, WORKERPOOL_NAME);

  @Test
  public void getInstanceCount_serviceInstances() throws IOException {
    when(cloudRunClient.getServiceInstanceCount(SERVICE_NAME, PROJECT_ID, LOCATION)).thenReturn(2);

    int actual = InstanceCountProvider.getInstanceCount(cloudRunClient, SERVICE_WORKLOAD_INFO);

    assertThat(actual).isEqualTo(2);
  }

  @Test
  public void getInstanceCount_workerPoolInstances() throws IOException {
    when(cloudRunClient.getWorkerPoolInstanceCount(WORKERPOOL_NAME, PROJECT_ID, LOCATION))
        .thenReturn(4);

    int actual = InstanceCountProvider.getInstanceCount(cloudRunClient, WORKERPOOL_WORKLOAD_INFO);

    assertThat(actual).isEqualTo(4);
  }
}
