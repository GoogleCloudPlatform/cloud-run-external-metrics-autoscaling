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

import com.google.gson.Gson;
import com.google.gson.reflect.TypeToken;
import java.util.Map;
import java.util.logging.Level;
import java.util.logging.LogRecord;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public class CremaLogFormatterTest {

  private static final Gson GSON = new Gson();

  @Test
  public void format_whenJson_returnsJsonPayload() {
    CremaLogFormatter formatter = new CremaLogFormatter("json");
    LogRecord record = new LogRecord(Level.INFO, "Test message");

    String output = formatter.format(record);

    Map<String, String> parsed = GSON.fromJson(output, new TypeToken<Map<String, String>>(){}.getType());
    assertThat(parsed).containsEntry("severity", "INFO");
    assertThat(parsed).containsEntry("message", "Test message");
    assertThat(parsed).doesNotContainKey("exception");
  }

  @Test
  public void format_whenJsonWithException_includesException() {
    CremaLogFormatter formatter = new CremaLogFormatter("json");
    LogRecord record = new LogRecord(Level.SEVERE, "Error occurred");
    record.setThrown(new RuntimeException("Test exception"));

    String output = formatter.format(record);

    Map<String, String> parsed = GSON.fromJson(output, new TypeToken<Map<String, String>>(){}.getType());
    assertThat(parsed).containsEntry("severity", "ERROR");
    assertThat(parsed).containsEntry("message", "Error occurred");
    assertThat(parsed.get("exception")).contains("Test exception");
    assertThat(parsed.get("exception")).contains("RuntimeException");
  }

  @Test
  public void format_whenNotJson_returnsPlainText() {
    CremaLogFormatter formatter = new CremaLogFormatter("plain");
    LogRecord record = new LogRecord(Level.INFO, "Test message");

    String output = formatter.format(record);

    // SimpleFormatter default format depends on system properties,
    // but we can ensure it's not a JSON object
    assertThat(output).doesNotMatch("^\\{.*\\}$");
    assertThat(output).contains("Test message");
  }

  @Test
  public void format_whenNullFormat_returnsPlainText() {
    CremaLogFormatter formatter = new CremaLogFormatter(null);
    LogRecord record = new LogRecord(Level.INFO, "Test message");

    String output = formatter.format(record);

    assertThat(output).doesNotMatch("^\\{.*\\}$");
    assertThat(output).contains("Test message");
  }

  @Test
  public void format_whenJsonAndMatchesFloggerMetadata_extractsStructuredFields() {
    CremaLogFormatter formatter = new CremaLogFormatter("json");
    LogRecord record = new LogRecord(Level.INFO, "Recommendation was clamped to range [CONTEXT resource=\"projects/foo/workerpools/bar\" minReplicaCount=1 maxReplicaCount=100]");

    String output = formatter.format(record);

    Map<String, Object> parsed = GSON.fromJson(output, new TypeToken<Map<String, Object>>(){}.getType());
    assertThat(parsed).containsEntry("severity", "INFO");
    assertThat(parsed).containsEntry("message", "Recommendation was clamped to range");
    assertThat(parsed).containsEntry("resource", "projects/foo/workerpools/bar");

    // GSON parses JSON numbers as Doubles by default for Object types
    assertThat(((Number) parsed.get("minReplicaCount")).intValue()).isEqualTo(1);
    assertThat(((Number) parsed.get("maxReplicaCount")).intValue()).isEqualTo(100);
  }
}
