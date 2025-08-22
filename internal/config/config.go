// internal/config/config.go
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
}

type ServerConfig struct {
	GRPCPort    string
	HTTPPort    string
	Environment string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type JWTConfig struct {
	AccessSecret         string
	RefreshSecret        string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
}

func Load() (*Config, error) {
	return &Config{
		Server: ServerConfig{
			GRPCPort:    getEnv("GRPC_PORT", "50051"),
			HTTPPort:    getEnv("HTTP_PORT", "8080"),
			Environment: getEnv("ENVIRONMENT", "development"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "taskmaster"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},
		JWT: JWTConfig{
			AccessSecret:         getEnv("JWT_ACCESS_SECRET", getEnv("JWT_SECRET", "dev-access-secret-change-in-production")),
			RefreshSecret:        getEnv("JWT_REFRESH_SECRET", getEnv("JWT_SECRET", "dev-refresh-secret-change-in-production")),
			AccessTokenDuration:  getEnvAsDuration("JWT_ACCESS_TOKEN_DURATION", 15*time.Minute),
			RefreshTokenDuration: getEnvAsDuration("JWT_REFRESH_TOKEN_DURATION", 7*24*time.Hour),
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

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	// Try parsing as duration string (e.g., "15m", "24h")
	if duration, err := time.ParseDuration(valueStr); err == nil {
		return duration
	}

	return defaultValue
}
