/*
Package migrator provides a production-ready database migration tool for PostgreSQL
with shadow database testing.

Migrator is designed for production use with a unique approach: it tests your
migrations on a temporary shadow database before applying them to production,
catching errors before they affect your production system.

# Core Features

  - 🧪 Shadow Database Testing: Tests migrations on a throwaway database first
  - 🔒 Transaction-Safe: All migrations run in transactions with automatic rollback
  - ✅ Validation: Ensures migration consistency between database and filesystem
  - 📊 PostgreSQL-Only: Built specifically for PostgreSQL, not a one-size-fits-all
  - ⚡ Context Support: Proper context handling with timeout support (5min default)
  - 🔄 Idempotent: Safe to run multiple times, skips already-applied migrations
  - 🧩 Modular Architecture: Clean internal package structure (tracker/validator/shadowdb)
  - 📝 Comprehensive Tests: 11 test scenarios covering edge cases

# Quick Start

Create a migrations directory with SQL files:

	migrations/
	├── 001_create_users.sql
	├── 002_create_posts.sql
	└── 003_add_indexes.sql

Basic usage:

	package main

	import (
		"context"
		"database/sql"
		"log"

		_ "github.com/lib/pq"
		"github.com/hasirciogluhq/migrator"
	)

	func main() {
		db, _ := sql.Open("postgres", "postgres://user:pass@localhost/mydb")
		defer db.Close()

		// Simple approach (uses env vars for shadow DB if available)
		m := migrator.New(db)

		if err := m.Migrate(context.Background()); err != nil {
			log.Fatal(err)
		}

		log.Println("✅ Migrations completed successfully")
	}

# Production-Recommended Configuration

For production, explicitly pass the database URL rather than relying on environment
variables. This makes configuration visible and testable:

	m := migrator.NewWithOptions(db, migrator.Options{
		MigrationsPath: "./migrations",
		DatabaseURL:    "postgres://user:pass@localhost:5432/mydb",
	})

	// Run with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := m.Migrate(ctx); err != nil {
		log.Fatal(err)
	}

# How Shadow Database Testing Works

This is what makes migrator unique. When you run Migrate():

1. Creates migrations tracking table (_go_migrations) if needed
2. Validates all previously applied migrations still exist in filesystem
3. Loads all migration files from the migrations directory
4. Identifies new (unapplied) migrations
5. Creates temporary shadow database (yourdb_gi_mig_shadow_db)
6. Applies ALL existing migrations to shadow database
7. Tests NEW migrations on shadow database
8. If shadow tests pass → applies new migrations to production
9. If shadow tests fail → aborts, alerts, cleans up shadow database
10. Cleans up shadow database after successful migration

This catches syntax errors, constraint violations, and type mismatches BEFORE
they touch your production database. Even with transaction rollback, testing
first prevents production error logs and potential downtime.

Example shadow database flow:

	Your Production DB: myapp_production
	Shadow Database:    myapp_production_gi_mig_shadow_db (temporary)

	1. Shadow DB created
	2. Existing migrations (001-005) applied to shadow
	3. New migrations (006-007) tested on shadow
	4. ✅ Tests pass → apply 006-007 to production
	5. Shadow DB dropped automatically

# Configuration Options

The Options struct provides flexible configuration:

	type Options struct {
		// Path to directory containing .sql migration files
		// Default: "./migrations" or MIGRATIONS_PATH env var
		MigrationsPath string

		// PostgreSQL connection string for shadow database operations
		// Falls back to DATABASE_URL env var if not provided
		// Shadow DB testing is skipped if neither is available
		DatabaseURL string

		// Set to true to skip shadow database testing entirely
		// Not recommended for production deployments
		SkipShadowDB bool
	}

Using NewWithOptions (recommended for production):

	m := migrator.NewWithOptions(db, migrator.Options{
		MigrationsPath: "./db/migrations",
		DatabaseURL:    os.Getenv("DATABASE_URL"), // Explicit configuration
	})

Using New (simple approach):

	// Relies on MIGRATIONS_PATH and DATABASE_URL env vars
	m := migrator.New(db)

Graceful Fallback: If DatabaseURL is not provided (neither in Options nor env var),
the migrator will skip shadow database testing and log a warning. Migrations will
still be applied to production, but without pre-testing.

# Migration Files

Migration files are plain SQL. They should:

  - Be named with numeric prefix for ordering (001_*.sql, 002_*.sql, etc.)
  - Contain valid PostgreSQL SQL statements
  - Be focused and atomic (one logical change per file)
  - Never be modified or deleted after being applied to production

Example migration file (001_create_users.sql):

	CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX idx_users_email ON users(email);

	-- You can include data migrations too
	INSERT INTO users (name, email) VALUES
		('System', 'system@example.com');

# Transaction Safety

Each migration file runs in its own transaction with READ COMMITTED isolation:

	BEGIN TRANSACTION;
	  -- Execute all SQL in migration file
	  -- Record migration in _go_migrations table
	COMMIT;

If ANY statement fails, the entire migration is rolled back and NOT recorded
as applied. The next time you run Migrate(), it will retry from the failed
migration.

# Checking Migration Status

Get applied migrations:

	applied, err := m.GetAppliedMigrations(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Applied: %v\n", applied)

Get pending migrations:

	pending, err := m.GetPendingMigrations(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Pending: %d migrations\n", len(pending))

# Internal Architecture

The package is organized into focused internal modules:

  - internal/tracker: Manages _go_migrations table and transactions
  - internal/validator: Validates migration files and filesystem consistency
  - internal/shadowdb: Creates, tests, and cleans up shadow databases

This separation ensures:
  - Single responsibility per module
  - Easy testing (11 comprehensive test scenarios)
  - Clear error messages with proper context
  - Maintainable codebase

# Known Limitations

PostgreSQL enum operations require careful ordering. This is NOT a bug in migrator,
but a PostgreSQL constraint that affects ALL migration tools:

	-- ❌ WRONG ORDER - Will fail in production
	ALTER TYPE status_enum DROP VALUE 'old_value';
	UPDATE users SET status = 'new_value' WHERE status = 'old_value';

	-- ✅ CORRECT ORDER
	UPDATE users SET status = 'new_value' WHERE status = 'old_value';
	ALTER TYPE status_enum DROP VALUE 'old_value';

Shadow database testing might pass the wrong order (no data in shadow DB), but
production will fail (has rows with 'old_value'). This is true for Prisma,
golang-migrate, and all migration tools. The solution: know your SQL, especially
with enum operations.

# Performance Considerations

Shadow database creation/destruction adds overhead (typically 1-3 seconds). This is
negligible for production deployments but might matter in CI pipelines that run
hundreds of times daily.

To skip shadow DB testing in specific environments:

	m := migrator.NewWithOptions(db, migrator.Options{
		MigrationsPath: "./migrations",
		DatabaseURL:    "", // Empty = skips shadow DB testing
	})

Not recommended for production deployments.

# Testing

The package includes 11 comprehensive test scenarios:

  - Basic migration flow
  - Idempotent migrations (running multiple times safely)
  - Transaction rollback on failure
  - Incremental migrations
  - Missing migration file detection
  - Complex multi-statement migrations
  - Concurrent migration attempts
  - Context cancellation handling
  - Shadow database testing with various scenarios
  - Explicit configuration via Options
  - Graceful fallback when DATABASE_URL not provided

All tests use real PostgreSQL databases (no mocks). If tests pass, it works in production.

Run tests:

	make test-docker  # Starts PostgreSQL in Docker automatically
	make test         # Uses existing PostgreSQL connection
	make test-coverage # Generates coverage report

# Best Practices

Migration File Naming:
  - Use numeric prefix: 001_initial_schema.sql, 002_add_users.sql
  - Keep names descriptive
  - Never reorder or renumber after applying to production

Migration Content:
  - Keep migrations small and focused
  - Test locally before deploying
  - Include both schema AND data changes if needed
  - Add indexes in the same migration as table creation
  - Be explicit with column types and constraints

Production Deployment:
  - Always take database backup before migrations
  - Use explicit DatabaseURL in Options (don't rely on env vars)
  - Monitor logs during migration execution
  - Have rollback plan ready (though transaction safety helps)
  - Test migrations on staging environment first

Never Do:
  - Modify migrations after applying to production
  - Delete migration files that have been applied
  - Skip shadow database testing in production
  - Run migrations without proper timeout context

# Automatic Releases

This package uses fully automated CI/CD. Every push to main:

  - Runs all 11 test scenarios with PostgreSQL
  - Calculates version from commit message ([*] = patch, [+] = minor, [MAJOR] = major)
  - Creates git tag and GitHub release automatically
  - Notifies pkg.go.dev for documentation updates

See RELEASE.md for details on commit message format.

# Examples and Documentation

For complete examples, see:
  - examples/basic/main.go - Full working example
  - examples/basic/migrations/ - Sample migration files
  - README.md - Comprehensive documentation
  - RELEASE.md - Automated release process

For API documentation:
  - https://pkg.go.dev/github.com/hasirciogluhq/migrator

For source code and issues:
  - https://github.com/hasirciogluhq/migrator

# License

MIT License - see LICENSE file for details.
*/
package migrator
