// cmd/server/main.go
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

	taskv1 "github.com/gurkanbulca/taskmaster/api/proto/task/v1/generated"
	ent "github.com/gurkanbulca/taskmaster/ent/generated"
	"github.com/gurkanbulca/taskmaster/ent/generated/migrate"
	"github.com/gurkanbulca/taskmaster/internal/config"
	"github.com/gurkanbulca/taskmaster/internal/database"
	"github.com/gurkanbulca/taskmaster/internal/repository"
	"github.com/gurkanbulca/taskmaster/internal/service"
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

	// Connect to database with Ent
	log.Println("📦 Connecting to PostgreSQL with Ent...")
	entClient, err := database.NewEntClient(database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
		Debug:    cfg.Server.Environment == "development",
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func(entClient *ent.Client) {
		err := entClient.Close()
		if err != nil {
			panic(err)
		}
	}(entClient)

	// Run auto migration
	if err := runAutoMigration(context.Background(), entClient); err != nil {
		log.Fatalf("Failed to run auto migration: %v", err)
	}

	// Initialize repository with Ent
	taskRepo := repository.NewEntTaskRepository(entClient)

	// Initialize services
	taskService := service.NewTaskService(taskRepo)

	// Create gRPC server with interceptors
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
	)

	// Register services
	taskv1.RegisterTaskServiceServer(grpcServer, taskService)

	// Register health check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection for grpcurl
	reflection.Register(grpcServer)

	// Create listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.Server.GRPCPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Start server in goroutine
	go func() {
		log.Printf("🚀 gRPC server listening on port %s", cfg.Server.GRPCPort)
		log.Println("📡 Server is ready to accept connections")
		log.Println("")
		log.Println("Test commands:")
		log.Printf("  grpcurl -plaintext localhost:%s list", cfg.Server.GRPCPort)
		log.Printf("  grpcurl -plaintext localhost:%s describe task.v1.TaskService", cfg.Server.GRPCPort)
		log.Printf("  grpcurl -plaintext localhost:%s grpc.health.v1.Health/Check", cfg.Server.GRPCPort)
		log.Println("")
		log.Println("Or run the test client:")
		log.Println("  go run cmd/client/main.go")

		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		log.Println("✅ Server shutdown complete")
	case <-ctx.Done():
		grpcServer.Stop()
		log.Println("⚠️  Server shutdown forced")
	}
}

// runAutoMigration runs the auto migration
func runAutoMigration(ctx context.Context, client *ent.Client) error {
	log.Println("🔧 Running auto migration...")

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

// Simple logging interceptor
func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()

	// Call the handler
	resp, err := handler(ctx, req)

	// Log the call
	duration := time.Since(start)
	if err != nil {
		log.Printf("❌ %s failed in %v: %v", info.FullMethod, duration, err)
	} else {
		log.Printf("✅ %s completed in %v", info.FullMethod, duration)
	}

	return resp, err
}
