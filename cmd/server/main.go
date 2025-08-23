// cmd/server/main.go - Updated with configurable security settings
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/joho/godotenv"

	authv1 "github.com/gurkanbulca/taskmaster/api/proto/auth/v1/generated"
	taskv1 "github.com/gurkanbulca/taskmaster/api/proto/task/v1/generated"
	ent "github.com/gurkanbulca/taskmaster/ent/generated"
	"github.com/gurkanbulca/taskmaster/ent/generated/migrate"
	"github.com/gurkanbulca/taskmaster/internal/config"
	"github.com/gurkanbulca/taskmaster/internal/database"
	"github.com/gurkanbulca/taskmaster/internal/middleware"
	"github.com/gurkanbulca/taskmaster/internal/repository"
	"github.com/gurkanbulca/taskmaster/internal/service"
	"github.com/gurkanbulca/taskmaster/pkg/auth"
	"github.com/gurkanbulca/taskmaster/pkg/email"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate configuration
	if err := cfg.ValidateConfig(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Connect to database with Ent
	log.Println("Connecting to PostgreSQL with Ent...")
	entClient, err := database.NewEntClient(database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
		Debug:    cfg.IsDevelopment(),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := entClient.Close(); err != nil {
			log.Printf("Failed to close database connection: %v", err)
		}
	}()

	// Run auto migration
	if cfg.Server.AutoMigrate {
		if err := runAutoMigration(context.Background(), entClient); err != nil {
			log.Fatalf("Failed to run auto migration: %v", err)
		}
	}

	// Initialize token manager
	tokenManager := auth.NewTokenManager(
		cfg.JWT.AccessSecret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessTokenDuration,
		cfg.JWT.RefreshTokenDuration,
	)

	// Initialize email service
	var emailService email.EmailService
	if cfg.Email.TestingMode || cfg.IsDevelopment() {
		log.Println("Using mock email service for development/testing")
		emailService = email.NewMockEmailService()
	} else {
		log.Println("Using SMTP email service")
		emailService = email.NewSMTPEmailService(cfg.ToEmailConfig())

		// Test SMTP connection
		if smtpService, ok := emailService.(*email.SMTPEmailService); ok {
			if err := smtpService.TestConnection(context.Background()); err != nil {
				log.Printf("Warning: SMTP connection test failed: %v", err)
			} else {
				log.Println("SMTP connection test successful")
			}
		}
	}

	// Initialize services
	securityService := service.NewSecurityService(entClient)
	securityLogger := service.NewSecurityLogger(securityService)

	emailVerificationService := service.NewEmailVerificationService(entClient, emailService, securityLogger)
	passwordResetService := service.NewPasswordResetService(entClient, emailService, auth.NewPasswordManager(), securityLogger)

	taskRepo := repository.NewEntTaskRepository(entClient)

	// Pass security config to auth service
	authService := service.NewAuthService(
		entClient,
		tokenManager,
		emailVerificationService,
		passwordResetService,
		securityLogger,
		cfg.Security, // Pass the security configuration
	)

	taskService := service.NewTaskService(taskRepo)

	// Initialize middleware
	metadataExtractor := middleware.NewMetadataExtractorInterceptor()
	authInterceptor := middleware.NewUpdatedAuthInterceptor(tokenManager)
	validationInterceptor := middleware.NewEnhancedValidationInterceptor(cfg.ToValidationConfig())

	// Create gRPC server with interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			metadataExtractor.Unary(),
			validationInterceptor.Unary(),
			authInterceptor.Unary(),
			loggingInterceptor,
		),
		grpc.ChainStreamInterceptor(
			metadataExtractor.Stream(),
			validationInterceptor.Stream(),
			authInterceptor.Stream(),
		),
	)

	// Register services
	authv1.RegisterAuthServiceServer(grpcServer, authService)
	taskv1.RegisterTaskServiceServer(grpcServer, taskService)

	// Register health check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	// Set the health status for all services
	healthServer.SetServingStatus("auth.v1.AuthService", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("task.v1.TaskService", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING) // For overall health

	// Register reflection for development
	if cfg.Server.EnableReflection {
		reflection.Register(grpcServer)
		log.Println("gRPC reflection enabled (disable in production)")
	}

	// Create listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.Server.GRPCPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Start background cleanup job
	go startCleanupJob(context.Background(), emailVerificationService, passwordResetService)

	// Start server in goroutine
	go func() {
		log.Printf("🚀 TaskMaster gRPC server listening on port %s", cfg.Server.GRPCPort)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("📴 Shutting down server...")
	grpcServer.GracefulStop()
	log.Println("✅ Server shutdown complete")
}

// runAutoMigration runs the auto migration
func runAutoMigration(ctx context.Context, client *ent.Client) error {
	log.Println("🔄 Running auto migration...")
	err := client.Schema.Create(
		ctx,
		migrate.WithDropIndex(true),
		migrate.WithDropColumn(true),
		migrate.WithForeignKeys(true),
	)
	if err != nil {
		return fmt.Errorf("run auto migration: %w", err)
	}
	log.Println("✅ Auto migration completed")
	return nil
}

// startCleanupJob starts background cleanup jobs
func startCleanupJob(ctx context.Context, emailVerificationService *service.EmailVerificationService, passwordResetService *service.PasswordResetService) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	log.Println("🧹 Starting background cleanup job (runs every hour)")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := emailVerificationService.CleanupExpiredTokens(ctx); err != nil {
				log.Printf("Failed to cleanup expired email verification tokens: %v", err)
			}
			if err := passwordResetService.CleanupExpiredTokens(ctx); err != nil {
				log.Printf("Failed to cleanup expired password reset tokens: %v", err)
			}
			log.Println("🧹 Token cleanup completed")
		}
	}
}

// loggingInterceptor logs incoming requests
func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	clientInfo := middleware.GetClientInfoFromContext(ctx)
	resp, err := handler(ctx, req)
	duration := time.Since(start)
	logLevel := "INFO"
	if err != nil {
		logLevel = "ERROR"
	}
	log.Printf("[%s] %s completed in %v (user: %s, ip: %s)",
		logLevel, info.FullMethod, duration, clientInfo.UserID, clientInfo.IPAddress)
	if err != nil {
		log.Printf("[ERROR] %s error: %v", info.FullMethod, err)
	}
	return resp, err
}
