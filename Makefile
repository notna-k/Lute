.PHONY: dev-up dev-down dev-clean dev-logs worker-build worker-build-all worker-build-linux go-format-check go-test go-lint ui-build api-build

export DOCKER_BUILDKIT := 1
export WORKER_VERSION ?= 0.1.0
export BUILD_TIME     := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

ENV_FILE ?= .env
COMPOSE  := docker compose -f infrastructure/dev/docker-compose.yml --env-file $(ENV_FILE)
LINT     := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
GOBUILD  := CGO_ENABLED=0 go build -ldflags '-X main.Version=$(WORKER_VERSION) -X main.BuildTime=$(BUILD_TIME)'
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

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
	echo $(WORKER_VERSION) > worker/bin/VERSION

worker-build-all:
	cd worker && for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		GOOS=$$os GOARCH=$$arch $(GOBUILD) -o bin/lute-worker-$$os-$$arch$$ext ./cmd/worker || exit 1; \
	done
	echo $(WORKER_VERSION) > worker/bin/VERSION

worker-build-linux:
	$(MAKE) worker-build-all PLATFORMS=linux/amd64

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

ui-build:
	cd ui && npm ci && VITE_API_URL= npm run build
	rm -rf api/internal/ui/web && cp -r ui/dist api/internal/ui/web

api-build: ui-build
	cd api && CGO_ENABLED=0 go build -ldflags '-s -w' -o ../bin/api ./cmd/api
