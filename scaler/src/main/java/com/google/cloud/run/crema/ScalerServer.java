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

import static java.util.concurrent.TimeUnit.SECONDS;

import com.google.cloud.run.crema.clients.CloudMonitoringClientWrapper;
import com.google.cloud.run.crema.clients.CloudRunClientWrapper;
import com.google.common.flogger.FluentLogger;
import com.google.protobuf.TextFormat;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.health.v1.HealthCheckResponse.ServingStatus;
import io.grpc.services.HealthStatusManager;
import io.grpc.stub.StreamObserver;
import java.io.IOException;
import java.util.List;

/** Scaler Server */
public class ScalerServer {

  private static final int PORT = 50051;
  private static final FluentLogger logger = FluentLogger.forEnclosingClass();
  private static final String APPLICATION_NAME = "Cloud Run External Metrics Autocaler";

  private Server server;
  private final HealthStatusManager health;
  private final ScalersManager scalersManager;

  public ScalerServer() throws IOException {
    this.health = new HealthStatusManager();
    this.scalersManager =
        new ScalersManager(
            new CloudRunClientWrapper(),
            new CloudMonitoringClientWrapper(CloudMonitoringClientWrapper.metricServiceClient()));
  }

  private void start() throws IOException {
    server =
        ServerBuilder.forPort(PORT)
            .addService(new ScalerServerImpl(scalersManager))
            .addService(health.getHealthService())
            .build()
            .start();
    logger.atInfo().log("ScalerServer started, listening on %d", PORT);
    health.setStatus(ScalerGrpc.SERVICE_NAME, ServingStatus.SERVING);
    health.setStatus("", ServingStatus.SERVING);

    Runtime.getRuntime()
        .addShutdownHook(
            new Thread() {
              @Override
              public void run() {
                // Use stderr here since the logger may have been reset by its JVM shutdown hook.
                System.err.println("[SCALER] Shutting down gRPC server since JVM is shutting down");
                try {
                  ScalerServer.this.stop();
                } catch (InterruptedException e) {
                  e.printStackTrace(System.err);
                }
                System.err.println("[SCALER] Server shut down");
              }
            });
  }

  private void stop() throws InterruptedException {
    if (server != null) {
      health.setStatus(ScalerGrpc.SERVICE_NAME, ServingStatus.NOT_SERVING);
      health.setStatus("", ServingStatus.NOT_SERVING);
      logger.atInfo().log("[SCALER] Shutting down Scaler...");
      server.shutdown();
      if (server.awaitTermination(30, SECONDS)) {
        logger.atInfo().log("[SCALER] Scaler stopped gracefully.");
      } else {
        logger.atWarning().log("[SCALER] Scaler did not stop gracefully after 30 seconds.");
        server.shutdownNow();
      }
    }
  }

  /** Await termination on the main thread since the grpc library uses daemon threads. */
  private void blockUntilShutdown() throws InterruptedException {
    if (server != null) {
      server.awaitTermination();
    }
  }

  public static void main(String[] args) throws IOException, InterruptedException {
    System.out.println("[SCALER] Starting Scaler");
    final ScalerServer server = new ScalerServer();
    server.start();
    server.blockUntilShutdown();
  }

  static class ScalerServerImpl extends ScalerGrpc.ScalerImplBase {
    private final ScalersManager scalersManager;

    public ScalerServerImpl(ScalersManager scalersManager) {
      this.scalersManager = scalersManager;
    }

    @Override
    public void scale(ScaleRequest request, StreamObserver<ScaleResponse> responseObserver) {
      logger.atInfo().log(
          "[SCALER] Received ScaleRequest: %s", TextFormat.printer().shortDebugString(request));

      ScaleResponse.Builder responseBuilder = ScaleResponse.newBuilder();
      try {
        List<ScalingResult> results = scalersManager.scale(request.getScaledObjectMetricsList());
        responseBuilder.addAllResults(results);
      } catch (InterruptedException e) {
        logger.atWarning().withCause(e).log("[SCALER] Failed to get scaling result");
        Thread.currentThread().interrupt();
      }

      responseObserver.onNext(responseBuilder.build());
      responseObserver.onCompleted();
    }

    @Override
    public void schedule(
        ScheduleRequest request, StreamObserver<ScheduleResponse> responseObserver) {
      responseObserver.onNext(ScheduleResponse.getDefaultInstance());
      responseObserver.onCompleted();
    }
  }
}
