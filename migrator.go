// Package migrator provides a production-ready database migration tool with shadow database testing.
//
// Features:
//   - Transaction-safe migrations with automatic rollback
//   - Shadow database testing before production deployment
//   - Migration consistency validation
//   - PostgreSQL support with proper connection handling
//   - Detailed logging and error reporting
//
// Usage:
//
//	import (
//		"context"
//		"database/sql"
//		_ "github.com/lib/pq"
//		"github.com/hasirciogluhq/migrator"
//	)
//
//	db, _ := sql.Open("postgres", "postgres://user:pass@localhost/dbname")
//	m := migrator.New(db)
//	if err := m.Migrate(context.Background()); err != nil {
//		log.Fatal(err)
//	}
package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/hasirciogluhq/migrator/internal/shadowdb"
	"github.com/hasirciogluhq/migrator/internal/tracker"
	"github.com/hasirciogluhq/migrator/internal/validator"
)

// Migrator handles database migrations with shadow database testing.
type Migrator struct {
	db             *sql.DB
	tracker        *tracker.Tracker
	validator      *validator.Validator
	shadowManager  *shadowdb.Manager
	migrationsPath string
}

// Options configures the Migrator behavior.
type Options struct {
	// MigrationsPath is the directory containing SQL migration files.
	// If empty, defaults to "./migrations" or MIGRATIONS_PATH env var.
	MigrationsPath string

	// DatabaseURL is the PostgreSQL connection string used for shadow database operations.
	// If empty, falls back to DATABASE_URL env var.
	// Required for shadow database testing feature.
	DatabaseURL string

	// SkipShadowDB disables shadow database testing.
	// Not recommended for production use.
	SkipShadowDB bool
}

// New creates a new Migrator instance with default options.
//
// The database connection should be properly configured and tested before
// passing it to the migrator. The migrator will use this connection for
// all migration operations.
func New(db *sql.DB) *Migrator {
	return NewWithOptions(db, Options{})
}

// NewWithOptions creates a new Migrator instance with custom options.
func NewWithOptions(db *sql.DB, opts Options) *Migrator {
	migrationsPath := opts.MigrationsPath
	if migrationsPath == "" {
		migrationsPath = os.Getenv("MIGRATIONS_PATH")
		if migrationsPath == "" {
			migrationsPath = "./migrations"
		}
	}

	// Get database URL from options or environment
	databaseURL := opts.DatabaseURL
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}

	t := tracker.New(db)
	v := validator.New(t, migrationsPath)

	// Initialize shadow manager with database URL if provided
	var shadowMgr *shadowdb.Manager
	if databaseURL != "" {
		shadowMgr, _ = shadowdb.NewWithURL(db, databaseURL)
	}

	return &Migrator{
		db:             db,
		tracker:        t,
		validator:      v,
		shadowManager:  shadowMgr,
		migrationsPath: migrationsPath,
	}
}

// Migrate runs the complete migration process with shadow database testing.
//
// Process:
//  1. Ensure migrations tracking table exists
//  2. Validate existing migrations
//  3. Get all migration files
//  4. Test new migrations on shadow database
//  5. Apply pending migrations to production
//  6. Clean up shadow database
//
// Returns an error if any step fails. All migrations are applied in transactions
// with automatic rollback on failure.
func (m *Migrator) Migrate(ctx context.Context) error {
	// Step 1: Ensure migrations table exists
	if err := m.tracker.EnsureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to ensure migrations table: %w", err)
	}

	// Step 2: Validate existing migrations
	if err := m.validator.ValidateExistingMigrations(ctx); err != nil {
		return fmt.Errorf("migration validation failed: %w", err)
	}

	// Step 3: Get all migration files
	migrationFiles, err := m.validator.GetMigrationFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	// Step 4: Find new migrations
	newMigrations, err := validator.FindNewMigrations(ctx, migrationFiles)
	if err != nil {
		return fmt.Errorf("failed to find new migrations: %w", err)
	}

	// Step 5: Test new migrations on shadow database
	if len(newMigrations) > 0 {
		// Initialize shadow manager lazily if not already initialized
		if m.shadowManager == nil {
			// Try to get DATABASE_URL from environment as fallback
			databaseURL := os.Getenv("DATABASE_URL")
			if databaseURL != "" {
				shadowMgr, err := shadowdb.NewWithURL(m.db, databaseURL)
				if err != nil {
					return fmt.Errorf("failed to initialize shadow database manager: %w", err)
				}
				m.shadowManager = shadowMgr
			} else {
				fmt.Println("⚠️  Warning: DATABASE_URL not provided, skipping shadow database test")
				fmt.Println("   To enable shadow database testing, provide DatabaseURL in Options or set DATABASE_URL env var")
			}
		}

		if m.shadowManager != nil {
			if err := m.shadowManager.TestNewMigrations(ctx, m.tracker, newMigrations); err != nil {
				return fmt.Errorf("shadow database test failed: %w", err)
			}
		}
	} else {
		fmt.Println("✓ No new migrations found, skipping shadow database test")
	}

	// Step 6: Apply all pending migrations to production
	if err := m.applyPendingMigrations(ctx, migrationFiles); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	// Step 7: Final cleanup - ensure shadow database is dropped
	if m.shadowManager != nil {
		if err := m.shadowManager.EnsureCleanup(ctx); err != nil {
			fmt.Printf("⚠️  Warning: Final shadow database cleanup failed: %v\n", err)
		}
	}

	return nil
}

// applyPendingMigrations applies all pending migrations to production database.
func (m *Migrator) applyPendingMigrations(ctx context.Context, migrations []*validator.MigrationFile) error {
	fmt.Println("🚀 Applying migrations to production database...")

	appliedCount := 0
	for _, migration := range migrations {
		isApplied, err := migration.IsApplied(ctx)
		if err != nil {
			return fmt.Errorf("failed to check migration %s: %w", migration.Name, err)
		}

		if isApplied {
			continue
		}

		// Apply each migration in its own context with timeout
		if err := m.applyMigrationWithTimeout(ctx, migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Name, err)
		}
		appliedCount++
	}

	if appliedCount > 0 {
		fmt.Printf("✓ Applied %d migrations successfully\n", appliedCount)
	} else {
		fmt.Println("✓ All migrations are already applied")
	}

	return nil
}

// applyMigrationWithTimeout applies a single migration with timeout protection.
func (m *Migrator) applyMigrationWithTimeout(ctx context.Context, migration *validator.MigrationFile) error {
	// Create a new context for this migration with timeout
	migrationCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	return migration.Apply(migrationCtx)
}

// GetAppliedMigrations returns a list of all applied migration names.
// This is useful for debugging and verification purposes.
func (m *Migrator) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	// Ensure migrations table exists first
	if err := m.tracker.EnsureMigrationsTable(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure migrations table: %w", err)
	}
	return m.tracker.GetAppliedMigrations(ctx)
}

// GetPendingMigrations returns a list of migrations that haven't been applied yet.
func (m *Migrator) GetPendingMigrations(ctx context.Context) ([]*validator.MigrationFile, error) {
	// Ensure migrations table exists first
	if err := m.tracker.EnsureMigrationsTable(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure migrations table: %w", err)
	}

	allMigrations, err := m.validator.GetMigrationFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get migration files: %w", err)
	}

	return validator.FindNewMigrations(ctx, allMigrations)
}
