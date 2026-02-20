package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server      ServerConfig
	MongoDB     MongoDBConfig
	GRPC        GRPCConfig
	Heartbeat   HeartbeatConfig
	WebSocket   WebSocketConfig
	Firebase    FirebaseConfig
	AgentBinary AgentBinaryConfig
}

type HeartbeatConfig struct {
	CheckInterval time.Duration
	PingTimeout   time.Duration
	MaxRetries    int
}

type AgentBinaryConfig struct {
	Dir string // directory containing compiled agent binaries
}

type ServerConfig struct {
	Port         string
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	Mode         string // "debug", "release", "test"
}

type MongoDBConfig struct {
	URI            string
	Database       string
	ConnectTimeout time.Duration
	MaxPoolSize    uint64
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

type FirebaseConfig struct {
	ProjectID        string
	CredentialsJSON  string
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getDurationEnv("SERVER_IDLE_TIMEOUT", 60*time.Second),
			Mode:         getEnv("GIN_MODE", "debug"),
		},
		MongoDB: MongoDBConfig{
			URI:            getEnv("MONGODB_URI", "mongodb://localhost:27017"),
			Database:       getEnv("MONGODB_DATABASE", "lute"),
			ConnectTimeout: getDurationEnv("MONGODB_CONNECT_TIMEOUT", 10*time.Second),
			MaxPoolSize:    getUint64Env("MONGODB_MAX_POOL_SIZE", 100),
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
			CheckOrigin:     getBoolEnv("WS_CHECK_ORIGIN", false),
			PingPeriod:      getDurationEnv("WS_PING_PERIOD", 54*time.Second),
			PongWait:        getDurationEnv("WS_PONG_WAIT", 60*time.Second),
			WriteWait:       getDurationEnv("WS_WRITE_WAIT", 10*time.Second),
		},
		Firebase: FirebaseConfig{
			ProjectID:       getEnv("FIREBASE_PROJECT_ID", ""),
			CredentialsJSON: getEnv("FIREBASE_CREDENTIALS_JSON", ""),
		},
		AgentBinary: AgentBinaryConfig{
			Dir: getEnv("AGENT_BINARY_DIR", "/opt/lute/agent-binaries"),
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

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getUint64Env(key string, defaultValue uint64) uint64 {
	if value := os.Getenv(key); value != "" {
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			return uintValue
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

