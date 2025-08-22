package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	ent "github.com/gurkanbulca/taskmaster/ent/generated"
	"github.com/gurkanbulca/taskmaster/ent/generated/migrate"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Build connection string
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "postgres"),
		getEnv("DB_NAME", "taskmaster"),
		getEnv("DB_SSL_MODE", "disable"),
	)

	// Connect to database
	drv, err := sql.Open(dialect.Postgres, dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer drv.Close()

	// Create Ent client
	client := ent.NewClient(ent.Driver(drv))
	defer client.Close()

	ctx := context.Background()

	// Run migrations
	log.Println("Running database migrations...")
	if err := client.Schema.Create(
		ctx,
		migrate.WithDropIndex(true),
		migrate.WithDropColumn(true),
		migrate.WithForeignKeys(true),
	); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("✅ Migrations completed successfully!")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
