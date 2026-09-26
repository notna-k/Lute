.PHONY: dev-up dev-down dev-clean dev-logs worker-build worker-build-linux worker-image go-format-check go-test go-lint e2e e2e-image e2e-vet ui-build api-build

export DOCKER_BUILDKIT := 1
export VERSION    ?= 0.2.0
export BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

ENV_FILE ?= .env
COMPOSE  := docker compose -f infrastructure/dev/docker-compose.yml --env-file $(ENV_FILE)
LINT     := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
GOBUILD  := CGO_ENABLED=0 go build -ldflags '-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)'
WORKER_IMAGE ?= lute-worker:dev

dev-up:
	$(COMPOSE) up -d --build

dev-down:
	$(COMPOSE) down --remove-orphans

dev-clean:
	$(COMPOSE) down -v --remove-orphans

dev-logs:
	$(COMPOSE) logs -f

worker-build:
	cd worker && $(GOBUILD) -o bin/lute-worker ./cmd/worker

# The name the e2e harness runs.
worker-build-linux:
	cd worker && GOOS=linux GOARCH=amd64 $(GOBUILD) -o bin/lute-worker-linux-amd64 ./cmd/worker

# linux/amd64 only; CI builds arm64 too when it publishes.
worker-image:
	docker build --platform linux/amd64 -f worker/Dockerfile \
		--build-arg VERSION=$(VERSION) --build-arg BUILD_TIME=$(BUILD_TIME) -t $(WORKER_IMAGE) .

go-format-check:
	@unformatted="$$(gofmt -l api worker shared/proto)"; \
	if [ -n "$$unformatted" ]; then \
		echo "The following Go files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

go-test:
	@for module in api worker shared/proto; do \
		echo "Testing $$module"; \
		(cd $$module && go test ./...) || exit 1; \
	done

go-lint:
	cd api && $(LINT) run ./...
	cd worker && $(LINT) run ./...

# End-to-end suite: core in-process, the real worker binary as a child process, real
# containers for job bodies, and a throwaway Postgres. The agent is compiled up front so
# no test pays for the build; -p 1 because each test runs its own stack and Docker.
# Point LUTE_E2E_POSTGRES_DSN at an existing server to skip starting a container.
e2e: worker-build-linux
	cd api && go test -tags e2e -count=1 -p 1 -timeout 20m ./e2e/...

# The agent as a container: mount checks and a deleted worker staying stopped.
e2e-image: worker-image
	cd api && LUTE_E2E_WORKER_IMAGE=$(WORKER_IMAGE) go test -tags e2e -count=1 -p 1 -timeout 10m -run TestWorkerImage ./e2e/...

e2e-vet:
	cd api && go vet -tags e2e ./e2e/...
	cd api && $(LINT) run --build-tags e2e ./e2e/...

ui-build:
	cd ui && npm ci && VITE_API_URL= npm run build
	rm -rf api/internal/ui/web && cp -r ui/dist api/internal/ui/web

api-build: ui-build
	cd api && CGO_ENABLED=0 go build -ldflags '-s -w' -o ../bin/api ./cmd/api
