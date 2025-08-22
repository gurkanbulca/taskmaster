// internal/config/config.go - Updated for Phase 2 with configurable security settings
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gurkanbulca/taskmaster/internal/middleware"
	"github.com/gurkanbulca/taskmaster/pkg/email"
)

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	JWT        JWTConfig
	Email      EmailConfig      // Phase 2
	Security   SecurityConfig   // Phase 2
	Validation ValidationConfig // Phase 2
}

type ServerConfig struct {
	GRPCPort         string
	HTTPPort         string
	Environment      string
	BaseURL          string
	AutoMigrate      bool
	EnableReflection bool
	EnableDebugLogs  bool
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

// Phase 2: Email Configuration
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
	FromName     string
	BaseURL      string
	AppName      string
	SupportEmail string
	TestingMode  bool

	// Email token settings
	VerificationTokenDuration  time.Duration
	PasswordResetTokenDuration time.Duration
	RateLimitPerHour           int
}

// Phase 2: Security Configuration
type SecurityConfig struct {
	MaxLoginAttempts             int           // Max failed login attempts before lockout
	AccountLockoutDuration       time.Duration // How long to lock the account
	MaxEmailVerificationAttempts int
	MaxPasswordResetAttempts     int
	PasswordResetRateLimit       time.Duration
	EnableSecurityNotifications  bool
	RequireEmailVerification     bool
	SessionTimeoutDuration       time.Duration
}

// Phase 2: Validation Configuration
type ValidationConfig struct {
	MinPasswordLength      int
	RequirePasswordUpper   bool
	RequirePasswordLower   bool
	RequirePasswordNumber  bool
	RequirePasswordSpecial bool
	MinUsernameLength      int
	MaxUsernameLength      int
	MaxEmailLength         int
	MaxNameLength          int
	MaxDescriptionLength   int
	MaxTitleLength         int
}

