.PHONY: dev-up dev-down dev-build dev-clean dev-logs dev-restart \
       worker-build worker-build-all worker-version go-lint help \
       ui-build api-build

# Pin for reproducible CI/local runs (install: https://golangci-lint.run/welcome/install/)
GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4

# Docker Compose file location
COMPOSE_DIR := infrastructure/dev

# Worker build settings
WORKER_VERSION ?= 0.1.0
BUILD_TIME     := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
WORKER_SRC     := worker
WORKER_OUT     := worker/bin
LDFLAGS        := -X main.Version=$(WORKER_VERSION) -X main.BuildTime=$(BUILD_TIME)

# Default target
help:
	@echo "Available commands:"
	@echo ""
	@echo "  Docker / Dev:"
	@echo "    make dev-up        - Clean, rebuild, and start all services"
	@echo "    make dev-down      - Stop and remove all containers"
	@echo "    make dev-build     - Build all images without cache"
	@echo "    make dev-clean     - Stop containers, remove containers, volumes, and networks"
	@echo "    make dev-logs      - Show logs from all services"
	@echo "    make dev-restart   - Restart all services"
	@echo ""
	@echo "  Worker:"
	@echo "    make worker-build         - Build worker for current platform"
	@echo "    make worker-build-all     - Cross-compile worker for linux/darwin/windows"
	@echo "    make worker-version       - Show current worker version"
	@echo ""
	@echo "  Go:"
	@echo "    make go-lint              - Run golangci-lint on api/ and worker/ (not shared/proto)"
	@echo ""
	@echo "  Release (embedded UI in api binary):"
	@echo "    make ui-build             - npm ci + vite build → api/internal/ui/dist"
	@echo "    make api-build            - ui-build then CGO_ENABLED=0 go build api → bin/api"

# === Embedded UI + API binary (requires Node for ui-build) ===

ui-build:
	@echo "==> Building UI (VITE_API_URL empty for same-origin API)"
	cd ui && npm ci && VITE_API_URL= npm run build
	rm -rf api/internal/ui/web
	cp -r ui/dist api/internal/ui/web

api-build: ui-build
	@echo "==> Building api binary with embedded UI"
	mkdir -p bin
	cd api && CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/api ./cmd/api
	@echo "Built bin/api"

# === Go lint ===

go-lint:
	@echo "==> lint api"
	cd api && $(GOLANGCI_LINT) run ./...
	@echo "==> lint worker"
	cd worker && $(GOLANGCI_LINT) run ./...

# === Docker / Dev targets ===

# BuildKit required for cache mounts (npm, go mod, go build cache) so deps aren't re-downloaded each time
export DOCKER_BUILDKIT := 1

# Main command: cleanup containers (keep volumes), rebuild, and start
dev-up-clean:
	@echo "=== Stopping and removing containers ==="
	@cd $(COMPOSE_DIR) && WORKER_VERSION=$(WORKER_VERSION) BUILD_TIME=$(BUILD_TIME) docker compose down --remove-orphans || true
	@echo ""
	@echo "=== Building all images (cache used; code changes trigger rebuild) ==="
	@cd $(COMPOSE_DIR) && WORKER_VERSION=$(WORKER_VERSION) BUILD_TIME=$(BUILD_TIME) docker compose build --parallel
	@echo ""
	@echo "=== Starting all services ==="
	@cd $(COMPOSE_DIR) && WORKER_VERSION=$(WORKER_VERSION) BUILD_TIME=$(BUILD_TIME) docker compose up -d
	@echo ""
	@echo "Services are starting. Use 'make dev-logs' to view logs."

dev-up: 
	@cd $(COMPOSE_DIR) && docker compose up -d

# Stop and remove containers, volumes, and networks
dev-clean:
	@echo "Cleaning up containers, volumes, and networks..."
	@cd $(COMPOSE_DIR) && docker compose down -v --remove-orphans || true
	@echo "Cleanup complete."

# Build all images without cache
dev-build:
	@echo "Building all images (this may take a while)..."
	@cd $(COMPOSE_DIR) && WORKER_VERSION=$(WORKER_VERSION) BUILD_TIME=$(BUILD_TIME) docker compose build
	@echo "Build complete."

# Stop all services
dev-down:
	@echo "Stopping all services..."
	@cd $(COMPOSE_DIR) && docker compose down
	@echo "Services stopped."

# Show logs
dev-logs:
	@cd $(COMPOSE_DIR) && docker compose logs -f

# Restart all services
dev-restart: dev-down dev-up

# === Worker targets ===

# Build worker for current OS/arch (useful for local testing)
worker-build:
	@echo "Building worker $(WORKER_VERSION) for current platform..."
	@mkdir -p $(WORKER_OUT)
	cd $(WORKER_SRC) && CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker ./cmd/worker
	@echo "$(WORKER_VERSION)" > $(WORKER_OUT)/VERSION
	@echo "Built $(WORKER_OUT)/lute-worker"

# Cross-compile worker for all supported platforms
worker-build-all:
	@echo "Cross-compiling worker $(WORKER_VERSION) for all platforms..."
	@mkdir -p $(WORKER_OUT)

	@echo "  -> linux/amd64"
	cd $(WORKER_SRC) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker-linux-amd64 ./cmd/worker

	@echo "  -> linux/arm64"
	cd $(WORKER_SRC) && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker-linux-arm64 ./cmd/worker

	@echo "  -> darwin/amd64"
	cd $(WORKER_SRC) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker-darwin-amd64 ./cmd/worker

	@echo "  -> darwin/arm64"
	cd $(WORKER_SRC) && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker-darwin-arm64 ./cmd/worker

	@echo "  -> windows/amd64"
	cd $(WORKER_SRC) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker-windows-amd64.exe ./cmd/worker

	@echo "$(WORKER_VERSION)" > $(WORKER_OUT)/VERSION
	@echo "All worker binaries built in $(WORKER_OUT)/"

# Show worker version
worker-version:
	@echo "Worker version: $(WORKER_VERSION)"
	@echo "Build time:     $(BUILD_TIME)"
