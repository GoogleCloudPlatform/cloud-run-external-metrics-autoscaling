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
    name = "ScalingStabilizerTest",
    srcs = ["ScalingStabilizerTest.java"],
    test_class = "com.google.cloud.run.crema.ScalingStabilizerTest",
    deps = [
        "//proto:scaled_object_java_proto",
        "//src/main/java/com/google/cloud/run/crema:scaling_stabilizer",
        "@maven//:com_google_flogger_flogger_system_backend",
        "@maven//:com_google_truth_truth",
        "@maven//:junit_junit",
    ],
)

java_test(
    name = "WorkloadInfoParserTest",
    srcs = ["WorkloadInfoParserTest.java"],
    test_class = "com.google.cloud.run.crema.WorkloadInfoParserTest",
    deps = [
        "//src/main/java/com/google/cloud/run/crema:workload_info_parser",
        "@maven//:com_google_flogger_flogger_system_backend",
        "@maven//:com_google_truth_truth",
        "@maven//:junit_junit",
    ],
)

java_test(
    name = "TargetValueScalingTest",
    srcs = ["TargetValueScalingTest.java"],
    test_class = "com.google.cloud.run.crema.TargetValueScalingTest",
    deps = [
        "//src/main/java/com/google/cloud/run/crema:target_value_scaling",
        "@maven//:com_google_flogger_flogger_system_backend",
        "@maven//:com_google_truth_truth",
        "@maven//:junit_junit",
    ],
)

java_test(
    name = "TargetAverageValueScalingTest",
    srcs = ["TargetAverageValueScalingTest.java"],
    test_class = "com.google.cloud.run.crema.TargetAverageValueScalingTest",
    deps = [
        "//src/main/java/com/google/cloud/run/crema:target_average_value_scaling",
        "@maven//:com_google_flogger_flogger_system_backend",
        "@maven//:com_google_truth_truth",
        "@maven//:junit_junit",
    ],
)

java_test(
    name = "ScalerTest",
    srcs = ["ScalerTest.java"],
    test_class = "com.google.cloud.run.crema.ScalerTest",
    deps = [
        "//proto:scaled_object_java_proto",
        "//proto:scaler_java_proto",
        "//src/main/java/com/google/cloud/run/crema:configuration_provider",
        "//src/main/java/com/google/cloud/run/crema:metrics_service",
        "//src/main/java/com/google/cloud/run/crema:scaler",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_run_client_wrapper",
        "@maven//:com_google_api_gax",
        "@maven//:com_google_flogger_flogger_system_backend",
        "@maven//:com_google_truth_truth",
        "@maven//:junit_junit",
        "@maven//:org_mockito_mockito_core",
    ],
)

java_test(
    name = "ConfigurationProviderTest",
    srcs = ["ConfigurationProviderTest.java"],
    test_class = "com.google.cloud.run.crema.ConfigurationProviderTest",
    deps = [
        "//src/main/java/com/google/cloud/run/crema:configuration_provider",
        "@maven//:com_google_flogger_flogger_system_backend",
        "@maven//:com_google_guava_guava",
        "@maven//:com_google_truth_truth",
        "@maven//:junit_junit",
    ],
)

java_test(
    name = "InstanceCountProviderTest",
    srcs = ["InstanceCountProviderTest.java"],
    test_class = "com.google.cloud.run.crema.InstanceCountProviderTest",
    deps = [
        "//src/main/java/com/google/cloud/run/crema:instance_count_provider",
        "//src/main/java/com/google/cloud/run/crema:workload_info_parser",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_run_client_wrapper",
        "@maven//:com_google_flogger_flogger_system_backend",
        "@maven//:com_google_truth_truth",
        "@maven//:junit_junit",
        "@maven//:org_mockito_mockito_core",
    ],
)

java_test(
    name = "ScalersManagerTest",
    srcs = ["ScalersManagerTest.java"],
    test_class = "com.google.cloud.run.crema.ScalersManagerTest",
    deps = [
        "//proto:scaled_object_java_proto",
        "//proto:scaler_java_proto",
        "//src/main/java/com/google/cloud/run/crema:configuration_provider",
        "//src/main/java/com/google/cloud/run/crema:metrics_service",
        "//src/main/java/com/google/cloud/run/crema:scalers_manager",
        "//src/main/java/com/google/cloud/run/crema:scaler",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_monitoring_client_wrapper",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_run_client_wrapper",
        "@maven//:com_google_flogger_flogger_system_backend",
        "@maven//:com_google_truth_truth",
        "@maven//:junit_junit",
        "@maven//:org_mockito_mockito_core",
    ],
)

java_test(
    name = "MetricsServiceTest",
    srcs = ["MetricsServiceTest.java"],
    test_class = "com.google.cloud.run.crema.MetricsServiceTest",
    deps = [
        "//src/main/java/com/google/cloud/run/crema:metrics_service",
        "//src/main/java/com/google/cloud/run/crema/clients:cloud_monitoring_client_wrapper",
        "@maven//:com_google_api_grpc_proto_google_cloud_monitoring_v3",
        "@maven//:com_google_api_grpc_proto_google_common_protos",
        "@maven//:com_google_flogger_flogger_system_backend",
        "@maven//:com_google_guava_guava",
        "@maven//:com_google_truth_truth",
        "@maven//:junit_junit",
        "@maven//:org_mockito_mockito_core",
    ],
)

java_test(
    name = "CremaLogFormatterTest",
    srcs = ["CremaLogFormatterTest.java"],
    test_class = "com.google.cloud.run.crema.CremaLogFormatterTest",
    deps = [
        "//src/main/java/com/google/cloud/run/crema:scaler_server",
        "@maven//:com_google_code_gson_gson",
        "@maven//:com_google_truth_truth",
        "@maven//:junit_junit",
    ],
)
