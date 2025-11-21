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

# This Dockerfile builds both scaler (Java) and metric-provider (Go)
# into a container image.
ARG SRC_DIR=/
ARG UBUNTU=ubuntu
ARG GOLANG=golang

# --- Java Builder ---
FROM $UBUNTU:22.04 AS java_builder
ARG SRC_DIR

# Install necessary dependencies for Bazel and the build
RUN apt-get update && apt-get install -y \
    build-essential \
    openjdk-21-jdk \
    unzip \
    zip \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install bazelisk which will use the .bazelversion file
RUN wget https://github.com/bazelbuild/bazelisk/releases/download/v1.19.0/bazelisk-linux-amd64 -O /usr/local/bin/bazel && \
    chmod +x /usr/local/bin/bazel

# Create and use  non-root user to run the build otherwise bazel fails with:
# `The current user is root, please run as non-root when using the hermetic Python interpreter.`
RUN useradd -ms /bin/bash nonroot

WORKDIR /workspace

COPY ${SRC_DIR}/scaler/ .
COPY ${SRC_DIR}/proto ./proto

RUN chown -R nonroot:nonroot /workspace
USER nonroot

# Build the Java binary
RUN bazel build src/main/java/com/google/cloud/run/crema:scaler_server_deploy.jar

# --- Go Builder ---
FROM $GOLANG:1.25.3 AS go_builder
ARG SRC_DIR

WORKDIR /app

# Install dependencies for proto generation
RUN apt-get update && apt-get install -y protobuf-compiler
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
ENV PATH="$PATH:/go/bin"

# Copy all source files needed for the Go build
COPY ${SRC_DIR}/proto ./proto
COPY ${SRC_DIR}/metric-provider ./metric-provider

# Generate Go code from proto files
# The output will be in metric-provider/proto based on the go_package option in the .proto files.
RUN protoc --proto_path=. \
    --go_out=. \
    --go-grpc_out=. \
    proto/*.proto

# Build the Go app from the metric-provider directory
WORKDIR /app/metric-provider
RUN go mod download
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o main -ldflags "-w -s" .

# --- Final Image ---
# TODO: Change this scratch for automatic base image update.
ARG UBUNTU
FROM $UBUNTU:22.04
ARG SRC_DIR

RUN apt-get update && apt-get install -y openjdk-21-jdk && rm -rf /var/lib/apt/lists/*

ENV APP_DIR=/app
WORKDIR $APP_DIR

# Copy Java app
COPY --from=java_builder /workspace/src/main/java/com/google/cloud/run/crema/logging.properties $APP_DIR/logging.properties
COPY --from=java_builder /workspace/bazel-bin/src/main/java/com/google/cloud/run/crema/scaler_server_deploy.jar $APP_DIR/scaler_server.jar

# Copy Go app and give it a more descriptive name
COPY --from=go_builder /app/metric-provider/main $APP_DIR/metric-provider

# Copy entrypoint script
COPY ${SRC_DIR}/entrypoint.sh $APP_DIR/entrypoint.sh
RUN chmod +x $APP_DIR/entrypoint.sh

ENV JAVA_TOOL_OPTIONS="-Djava.util.logging.config.file=$APP_DIR/logging.properties"

EXPOSE 8080

CMD ["/app/entrypoint.sh"]
