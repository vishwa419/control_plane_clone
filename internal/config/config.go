package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the application
type Config struct {
	Redis      RedisConfig
	Storage    StorageConfig
	Upload     ServerConfig
	Consumer   ServerConfig
	GRPC       GRPCConfig
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Host     string
	Port     string
	Password string
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	Type string // "local" or "minio"
	Path string // File storage path
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port string
}

// GRPCConfig holds gRPC server configuration
type GRPCConfig struct {
	Port      string
	MaxStreams int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "redis"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		Storage: StorageConfig{
			Type: getEnv("STORAGE_TYPE", "local"),
			Path: getEnv("STORAGE_PATH", "/app/files"),
		},
		Upload: ServerConfig{
			Port: getEnv("UPLOAD_PORT", "8080"),
		},
		Consumer: ServerConfig{
			Port: getEnv("CONSUMER_PORT", "8081"),
		},
		GRPC: GRPCConfig{
			Port:       getEnv("GRPC_PORT", "50051"),
			MaxStreams: parseInt(getEnv("GRPC_MAX_STREAMS", "1000")),
		},
	}
}

// parseInt parses an integer from string, returns 0 on error
func parseInt(s string) int {
	result, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return result
}

// GetRedisAddr returns the Redis address in host:port format
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.Redis.Host, c.Redis.Port)
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
