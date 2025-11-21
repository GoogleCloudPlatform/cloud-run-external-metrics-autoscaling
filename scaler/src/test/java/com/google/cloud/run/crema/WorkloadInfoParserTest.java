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

import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class WorkloadInfoParserTest {

  @Test
  public void workloadParser_stringForInvalidWorkload_throwsIllegalArgumentException() {
    assertThrows(
        IllegalArgumentException.class,
        () ->
            WorkloadInfoParser.parse("projects/my-project/locations/us-central1/completely/wrong"));
    assertThrows(
        IllegalArgumentException.class,
        () ->
            WorkloadInfoParser.parse(
                "projects/my-project/locations/us-central1/services/workerpools/both-workloads"));
  }

  @Test
  public void workloadParser_stringForServiceWorkload_parsesServiceInfo() {
    String input = "projects/my-project/locations/us-central1/services/my-service";

    WorkloadInfoParser.WorkloadInfo workloadInfo = WorkloadInfoParser.parse(input);

    assertThat(workloadInfo.workloadType()).isEqualTo(WorkloadInfoParser.WorkloadType.SERVICE);
    assertThat(workloadInfo.projectId()).isEqualTo("my-project");
    assertThat(workloadInfo.location()).isEqualTo("us-central1");
    assertThat(workloadInfo.name()).isEqualTo("my-service");
  }

  @Test
  public void workloadParser_stringForWorkerPoolWorkload_parsesWorkerPoolInfo() {
    String input = "projects/my-project/locations/us-central1/workerpools/my-workerpool";

    WorkloadInfoParser.WorkloadInfo workloadInfo = WorkloadInfoParser.parse(input);

    assertThat(workloadInfo.workloadType()).isEqualTo(WorkloadInfoParser.WorkloadType.WORKERPOOL);
    assertThat(workloadInfo.projectId()).isEqualTo("my-project");
    assertThat(workloadInfo.location()).isEqualTo("us-central1");
    assertThat(workloadInfo.name()).isEqualTo("my-workerpool");
  }
}
