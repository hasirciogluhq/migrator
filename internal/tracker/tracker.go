// Package tracker handles migration tracking and database operations.
package tracker

import (
	"context"
	"database/sql"
	"fmt"
)

const (
	// MigrationsTable is the name of the table that tracks applied migrations
	MigrationsTable = "_go_migrations"
)

// Tracker manages migration tracking in the database.
type Tracker struct {
	db *sql.DB
}

// New creates a new Tracker instance.
func New(db *sql.DB) *Tracker {
	return &Tracker{db: db}
}

// EnsureMigrationsTable creates the migrations tracking table if it doesn't exist.
func (t *Tracker) EnsureMigrationsTable(ctx context.Context) error {
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, MigrationsTable)

	if _, err := t.db.ExecContext(ctx, createTableSQL); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	return nil
}

// IsApplied checks if a migration has been applied.
func (t *Tracker) IsApplied(ctx context.Context, migrationName string) (bool, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE name = $1", MigrationsTable)

	var count int
	err := t.db.QueryRowContext(ctx, query, migrationName).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check migration status: %w", err)
	}

	return count > 0, nil
}

// Record records a migration as applied.
func (t *Tracker) Record(ctx context.Context, migrationName string) error {
	query := fmt.Sprintf("INSERT INTO %s (name) VALUES ($1)", MigrationsTable)

	if _, err := t.db.ExecContext(ctx, query, migrationName); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return nil
}

// GetAppliedMigrations retrieves all applied migration names.
func (t *Tracker) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	query := fmt.Sprintf("SELECT name FROM %s ORDER BY applied_at", MigrationsTable)

	rows, err := t.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}
	defer rows.Close()

	var migrations []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan migration name: %w", err)
		}
		migrations = append(migrations, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating migrations: %w", err)
	}

	return migrations, nil
}

// ApplyMigration applies a single migration within a transaction.
func (t *Tracker) ApplyMigration(ctx context.Context, migrationName, content string) error {
	// Start transaction with isolation level
	tx, err := t.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Track if we need to rollback
	shouldRollback := true
	defer func() {
		if shouldRollback {
			if rbErr := tx.Rollback(); rbErr != nil {
				fmt.Printf("⚠️  Warning: Failed to rollback transaction for %s: %v\n", migrationName, rbErr)
			}
		}
	}()

	// Apply the migration SQL
	if _, err := tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record the migration in tracking table
	recordQuery := fmt.Sprintf("INSERT INTO %s (name) VALUES ($1)", MigrationsTable)
	if _, err := tx.ExecContext(ctx, recordQuery, migrationName); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	// Mark that we don't need to rollback since commit succeeded
	shouldRollback = false

	fmt.Printf("✓ Applied migration (atomic): %s\n", migrationName)
	return nil
}
