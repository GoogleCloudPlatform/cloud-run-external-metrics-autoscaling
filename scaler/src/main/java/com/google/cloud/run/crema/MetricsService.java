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
import com.google.cloud.monitoring.v3.MetricServiceClient.ListTimeSeriesPagedResponse;
import com.google.cloud.run.crema.clients.CloudMonitoringClientWrapper;
import com.google.common.base.Preconditions;
import com.google.common.base.Strings;
import com.google.common.collect.ImmutableMap;
import com.google.common.flogger.FluentLogger;
import com.google.monitoring.v3.Aggregation;
import com.google.monitoring.v3.CreateTimeSeriesRequest;
import com.google.monitoring.v3.ListTimeSeriesRequest;
import com.google.monitoring.v3.Point;
import com.google.monitoring.v3.ProjectName;
import com.google.monitoring.v3.TimeInterval;
import com.google.monitoring.v3.TimeSeries;
import com.google.monitoring.v3.TypedValue;
import com.google.protobuf.util.Durations;
import com.google.protobuf.util.Timestamps;
import java.io.IOException;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/** Encapsulates the logic of interacting with Cloud Monitoring */
public class MetricsService {
  private static final FluentLogger logger = FluentLogger.forEnclosingClass();

  /** Record class to hold instance count and utilization data. */
  public record InstanceCountUtilization(double instanceCount, double utilization) {}

  private static final Duration CLOUD_MONITORING_INGESTION_LAG = Duration.ofMinutes(2);

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

  /**
   * Returns a list of instance count and utilization data for a Cloud Run service.
   *
   * @param serviceName The name of the Cloud Run service.
   * @param windowDuration The duration of the window to look at data over.
   * @return A list of instance count and utilization data or an empty optional if there were no
   *     data points.
   * @throws IOException If there is an error communicating with Cloud Monitoring.
   */
  public Optional<List<InstanceCountUtilization>> getServiceCpuUtilizationData(
      String serviceName, Duration windowDuration) throws IOException {
    String cpuUtilizationFilter =
        String.format(
            "metric.type=\"run.googleapis.com/container/cpu/utilizations\" AND"
                + " resource.type=\"cloud_run_revision\" AND"
                + " resource.label.service_name=\"%s\"",
            serviceName);
    String instanceCountFilter =
        String.format(
            "metric.type=\"run.googleapis.com/container/instance_count\" AND"
                + " resource.type=\"cloud_run_revision\" AND"
                + " resource.label.service_name=\"%s\""
                + " metric.labels.state=\"active\"",
            serviceName);
    return getCpuUtilizationData(cpuUtilizationFilter, instanceCountFilter, windowDuration);
  }

  /**
   * Returns a list of instance count and utilization data for a Cloud Run worker pool.
   *
   * @param workerPoolName The name of the Cloud Run worker pool.
   * @param windowDuration The duration of the window to look at data over.
   * @return A list of instance count and utilization data or an empty optional if there were no
   *     data points.
   * @throws IOException If there is an error communicating with Cloud Monitoring.
   */
  public Optional<List<InstanceCountUtilization>> getWorkerPoolCpuUtilizationData(
      String workerPoolName, Duration windowDuration) throws IOException {
    String cpuUtilizationFilter =
        String.format(
            "metric.type=\"run.googleapis.com/container/cpu/utilizations\" AND"
                + " resource.type=\"cloud_run_worker_pool\" AND"
                + " resource.label.worker_pool_name=\"%s\"",
            workerPoolName);
    String instanceCountFilter =
        String.format(
            "metric.type=\"run.googleapis.com/container/instance_count\" AND"
                + " resource.type=\"cloud_run_worker_pool\" AND"
                + " resource.label.worker_pool_name=\"%s\" AND"
                + " metric.labels.state=\"active\"",
            workerPoolName);
    return getCpuUtilizationData(cpuUtilizationFilter, instanceCountFilter, windowDuration);
  }

