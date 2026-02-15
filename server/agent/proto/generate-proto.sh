#!/bin/bash

# Generate Go code from protobuf definitions

set -e

PROTO_DIR="."
OUTPUT_DIR="."

echo "Generating protobuf code..."

cd proto

protoc \
  --go_out=${OUTPUT_DIR} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${OUTPUT_DIR} \
  --go-grpc_opt=paths=source_relative \
  agent.proto

# Move generated files to agent subdirectory
mkdir -p agent
mv -f agent.pb.go agent_grpc.pb.go agent/

echo "Protobuf code generated successfully!"

