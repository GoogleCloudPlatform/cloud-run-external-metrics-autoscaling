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

import com.google.api.Metric;
import com.google.api.MetricDescriptor.MetricKind;
import com.google.api.MetricDescriptor.ValueType;
import com.google.api.MonitoredResource;
import com.google.cloud.run.crema.clients.CloudMonitoringClientWrapper;
import com.google.common.base.Preconditions;
import com.google.common.base.Strings;
import com.google.common.collect.ImmutableMap;
import com.google.common.flogger.FluentLogger;
import com.google.monitoring.v3.CreateTimeSeriesRequest;
import com.google.monitoring.v3.Point;
import com.google.monitoring.v3.ProjectName;
import com.google.monitoring.v3.TimeInterval;
import com.google.monitoring.v3.TimeSeries;
import com.google.monitoring.v3.TypedValue;
import com.google.protobuf.util.Timestamps;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/** Encapsulates the logic of interacting with Cloud Monitoring */
public class MetricsService {
  private static final FluentLogger logger = FluentLogger.forEnclosingClass();

  private final CloudMonitoringClientWrapper cloudMonitoringClient;
  private final ProjectName projectName;
  private final ImmutableMap<String, String> resourceLabels;

  public MetricsService(CloudMonitoringClientWrapper cloudMonitoringClient, String projectId) {
    Preconditions.checkNotNull(cloudMonitoringClient, "Cloud Monitoring Client cannot be null.");

    if (Strings.isNullOrEmpty(projectId)) {
      throw new IllegalArgumentException("Project ID cannot be null or empty.");
    }

    this.cloudMonitoringClient = cloudMonitoringClient;
    this.projectName = ProjectName.of(projectId);
    this.resourceLabels = ImmutableMap.of("project_id", projectId);
  }

  /**
   * Writes a custom gauge metric to Cloud Monitoring ignoring any failures.
   *
   * @param metricName The name of the custom metric.
   * @param value The value of the metric.
   * @param metricLabels A map of labels for the metric.
   */
  public void writeMetricIgnoreFailure(
      String metricName, double value, Map<String, String> metricLabels) {
    if (Strings.isNullOrEmpty(metricName)) {
      throw new IllegalArgumentException("metricName cannot be null or empty.");
    }

    Instant now = Instant.now();

    TimeInterval interval =
        TimeInterval.newBuilder().setEndTime(Timestamps.fromMillis(now.toEpochMilli())).build();

    TypedValue typedValue = TypedValue.newBuilder().setDoubleValue(value).build();

    Point point = Point.newBuilder().setInterval(interval).setValue(typedValue).build();

    String metricType = String.format("custom.googleapis.com/%s", metricName);
    metricLabels = metricLabels != null ? metricLabels : new HashMap<>();
    Metric metric = Metric.newBuilder().setType(metricType).putAllLabels(metricLabels).build();

    MonitoredResource resource =
        MonitoredResource.newBuilder().setType("global").putAllLabels(resourceLabels).build();

    TimeSeries timeSeries =
        TimeSeries.newBuilder()
            .setMetric(metric)
            .setResource(resource)
            .addAllPoints(Collections.singletonList(point))
            .setMetricKind(MetricKind.GAUGE)
            .setValueType(ValueType.DOUBLE)
            .build();

    List<TimeSeries> timeSeriesList = new ArrayList<>();
    timeSeriesList.add(timeSeries);

    CreateTimeSeriesRequest request =
        CreateTimeSeriesRequest.newBuilder()
            .setName(projectName.toString())
            .addAllTimeSeries(timeSeriesList)
            .build();

    try {
      cloudMonitoringClient.createTimeSeries(request);
    } catch (RuntimeException e) {
      // Exceptions may occur if emitting metrics too frequently i.e. for frequently invoked scaler
      // or even on instance restart.
      logger.atFine().log("Failed to create time series: %s", e.getMessage());
    }
  }
}
