# Lute API - Backend Service

A scalable Go backend service for the Lute VM management platform, built with Gin, MongoDB, WebSockets, and gRPC.

## Architecture

The application follows a clean architecture pattern with clear separation of concerns:

```
api/
├── cmd/
│   └── server/          # Application entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── database/        # Database connections
│   ├── grpc/            # gRPC server implementation
│   ├── handlers/        # HTTP request handlers
│   ├── middleware/      # HTTP middleware (CORS, auth, logging)
│   ├── models/          # Data models
│   ├── repository/      # Data access layer
│   ├── router/          # HTTP router setup
│   └── websocket/       # WebSocket hub and clients
├── proto/               # Protocol buffer definitions
└── go.mod               # Go module definition
```

## Features

- **RESTful API** with Gin framework
- **WebSocket support** for real-time browser communication
- **gRPC server** for agent communication
- **MongoDB** for data persistence
- **Clean architecture** with separation of concerns
- **Health check endpoints** for monitoring
- **CORS middleware** for cross-origin requests
- **Structured logging** and error handling

## Prerequisites

- Go 1.21 or higher
- MongoDB 4.4 or higher
- Protocol Buffers compiler (protoc) for gRPC

## Installation

1. **Install Go** (if not already installed):
   - Visit https://go.dev/doc/install
   - Ensure Go 1.21 or higher is installed
   - Verify: `go version`

2. **Setup IDE Support** (for autocomplete and IntelliSense):
   ```bash
   # Run the setup script
   chmod +x scripts/setup-ide.sh
   ./scripts/setup-ide.sh
   
   # Or manually:
   go mod download
   go install golang.org/x/tools/gopls@latest
   go install golang.org/x/tools/cmd/goimports@latest
   ```

3. **Install IDE Extension**:
   - For Cursor/VS Code: Install the "Go" extension by Google
   - The extension should auto-detect Go and start the language server (gopls)
   - If autocomplete doesn't work, reload the window: `Cmd/Ctrl + Shift + P` -> "Reload Window"

4. **Download dependencies**:
   ```bash
   make deps
   # or
   go mod download
   go mod tidy
   ```

2. Install Protocol Buffers compiler and Go plugins:
```bash
# Install protoc (see https://grpc.io/docs/protoc-installation/)
# On macOS: brew install protobuf
# On Ubuntu: sudo apt-get install protobuf-compiler

# Install Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

3. Generate gRPC code (if you modify proto files):
```bash
make proto
# or
./scripts/generate-proto.sh
```

**Note:** The repository includes placeholder proto files for compilation. You should generate the actual proto code before running the server.

## Configuration

The application uses environment variables for configuration. Create a `.env` file or set the following variables:

```env
# Server Configuration
SERVER_PORT=8080
SERVER_HOST=0.0.0.0
SERVER_READ_TIMEOUT=15s
SERVER_WRITE_TIMEOUT=15s
SERVER_IDLE_TIMEOUT=60s
GIN_MODE=debug

# MongoDB Configuration
MONGODB_URI=mongodb://localhost:27017
MONGODB_DATABASE=lute
MONGODB_CONNECT_TIMEOUT=10s
MONGODB_MAX_POOL_SIZE=100

# gRPC Configuration
GRPC_PORT=50051
GRPC_HOST=0.0.0.0

# WebSocket Configuration
WS_READ_BUFFER_SIZE=1024
WS_WRITE_BUFFER_SIZE=1024
WS_CHECK_ORIGIN=false
WS_PING_PERIOD=54s
WS_PONG_WAIT=60s
WS_WRITE_WAIT=10s
```

## Running the Application

### Development
```bash
make run
# or
go run ./cmd/server
```

### Production Build
```bash
make build
./bin/server
```

### With Hot Reload (requires Air)
```bash
make dev
```

## API Endpoints

### Health Checks
- `GET /api/health` - Health check endpoint
- `GET /api/ready` - Readiness probe endpoint

### WebSocket
- `GET /api/ws` - WebSocket connection endpoint

### Machines (Protected - requires authentication)
- `POST /api/v1/machines` - Create a new machine
- `GET /api/v1/machines` - List user's machines
- `GET /api/v1/machines/public` - List public machines
- `GET /api/v1/machines/:id` - Get machine by ID
- `PUT /api/v1/machines/:id` - Update machine
- `DELETE /api/v1/machines/:id` - Delete machine

## gRPC Services

The gRPC server runs on port 50051 (configurable) and provides the following services:

- `RegisterAgent` - Register a new agent connection
- `Heartbeat` - Periodic heartbeat from agent
- `UpdateMachineStatus` - Update machine status
- `GetMachineConfig` - Retrieve machine configuration
- `ExecuteCommand` - Execute commands on machines
- `StreamLogs` - Stream logs from agents

See `proto/agent.proto` for detailed service definitions.

## WebSocket Communication

The WebSocket endpoint (`/api/ws`) allows real-time bidirectional communication between the browser and server. The hub manages all connected clients and can broadcast messages to all or specific clients.

## Authentication

Currently, authentication middleware is a placeholder. You need to implement Firebase JWT token verification in `internal/middleware/auth.go`.

## Development

### Project Structure

- **cmd/server**: Main application entry point
- **internal/config**: Configuration loading and management
- **internal/database**: Database connection and health checks
- **internal/handlers**: HTTP request handlers
- **internal/middleware**: HTTP middleware (CORS, auth, logging)
- **internal/models**: Data models with MongoDB tags
- **internal/repository**: Data access layer with MongoDB operations
- **internal/router**: Gin router setup and route definitions
- **internal/websocket**: WebSocket hub and client management
- **internal/grpc**: gRPC server implementation
- **proto**: Protocol buffer definitions

### Adding New Features

1. **New Model**: Add to `internal/models/models.go`
2. **New Repository**: Create in `internal/repository/`
3. **New Handler**: Create in `internal/handlers/`
4. **New Route**: Add to `internal/router/router.go`
5. **New gRPC Service**: Update `proto/agent.proto` and regenerate code

## Testing

```bash
make test
# or
go test ./...
```

## Building for Production

```bash
make build
```

The binary will be in `bin/server`.

## Docker (Optional)

You can containerize the application. Example Dockerfile:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o server ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
CMD ["./server"]
```

