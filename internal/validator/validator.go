// Package validator handles migration file validation.
package validator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hasirciogluhq/migrator/internal/tracker"
)

// Validator validates migration files and their consistency.
type Validator struct {
	tracker        *tracker.Tracker
	migrationsPath string
}

// New creates a new Validator instance.
func New(t *tracker.Tracker, migrationsPath string) *Validator {
	return &Validator{
		tracker:        t,
		migrationsPath: migrationsPath,
	}
}

// ValidateExistingMigrations checks if all applied migrations still exist in filesystem.
func (v *Validator) ValidateExistingMigrations(ctx context.Context) error {
	fmt.Println("🔍 Validating existing migrations...")

	// Get all applied migrations from database
	appliedMigrations, err := v.tracker.GetAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Get all migration files from filesystem
	files, err := os.ReadDir(v.migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Create a map of filesystem files for quick lookup
	fsFiles := make(map[string]bool)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sql") {
			fsFiles[file.Name()] = true
		}
	}

	// Check if all applied migrations exist in filesystem
	var missingMigrations []string
	for _, appliedMigration := range appliedMigrations {
		if !fsFiles[appliedMigration] {
			missingMigrations = append(missingMigrations, appliedMigration)
		}
	}

	if len(missingMigrations) > 0 {
		return fmt.Errorf("critical: %d applied migrations are missing from filesystem: %v",
			len(missingMigrations), missingMigrations)
	}

	fmt.Printf("✓ All %d applied migrations validated successfully\n", len(appliedMigrations))
	return nil
}

// GetMigrationFiles reads and parses all migration files from the migrations directory.
func (v *Validator) GetMigrationFiles(ctx context.Context) ([]*MigrationFile, error) {
	files, err := os.ReadDir(v.migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrationFiles []*MigrationFile

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		migrationFile, err := v.createMigrationFile(ctx, file)
		if err != nil {
			return nil, fmt.Errorf("failed to create migration file for %s: %w", file.Name(), err)
		}

		migrationFiles = append(migrationFiles, migrationFile)
	}

	return migrationFiles, nil
}

// createMigrationFile creates a MigrationFile struct for a given file.
func (v *Validator) createMigrationFile(ctx context.Context, file os.DirEntry) (*MigrationFile, error) {
	filePath := filepath.Join(v.migrationsPath, file.Name())
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &MigrationFile{
		Name:    file.Name(),
		Content: string(content),
		tracker: v.tracker,
	}, nil
}

// MigrationFile represents a single migration file.
type MigrationFile struct {
	Name    string
	Content string
	tracker *tracker.Tracker
}

// IsApplied checks if this migration has been applied to the database.
func (m *MigrationFile) IsApplied(ctx context.Context) (bool, error) {
	return m.tracker.IsApplied(ctx, m.Name)
}

// Apply applies this migration to the database.
func (m *MigrationFile) Apply(ctx context.Context) error {
	return m.tracker.ApplyMigration(ctx, m.Name, m.Content)
}

// FindNewMigrations identifies which migrations haven't been applied yet.
func FindNewMigrations(ctx context.Context, allMigrations []*MigrationFile) ([]*MigrationFile, error) {
	var newMigrations []*MigrationFile

	for _, migration := range allMigrations {
		isApplied, err := migration.IsApplied(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to check migration %s: %w", migration.Name, err)
		}

		if !isApplied {
			newMigrations = append(newMigrations, migration)
		}
	}

	return newMigrations, nil
}
