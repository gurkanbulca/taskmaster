// internal/config/config.go
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
}

type ServerConfig struct {
	GRPCPort    string
	HTTPPort    string
	Environment string // Added this field
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func Load() (*Config, error) {
	return &Config{
		Server: ServerConfig{
			GRPCPort:    getEnv("GRPC_PORT", "50051"),
			HTTPPort:    getEnv("HTTP_PORT", "8080"),
			Environment: getEnv("ENVIRONMENT", "development"), // Added this
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "taskmaster"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}
