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

import static java.nio.charset.StandardCharsets.UTF_8;

import com.google.common.base.Utf8;
import com.google.common.collect.ImmutableMap;
import com.google.common.flogger.FluentLogger;
import com.sun.net.httpserver.HttpExchange;
import java.io.IOException;
import java.io.OutputStream;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/** Utility class for sending HTTP responses. */
final class RequestResponseHandler {
  private static final FluentLogger logger = FluentLogger.forEnclosingClass();

  private RequestResponseHandler() {}

  static ImmutableMap<String, String> getRequestHeaderMap(HttpExchange exchange) {
    if (exchange == null || exchange.getRequestHeaders() == null) {
      return ImmutableMap.of();
    }

    Map<String, String> headers = new HashMap<>();
    for (Map.Entry<String, List<String>> entry : exchange.getRequestHeaders().entrySet()) {
      headers.put(entry.getKey(), String.join(",", entry.getValue()));
    }

    return ImmutableMap.copyOf(headers);
  }

  /**
   * Sends a failure response with a 500 status code and the exception message.
   *
   * @param exchange The HttpExchange object.
   * @param e The exception that caused the failure.
   */
  static void sendFailureResponse(HttpExchange exchange, Exception e) {
    String response = "Failure: " + e.getMessage();

    try (OutputStream os = exchange.getResponseBody()) {
      exchange.sendResponseHeaders(500, Utf8.encodedLength(response));
      os.write(response.getBytes(UTF_8));
    } catch (IOException io) {
      // Processing has already completed. Just log the error.
      logger.atWarning().withCause(io).log("Failed while writing response");
    }
  }

  /**
   * Sends a success response with a 200 status code.
   *
   * @param exchange The HttpExchange object.
   */
  static void sendSuccessResponse(HttpExchange exchange) {
    String response = "OK: Successfully refreshed scaling recommendation.";

    try (OutputStream os = exchange.getResponseBody()) {
      exchange.sendResponseHeaders(200, Utf8.encodedLength(response));
      os.write(response.getBytes(UTF_8));
    } catch (IOException io) {
      // Processing has already completed. Just log the error.
      logger.atWarning().withCause(io).log("Failed while writing successful response");
    }
  }
}