func Load() (*Config, error) {
	return &Config{
		Server: ServerConfig{
			GRPCPort:         getEnv("GRPC_PORT", "50051"),
			HTTPPort:         getEnv("HTTP_PORT", "8080"),
			Environment:      getEnv("ENVIRONMENT", "development"),
			BaseURL:          getEnv("BASE_URL", "http://localhost:3000"),
			AutoMigrate:      getEnvAsBool("AUTO_MIGRATE", true),
			EnableReflection: getEnvAsBool("ENABLE_REFLECTION", true),
			EnableDebugLogs:  getEnvAsBool("ENABLE_DEBUG_LOGS", true),
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
		// Phase 2: Email Configuration
		Email: EmailConfig{
			SMTPHost:     getEnv("SMTP_HOST", "localhost"),
			SMTPPort:     getEnvAsInt("SMTP_PORT", 587),
			SMTPUsername: getEnv("SMTP_USERNAME", ""),
			SMTPPassword: getEnv("SMTP_PASSWORD", ""),
			FromEmail:    getEnv("EMAIL_FROM", "noreply@taskmaster.com"),
			FromName:     getEnv("EMAIL_FROM_NAME", "TaskMaster"),
			BaseURL:      getEnv("BASE_URL", "http://localhost:3000"),
			AppName:      getEnv("APP_NAME", "TaskMaster"),
			SupportEmail: getEnv("SUPPORT_EMAIL", "support@taskmaster.com"),
			TestingMode:  getEnvAsBool("EMAIL_TESTING_MODE", false),

			VerificationTokenDuration:  getEnvAsDuration("EMAIL_VERIFICATION_TOKEN_DURATION", 24*time.Hour),
			PasswordResetTokenDuration: getEnvAsDuration("PASSWORD_RESET_TOKEN_DURATION", 1*time.Hour),
			RateLimitPerHour:           getEnvAsInt("EMAIL_RATE_LIMIT_PER_HOUR", 5),
		},
		// Phase 2: Security Configuration with configurable failed attempts and lockout duration
		Security: SecurityConfig{
			MaxLoginAttempts:             getEnvAsInt("MAX_LOGIN_ATTEMPTS", 5),
			AccountLockoutDuration:       getEnvAsDuration("ACCOUNT_LOCKOUT_DURATION", 15*time.Minute),
			MaxEmailVerificationAttempts: getEnvAsInt("MAX_EMAIL_VERIFICATION_ATTEMPTS", 5),
			MaxPasswordResetAttempts:     getEnvAsInt("MAX_PASSWORD_RESET_ATTEMPTS", 5),
			PasswordResetRateLimit:       getEnvAsDuration("PASSWORD_RESET_RATE_LIMIT", 15*time.Minute),
			EnableSecurityNotifications:  getEnvAsBool("ENABLE_SECURITY_NOTIFICATIONS", true),
			RequireEmailVerification:     getEnvAsBool("REQUIRE_EMAIL_VERIFICATION", false),
			SessionTimeoutDuration:       getEnvAsDuration("SESSION_TIMEOUT_DURATION", 30*24*time.Hour),
		},
		// Phase 2: Validation Configuration
		Validation: ValidationConfig{
			MinPasswordLength:      getEnvAsInt("MIN_PASSWORD_LENGTH", 8),
			RequirePasswordUpper:   getEnvAsBool("REQUIRE_PASSWORD_UPPER", true),
			RequirePasswordLower:   getEnvAsBool("REQUIRE_PASSWORD_LOWER", true),
			RequirePasswordNumber:  getEnvAsBool("REQUIRE_PASSWORD_NUMBER", true),
			RequirePasswordSpecial: getEnvAsBool("REQUIRE_PASSWORD_SPECIAL", false),
			MinUsernameLength:      getEnvAsInt("MIN_USERNAME_LENGTH", 3),
			MaxUsernameLength:      getEnvAsInt("MAX_USERNAME_LENGTH", 50),
			MaxEmailLength:         getEnvAsInt("MAX_EMAIL_LENGTH", 255),
			MaxNameLength:          getEnvAsInt("MAX_NAME_LENGTH", 100),
			MaxDescriptionLength:   getEnvAsInt("MAX_DESCRIPTION_LENGTH", 5000),
			MaxTitleLength:         getEnvAsInt("MAX_TITLE_LENGTH", 200),
		},
	}, nil
}

// ToEmailConfig converts config to email service config
func (c *Config) ToEmailConfig() *email.Config {
	return &email.Config{
		SMTPHost:     c.Email.SMTPHost,
		SMTPPort:     c.Email.SMTPPort,
		SMTPUsername: c.Email.SMTPUsername,
		SMTPPassword: c.Email.SMTPPassword,
		FromEmail:    c.Email.FromEmail,
		FromName:     c.Email.FromName,
		BaseURL:      c.Email.BaseURL,
		AppName:      c.Email.AppName,
		SupportEmail: c.Email.SupportEmail,
	}
}

// ToValidationConfig converts config to validation middleware config
func (c *Config) ToValidationConfig() *middleware.ValidationConfig {
	return &middleware.ValidationConfig{
		MinPasswordLength:      c.Validation.MinPasswordLength,
		RequirePasswordUpper:   c.Validation.RequirePasswordUpper,
		RequirePasswordLower:   c.Validation.RequirePasswordLower,
		RequirePasswordNumber:  c.Validation.RequirePasswordNumber,
		RequirePasswordSpecial: c.Validation.RequirePasswordSpecial,
		MinUsernameLength:      c.Validation.MinUsernameLength,
		MaxUsernameLength:      c.Validation.MaxUsernameLength,
		MaxEmailLength:         c.Validation.MaxEmailLength,
		MaxNameLength:          c.Validation.MaxNameLength,
		MaxDescriptionLength:   c.Validation.MaxDescriptionLength,
		MaxTitleLength:         c.Validation.MaxTitleLength,
	}
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Server.Environment == "production"
}

// ValidateConfig validates the configuration
func (c *Config) ValidateConfig() error {
	if c.IsProduction() {
		// Production validation
		if c.JWT.AccessSecret == "dev-access-secret-change-in-production" ||
			c.JWT.RefreshSecret == "dev-refresh-secret-change-in-production" {
			return fmt.Errorf("JWT secrets must be changed in production")
		}

		if c.Email.SMTPUsername == "" || c.Email.SMTPPassword == "" {
			return fmt.Errorf("SMTP credentials must be configured in production")
		}

		if c.Database.SSLMode != "require" {
			return fmt.Errorf("database SSL must be required in production")
		}
	}

	// General validation
	if c.Validation.MinPasswordLength < 6 {
		return fmt.Errorf("minimum password length cannot be less than 6")
	}

	if c.Security.MaxLoginAttempts < 1 {
		return fmt.Errorf("max login attempts must be at least 1")
	}

	if c.Security.AccountLockoutDuration < 1*time.Minute {
		return fmt.Errorf("account lockout duration must be at least 1 minute")
	}

	return nil
}

// Helper functions

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

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	if value, err := strconv.ParseBool(valueStr); err == nil {
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
