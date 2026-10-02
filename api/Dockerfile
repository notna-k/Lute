# syntax=docker/dockerfile:1

# The panel, embedded into core. Static output, so it builds on the host platform.
FROM --platform=$BUILDPLATFORM node:25-alpine AS ui
WORKDIR /ui
COPY ui/package.json ui/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY ui .
# Empty VITE_API_URL: the panel calls /api on the origin that served it.
RUN VITE_API_URL= npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
WORKDIR /src

COPY shared/proto/go.mod shared/proto/go.sum shared/proto/
COPY api/go.mod api/go.sum api/
RUN --mount=type=cache,target=/go/pkg/mod cd api && go mod download

COPY shared/proto shared/proto
COPY api api
COPY --from=ui /ui/dist api/internal/ui/web
ARG CORE_VERSION=dev
ARG WORKER_VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd api && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
      -ldflags "-s -w -X github.com/lute/api/internal/version.Core=${CORE_VERSION} -X github.com/lute/api/internal/version.Worker=${WORKER_VERSION}" \
      -o /out/lute-core ./cmd/api

FROM alpine:3
COPY --from=builder /out/lute-core /usr/local/bin/lute-core
EXPOSE 8080 50051
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://localhost:8080/api/health || exit 1
CMD ["lute-core"]
