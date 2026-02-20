.PHONY: dev-up dev-down dev-build dev-clean dev-logs dev-restart \
       agent-build agent-build-all agent-version help

# Docker Compose file location
COMPOSE_DIR := infrastructure/dev

# Agent build settings
AGENT_VERSION ?= 0.1.0
BUILD_TIME    := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
AGENT_SRC     := server/cmd/agent
AGENT_OUT     := server/agent-binaries
LDFLAGS       := -X main.Version=$(AGENT_VERSION) -X main.BuildTime=$(BUILD_TIME)

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
	@echo "  Agent:"
	@echo "    make agent-build         - Build agent for current platform"
	@echo "    make agent-build-all     - Cross-compile agent for linux/darwin/windows"
	@echo "    make agent-version       - Show current agent version"

# === Docker / Dev targets ===

# Main command: cleanup, rebuild, and start
dev-up-clean:
	@echo "=== Cleaning up existing containers and volumes ==="
	@cd $(COMPOSE_DIR) && AGENT_VERSION=$(AGENT_VERSION) BUILD_TIME=$(BUILD_TIME) docker compose down -v --remove-orphans || true
	@echo ""
	@echo "=== Building all images (no cache) ==="
	@cd $(COMPOSE_DIR) && AGENT_VERSION=$(AGENT_VERSION) BUILD_TIME=$(BUILD_TIME) docker compose build --no-cache
	@echo ""
	@echo "=== Starting all services ==="
	@cd $(COMPOSE_DIR) && AGENT_VERSION=$(AGENT_VERSION) BUILD_TIME=$(BUILD_TIME) docker compose up -d
	@echo ""
	@echo "✓ Services are starting! Use 'make dev-logs' to view logs."

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
	@cd $(COMPOSE_DIR) && AGENT_VERSION=$(AGENT_VERSION) BUILD_TIME=$(BUILD_TIME) docker compose build
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

# === Agent targets ===

# Build agent for current OS/arch (useful for local testing)
agent-build:
	@echo "Building agent $(AGENT_VERSION) for current platform..."
	@mkdir -p $(AGENT_OUT)
	cd server && CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o ../$(AGENT_OUT)/lute-agent ./cmd/agent
	@echo "$(AGENT_VERSION)" > $(AGENT_OUT)/VERSION
	@echo "✓ Built $(AGENT_OUT)/lute-agent"

# Cross-compile agent for all supported platforms
agent-build-all:
	@echo "Cross-compiling agent $(AGENT_VERSION) for all platforms..."
	@mkdir -p $(AGENT_OUT)

	@echo "  → linux/amd64"
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o ../$(AGENT_OUT)/lute-agent-linux-amd64 ./cmd/agent

	@echo "  → linux/arm64"
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o ../$(AGENT_OUT)/lute-agent-linux-arm64 ./cmd/agent

	@echo "  → darwin/amd64"
	cd server && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o ../$(AGENT_OUT)/lute-agent-darwin-amd64 ./cmd/agent

	@echo "  → darwin/arm64"
	cd server && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o ../$(AGENT_OUT)/lute-agent-darwin-arm64 ./cmd/agent

	@echo "  → windows/amd64"
	cd server && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o ../$(AGENT_OUT)/lute-agent-windows-amd64.exe ./cmd/agent

	@echo "$(AGENT_VERSION)" > $(AGENT_OUT)/VERSION
	@echo "✓ All agent binaries built in $(AGENT_OUT)/"

# Show agent version
agent-version:
	@echo "Agent version: $(AGENT_VERSION)"
	@echo "Build time:    $(BUILD_TIME)"
