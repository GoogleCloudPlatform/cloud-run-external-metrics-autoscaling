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

import com.google.cloud.monitoring.v3.MetricServiceClient;
import com.google.monitoring.v3.CreateTimeSeriesRequest;
import com.google.monitoring.v3.ListTimeSeriesRequest;
import com.google.common.flogger.FluentLogger;
import java.io.IOException;

/** Wrapper around Cloud Monitoring to allow mocking in tests. */
public class CloudMonitoringClientWrapper {

  private static final FluentLogger logger = FluentLogger.forEnclosingClass();
  private final MetricServiceClient metricServiceClient;

  public static MetricServiceClient metricServiceClient() throws IOException {
    logger.atInfo().log("Creating MetricServiceClient...");
    MetricServiceClient client = MetricServiceClient.create();
    return client;
  }

  public CloudMonitoringClientWrapper(MetricServiceClient metricServiceClient) {
    this.metricServiceClient = metricServiceClient;
  }

  public void createTimeSeries(CreateTimeSeriesRequest request) {
    metricServiceClient.createTimeSeries(request);
  }

  public MetricServiceClient.ListTimeSeriesPagedResponse listTimeSeries(
      ListTimeSeriesRequest request) {
    return metricServiceClient.listTimeSeries(request);
  }
}
