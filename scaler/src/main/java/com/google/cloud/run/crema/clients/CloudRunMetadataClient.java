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

import static java.nio.charset.StandardCharsets.UTF_8;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.net.HttpURLConnection;
import java.net.URI;
import java.net.URISyntaxException;
import java.net.URL;

/**
 * Client for retrieving metadata about this instance from the Cloud Run metadata server.
 *
 * <p>See: https://cloud.google.com/run/docs/container-contract#metadata-server
 */
public class CloudRunMetadataClient {

  private static final String METADATA_SERVER_REGION_URL =
      "http://metadata.google.internal/computeMetadata/v1/instance/region";

  /** Constructs a new {@code CloudRunMetadataClient}. */
  public CloudRunMetadataClient() {}

  /**
   * Retrieves the project number and region from the Cloud Run metadata server.
   *
   * @return The project number and region as a string, e.g., "projects/12345/regions/us-central1".
   * @throws IOException If an I/O error occurs while communicating with the metadata server.
   */
  public String projectNumberRegion() throws IOException {
    try {
      URL url = new URI(METADATA_SERVER_REGION_URL).toURL();
      HttpURLConnection connection = (HttpURLConnection) url.openConnection();
      connection.setRequestMethod("GET");
      connection.setRequestProperty("Metadata-Flavor", "Google");
      StringBuilder response = new StringBuilder();

      try (BufferedReader in =
          new BufferedReader(new InputStreamReader(connection.getInputStream(), UTF_8))) {
        String inputLine;
        while ((inputLine = in.readLine()) != null) {
          response.append(inputLine);
        }
      }

      return response.toString();
    } catch (URISyntaxException e) {
      throw new IOException("Failed to parse metadata URL " + METADATA_SERVER_REGION_URL, e);
    }
  }
}
