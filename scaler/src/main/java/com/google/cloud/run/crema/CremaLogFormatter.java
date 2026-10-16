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

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import java.io.PrintWriter;
import java.io.StringWriter;
import java.util.HashMap;
import java.util.Map;
import java.util.logging.Formatter;
import java.util.logging.Level;
import java.util.logging.LogRecord;
import java.util.logging.SimpleFormatter;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
/**
 * A custom formatter that outputs either plain-text or JSON structured logs
 * depending on the LOG_FORMAT environment variable.
 */
public class CremaLogFormatter extends Formatter {
  private final Formatter delegate;
  private static final Gson GSON = new GsonBuilder().create();

  public CremaLogFormatter() {
    this(System.getenv("LOG_FORMAT"));
  }

  // Visible for testing
  CremaLogFormatter(String logFormat) {
    if ("json".equalsIgnoreCase(logFormat)) {
      delegate = new JsonFormatter();
    } else {
      delegate = new SimpleFormatter();
    }
  }

  @Override
  public String format(LogRecord record) {
    return delegate.format(record);
  }

  private static class JsonFormatter extends Formatter {
    private static final String COMPONENT_NAME = "scaler";

    // 1. Matches the entire "[CONTEXT ...]" block at the end of the string
    private static final Pattern CONTEXT_BLOCK = Pattern.compile(" \\[(CONTEXT .*?)\\]$");

    // 2. Matches individual key=value or key="value" pairs inside the block
    private static final Pattern CONTEXT_PAIRS = Pattern.compile("(\\w+)=(?:\"([^\"]*)\"|([^ ]+))");

    @Override
    public String format(LogRecord record) {
      Map<String, Object> logEntry = new HashMap<>();

      logEntry.put("severity", getSeverityString(record.getLevel()));
      logEntry.put("component", COMPONENT_NAME);

      String msg = formatMessage(record);

      Matcher blockMatcher = CONTEXT_BLOCK.matcher(msg);
      if (blockMatcher.find()) {
        String msgContent = msg.substring(0, blockMatcher.start());
        logEntry.put("message", msgContent);

        String contextData = blockMatcher.group(1);
        Matcher pairMatcher = CONTEXT_PAIRS.matcher(contextData);

        while (pairMatcher.find()) {
          String key = pairMatcher.group(1);
          String stringVal = pairMatcher.group(2);
          String otherVal = pairMatcher.group(3);

          if (stringVal != null) {
            logEntry.put(key, stringVal);
          } else if (otherVal != null) {
            try {
              logEntry.put(key, Integer.parseInt(otherVal));
            } catch (NumberFormatException e) {
              logEntry.put(key, otherVal);
            }
          }
        }
      } else {
        logEntry.put("message", msg != null ? msg : "");
      }

      if (record.getThrown() != null) {
        StringWriter sw = new StringWriter();
        PrintWriter pw = new PrintWriter(sw);
        record.getThrown().printStackTrace(pw);
        logEntry.put("exception", sw.toString());
      }

      return GSON.toJson(logEntry) + "\n";
    }

    /**
     * Maps java.util.logging.Level to standard Cloud Logging severity values.
     */
    private String getSeverityString(Level level) {
      if (level == Level.SEVERE) {
        return "ERROR";
      } else if (level == Level.WARNING) {
        return "WARNING";
      } else if (level == Level.INFO) {
        return "INFO";
      } else if (level == Level.CONFIG || level == Level.FINE || level == Level.FINER || level == Level.FINEST) {
        return "DEBUG";
      }
      return "INFO";
    }
  }
}
