package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	LogLevel     slog.Level
	Server       ServerConfig
	Database     DatabaseConfig
	GRPC         GRPCConfig
	Heartbeat    HeartbeatConfig
	WebSocket    WebSocketConfig
	Auth         AuthConfig
	WorkerBinary WorkerBinaryConfig
	Metrics      MetricsConfig
	JobDefs      JobDefsConfig
	Queue        QueueConfig
	Webhooks     WebhooksConfig
}

// QueueConfig tunes the queue sweep; see queue.Timings for LeaseGrace and ReclaimAfter.
type QueueConfig struct {
	PollInterval time.Duration
	LeaseGrace   time.Duration
	ReclaimAfter time.Duration
}

type WebhooksConfig struct {
	PollInterval time.Duration
}

// JobDefsConfig is the directory of Git-managed job-definition YAML synced on startup.
type JobDefsConfig struct {
	Dir string
}

type MetricsConfig struct {
	// SnapshotInterval is also how often the UI polls metrics.
	SnapshotInterval time.Duration
}

type HeartbeatConfig struct {
	CheckInterval time.Duration
	PingTimeout   time.Duration
	MaxRetries    int
}

type WorkerBinaryConfig struct {
	Dir string
}

type ServerConfig struct {
	Port         string
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	Mode         string // "debug", "release", "test"
	// AllowedOrigins must include the panel's own origin: browsers send Origin even on
	// same-origin POSTs, and unlisted origins get a 403.
	AllowedOrigins []string
}

type DatabaseConfig struct {
	DSN string
}

type GRPCConfig struct {
	Port string
	Host string
}

type WebSocketConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	CheckOrigin     bool
	PingPeriod      time.Duration
	PongWait        time.Duration
	WriteWait       time.Duration
}

type AuthConfig struct {
	JWTSecret     string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	Issuer        string
	CookieSecure  bool
	AdminEmail    string
	AdminPassword string
}

func Load() (*Config, error) {
	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(getEnv("LOG_LEVEL", "info"))); err != nil {
		return nil, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	cfg := &Config{
		LogLevel: logLevel,
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getDurationEnv("SERVER_IDLE_TIMEOUT", 60*time.Second),
			Mode:         getEnv("GIN_MODE", "debug"),
			AllowedOrigins: getCSVEnv("CORS_ALLOWED_ORIGINS", []string{
				"http://localhost:" + getEnv("ADMIN_PORT", "8090"), // admin panel
				"http://localhost:8080",                            // core, direct
				"http://localhost:5173",                            // vite dev server
			}),
		},
		Database: DatabaseConfig{
			DSN: getEnv("POSTGRES_DSN", "postgres://lute:lute@localhost:5432/lute?sslmode=disable"),
		},
		GRPC: GRPCConfig{
			Port: getEnv("GRPC_PORT", "50051"),
			Host: getEnv("GRPC_HOST", "0.0.0.0"),
		},
		Heartbeat: HeartbeatConfig{
			CheckInterval: getDurationEnv("HEARTBEAT_CHECK_INTERVAL", 30*time.Second),
			PingTimeout:   getDurationEnv("HEARTBEAT_PING_TIMEOUT", 5*time.Second),
			MaxRetries:    getIntEnv("HEARTBEAT_MAX_RETRIES", 3),
		},
		WebSocket: WebSocketConfig{
			ReadBufferSize:  getIntEnv("WS_READ_BUFFER_SIZE", 1024),
			WriteBufferSize: getIntEnv("WS_WRITE_BUFFER_SIZE", 1024),
			CheckOrigin:     getBoolEnv("WS_CHECK_ORIGIN", true),
			PingPeriod:      getDurationEnv("WS_PING_PERIOD", 54*time.Second),
			PongWait:        getDurationEnv("WS_PONG_WAIT", 60*time.Second),
			WriteWait:       getDurationEnv("WS_WRITE_WAIT", 10*time.Second),
		},
		Auth: AuthConfig{
			JWTSecret:     getEnv("JWT_SECRET", ""),
			AccessTTL:     getDurationEnv("ACCESS_TOKEN_TTL", 15*time.Minute),
			RefreshTTL:    getDurationEnv("REFRESH_TOKEN_TTL", 30*24*time.Hour),
			Issuer:        getEnv("JWT_ISSUER", "lute"),
			CookieSecure:  getBoolEnv("AUTH_COOKIE_SECURE", false),
			AdminEmail:    getEnv("ADMIN_EMAIL", ""),
			AdminPassword: getEnv("ADMIN_PASSWORD", ""),
		},
		WorkerBinary: WorkerBinaryConfig{
			Dir: getEnv("WORKER_BINARY_DIR", "/opt/lute/worker-binaries"),
		},
		Metrics: MetricsConfig{
			SnapshotInterval: getDurationEnv("METRICS_SNAPSHOT_INTERVAL", 5*time.Minute),
		},
		JobDefs: JobDefsConfig{
			Dir: getEnv("JOB_DEFS_DIR", ""),
		},
		Queue: QueueConfig{
			PollInterval: getDurationEnv("QUEUE_POLL_INTERVAL", time.Second),
			LeaseGrace:   getDurationEnv("QUEUE_LEASE_GRACE", 60*time.Second),
			ReclaimAfter: getDurationEnv("QUEUE_RECLAIM_AFTER", 60*time.Second),
		},
		Webhooks: WebhooksConfig{
			PollInterval: getDurationEnv("WEBHOOK_POLL_INTERVAL", 5*time.Second),
		},
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getCSVEnv(key string, defaultValue []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return defaultValue
	}
	return out
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
