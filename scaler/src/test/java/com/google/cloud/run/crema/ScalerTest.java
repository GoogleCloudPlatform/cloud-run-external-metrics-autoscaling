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
import static org.junit.Assert.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyDouble;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.google.cloud.run.crema.clients.CloudRunClientWrapper;
import java.io.IOException;
import java.time.Duration;
import java.time.Instant;
import java.util.concurrent.ExecutionException;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class ScalerTest {

  private CloudRunClientWrapper cloudRunClientWrapper;
  private MetricsService metricsService;

  private static final String SERVICE_NAME = "test-service";
  private static final String WORKERPOOL_NAME = "test-workerpool";
  private static final String PROJECT_ID = "test-project";
  private static final String SERVICE_WORKLOAD_NAME =
      "projects/test-project/locations/test-location/services/test-service";
  private static final String WORKERPOOL_WORKLOAD_NAME =
      "projects/test-project/locations/test-location/workerpools/test-workerpool";

  private static final ConfigurationProvider.StaticConfig MANUAL_SCALING_STATIC_CONFIG =
      new ConfigurationProvider.StaticConfig(
          /* useMinInstances= */ false, /* outputScalerMetrics= */ false);
  private static final ConfigurationProvider.StaticConfig AUTO_SCALING_STATIC_CONFIG =
      new ConfigurationProvider.StaticConfig(
          /* useMinInstances= */ true, /* outputScalerMetrics= */ false);

  private static final Advanced ADVANCED =
      Advanced.newBuilder()
          .setScalerConfig(
              Advanced.ScalerConfig.newBuilder()
                  .setMinInstances(0)
                  .setMaxInstances(100)
                  .setBehavior(
                      Advanced.ScalerConfig.Behavior.newBuilder()
                          .setScaleUp(
                              Advanced.ScalerConfig.Behavior.Scaling.newBuilder()
                                  .setStabilizationWindowSeconds(0))
                          .setScaleDown(
                              Advanced.ScalerConfig.Behavior.Scaling.newBuilder()
                                  .setStabilizationWindowSeconds(0))))
          .build();

  @Before
  public void setUp() {
    cloudRunClientWrapper = mock(CloudRunClientWrapper.class);
    metricsService = mock(MetricsService.class);

    when(cloudRunClientWrapper.getServiceLastDeploymentTime(
            SERVICE_NAME, "test-project", "test-location"))
        .thenReturn(Instant.now().minus(Duration.ofDays(1)));
    when(cloudRunClientWrapper.getWorkerPoolLastDeploymentTime(
            WORKERPOOL_NAME, "test-project", "test-location"))
        .thenReturn(Instant.now().minus(Duration.ofDays(1)));
  }

  @Test
  public void scale_minInstancesForWorkerPool_throwsIllegalArgumentException() {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, AUTO_SCALING_STATIC_CONFIG, "test-project");
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(
                ScaleTargetRef.newBuilder().setName(WORKERPOOL_WORKLOAD_NAME).build())
            .build();
    assertThrows(
        IllegalArgumentException.class,
        () -> scaler.scale(ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).build()));
  }

  @Test
  public void scale_getServiceInstanceCountException_doesNotUpdateInstances() throws IOException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, MANUAL_SCALING_STATIC_CONFIG, "test-project");
    when(cloudRunClientWrapper.getServiceInstanceCount(
            SERVICE_NAME, "test-project", "test-location"))
        .thenThrow(new RuntimeException("test exception"));
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(ScaleTargetRef.newBuilder().setName(SERVICE_WORKLOAD_NAME).build())
            .build();

    assertThrows(
        RuntimeException.class,
        () -> scaler.scale(ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).build()));

    try {
      verify(cloudRunClientWrapper, never())
          .updateServiceManualInstances(any(), anyInt(), any(), any());
      verify(cloudRunClientWrapper, never())
          .updateServiceMinInstances(any(), anyInt(), any(), any());
    } catch (ExecutionException | InterruptedException e) {
      throw new RuntimeException(e);
    }
  }

  @Test
  public void scale_getWorkerPoolInstanceCountException_doesNotUpdateInstances()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, MANUAL_SCALING_STATIC_CONFIG, "test-project");
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(
                ScaleTargetRef.newBuilder().setName(WORKERPOOL_WORKLOAD_NAME).build())
            .build();
    when(cloudRunClientWrapper.getWorkerPoolInstanceCount(
            WORKERPOOL_NAME, "test-project", "test-location"))
        .thenThrow(new RuntimeException("test exception"));

    assertThrows(
        RuntimeException.class,
        () -> scaler.scale(ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).build()));

    verify(cloudRunClientWrapper, never())
        .updateWorkerPoolManualInstances(any(), anyInt(), any(), any());
  }

  @Test
  public void scale_noMetrics_scalesDownToZero()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, MANUAL_SCALING_STATIC_CONFIG, "test-project");
    when(cloudRunClientWrapper.getServiceInstanceCount(
            SERVICE_NAME, "test-project", "test-location"))
        .thenReturn(5);
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(ScaleTargetRef.newBuilder().setName(SERVICE_WORKLOAD_NAME).build())
            .setAdvanced(ADVANCED)
            .build();

    var unused =
        scaler.scale(ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).build());

    verify(cloudRunClientWrapper)
        .updateServiceManualInstances(SERVICE_NAME, 0, "test-project", "test-location");
  }

  @Test
  public void scale_noMetricsConfigured_scalesDownToZero()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, MANUAL_SCALING_STATIC_CONFIG, "test-project");
    when(cloudRunClientWrapper.getServiceInstanceCount(
            SERVICE_NAME, "test-project", "test-location"))
        .thenReturn(5);
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(ScaleTargetRef.newBuilder().setName(SERVICE_WORKLOAD_NAME).build())
            .setAdvanced(ADVANCED)
            .build();

    var unused =
        scaler.scale(ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).build());

    verify(cloudRunClientWrapper)
        .updateServiceManualInstances(SERVICE_NAME, 0, "test-project", "test-location");
  }

  @Test
  public void scale_toIncreaseInstances_updatesServiceManualInstanceCount()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, MANUAL_SCALING_STATIC_CONFIG, "test-project");
    Metric metric = Metric.newBuilder().setValue(2000.0).setTargetValue(1000.0).build();
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(ScaleTargetRef.newBuilder().setName(SERVICE_WORKLOAD_NAME).build())
            .setAdvanced(ADVANCED)
            .build();
    ScaledObjectMetrics scaledObjectMetrics =
        ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).addMetrics(metric).build();
    when(cloudRunClientWrapper.getServiceInstanceCount(
            SERVICE_NAME, "test-project", "test-location"))
        .thenReturn(5);

    var unused = scaler.scale(scaledObjectMetrics);

    // Recommendation: ceil(5 * 2000 / 1000) = 10
    verify(cloudRunClientWrapper)
        .updateServiceManualInstances(SERVICE_NAME, 10, "test-project", "test-location");
  }

  @Test
  public void scale_toIncreaseInstances_updatesWorkerPoolManualInstanceCount()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, MANUAL_SCALING_STATIC_CONFIG, "test-project");
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(
                ScaleTargetRef.newBuilder().setName(WORKERPOOL_WORKLOAD_NAME).build())
            .setAdvanced(ADVANCED)
            .build();
    Metric metric = Metric.newBuilder().setValue(2000.0).setTargetValue(1000.0).build();
    ScaledObjectMetrics scaledObjectMetrics =
        ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).addMetrics(metric).build();
    when(cloudRunClientWrapper.getWorkerPoolInstanceCount(
            WORKERPOOL_NAME, "test-project", "test-location"))
        .thenReturn(5);

    var unused = scaler.scale(scaledObjectMetrics);

    // Recommendation: ceil(5 * 2000 / 1000) = 10
    verify(cloudRunClientWrapper)
        .updateWorkerPoolManualInstances(WORKERPOOL_NAME, 10, "test-project", "test-location");
  }

  @Test
  public void scale_toIncreaseInstances_updatesServiceMinInstanceCount()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, AUTO_SCALING_STATIC_CONFIG, "test-project");
    Metric metric = Metric.newBuilder().setValue(2000.0).setTargetValue(1000.0).build();
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(ScaleTargetRef.newBuilder().setName(SERVICE_WORKLOAD_NAME).build())
            .setAdvanced(ADVANCED)
            .build();
    ScaledObjectMetrics scaledObjectMetrics =
        ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).addMetrics(metric).build();
    when(cloudRunClientWrapper.getServiceInstanceCount(
            SERVICE_NAME, "test-project", "test-location"))
        .thenReturn(5);

    var unused = scaler.scale(scaledObjectMetrics);

    // Recommendation: ceil(5 * 2000 / 1000) = 10
    verify(cloudRunClientWrapper)
        .updateServiceMinInstances(SERVICE_NAME, 10, "test-project", "test-location");
  }

  @Test
  public void scale_withOutputMetricsEnabled_outputsMetrics()
      throws IOException, ExecutionException, InterruptedException {
    ConfigurationProvider.StaticConfig outputMetricsConfig =
        new ConfigurationProvider.StaticConfig(
            /* useMinInstances= */ false, /* outputScalerMetrics= */ true);
    Scaler scaler =
        new Scaler(cloudRunClientWrapper, metricsService, outputMetricsConfig, "test-project");
    Metric metric = Metric.newBuilder().setValue(2000.0).setTargetValue(1000.0).build();
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(ScaleTargetRef.newBuilder().setName(SERVICE_WORKLOAD_NAME).build())
            .setAdvanced(ADVANCED)
            .build();
    ScaledObjectMetrics scaledObjectMetrics =
        ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).addMetrics(metric).build();
    when(cloudRunClientWrapper.getServiceInstanceCount(
            SERVICE_NAME, "test-project", "test-location"))
        .thenReturn(5);

    var unused = scaler.scale(scaledObjectMetrics);

    verify(metricsService, times(4)).writeMetricIgnoreFailure(any(), anyDouble(), any());
  }

  @Test
  public void scale_lastDeploymentIsOldEnough_updatesWorkerPool()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, MANUAL_SCALING_STATIC_CONFIG, "test-project");
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(
                ScaleTargetRef.newBuilder().setName(WORKERPOOL_WORKLOAD_NAME).build())
            .setAdvanced(ADVANCED)
            .build();
    when(cloudRunClientWrapper.getWorkerPoolLastDeploymentTime(
            WORKERPOOL_NAME, "test-project", "test-location"))
        .thenReturn(Instant.now().minusSeconds(61));
    Metric metric = Metric.newBuilder().setValue(2000.0).setTargetValue(1000.0).build();
    ScaledObjectMetrics scaledObjectMetrics =
        ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).addMetrics(metric).build();
    when(cloudRunClientWrapper.getWorkerPoolInstanceCount(
            WORKERPOOL_NAME, "test-project", "test-location"))
        .thenReturn(5);

    var unused = scaler.scale(scaledObjectMetrics);

    verify(cloudRunClientWrapper)
        .updateWorkerPoolManualInstances(WORKERPOOL_NAME, 10, "test-project", "test-location");
  }

  @Test
  public void scale_usesTargetAverageValue_whenTargetValueIsZero()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, AUTO_SCALING_STATIC_CONFIG, "test-project");
    Metric metric =
        Metric.newBuilder().setValue(200).setTargetValue(0).setTargetAverageValue(100).build();
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(ScaleTargetRef.newBuilder().setName(SERVICE_WORKLOAD_NAME).build())
            .setAdvanced(ADVANCED)
            .build();
    ScaledObjectMetrics scaledObjectMetrics =
        ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).addMetrics(metric).build();
    when(cloudRunClientWrapper.getServiceInstanceCount(
            SERVICE_NAME, "test-project", "test-location"))
        .thenReturn(1);

    ScalingStatus status = scaler.scale(scaledObjectMetrics);

    assertThat(status).isEqualTo(ScalingStatus.SUCCEEDED);
    // Recommendation should be value / targetAverageValue = 200 / 100 = 2
    verify(cloudRunClientWrapper)
        .updateServiceMinInstances(SERVICE_NAME, 2, "test-project", "test-location");
  }

  @Test
  public void scale_usesTargetAverageValue_whenBothAreSet()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, AUTO_SCALING_STATIC_CONFIG, "test-project");
    Metric metric =
        Metric.newBuilder().setValue(200).setTargetValue(50).setTargetAverageValue(100).build();
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(ScaleTargetRef.newBuilder().setName(SERVICE_WORKLOAD_NAME).build())
            .setAdvanced(ADVANCED)
            .build();
    ScaledObjectMetrics scaledObjectMetrics =
        ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).addMetrics(metric).build();
    when(cloudRunClientWrapper.getServiceInstanceCount(
            SERVICE_NAME, "test-project", "test-location"))
        .thenReturn(1);

    ScalingStatus status = scaler.scale(scaledObjectMetrics);

    assertThat(status).isEqualTo(ScalingStatus.SUCCEEDED);
    // Recommendation should be value / targetAverageValue = 200 / 100 = 2
    verify(cloudRunClientWrapper)
        .updateServiceMinInstances(SERVICE_NAME, 2, "test-project", "test-location");
  }

  @Test
  public void scale_skipsMetric_whenBothTargetsAreZero()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, AUTO_SCALING_STATIC_CONFIG, "test-project");
    Metric metric =
        Metric.newBuilder().setValue(200).setTargetValue(0).setTargetAverageValue(0).build();
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(ScaleTargetRef.newBuilder().setName(SERVICE_WORKLOAD_NAME).build())
            .build();
    ScaledObjectMetrics scaledObjectMetrics =
        ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).addMetrics(metric).build();
    when(cloudRunClientWrapper.getServiceInstanceCount(
            SERVICE_NAME, "test-project", "test-location"))
        .thenReturn(1);

    ScalingStatus status = scaler.scale(scaledObjectMetrics);

    assertThat(status).isEqualTo(ScalingStatus.FAILED);
    verify(cloudRunClientWrapper, never())
        .updateServiceMinInstances(anyString(), anyInt(), anyString(), anyString());
  }

  @Test
  public void scale_newInstanceCountMatchesCurrent_doesNotUpdateInstances()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, MANUAL_SCALING_STATIC_CONFIG, "test-project");
    Metric metric = Metric.newBuilder().setValue(1000.0).setTargetValue(1000.0).build();
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(ScaleTargetRef.newBuilder().setName(SERVICE_WORKLOAD_NAME).build())
            .setAdvanced(ADVANCED)
            .build();
    ScaledObjectMetrics scaledObjectMetrics =
        ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).addMetrics(metric).build();
    when(cloudRunClientWrapper.getServiceInstanceCount(
            SERVICE_NAME, "test-project", "test-location"))
        .thenReturn(5);

    var unused = scaler.scale(scaledObjectMetrics);

    verify(cloudRunClientWrapper, never())
        .updateServiceManualInstances(anyString(), anyInt(), anyString(), anyString());
    verify(cloudRunClientWrapper, never())
        .updateServiceMinInstances(anyString(), anyInt(), anyString(), anyString());
    verify(cloudRunClientWrapper, never())
        .updateWorkerPoolManualInstances(anyString(), anyInt(), anyString(), anyString());
  }

  @Test
  public void scale_appliesMaxInstances()
      throws IOException, ExecutionException, InterruptedException {
    Scaler scaler =
        new Scaler(
            cloudRunClientWrapper, metricsService, AUTO_SCALING_STATIC_CONFIG, "test-project");
    Metric metric =
        Metric.newBuilder().setValue(3000.0).setTargetValue(200.0).build(); // Recommendation: 15
    ScaledObject scaledObject =
        ScaledObject.newBuilder()
            .setScaleTargetRef(ScaleTargetRef.newBuilder().setName(SERVICE_WORKLOAD_NAME).build())
            .setAdvanced(
                Advanced.newBuilder()
                    .setScalerConfig(
                        Advanced.ScalerConfig.newBuilder()
                            .setMinInstances(5)
                            .setMaxInstances(10)
                            .setBehavior(
                                Advanced.ScalerConfig.Behavior.newBuilder()
                                    .setScaleUp(
                                        Advanced.ScalerConfig.Behavior.Scaling.newBuilder()
                                            .setStabilizationWindowSeconds(0))
                                    .setScaleDown(
                                        Advanced.ScalerConfig.Behavior.Scaling.newBuilder()
                                            .setStabilizationWindowSeconds(0))))
                    .build())
            .build();
    ScaledObjectMetrics scaledObjectMetrics =
        ScaledObjectMetrics.newBuilder().setScaledObject(scaledObject).addMetrics(metric).build();
    when(cloudRunClientWrapper.getServiceInstanceCount(
            SERVICE_NAME, "test-project", "test-location"))
        .thenReturn(1); // Current instances

    var unused = scaler.scale(scaledObjectMetrics);

    // Unbounded recommendation is 15, but maxInstances is 10, so it should scale to 10.
    verify(cloudRunClientWrapper)
        .updateServiceMinInstances(SERVICE_NAME, 10, "test-project", "test-location");
  }
}
