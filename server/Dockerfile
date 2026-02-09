# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies including protoc
RUN apk add --no-cache git make protoc protobuf-dev

# Install Go protobuf plugins
# Ensure Go bin is in PATH (default in golang image, but explicit for clarity)
ENV PATH=$PATH:/go/bin
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf code
# Use module option so protoc generates files based on go_package path
RUN protoc \
    --go_out=. \
    --go_opt=module=github.com/lute/server \
    --go-grpc_out=. \
    --go-grpc_opt=module=github.com/lute/server \
    proto/agent.proto

# Build the server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# ---- Build agent binaries for multiple platforms ----
ARG AGENT_VERSION=dev
ARG BUILD_TIME=unknown

# Linux amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${AGENT_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o agent-binaries/lute-agent-linux-amd64 ./cmd/agent

# Linux arm64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${AGENT_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o agent-binaries/lute-agent-linux-arm64 ./cmd/agent

# Darwin (macOS) amd64
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${AGENT_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o agent-binaries/lute-agent-darwin-amd64 ./cmd/agent

# Darwin (macOS) arm64
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${AGENT_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o agent-binaries/lute-agent-darwin-arm64 ./cmd/agent

# Windows amd64
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${AGENT_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o agent-binaries/lute-agent-windows-amd64.exe ./cmd/agent

# Write the version file
RUN echo "${AGENT_VERSION}" > agent-binaries/VERSION

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /root/

# Copy server binary
COPY --from=builder /app/server .

# Copy agent binaries into the directory the server will serve
COPY --from=builder /app/agent-binaries /opt/lute/agent-binaries

# Expose ports
EXPOSE 8080 50051

# Set default env for agent binary directory
ENV AGENT_BINARY_DIR=/opt/lute/agent-binaries

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/api/health || exit 1

# Run the server
CMD ["./server"]
