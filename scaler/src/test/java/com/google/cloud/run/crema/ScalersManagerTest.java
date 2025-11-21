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
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import com.google.cloud.run.crema.clients.CloudMonitoringClientWrapper;
import com.google.cloud.run.crema.clients.CloudRunClientWrapper;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Future;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;
import org.mockito.MockitoAnnotations;

@RunWith(JUnit4.class)
public class ScalersManagerTest {

  private CloudRunClientWrapper mockCloudRunClientWrapper;
  private CloudMonitoringClientWrapper mockCloudMonitoringClientWrapper;

  @Before
  public void setUp() {
    MockitoAnnotations.initMocks(this);
    mockCloudRunClientWrapper = mock(CloudRunClientWrapper.class);
    mockCloudMonitoringClientWrapper = mock(CloudMonitoringClientWrapper.class);
  }

  @Test
  public void scale_returnsSuccess() throws IOException, ExecutionException, InterruptedException {
    when(mockCloudRunClientWrapper.getServiceInstanceCount(
            "test-service", "test-project", "test-location"))
        .thenReturn(1);

    ScalersManager scalersManager =
        new ScalersManager(mockCloudRunClientWrapper, mockCloudMonitoringClientWrapper);
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(
                ScaleTargetRef.newBuilder()
                    .setName("projects/test-project/locations/test-location/services/test-service")
                    .build())
            .build();
    ScaledObjectMetrics scaledObjectMetrics =
        ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).build();

    List<ScaledObjectMetrics> metricsList = new ArrayList<>();
    metricsList.add(scaledObjectMetrics);

    List<ScalingResult> results = scalersManager.scale(metricsList);
    assertThat(results).hasSize(1);
    ScalingResult result = results.get(0);

    assertThat(result.getStatus()).isEqualTo(ScalingStatus.SUCCEEDED);
    assertThat(result.getScaleTargetName())
        .isEqualTo("projects/test-project/locations/test-location/services/test-service");
  }
}