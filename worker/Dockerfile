# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
WORKDIR /src

COPY shared/proto/go.mod shared/proto/go.sum shared/proto/
COPY worker/go.mod worker/go.sum worker/
RUN --mount=type=cache,target=/go/pkg/mod cd worker && go mod download

COPY shared/proto shared/proto
COPY worker worker
ARG WORKER_VERSION=dev
ARG BUILD_TIME=unknown
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd worker && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
      -ldflags "-s -w -X main.Version=${WORKER_VERSION} -X main.BuildTime=${BUILD_TIME}" \
      -o /out/lute-worker ./cmd/worker

FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/lute-worker /lute-worker
ENV LUTE_DATA_DIR=/var/lib/lute-worker \
    LUTE_REQUIRE_MOUNT=1 \
    DOCKER_HOST=unix:///var/run/docker.sock
# Root in the container is the unprivileged host user under rootless Docker.
USER 0
ENTRYPOINT ["/lute-worker"]
CMD ["run"]
