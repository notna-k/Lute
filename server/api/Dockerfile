# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy module files for api and agent (api has a local replace)
COPY server/api/go.mod server/api/go.sum server/api/
COPY server/agent/go.mod server/agent/go.sum server/agent/

RUN cd server/api && go mod download
RUN cd server/agent && go mod download

# Copy source code
COPY server/api server/api
COPY server/agent server/agent

# Build the API server
WORKDIR /app/server/api
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server .

# ---- Build agent binaries for multiple platforms ----
ARG AGENT_VERSION=dev
ARG BUILD_TIME=unknown

WORKDIR /app/server/agent

# Linux amd64
RUN mkdir -p /app/agent-binaries && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${AGENT_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/agent-binaries/lute-agent-linux-amd64 .

# Linux arm64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${AGENT_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/agent-binaries/lute-agent-linux-arm64 .

# Darwin (macOS) amd64
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${AGENT_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/agent-binaries/lute-agent-darwin-amd64 .

# Darwin (macOS) arm64
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${AGENT_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/agent-binaries/lute-agent-darwin-arm64 .

# Windows amd64
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${AGENT_VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/agent-binaries/lute-agent-windows-amd64.exe .

# Write the version file
RUN echo "${AGENT_VERSION}" > /app/agent-binaries/VERSION

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /root/

# Copy server binary
COPY --from=builder /app/server/api/server .

# Copy agent binaries into the directory the server will serve
COPY --from=builder /app/agent-binaries /opt/lute/agent-binaries

# Expose ports
EXPOSE 8080 50051

# Set default env for agent binary directory
ENV AGENT_BINARY_DIR=/opt/lute/agent-binaries

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 -O /dev/null http://localhost:8080/api/health || exit 1

# Run the server
CMD ["./api"]
