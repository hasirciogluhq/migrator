/*
Package migrator provides a production-ready database migration tool for PostgreSQL.

Migrator is designed for production use with the following key features:

  - Transaction-safe migrations with automatic rollback on failure
  - Shadow database testing to catch errors before production deployment
  - Migration file validation to ensure consistency
  - Detailed logging and error reporting
  - Context support with proper timeout handling
  - Clean, modular architecture

# Quick Start

Create a migrations directory and add your SQL migration files:

	migrations/
	├── 001_create_users.sql
	├── 002_create_posts.sql
	└── 003_add_indexes.sql

Then in your Go code:

	package main

	import (
		"context"
		"database/sql"
		"log"

		_ "github.com/lib/pq"
		"github.com/hasirciogluhq/migrator"
	)

	func main() {
		db, _ := sql.Open("postgres", "postgres://user:pass@localhost/db")
		m := migrator.New(db)

		if err := m.Migrate(context.Background()); err != nil {
			log.Fatal(err)
		}
	}

# How It Works

The migration process follows these steps:

1. Create migrations tracking table (_go_migrations) if it doesn't exist
2. Validate that all applied migrations still exist in the filesystem
3. Load all migration files from the migrations directory
4. Test new migrations on a temporary shadow database
5. Apply pending migrations to the production database in transactions
6. Clean up the shadow database

Each migration runs in its own transaction with automatic rollback on failure.
The shadow database is a temporary copy used for testing before applying to production.

# Configuration

Configure the migrator using environment variables:

  - DATABASE_URL: PostgreSQL connection string (optional, fallback for shadow DB)
  - MIGRATIONS_PATH: Path to migrations directory (default: ./migrations)

Or use custom options:

	m := migrator.NewWithOptions(db, migrator.Options{
		MigrationsPath: "./db/migrations",
		DatabaseURL:    "postgres://user:pass@localhost/db", // For shadow DB testing
	})

For production use, it's recommended to pass DatabaseURL in Options rather than
relying on environment variables.

# Architecture

The package is organized into focused internal modules:

  - internal/tracker: Migration tracking and database operations
  - internal/validator: Migration file validation and loading
  - internal/shadowdb: Shadow database creation and testing

This modular design ensures clean separation of concerns and makes the code
easy to understand, test, and maintain.

# Safety Features

Transaction Safety: Each migration runs in a transaction. If any SQL statement
fails, the entire migration is rolled back and not recorded as applied.

Shadow Database Testing: Before applying to production, new migrations are tested
on a temporary shadow database. This catches syntax errors and schema conflicts
before they affect production.

Validation: The migrator validates that all previously applied migrations still
exist in the filesystem. This prevents accidental deletion of migration files.

Idempotency: Running migrations multiple times is safe. Already-applied migrations
are skipped automatically.

# Best Practices

Migration files should be named with a numeric prefix for ordering:
001_initial_schema.sql, 002_add_users.sql, etc.

Keep migrations focused and small. It's better to have many small migrations
than few large ones.

Never modify or delete migrations that have been applied to production.
Instead, create new migrations to make changes.

Always test migrations locally before deploying to production.

Take database backups before running migrations in production environments.

For more information and examples, visit:
https://github.com/hasirciogluhq/migrator
*/
package migrator
