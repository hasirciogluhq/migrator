// Package shadowdb handles shadow database creation, testing, and cleanup.
package shadowdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/hasirciogluhq/migrator/internal/tracker"
	"github.com/hasirciogluhq/migrator/internal/validator"
)

// Manager manages shadow database operations.
type Manager struct {
	mainDB        *sql.DB
	currentDBName string
	shadowDBName  string
	databaseURL   string
}

// NewWithURL creates a new shadow database Manager with explicit database URL.
func NewWithURL(mainDB *sql.DB, databaseURL string) (*Manager, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is required for shadow database operations")
	}

	return &Manager{
		mainDB:      mainDB,
		databaseURL: databaseURL,
	}, nil
}

// New creates a new shadow database Manager using DATABASE_URL environment variable.
// Deprecated: Use NewWithURL instead for more explicit configuration.
func New(mainDB *sql.DB) (*Manager, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable not set")
	}

	return NewWithURL(mainDB, databaseURL)
}

// TestNewMigrations tests new migrations on a shadow database.
func (m *Manager) TestNewMigrations(ctx context.Context, mainTracker *tracker.Tracker, newMigrations []*validator.MigrationFile) error {
	if len(newMigrations) == 0 {
		fmt.Println("✓ No new migrations found, skipping shadow database test")
		return nil
	}

	fmt.Printf("🔍 Found %d new migrations, testing on shadow database...\n", len(newMigrations))

	// Get current database name
	currentDBName, err := getCurrentDatabaseName(ctx, m.mainDB)
	if err != nil {
		return fmt.Errorf("failed to get current database name: %w", err)
	}
	m.currentDBName = currentDBName
	m.shadowDBName = currentDBName + "_gi_mig_shadow_db"

	// Setup shadow database
	shadowDB, cleanup, err := m.setupShadowDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to setup shadow database: %w", err)
	}
	defer cleanup()

	// Create shadow tracker
	shadowTracker := tracker.New(shadowDB)
	if err := shadowTracker.EnsureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table in shadow: %w", err)
	}

	// Apply existing migrations to shadow database
	if err := m.applyExistingMigrationsToShadow(ctx, mainTracker, shadowTracker); err != nil {
		return fmt.Errorf("failed to apply existing migrations to shadow: %w", err)
	}

	// Test new migrations on shadow database
	if err := m.testMigrationsOnShadow(ctx, shadowDB, newMigrations); err != nil {
		return fmt.Errorf("failed to test migrations on shadow: %w", err)
	}

	fmt.Println("✓ Shadow database test passed")
	return nil
}

// setupShadowDatabase creates and configures a shadow database for testing.
func (m *Manager) setupShadowDatabase(ctx context.Context) (*sql.DB, func(), error) {
	// Connect to postgres database for management
	postgresDB, err := m.connectToPostgresDatabase()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to postgres database: %w", err)
	}

	fmt.Println("🧹 Cleaning up any previous shadow database before testing...")

	// Clean up existing shadow database
	if err := dropDatabaseIfExists(ctx, postgresDB, m.shadowDBName); err != nil {
		postgresDB.Close()
		return nil, nil, fmt.Errorf("failed to drop existing shadow database: %w", err)
	}

	// Create new shadow database
	if err := createDatabase(ctx, postgresDB, m.shadowDBName); err != nil {
		postgresDB.Close()
		return nil, nil, fmt.Errorf("failed to create shadow database: %w", err)
	}

	// Connect to shadow database
	shadowDB, err := m.connectToDatabase(m.shadowDBName)
	if err != nil {
		postgresDB.Close()
		return nil, nil, fmt.Errorf("failed to connect to shadow database: %w", err)
	}

	// Return cleanup function
	cleanup := func() {
		shadowDB.Close()

		// Clean up shadow database with background context
		bgCtx := context.Background()
		fmt.Printf("🗑️  Cleaning up shadow database %s...\n", m.shadowDBName)
		if err := dropDatabaseIfExists(bgCtx, postgresDB, m.shadowDBName); err != nil {
			fmt.Printf("⚠️  Warning: Failed to clean up shadow database %s: %v\n", m.shadowDBName, err)
		}

		postgresDB.Close()
	}

	return shadowDB, cleanup, nil
}

// applyExistingMigrationsToShadow applies all existing migrations to shadow database.
func (m *Manager) applyExistingMigrationsToShadow(ctx context.Context, mainTracker, shadowTracker *tracker.Tracker) error {
	appliedMigrations, err := mainTracker.GetAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Get migrations path
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "./migrations"
	}

	// Apply each existing migration to shadow
	for _, migrationName := range appliedMigrations {
		content, err := os.ReadFile(migrationsPath + "/" + migrationName)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", migrationName, err)
		}

		if err := shadowTracker.ApplyMigration(ctx, migrationName, string(content)); err != nil {
			return fmt.Errorf("failed to apply existing migration %s to shadow: %w", migrationName, err)
		}
	}

	return nil
}

