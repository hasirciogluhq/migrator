package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/hasirciogluhq/migrator"
	_ "github.com/lib/pq"
)

func main() {
	// Get database URL from environment variable
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/migrator_example?sslmode=disable"
		log.Printf("DATABASE_URL not set, using default: %s", databaseURL)
	}

	// Connect to database
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Println("✓ Connected to database")

	// Set migrations path to example migrations
	migrationsPath := "./examples/basic/migrations"
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		// Try from project root
		migrationsPath = "./migrations"
	}
	os.Setenv("MIGRATIONS_PATH", migrationsPath)

	// Create migrator
	m := migrator.New(db)

	// Get current status before migration
	applied, err := m.GetAppliedMigrations(context.Background())
	if err != nil {
		log.Printf("Warning: Could not get applied migrations: %v", err)
	} else {
		log.Printf("Currently applied migrations: %d", len(applied))
	}

	// Run migrations
	log.Println("🚀 Starting migration process...")
	if err := m.Migrate(context.Background()); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("✅ Migrations completed successfully!")

	// Show final status
	applied, err = m.GetAppliedMigrations(context.Background())
	if err != nil {
		log.Printf("Warning: Could not get applied migrations: %v", err)
	} else {
		log.Printf("Total applied migrations: %d", len(applied))
		for i, name := range applied {
			fmt.Printf("  %d. %s\n", i+1, name)
		}
	}

	// Query some data to verify
	var userCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount); err == nil {
		log.Printf("Total users in database: %d", userCount)
	}
}
