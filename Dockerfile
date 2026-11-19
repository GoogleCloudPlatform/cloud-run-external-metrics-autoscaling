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

# This Dockerfile builds CREMA into a container image.
ARG SRC_DIR=/

# --- Go Builder ---
FROM gcr.io/cloud-marketplace/google/ubuntu2204 AS go_builder
ARG SRC_DIR

WORKDIR /app

# Install dependencies for Go installation and proto generation
RUN apt-get update && apt-get install -y \
    wget \
    git \
    protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*

# Install Go
RUN wget https://go.dev/dl/go1.26.9.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.26.9.linux-amd64.tar.gz && \
    rm go1.26.9.linux-amd64.tar.gz

# Set Go paths
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"

# Install proto plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1

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
RUN CGO_ENABLED=0 GOOS=linux go build -o main -ldflags "-w -s" .
RUN chmod +x main

# --- Final Image ---
FROM scratch
ARG SRC_DIR

ENV APP_DIR=/app
WORKDIR $APP_DIR

# Copy Go app
COPY --from=go_builder /app/metric-provider/main $APP_DIR/metric-provider

EXPOSE 8080

CMD ["/app/metric-provider"]
