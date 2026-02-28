#!/bin/bash

# Generate Go code from protobuf definitions

set -e

echo "Generating protobuf code..."

cd "$(dirname "$0")/.."

protoc \
  --go_out=. \
  --go_opt=module=github.com/lute/worker \
  --go-grpc_out=. \
  --go-grpc_opt=module=github.com/lute/worker \
  -I. \
  proto/worker.proto

echo "Protobuf code generated successfully!"
