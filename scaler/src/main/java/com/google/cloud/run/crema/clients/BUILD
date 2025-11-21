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

package(default_visibility = ["//visibility:public"])

java_library(
    name = "cloud_tasks_client_wrapper",
    srcs = ["CloudTasksClientWrapper.java"],
    deps = [
        "@maven//:com_google_api_grpc_proto_google_cloud_tasks_v2",
        "@maven//:com_google_cloud_google_cloud_tasks",
    ],
)

java_library(
    name = "kafka_admin_client_wrapper",
    srcs = ["KafkaAdminClientWrapper.java"],
    deps = [
        "@maven//:org_apache_kafka_kafka_clients",
    ],
)

java_library(
    name = "cloud_run_client_wrapper",
    srcs = ["CloudRunClientWrapper.java"],
    deps = [
        "@maven//:com_google_api_gax",
        "@maven//:com_google_api_grpc_proto_google_cloud_run_v2",
        "@maven//:com_google_cloud_google_cloud_run",
        "@maven//:com_google_flogger_flogger",
        "@protobuf//java/core",
    ],
)

java_library(
    name = "cloud_monitoring_client_wrapper",
    srcs = ["CloudMonitoringClientWrapper.java"],
    deps = [
        "@maven//:com_google_api_grpc_proto_google_cloud_monitoring_v3",
        "@maven//:com_google_cloud_google_cloud_monitoring",
        "@maven//:com_google_flogger_flogger",
    ],
)

java_library(
    name = "cloud_run_metadata_client",
    srcs = ["CloudRunMetadataClient.java"],
)
