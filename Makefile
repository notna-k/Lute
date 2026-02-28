.PHONY: dev-up dev-down dev-build dev-clean dev-logs dev-restart \
       worker-build worker-build-all worker-version help

# Docker Compose file location
COMPOSE_DIR := infrastructure/dev

# Worker build settings
WORKER_VERSION ?= 0.1.0
BUILD_TIME     := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
WORKER_SRC     := server/worker
WORKER_OUT     := server/worker-binaries
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
	cd server && CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker ./worker
	@echo "$(WORKER_VERSION)" > $(WORKER_OUT)/VERSION
	@echo "Built $(WORKER_OUT)/lute-worker"

# Cross-compile worker for all supported platforms
worker-build-all:
	@echo "Cross-compiling worker $(WORKER_VERSION) for all platforms..."
	@mkdir -p $(WORKER_OUT)

	@echo "  -> linux/amd64"
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker-linux-amd64 ./worker

	@echo "  -> linux/arm64"
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker-linux-arm64 ./worker

	@echo "  -> darwin/amd64"
	cd server && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker-darwin-amd64 ./worker

	@echo "  -> darwin/arm64"
	cd server && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker-darwin-arm64 ./worker

	@echo "  -> windows/amd64"
	cd server && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o ../$(WORKER_OUT)/lute-worker-windows-amd64.exe ./worker

	@echo "$(WORKER_VERSION)" > $(WORKER_OUT)/VERSION
	@echo "All worker binaries built in $(WORKER_OUT)/"

# Show worker version
worker-version:
	@echo "Worker version: $(WORKER_VERSION)"
	@echo "Build time:     $(BUILD_TIME)"
