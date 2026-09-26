# syntax=docker/dockerfile:1
#
# Core image — stateless Go API (HTTP + WebSocket + gRPC). The admin panel is a
# SEPARATE image (ui/Dockerfile), and the agent ships as its own image
# (worker/Dockerfile). A minimal placeholder `api/internal/ui/web/index.html`
# satisfies the //go:embed so the binary still compiles.

FROM golang:1.26-alpine AS builder
WORKDIR /app
ENV GOMODCACHE=/go/pkg/mod GOCACHE=/root/.cache/go-build

# api replaces github.com/lute/proto => ../shared/proto
COPY shared/proto/go.mod shared/proto/go.sum shared/proto/
COPY api/go.mod api/go.sum api/
RUN --mount=type=cache,target=/go/pkg/mod cd api && go mod download

COPY shared/proto shared/proto
COPY api api
ARG CORE_VERSION=dev
ARG WORKER_VERSION=dev
WORKDIR /app/api
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -ldflags "-s -w -X github.com/lute/api/internal/version.Core=${CORE_VERSION} -X github.com/lute/api/internal/version.Worker=${WORKER_VERSION}" \
      -o api ./cmd/api

FROM alpine:3
RUN apk --no-cache add ca-certificates wget
WORKDIR /root/
COPY --from=builder /app/api/api .
EXPOSE 8080 50051
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 -O /dev/null http://localhost:8080/api/health || exit 1
CMD ["./api"]
