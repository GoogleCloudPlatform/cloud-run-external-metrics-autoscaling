#
# Copyright 2025 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

package(default_visibility = ["//visibility:private"])

java_test(
    name = "CloudRunClientWrapperTest",
    srcs = ["CloudRunClientWrapperTest.java"],
    test_class = "com.google.cloud.run.crema.clients.CloudRunClientWrapperTest",
    deps = [
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_run_client_wrapper",
        "@maven//:com_google_api_gax",
        "@maven//:com_google_api_grpc_proto_google_cloud_run_v2",
        "@maven//:com_google_cloud_google_cloud_run",
        "@maven//:com_google_flogger_flogger",
        "@maven//:com_google_flogger_flogger_system_backend",
        "@maven//:com_google_truth_truth",
        "@maven//:junit_junit",
        "@maven//:org_mockito_mockito_core",
        "@protobuf//java/core",
    ],
)
