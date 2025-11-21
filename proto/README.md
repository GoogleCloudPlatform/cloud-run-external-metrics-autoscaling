# Protocol Buffers

This directory contains the protocol buffer definitions for communication between the two CREMA containers.

## Go Generation

To generate the Go code from the `.proto` files, you need to have the following tools installed:
- `protoc`: The protocol buffer compiler.
- `protoc-gen-go`: The Go plugin for `protoc`.
- `protoc-gen-go-grpc`: The Go gRPC plugin for `protoc`.

### Command

Run the following command from the root of the repository:

```sh
protoc --proto_path=. \
    --go_out=. \
    --go-grpc_out=. \
    proto/*.proto
```

### Output

The generated Go files will be placed in the `metric-provider/proto/` directory.