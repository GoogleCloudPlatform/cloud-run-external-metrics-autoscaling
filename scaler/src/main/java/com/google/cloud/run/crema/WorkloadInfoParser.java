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

import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** A utility class for parsing workload strings into their constituent parts. */
public class WorkloadInfoParser {

  private static final Pattern SERVICE_PATTERN =
      Pattern.compile("projects/([^/]+)/locations/([^/]+)/services/([^/]+)");

  // `workerpools` and `workerPools` are both valid depending on Cloud Run API version
  private static final Pattern WORKERPOOL_PATTERN =
      Pattern.compile("projects/([^/]+)/locations/([^/]+)/worker[Pp]ools/([^/]+)");

  private WorkloadInfoParser() {}

  /** An enum representing the type of workload. */
  public static enum WorkloadType {
    SERVICE,
    WORKERPOOL
  }

  /** A class representing the parsed information from a workload string. */
  public record WorkloadInfo(
      WorkloadType workloadType, String projectId, String location, String name) {}

  /**
   * Parses a workload string into a WorkloadInfo object.
   *
   * @param input The workload string to parse.
   * @return A WorkloadInfo object containing the parsed information.
   * @throws IllegalArgumentException if the input string is not a valid workload string.
   */
  public static WorkloadInfo parse(String input) {
    Matcher serviceMatcher = SERVICE_PATTERN.matcher(input);
    Matcher workerpoolMatcher = WORKERPOOL_PATTERN.matcher(input);

    if (serviceMatcher.matches()) {
      return new WorkloadInfo(
          WorkloadType.SERVICE,
          serviceMatcher.group(1),
          serviceMatcher.group(2),
          serviceMatcher.group(3));
    } else if (workerpoolMatcher.matches()) {
      return new WorkloadInfo(
          WorkloadType.WORKERPOOL,
          workerpoolMatcher.group(1),
          workerpoolMatcher.group(2),
          workerpoolMatcher.group(3));
    } else {
      throw new IllegalArgumentException(
          String.format(
              "Invalid workload name `%s`. Expected "
                  + "'projects/[PROJECT]/locations/[LOCATION]/services/[SERVICE]' or "
                  + "'projects/[PROJECT]/locations/[LOCATION]/workerpools/[WORKERPOOL]'.",
              input));
    }
  }
}
