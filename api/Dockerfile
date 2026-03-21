# syntax=docker/dockerfile:1
# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy module files (api/worker replace github.com/lute/proto => ../shared/proto)
COPY shared/proto/go.mod shared/proto/go.sum shared/proto/
COPY api/go.mod api/go.sum api/
COPY worker/go.mod worker/go.sum worker/

# Download deps with cache mount so repeated builds skip download
RUN --mount=type=cache,target=/go/pkg/mod \
    cd api && go mod download
RUN --mount=type=cache,target=/go/pkg/mod \
    cd worker && go mod download

# Copy source code
COPY shared/proto shared/proto
COPY api api
COPY worker worker

# Build the API server (use Go build cache for faster recompiles)
# Entrypoint is cmd/api after refactor to internal layout
WORKDIR /app/api
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o api ./cmd/api

# ---- Build worker binaries for multiple platforms ----
ARG WORKER_VERSION=dev
ARG BUILD_TIME=unknown

WORKDIR /app/worker

# Linux amd64
RUN --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /app/worker-binaries && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w -X main.Version=${WORKER_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/worker-binaries/lute-worker-linux-amd64 .

# Linux arm64
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -ldflags "-s -w -X main.Version=${WORKER_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/worker-binaries/lute-worker-linux-arm64 .

# Darwin (macOS) amd64
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
    -ldflags "-s -w -X main.Version=${WORKER_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/worker-binaries/lute-worker-darwin-amd64 .

# Darwin (macOS) arm64
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
    -ldflags "-s -w -X main.Version=${WORKER_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/worker-binaries/lute-worker-darwin-arm64 .

# Windows amd64
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
    -ldflags "-s -w -X main.Version=${WORKER_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/worker-binaries/lute-worker-windows-amd64.exe .

# Write the version file
RUN echo "${WORKER_VERSION}" > /app/worker-binaries/VERSION

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /root/

# Copy server binary
COPY --from=builder /app/api/api .

# Copy worker binaries into the directory the server will serve
COPY --from=builder /app/worker-binaries /opt/lute/worker-binaries

# Expose ports
EXPOSE 8080 50051

# Set default env for worker binary directory
ENV WORKER_BINARY_DIR=/opt/lute/worker-binaries

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 -O /dev/null http://localhost:8080/api/health || exit 1

# Run the server
CMD ["./api"]
