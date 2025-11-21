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
    name = "scaling_stabilizer",
    srcs = ["ScalingStabilizer.java"],
    deps = [
        "//proto:scaled_object_java_proto",
        "@maven//:com_google_flogger_flogger",
    ],
)

java_library(
    name = "workload_info_parser",
    srcs = ["WorkloadInfoParser.java"],
)

java_library(
    name = "self_scheduler",
    srcs = ["SelfScheduler.java"],
    deps = [
        ":configuration_provider",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_run_metadata_client",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_tasks_client_wrapper",
        "@maven//:com_google_api_grpc_proto_google_cloud_tasks_v2",
        "@maven//:com_google_flogger_flogger",
        "@maven//:com_google_guava_guava",
        "@maven//:com_google_protobuf_protobuf_java",
    ],
)

java_library(
    name = "scaler",
    srcs = ["Scaler.java"],
    deps = [
        ":configuration_provider",
        ":instance_count_provider",
        ":metrics_service",
        ":scaling_stabilizer",
        ":target_average_value_scaling",
        ":target_value_scaling",
        ":workload_info_parser",
        "//proto:scaled_object_java_proto",
        "//proto:scaler_java_proto",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_run_client_wrapper",
        "@maven//:com_google_flogger_flogger",
        "@maven//:com_google_guava_guava",
    ],
)

java_library(
    name = "scalers_manager",
    srcs = ["ScalersManager.java"],
    deps = [
        ":configuration_provider",
        ":instance_count_provider",
        ":metrics_service",
        ":scaler",
        ":scaling_stabilizer",
        ":target_average_value_scaling",
        ":target_value_scaling",
        ":workload_info_parser",
        "//proto:scaled_object_java_proto",
        "//proto:scaler_java_proto",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_monitoring_client_wrapper",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_run_client_wrapper",
        "@maven//:com_google_flogger_flogger",
        "@maven//:com_google_guava_guava",
    ],
)

java_library(
    name = "target_value_scaling",
    srcs = ["TargetValueScaling.java"],
    deps = [
    ],
)

java_library(
    name = "target_average_value_scaling",
    srcs = ["TargetAverageValueScaling.java"],
    deps = [
    ],
)

java_library(
    name = "kafka",
    srcs = ["Kafka.java"],
    deps = [
        "//src/main/java/com/google/cloud/run/crema/clients:kafka_admin_client_wrapper",
        "@maven//:org_apache_kafka_kafka_clients",
    ],
)

java_library(
    name = "configuration_provider",
    srcs = ["ConfigurationProvider.java"],
    deps = [
        ":workload_info_parser",
        "//proto:scaled_object_java_proto",
        "@maven//:com_google_code_gson_gson",
        "@maven//:com_google_flogger_flogger",
        "@maven//:com_google_guava_guava",
        "@maven//:com_google_protobuf_protobuf_java_util",
        "@maven//:org_apache_kafka_kafka_clients",
        "@maven//:org_yaml_snakeyaml",
    ],
)

java_library(
    name = "request_response_handler",
    srcs = ["RequestResponseHandler.java"],
    deps = [
        "@maven//:com_google_flogger_flogger",
        "@maven//:com_google_guava_guava",
    ],
)

java_library(
    name = "instance_count_provider",
    srcs = ["InstanceCountProvider.java"],
    deps = [
        ":workload_info_parser",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_run_client_wrapper",
    ],
)

java_library(
    name = "metrics_service",
    srcs = ["MetricsService.java"],
    deps = [
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_monitoring_client_wrapper",
        "@maven//:com_google_api_grpc_proto_google_cloud_monitoring_v3",
        "@maven//:com_google_api_grpc_proto_google_common_protos",
        "@maven//:com_google_cloud_google_cloud_monitoring",
        "@maven//:com_google_flogger_flogger",
        "@maven//:com_google_guava_guava",
        "@maven//:com_google_protobuf_protobuf_java_util",
    ],
)

java_binary(
    name = "scaler_server",
    srcs = ["ScalerServer.java"],
    main_class = "com.google.cloud.run.crema.ScalerServer",
    runtime_deps = ["@maven//:com_google_flogger_flogger_system_backend"],
    deps = [
        ":configuration_provider",
        ":metrics_service",
        ":scalers_manager",
        ":scaler",
        ":workload_info_parser",
        "//proto:scaled_object_java_proto",
        "//proto:scaler_java_grpc",
        "//proto:scaler_java_proto",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_monitoring_client_wrapper",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_run_client_wrapper",
        "@maven//:io_grpc_grpc_core",
        "@grpc-java//services:health",
        "@grpc-java//services:healthlb",
        "@grpc-java//stub",
        "@grpc-proto//:health_java_proto",
        "@grpc-proto//:health_proto",
        "@maven//:io_grpc_grpc_stub",
        "@maven//:com_google_flogger_flogger",
        "@maven//:com_google_protobuf_protobuf_java",
    ],
)
