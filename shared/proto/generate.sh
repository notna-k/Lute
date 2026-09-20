#!/usr/bin/env bash
# Generate Go for module github.com/lute/proto (run from this directory).
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

echo "Generating protobuf Go code in $DIR ..."

protoc \
  --go_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_out=. \
  --go-grpc_opt=paths=source_relative \
  -I. \
  worker.proto

echo "Done."
