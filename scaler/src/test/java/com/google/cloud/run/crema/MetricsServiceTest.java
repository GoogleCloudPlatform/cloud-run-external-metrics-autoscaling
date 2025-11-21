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
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.google.api.MetricDescriptor.MetricKind;
import com.google.api.MetricDescriptor.ValueType;
import com.google.cloud.monitoring.v3.MetricServiceClient.ListTimeSeriesPagedResponse;
import com.google.cloud.run.crema.clients.CloudMonitoringClientWrapper;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.monitoring.v3.Aggregation;
import com.google.monitoring.v3.CreateTimeSeriesRequest;
import com.google.monitoring.v3.ListTimeSeriesRequest;
import com.google.monitoring.v3.Point;
import com.google.monitoring.v3.ProjectName;
import com.google.monitoring.v3.TimeInterval;
import com.google.monitoring.v3.TimeSeries;
import com.google.monitoring.v3.TypedValue;
import java.io.IOException;
import java.time.Duration;
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
public final class MetricsServiceTest {

  @Rule public final MockitoRule mockito = MockitoJUnit.rule();

  @Mock private CloudMonitoringClientWrapper cloudMonitoringClientWrapper;

  @Mock private ListTimeSeriesPagedResponse instanceCountListTimeSeriesPagedResponse;
  @Mock private ListTimeSeriesPagedResponse cpuUtilizationListTimeSeriesPagedResponse;
  @Captor private ArgumentCaptor<CreateTimeSeriesRequest> createTimeSeriesRequestCaptor;
  @Captor private ArgumentCaptor<ListTimeSeriesRequest> listTimeSeriesRequestCaptor;

  private MetricsService metricsService;

  private static final String PROJECT_ID = "projectId";

  @Before
  public void setUp() {
    metricsService = new MetricsService(cloudMonitoringClientWrapper, PROJECT_ID);
  }

  @Test
  public void cloudMonitoringClientWrapper_throwsNullPointerException() {
    assertThrows(NullPointerException.class, () -> new MetricsService(null, PROJECT_ID));
  }

  @Test
  public void constructor_withoutProjectId_throwsIllegalArgumentException() {
    assertThrows(
        IllegalArgumentException.class,
        () -> new MetricsService(cloudMonitoringClientWrapper, null));
    assertThrows(
        IllegalArgumentException.class, () -> new MetricsService(cloudMonitoringClientWrapper, ""));
  }

  @Test
  public void writeMetricIgnoreFailure_withoutMetricName_throwsIllegalArgumentException() {
    assertThrows(
        IllegalArgumentException.class,
        () -> metricsService.writeMetricIgnoreFailure(null, 1.0, ImmutableMap.of()));
    assertThrows(
        IllegalArgumentException.class,
        () -> metricsService.writeMetricIgnoreFailure("", 1.0, ImmutableMap.of()));
  }

  @Test
  public void writeMetricIgnoreFailure_createTimeSeriesThrowsException_doesNotThrow()
      throws IOException {
    String metricName = "test_metric";
    double value = 1.0;
    ImmutableMap<String, String> metricLabels = ImmutableMap.of("test_label", "test_value");

    doThrow(new RuntimeException()).when(cloudMonitoringClientWrapper).createTimeSeries(any());

    try {
      metricsService.writeMetricIgnoreFailure(metricName, value, metricLabels);
    } catch (RuntimeException e) {
      throw new AssertionError("Expected no exception to be thrown.", e);
    }
  }