// testMigrationsOnShadow tests new migrations on shadow database.
func (m *Manager) testMigrationsOnShadow(ctx context.Context, shadowDB *sql.DB, migrations []*validator.MigrationFile) error {
	shadowTracker := tracker.New(shadowDB)

	for _, migration := range migrations {
		fmt.Printf("  🧪 Testing migration: %s\n", migration.Name)

		if err := shadowTracker.ApplyMigration(ctx, migration.Name, migration.Content); err != nil {
			return fmt.Errorf("migration %s failed on shadow database: %w", migration.Name, err)
		}

		fmt.Printf("  ✓ Migration %s passed shadow test\n", migration.Name)
	}

	return nil
}

// EnsureCleanup ensures shadow database is dropped.
func (m *Manager) EnsureCleanup(ctx context.Context) error {
	// Get current database name if not set
	if m.currentDBName == "" {
		currentDBName, err := getCurrentDatabaseName(ctx, m.mainDB)
		if err != nil {
			return fmt.Errorf("failed to get current database name: %w", err)
		}
		m.currentDBName = currentDBName
		m.shadowDBName = currentDBName + "_gi_mig_shadow_db"
	}

	// Connect to postgres database for management
	postgresDB, err := m.connectToPostgresDatabase()
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer postgresDB.Close()

	// Check if shadow database exists
	var exists bool
	err = postgresDB.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		m.shadowDBName,
	).Scan(&exists)

	if err != nil {
		return fmt.Errorf("failed to check if shadow database exists: %w", err)
	}

	if exists {
		fmt.Printf("🧹 Final cleanup: Shadow database %s still exists, dropping...\n", m.shadowDBName)
		if err := dropDatabaseIfExists(ctx, postgresDB, m.shadowDBName); err != nil {
			return fmt.Errorf("failed to drop shadow database: %w", err)
		}
	}

	return nil
}

// Helper functions

func getCurrentDatabaseName(ctx context.Context, db *sql.DB) (string, error) {
	var dbName string
	err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&dbName)
	return dbName, err
}

func (m *Manager) connectToPostgresDatabase() (*sql.DB, error) {
	currentDB := extractDBNameFromDSN(m.databaseURL)
	dsn := strings.Replace(m.databaseURL, "/"+currentDB, "/postgres", 1)
	return sql.Open("postgres", dsn)
}

func (m *Manager) connectToDatabase(dbName string) (*sql.DB, error) {
	currentDB := extractDBNameFromDSN(m.databaseURL)
	dsn := strings.Replace(m.databaseURL, "/"+currentDB, "/"+dbName, 1)
	return sql.Open("postgres", dsn)
}

func extractDBNameFromDSN(dsn string) string {
	parts := strings.Split(dsn, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if idx := strings.Index(lastPart, "?"); idx != -1 {
			return lastPart[:idx]
		}
		return lastPart
	}
	return "postgres"
}

func dropDatabaseIfExists(ctx context.Context, db *sql.DB, dbName string) error {
	// Terminate all connections to the database first
	_, err := db.ExecContext(ctx, `
		SELECT pg_terminate_backend(pid) 
		FROM pg_stat_activity 
		WHERE datname = $1 AND pid <> pg_backend_pid()
	`, dbName)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to terminate connections for %s: %v\n", dbName, err)
	}

	// Drop the database - Note: Database names cannot be parameterized
	// This is safe because dbName is constructed internally
	dropSQL := fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)
	_, err = db.ExecContext(ctx, dropSQL)
	if err != nil {
		return fmt.Errorf("failed to drop database %s: %w", dbName, err)
	}

	fmt.Printf("✅ Successfully dropped database: %s\n", dbName)
	return nil
}

func createDatabase(ctx context.Context, db *sql.DB, dbName string) error {
	fmt.Printf("🏗️  Creating database: %s\n", dbName)

	// Note: Database names cannot be parameterized
	// This is safe because dbName is constructed internally
	createSQL := fmt.Sprintf("CREATE DATABASE %s", dbName)
	_, err := db.ExecContext(ctx, createSQL)
	if err != nil {
		return fmt.Errorf("failed to create database %s: %w", dbName, err)
	}

	fmt.Printf("✅ Successfully created database: %s\n", dbName)
	return nil
}
