# syntax=docker/dockerfile:1
#
# lute-worker: the build agent. It runs jobs as sibling containers on the engine whose
# socket is mounted, and keeps its identity and job logs in a mounted data dir.
# Build from the repo root: docker build -f worker/Dockerfile .

# Cross-compiles on the build machine instead of emulating the target.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
WORKDIR /src
ENV GOMODCACHE=/go/pkg/mod GOCACHE=/root/.cache/go-build

COPY shared/proto/go.mod shared/proto/go.sum shared/proto/
COPY worker/go.mod worker/go.sum worker/
RUN --mount=type=cache,target=/go/pkg/mod cd worker && go mod download

COPY shared/proto shared/proto
COPY worker worker
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd worker && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
      -ldflags "-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
      -o /out/lute-worker ./cmd/worker

# Root in the container is the unprivileged host user under rootless Docker.
FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/lute-worker /lute-worker
ENV LUTE_DATA_DIR=/var/lib/lute-worker \
    LUTE_REQUIRE_MOUNT=1 \
    DOCKER_HOST=unix:///var/run/docker.sock
USER 0
ENTRYPOINT ["/lute-worker"]
CMD ["run"]