  @Test
  public void writeMetricIgnoreFailure_writesGaugeMetric() throws IOException {
    String metricName = "test_metric";
    double value = 1.0;
    ImmutableMap<String, String> metricLabels = ImmutableMap.of("test_label", "test_value");

    metricsService.writeMetricIgnoreFailure(metricName, value, metricLabels);
    verify(cloudMonitoringClientWrapper).createTimeSeries(createTimeSeriesRequestCaptor.capture());

    CreateTimeSeriesRequest actual = createTimeSeriesRequestCaptor.getValue();
    assertThat(actual.getTimeSeriesCount()).isEqualTo(1);
    assertThat(actual.getTimeSeries(0).getMetric().getType())
        .isEqualTo("custom.googleapis.com/test_metric");
    assertThat(actual.getTimeSeries(0).getMetric().getLabelsMap())
        .containsExactly("test_label", "test_value");
    assertThat(actual.getTimeSeries(0).getMetricKind()).isEqualTo(MetricKind.GAUGE);
    assertThat(actual.getTimeSeries(0).getValueType()).isEqualTo(ValueType.DOUBLE);
    assertThat(actual.getTimeSeries(0).getPointsCount()).isEqualTo(1);
    assertThat(actual.getTimeSeries(0).getPoints(0).getValue().getDoubleValue()).isEqualTo(value);
    assertThat(actual.getTimeSeries(0).getResource().getType()).isEqualTo("global");
    assertThat(actual.getTimeSeries(0).getResource().getLabelsMap())
        .containsExactly("project_id", PROJECT_ID);
  }

  @Test
  public void writeMetricIgnoreFailure_withNullLabels_writesGaugeMetric() throws IOException {
    String metricName = "test_metric";
    double value = 1.0;

    metricsService.writeMetricIgnoreFailure(metricName, value, null);
    verify(cloudMonitoringClientWrapper).createTimeSeries(createTimeSeriesRequestCaptor.capture());

    CreateTimeSeriesRequest actual = createTimeSeriesRequestCaptor.getValue();
    assertThat(actual.getTimeSeriesCount()).isEqualTo(1);
    assertThat(actual.getTimeSeries(0).getMetric().getLabelsMap()).isEmpty();
  }

  @Test
  public void getWorkerPoolCpuUtilizationData_emptyResponses_returnsEmptyList() throws IOException {
    String workerPoolName = "test-worker-pool";
    Duration windowDuration = Duration.ofMinutes(2);

    TimeSeries instanceCountTimeSeries = TimeSeries.getDefaultInstance();
    TimeSeries cpuUtilizationTimeSeries = TimeSeries.getDefaultInstance();

    when(cloudMonitoringClientWrapper.listTimeSeries(any()))
        .thenReturn(instanceCountListTimeSeriesPagedResponse)
        .thenReturn(cpuUtilizationListTimeSeriesPagedResponse);
    when(instanceCountListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(instanceCountTimeSeries));
    when(cpuUtilizationListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(cpuUtilizationTimeSeries));

    assertThat(metricsService.getWorkerPoolCpuUtilizationData(workerPoolName, windowDuration))
        .hasValue(ImmutableList.of());
  }

