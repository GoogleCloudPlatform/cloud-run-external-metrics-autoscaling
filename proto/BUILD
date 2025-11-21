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

load("@protobuf//bazel:proto_library.bzl", "proto_library")
load("@protobuf//bazel:java_proto_library.bzl", "java_proto_library")
load("@grpc-java//:java_grpc_library.bzl", "java_grpc_library")

package(default_visibility = ["//visibility:public"])

proto_library(
    name = "scaled_object_proto",
    srcs = ["scaled_object.proto"],
    deps = [
        "@protobuf//:duration_proto",
    ],
)

proto_library(
    name = "scaler_proto",
    srcs = ["scaler.proto"],
    deps = [":scaled_object_proto"],
)

java_proto_library(
    name = "scaled_object_java_proto",
    deps = [":scaled_object_proto"],
)

java_proto_library(
    name = "scaler_java_proto",
    deps = [":scaler_proto"],
)

java_grpc_library(
    name = "scaler_java_grpc",
    srcs = [":scaler_proto"],
    deps = [":scaler_java_proto"],
)