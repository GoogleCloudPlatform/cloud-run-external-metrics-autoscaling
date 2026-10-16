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

import com.google.api.MetricDescriptor.MetricKind;
import com.google.api.MetricDescriptor.ValueType;
import com.google.cloud.run.crema.clients.CloudMonitoringClientWrapper;
import com.google.common.collect.ImmutableMap;
import com.google.monitoring.v3.CreateTimeSeriesRequest;
import java.io.IOException;
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

  @Captor private ArgumentCaptor<CreateTimeSeriesRequest> createTimeSeriesRequestCaptor;

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
}