  private Optional<List<InstanceCountUtilization>> getCpuUtilizationData(
      String cpuUtilizationFilter, String instanceCountFilter, Duration windowDuration)
      throws IOException {
    List<InstanceCountUtilization> instanceCountToCpuUtilizationData = new ArrayList<>();

    Instant now = Instant.now();
    long startMillis =
        now.minus(CLOUD_MONITORING_INGESTION_LAG).minus(windowDuration).toEpochMilli();
    long endMillis = now.minus(CLOUD_MONITORING_INGESTION_LAG).toEpochMilli();

    TimeInterval interval =
        TimeInterval.newBuilder()
            .setStartTime(Timestamps.fromMillis(startMillis))
            .setEndTime(Timestamps.fromMillis(endMillis))
            .build();

    // /container/instance_count metric returns a time series per revision.
    // Take a mean-aligned sum to estimate the number of total instances.
    Aggregation instanceCountAggregation =
        Aggregation.newBuilder()
            .setAlignmentPeriod(Durations.fromSeconds(60))
            .setPerSeriesAligner(Aggregation.Aligner.ALIGN_MEAN)
            .setCrossSeriesReducer(Aggregation.Reducer.REDUCE_SUM)
            .build();

    ListTimeSeriesRequest instanceCountRequest =
        ListTimeSeriesRequest.newBuilder()
            .setName(projectName.toString())
            .setFilter(instanceCountFilter)
            .setInterval(interval)
            .setAggregation(instanceCountAggregation)
            .build();

    Aggregation cpuUtilizationAggregation =
        Aggregation.newBuilder()
            .setAlignmentPeriod(Durations.fromSeconds(60))
            .setPerSeriesAligner(Aggregation.Aligner.ALIGN_PERCENTILE_50)
            .setCrossSeriesReducer(Aggregation.Reducer.REDUCE_MEAN)
            .build();

    ListTimeSeriesRequest cpuUtilizationRequest =
        ListTimeSeriesRequest.newBuilder()
            .setName(projectName.toString())
            .setFilter(cpuUtilizationFilter)
            .setInterval(interval)
            .setAggregation(cpuUtilizationAggregation)
            .build();

    try {
      ListTimeSeriesPagedResponse instanceCountResponse =
          cloudMonitoringClient.listTimeSeries(instanceCountRequest);
      ListTimeSeriesPagedResponse cpuUtilizationResponse =
          cloudMonitoringClient.listTimeSeries(cpuUtilizationRequest);

      ArrayList<Double> instanceCounts = new ArrayList<>();
      ArrayList<Double> utilizations = new ArrayList<>();

      for (TimeSeries ts : instanceCountResponse.iterateAll()) {
        for (Point p : ts.getPointsList()) {
          instanceCounts.add(p.getValue().getDoubleValue());
        }
      }
      for (TimeSeries ts : cpuUtilizationResponse.iterateAll()) {
        for (Point p : ts.getPointsList()) {
          utilizations.add(p.getValue().getDoubleValue());
        }
      }

      // We assume that the intervals are aligned as long as there are the same number of data
      // points.
      if (instanceCounts.size() != utilizations.size()) {
        logger.atWarning().log(
            "Instance count data size (%d) does not match utilization data size (%d). Instance"
                + " count data: %s, Utilization data: %s",
            instanceCounts.size(), utilizations.size(), instanceCounts, utilizations);
        return Optional.empty();
      } else if (instanceCounts.size() != windowDuration.toMinutes()) {
        logger.atWarning().log(
            "Metrics Missing: Metrics data size (%d) does not match window minutes (%d)",
            instanceCounts.size(), windowDuration.toMinutes());
      }

      for (int i = 0; i < instanceCounts.size(); i++) {
        instanceCountToCpuUtilizationData.add(
            new InstanceCountUtilization(instanceCounts.get(i), utilizations.get(i)));
      }

    } catch (RuntimeException e) {
      throw new IOException("Failed to get timeseries from Cloud Monitoring", e);
    }

    return Optional.of(instanceCountToCpuUtilizationData);
  }
}