  @Test
  public void getWorkerPoolCpuUtilizationData_withMismatchSizeResponseSizes_returnsEmpty()
      throws IOException {
    String workerPoolName = "test-worker-pool";
    Duration windowDuration = Duration.ofMinutes(2);

    TimeSeries instanceCountTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(3)))
            .build();

    TimeSeries cpuUtilizationTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(5.0)))
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(2.0)))
            .build();

    when(cloudMonitoringClientWrapper.listTimeSeries(any()))
        .thenReturn(instanceCountListTimeSeriesPagedResponse)
        .thenReturn(cpuUtilizationListTimeSeriesPagedResponse);
    when(instanceCountListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(instanceCountTimeSeries));
    when(cpuUtilizationListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(cpuUtilizationTimeSeries));

    assertThat(metricsService.getWorkerPoolCpuUtilizationData(workerPoolName, windowDuration))
        .isEmpty();
  }

  @Test
  public void
      getWorkerPoolCpuUtilizationData_withResponseSizeNotMatchingWindowDuration_returnsEmpty()
          throws IOException {
    String workerPoolName = "test-worker-pool";
    Duration windowDuration = Duration.ofMinutes(2);

    TimeSeries instanceCountTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(3)))
            .build();

    TimeSeries cpuUtilizationTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(5.0)))
            .build();

    when(cloudMonitoringClientWrapper.listTimeSeries(any()))
        .thenReturn(instanceCountListTimeSeriesPagedResponse)
        .thenReturn(cpuUtilizationListTimeSeriesPagedResponse);
    when(instanceCountListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(instanceCountTimeSeries));
    when(cpuUtilizationListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(cpuUtilizationTimeSeries));

    assertThat(metricsService.getWorkerPoolCpuUtilizationData(workerPoolName, windowDuration))
        .hasValue(
            ImmutableList.of(
                new MetricsService.InstanceCountUtilization(
                    instanceCountTimeSeries.getPoints(0).getValue().getDoubleValue(),
                    cpuUtilizationTimeSeries.getPoints(0).getValue().getDoubleValue())));
  }

  @Test
  public void getWorkerPoolCpuUtilizationData_returnsData() throws IOException {
    String workerPoolName = "test-worker-pool";
    Duration windowDuration = Duration.ofMinutes(2);

    TimeSeries instanceCountTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(3)))
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(2)))
            .build();

    TimeSeries cpuUtilizationTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(5.0)))
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(10.0)))
            .build();

    when(cloudMonitoringClientWrapper.listTimeSeries(listTimeSeriesRequestCaptor.capture()))
        .thenReturn(instanceCountListTimeSeriesPagedResponse)
        .thenReturn(cpuUtilizationListTimeSeriesPagedResponse);
    when(instanceCountListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(instanceCountTimeSeries));
    when(cpuUtilizationListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(cpuUtilizationTimeSeries));

    assertThat(metricsService.getWorkerPoolCpuUtilizationData(workerPoolName, windowDuration))
        .hasValue(
            ImmutableList.of(
                new MetricsService.InstanceCountUtilization(3, 5),
                new MetricsService.InstanceCountUtilization(2, 10)));

    ListTimeSeriesRequest instanceCountRequest = listTimeSeriesRequestCaptor.getAllValues().get(0);
    assertThat(instanceCountRequest.getName()).isEqualTo(ProjectName.of(PROJECT_ID).toString());
    assertThat(instanceCountRequest.getFilter())
        .isEqualTo(
            String.format(
                "metric.type=\"run.googleapis.com/container/instance_count\" AND"
                    + " resource.type=\"cloud_run_worker_pool\" AND"
                    + " resource.label.worker_pool_name=\"%s\" AND"
                    + " metric.labels.state=\"active\"",
                workerPoolName));
    assertThat(instanceCountRequest.getAggregation().getAlignmentPeriod().getSeconds())
        .isEqualTo(60);
    assertThat(instanceCountRequest.getAggregation().getPerSeriesAligner())
        .isEqualTo(Aggregation.Aligner.ALIGN_MEAN);
    assertThat(instanceCountRequest.getAggregation().getCrossSeriesReducer())
        .isEqualTo(Aggregation.Reducer.REDUCE_SUM);

    ListTimeSeriesRequest cpuUtilizationRequest = listTimeSeriesRequestCaptor.getAllValues().get(1);

    assertThat(cpuUtilizationRequest.getName()).isEqualTo(ProjectName.of(PROJECT_ID).toString());
    assertThat(cpuUtilizationRequest.getFilter())
        .isEqualTo(
            String.format(
                "metric.type=\"run.googleapis.com/container/cpu/utilizations\" AND"
                    + " resource.type=\"cloud_run_worker_pool\" AND"
                    + " resource.label.worker_pool_name=\"%s\"",
                workerPoolName));
    assertThat(cpuUtilizationRequest.getAggregation().getAlignmentPeriod().getSeconds())
        .isEqualTo(60);
    assertThat(cpuUtilizationRequest.getAggregation().getPerSeriesAligner())
        .isEqualTo(Aggregation.Aligner.ALIGN_PERCENTILE_50);
    assertThat(cpuUtilizationRequest.getAggregation().getCrossSeriesReducer())
        .isEqualTo(Aggregation.Reducer.REDUCE_MEAN);

    TimeInterval cpuUtilizationInterval = cpuUtilizationRequest.getInterval();
    TimeInterval instanceCountRequestInterval = instanceCountRequest.getInterval();
    assertThat(
            cpuUtilizationInterval.getEndTime().getSeconds()
                - cpuUtilizationInterval.getStartTime().getSeconds())
        .isEqualTo(windowDuration.toSeconds());
    assertThat(cpuUtilizationInterval).isEqualTo(instanceCountRequestInterval);
  }

  @Test
  public void getServiceCpuUtilizationData_emptyResponses_returnsEmptyList() throws IOException {
    String serviceName = "test-worker-pool";
    Duration windowDuration = Duration.ofMinutes(2);

    TimeSeries instanceCountTimeSeries = TimeSeries.newBuilder().build();

    TimeSeries cpuUtilizationTimeSeries = TimeSeries.newBuilder().build();

    when(cloudMonitoringClientWrapper.listTimeSeries(any()))
        .thenReturn(instanceCountListTimeSeriesPagedResponse)
        .thenReturn(cpuUtilizationListTimeSeriesPagedResponse);
    when(instanceCountListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(instanceCountTimeSeries));
    when(cpuUtilizationListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(cpuUtilizationTimeSeries));

    assertThat(metricsService.getServiceCpuUtilizationData(serviceName, windowDuration))
        .hasValue(ImmutableList.of());
  }

  @Test
  public void getServiceCpuUtilizationData_withMismatchSizeResponseSizes_returnsEmpty()
      throws IOException {
    String serviceName = "test-worker-pool";
    Duration windowDuration = Duration.ofMinutes(2);

    TimeSeries instanceCountTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(3)))
            .build();

    TimeSeries cpuUtilizationTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(5.0)))
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(2.0)))
            .build();

    when(cloudMonitoringClientWrapper.listTimeSeries(any()))
        .thenReturn(instanceCountListTimeSeriesPagedResponse)
        .thenReturn(cpuUtilizationListTimeSeriesPagedResponse);
    when(instanceCountListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(instanceCountTimeSeries));
    when(cpuUtilizationListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(cpuUtilizationTimeSeries));

    assertThat(metricsService.getServiceCpuUtilizationData(serviceName, windowDuration)).isEmpty();
  }

  @Test
  public void getServiceCpuUtilizationData_withResponseSizeNotMatchingWindowDuration_returnsEmpty()
      throws IOException {
    String serviceName = "test-worker-pool";
    Duration windowDuration = Duration.ofMinutes(2);

    TimeSeries instanceCountTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(3)))
            .build();

    TimeSeries cpuUtilizationTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(5.0)))
            .build();

    when(cloudMonitoringClientWrapper.listTimeSeries(any()))
        .thenReturn(instanceCountListTimeSeriesPagedResponse)
        .thenReturn(cpuUtilizationListTimeSeriesPagedResponse);
    when(instanceCountListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(instanceCountTimeSeries));
    when(cpuUtilizationListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(cpuUtilizationTimeSeries));

    assertThat(metricsService.getServiceCpuUtilizationData(serviceName, windowDuration))
        .hasValue(
            ImmutableList.of(
                new MetricsService.InstanceCountUtilization(
                    instanceCountTimeSeries.getPoints(0).getValue().getDoubleValue(),
                    cpuUtilizationTimeSeries.getPoints(0).getValue().getDoubleValue())));
  }

  @Test
  public void getServiceCpuUtilizationData_returnsData() throws IOException {
    String serviceName = "test-worker-pool";
    Duration windowDuration = Duration.ofMinutes(2);

    TimeSeries instanceCountTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(3)))
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(2)))
            .build();

    TimeSeries cpuUtilizationTimeSeries =
        TimeSeries.newBuilder()
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(5.0)))
            .addPoints(Point.newBuilder().setValue(TypedValue.newBuilder().setDoubleValue(10.0)))
            .build();

    when(cloudMonitoringClientWrapper.listTimeSeries(listTimeSeriesRequestCaptor.capture()))
        .thenReturn(instanceCountListTimeSeriesPagedResponse)
        .thenReturn(cpuUtilizationListTimeSeriesPagedResponse);
    when(instanceCountListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(instanceCountTimeSeries));
    when(cpuUtilizationListTimeSeriesPagedResponse.iterateAll())
        .thenReturn(ImmutableList.of(cpuUtilizationTimeSeries));

    assertThat(metricsService.getServiceCpuUtilizationData(serviceName, windowDuration))
        .hasValue(
            ImmutableList.of(
                new MetricsService.InstanceCountUtilization(
                    instanceCountTimeSeries.getPoints(0).getValue().getDoubleValue(),
                    cpuUtilizationTimeSeries.getPoints(0).getValue().getDoubleValue()),
                new MetricsService.InstanceCountUtilization(
                    instanceCountTimeSeries.getPoints(1).getValue().getDoubleValue(),
                    cpuUtilizationTimeSeries.getPoints(1).getValue().getDoubleValue())));

    ListTimeSeriesRequest instanceCountRequest = listTimeSeriesRequestCaptor.getAllValues().get(0);
    assertThat(instanceCountRequest.getName()).isEqualTo(ProjectName.of(PROJECT_ID).toString());
    assertThat(instanceCountRequest.getFilter())
        .isEqualTo(
            String.format(
                "metric.type=\"run.googleapis.com/container/instance_count\" AND"
                    + " resource.type=\"cloud_run_revision\" AND"
                    + " resource.label.service_name=\"%s\""
                    + " metric.labels.state=\"active\"",
                serviceName));
    assertThat(instanceCountRequest.getAggregation().getAlignmentPeriod().getSeconds())
        .isEqualTo(60);
    assertThat(instanceCountRequest.getAggregation().getPerSeriesAligner())
        .isEqualTo(Aggregation.Aligner.ALIGN_MEAN);
    assertThat(instanceCountRequest.getAggregation().getCrossSeriesReducer())
        .isEqualTo(Aggregation.Reducer.REDUCE_SUM);

    ListTimeSeriesRequest cpuUtilizationRequest = listTimeSeriesRequestCaptor.getAllValues().get(1);

    assertThat(cpuUtilizationRequest.getName()).isEqualTo(ProjectName.of(PROJECT_ID).toString());
    assertThat(cpuUtilizationRequest.getFilter())
        .isEqualTo(
            String.format(
                "metric.type=\"run.googleapis.com/container/cpu/utilizations\" AND"
                    + " resource.type=\"cloud_run_revision\" AND"
                    + " resource.label.service_name=\"%s\"",
                serviceName));
    assertThat(cpuUtilizationRequest.getAggregation().getAlignmentPeriod().getSeconds())
        .isEqualTo(60);
    assertThat(cpuUtilizationRequest.getAggregation().getPerSeriesAligner())
        .isEqualTo(Aggregation.Aligner.ALIGN_PERCENTILE_50);
    assertThat(cpuUtilizationRequest.getAggregation().getCrossSeriesReducer())
        .isEqualTo(Aggregation.Reducer.REDUCE_MEAN);

    TimeInterval cpuUtilizationInterval = cpuUtilizationRequest.getInterval();
    TimeInterval instanceCountRequestInterval = instanceCountRequest.getInterval();
    assertThat(
            cpuUtilizationInterval.getEndTime().getSeconds()
                - cpuUtilizationInterval.getStartTime().getSeconds())
        .isEqualTo(windowDuration.toSeconds());
    assertThat(cpuUtilizationInterval).isEqualTo(instanceCountRequestInterval);
  }
}
