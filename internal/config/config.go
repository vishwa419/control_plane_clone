package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the application
type Config struct {
	Redis      RedisConfig
	Storage    StorageConfig
	Upload     ServerConfig
	Consumer   ServerConfig
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
	}
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
