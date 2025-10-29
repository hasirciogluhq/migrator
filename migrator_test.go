package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelper provides utility functions for testing
type TestHelper struct {
	db            *sql.DB
	migrationsDir string
	cleanupFuncs  []func()
	originalDBURL string
}

// setupTestDB creates an in-memory test database
func setupTestDB(t *testing.T) *TestHelper {
	t.Helper()

	// Get PostgreSQL connection URL from environment
	postgresURL := os.Getenv("TEST_DATABASE_URL")
	if postgresURL == "" {
		postgresURL = os.Getenv("DATABASE_URL")
	}
	if postgresURL == "" {
		t.Skip("Skipping test: DATABASE_URL or TEST_DATABASE_URL not set")
	}

	// Save original DATABASE_URL
	originalDBURL := os.Getenv("DATABASE_URL")

	// Connect to postgres database to create test database
	postgresDB, err := sql.Open("postgres", postgresURL)
	if err != nil {
		t.Skipf("Skipping test: cannot connect to PostgreSQL: %v", err)
	}

	// Verify connection
	if err := postgresDB.Ping(); err != nil {
		postgresDB.Close()
		t.Skipf("Skipping test: cannot ping PostgreSQL: %v", err)
	}

	// Create unique test database name
	testDBName := fmt.Sprintf("migrator_test_%d", time.Now().UnixNano())

	// Drop if exists and create fresh database
	_, err = postgresDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	require.NoError(t, err)

	_, err = postgresDB.Exec(fmt.Sprintf("CREATE DATABASE %s", testDBName))
	require.NoError(t, err)
	postgresDB.Close()

	// Build test database URL by replacing database name
	testDBURL := replaceDatabaseInURL(postgresURL, testDBName)
	db, err := sql.Open("postgres", testDBURL)
	require.NoError(t, err)

	// Verify connection
	err = db.Ping()
	require.NoError(t, err)

	// Set DATABASE_URL for migrator
	os.Setenv("DATABASE_URL", testDBURL)

	// Create temporary migrations directory
	migrationsDir := filepath.Join(t.TempDir(), "migrations")
	err = os.MkdirAll(migrationsDir, 0755)
	require.NoError(t, err)

	helper := &TestHelper{
		db:            db,
		migrationsDir: migrationsDir,
		originalDBURL: originalDBURL,
		cleanupFuncs: []func(){
			func() {
				db.Close()

				// Restore original DATABASE_URL
				if originalDBURL != "" {
					os.Setenv("DATABASE_URL", originalDBURL)
				} else {
					os.Unsetenv("DATABASE_URL")
				}

				// Drop test database
				cleanupDB, err := sql.Open("postgres", postgresURL)
				if err == nil {
					_, _ = cleanupDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
					cleanupDB.Close()
				}
			},
		},
	}

	return helper
}

// replaceDatabaseInURL replaces the database name in a PostgreSQL URL
func replaceDatabaseInURL(url, newDBName string) string {
	// Find the last / before ? or end of string
	var beforeDB, afterDB string

	if idx := lastIndexBefore(url, "/", "?"); idx != -1 {
		beforeDB = url[:idx+1]
		rest := url[idx+1:]

		if qIdx := findChar(rest, '?'); qIdx != -1 {
			afterDB = rest[qIdx:]
		}
	}

	return beforeDB + newDBName + afterDB
}

func lastIndexBefore(s, needle, before string) int {
	beforeIdx := len(s)
	if before != "" {
		if idx := findString(s, before); idx != -1 {
			beforeIdx = idx
		}
	}

	for i := beforeIdx - 1; i >= 0; i-- {
		if i+len(needle) <= beforeIdx && s[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func findString(s, needle string) int {
	for i := 0; i <= len(s)-len(needle); i++ {
		if s[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func findChar(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func (h *TestHelper) cleanup() {
	for i := len(h.cleanupFuncs) - 1; i >= 0; i-- {
		h.cleanupFuncs[i]()
	}
}

func (h *TestHelper) createMigrationFile(t *testing.T, name, content string) {
	t.Helper()
	filePath := filepath.Join(h.migrationsDir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)
}

func (h *TestHelper) tableExists(t *testing.T, tableName string) bool {
	t.Helper()
	var exists bool
	query := `SELECT EXISTS (
		SELECT FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_name = $1
	)`
	err := h.db.QueryRow(query, tableName).Scan(&exists)
	require.NoError(t, err)
	return exists
}

func (h *TestHelper) getAppliedMigrations(t *testing.T) []string {
	t.Helper()
	rows, err := h.db.Query("SELECT name FROM _go_migrations ORDER BY applied_at")
	require.NoError(t, err)
	defer rows.Close()

	var migrations []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		migrations = append(migrations, name)
	}
	return migrations
}

// Tests

func TestMigrator_BasicMigration(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	// Create test migrations
	helper.createMigrationFile(t, "001_create_users.sql", `
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL
		);
	`)

	helper.createMigrationFile(t, "002_create_posts.sql", `
		CREATE TABLE posts (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			title VARCHAR(255) NOT NULL,
			content TEXT
		);
	`)

	// Set migrations path
	os.Setenv("MIGRATIONS_PATH", helper.migrationsDir)
	defer os.Unsetenv("MIGRATIONS_PATH")

	// Run migrations
	m := New(helper.db)
	err := m.Migrate(context.Background())
	require.NoError(t, err)

	// Verify tables were created
	assert.True(t, helper.tableExists(t, "users"))
	assert.True(t, helper.tableExists(t, "posts"))
	assert.True(t, helper.tableExists(t, "_go_migrations"))

	// Verify migrations were tracked
	applied := helper.getAppliedMigrations(t)
	assert.Equal(t, []string{"001_create_users.sql", "002_create_posts.sql"}, applied)
}

func TestMigrator_IdempotentMigrations(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	helper.createMigrationFile(t, "001_create_users.sql", `
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		);
	`)

	os.Setenv("MIGRATIONS_PATH", helper.migrationsDir)
	defer os.Unsetenv("MIGRATIONS_PATH")

	// Run migrations first time
	m := New(helper.db)
	err := m.Migrate(context.Background())
	require.NoError(t, err)

	// Run migrations second time - should be idempotent
	err = m.Migrate(context.Background())
	require.NoError(t, err)

	// Verify migration only recorded once
	applied := helper.getAppliedMigrations(t)
	assert.Equal(t, []string{"001_create_users.sql"}, applied)
}

func TestMigrator_FailedMigrationRollback(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	// Create valid migration
	helper.createMigrationFile(t, "001_create_users.sql", `
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		);
	`)

	// Create invalid migration (syntax error)
	helper.createMigrationFile(t, "002_invalid.sql", `
		CREATE TABLE invalid (
			id SERIAL PRIMARY KEY
			name VARCHAR(255) -- Missing comma
		);
	`)

	os.Setenv("MIGRATIONS_PATH", helper.migrationsDir)
	defer os.Unsetenv("MIGRATIONS_PATH")

	// Run migrations - should fail on shadow database test
	m := New(helper.db)
	err := m.Migrate(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shadow database test failed")

	// Verify NO migrations were applied (shadow DB caught the error)
	// This is the correct behavior - shadow DB prevents bad migrations from reaching production
	applied := helper.getAppliedMigrations(t)
	assert.Empty(t, applied, "No migrations should be applied when shadow DB test fails")

	// Verify users table was not created (migration was blocked)
	assert.False(t, helper.tableExists(t, "users"))

	// Verify invalid table was not created
	assert.False(t, helper.tableExists(t, "invalid"))
}

func TestMigrator_IncrementalMigrations(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	helper.createMigrationFile(t, "001_create_users.sql", `
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		);
	`)

	os.Setenv("MIGRATIONS_PATH", helper.migrationsDir)
	defer os.Unsetenv("MIGRATIONS_PATH")

	// Run first migration
	m := New(helper.db)
	err := m.Migrate(context.Background())
	require.NoError(t, err)

	// Add new migration
	helper.createMigrationFile(t, "002_create_posts.sql", `
		CREATE TABLE posts (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id)
		);
	`)

	// Run migrations again - should only apply new one
	err = m.Migrate(context.Background())
	require.NoError(t, err)

	// Verify both migrations applied
	applied := helper.getAppliedMigrations(t)
	assert.Equal(t, []string{"001_create_users.sql", "002_create_posts.sql"}, applied)
}

func TestMigrator_MissingMigrationFile(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	helper.createMigrationFile(t, "001_create_users.sql", `
		CREATE TABLE users (id SERIAL PRIMARY KEY);
	`)

	os.Setenv("MIGRATIONS_PATH", helper.migrationsDir)
	defer os.Unsetenv("MIGRATIONS_PATH")

	// Apply migration
	m := New(helper.db)
	err := m.Migrate(context.Background())
	require.NoError(t, err)

	// Delete migration file
	os.Remove(filepath.Join(helper.migrationsDir, "001_create_users.sql"))

	// Run migrations again - should fail validation
	err = m.Migrate(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing from filesystem")
}

func TestMigrator_EmptyMigrationsDirectory(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	os.Setenv("MIGRATIONS_PATH", helper.migrationsDir)
	defer os.Unsetenv("MIGRATIONS_PATH")

	// Run with no migrations
	m := New(helper.db)
	err := m.Migrate(context.Background())
	require.NoError(t, err)

	// Verify tracking table was created
	assert.True(t, helper.tableExists(t, "_go_migrations"))
}

func TestMigrator_GetAppliedMigrations(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	helper.createMigrationFile(t, "001_create_users.sql", `
		CREATE TABLE users (id SERIAL PRIMARY KEY);
	`)

	os.Setenv("MIGRATIONS_PATH", helper.migrationsDir)
	defer os.Unsetenv("MIGRATIONS_PATH")

	m := New(helper.db)

	// Initially should be empty (GetAppliedMigrations will ensure table exists)
	applied, err := m.GetAppliedMigrations(context.Background())
	require.NoError(t, err)
	assert.Empty(t, applied)

	// Apply migrations
	err = m.Migrate(context.Background())
	require.NoError(t, err)

	// Should now have one migration
	applied, err = m.GetAppliedMigrations(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"001_create_users.sql"}, applied)
}

func TestMigrator_GetPendingMigrations(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	helper.createMigrationFile(t, "001_create_users.sql", `
		CREATE TABLE users (id SERIAL PRIMARY KEY);
	`)

	helper.createMigrationFile(t, "002_create_posts.sql", `
		CREATE TABLE posts (id SERIAL PRIMARY KEY);
	`)

	os.Setenv("MIGRATIONS_PATH", helper.migrationsDir)
	defer os.Unsetenv("MIGRATIONS_PATH")

	m := New(helper.db)

	// Initially all should be pending (GetPendingMigrations will ensure table exists)
	pending, err := m.GetPendingMigrations(context.Background())
	require.NoError(t, err)
	assert.Len(t, pending, 2)

	// Apply first migration manually
	err = m.Migrate(context.Background())
	require.NoError(t, err)

	// Should have no pending migrations
	pending, err = m.GetPendingMigrations(context.Background())
	require.NoError(t, err)
	assert.Empty(t, pending)
}

func TestMigrator_ComplexMigration(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	// Create a complex migration with multiple statements
	helper.createMigrationFile(t, "001_complex.sql", `
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX idx_users_email ON users(email);

		CREATE TABLE posts (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			content TEXT,
			published BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX idx_posts_user_id ON posts(user_id);
		CREATE INDEX idx_posts_published ON posts(published);

		INSERT INTO users (name, email) VALUES 
			('Test User 1', 'test1@example.com'),
			('Test User 2', 'test2@example.com');
	`)

	os.Setenv("MIGRATIONS_PATH", helper.migrationsDir)
	defer os.Unsetenv("MIGRATIONS_PATH")

	// Run migration
	m := New(helper.db)
	err := m.Migrate(context.Background())
	require.NoError(t, err)

	// Verify tables and data
	assert.True(t, helper.tableExists(t, "users"))
	assert.True(t, helper.tableExists(t, "posts"))

	var count int
	err = helper.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestMigrator_ContextCancellation(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	helper.createMigrationFile(t, "001_create_users.sql", `
		CREATE TABLE users (id SERIAL PRIMARY KEY);
	`)

	os.Setenv("MIGRATIONS_PATH", helper.migrationsDir)
	defer os.Unsetenv("MIGRATIONS_PATH")

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m := New(helper.db)
	err := m.Migrate(ctx)

	// Should handle cancellation gracefully
	// Note: Depending on timing, this might succeed or fail with context error
	if err != nil {
		assert.Contains(t, err.Error(), "context")
	}
}

func TestMigrator_ConcurrentMigrations(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	helper.createMigrationFile(t, "001_create_users.sql", `
		CREATE TABLE users (id SERIAL PRIMARY KEY);
	`)

	os.Setenv("MIGRATIONS_PATH", helper.migrationsDir)
	defer os.Unsetenv("MIGRATIONS_PATH")

	// Try to run migrations concurrently
	done := make(chan error, 2)

	for i := 0; i < 2; i++ {
		go func() {
			m := New(helper.db)
			done <- m.Migrate(context.Background())
		}()
	}

	// Wait for both to complete
	err1 := <-done
	err2 := <-done

	// At least one should succeed
	// The other might fail with unique constraint violation or succeed due to idempotency
	assert.True(t, err1 == nil || err2 == nil)

	// Verify migration only applied once
	applied := helper.getAppliedMigrations(t)
	assert.Equal(t, []string{"001_create_users.sql"}, applied)
}

func TestMigrator_WithOptions_ExplicitDatabaseURL(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	helper.createMigrationFile(t, "001_create_users.sql", `
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		);
	`)

	// Get database URL from environment (set by setupTestDB)
	databaseURL := os.Getenv("DATABASE_URL")

	// Create migrator with explicit options (no environment variables needed)
	m := NewWithOptions(helper.db, Options{
		MigrationsPath: helper.migrationsDir,
		DatabaseURL:    databaseURL,
	})

	// Run migrations
	err := m.Migrate(context.Background())
	require.NoError(t, err)

	// Verify table was created
	assert.True(t, helper.tableExists(t, "users"))

	// Verify migration was tracked
	applied := helper.getAppliedMigrations(t)
	assert.Equal(t, []string{"001_create_users.sql"}, applied)
}

func TestMigrator_WithOptions_NoDatabaseURL_SkipsShadowDB(t *testing.T) {
	helper := setupTestDB(t)
	defer helper.cleanup()

	helper.createMigrationFile(t, "001_create_users.sql", `
		CREATE TABLE users (id SERIAL PRIMARY KEY);
	`)

	// Temporarily unset DATABASE_URL to test fallback behavior
	originalURL := os.Getenv("DATABASE_URL")
	os.Unsetenv("DATABASE_URL")
	defer func() {
		if originalURL != "" {
			os.Setenv("DATABASE_URL", originalURL)
		}
	}()

	// Create migrator without database URL
	m := NewWithOptions(helper.db, Options{
		MigrationsPath: helper.migrationsDir,
		DatabaseURL:    "", // No database URL provided
	})

	// Run migrations - should skip shadow DB testing but still work
	err := m.Migrate(context.Background())
	require.NoError(t, err)

	// Verify table was created (migration applied directly without shadow DB test)
	assert.True(t, helper.tableExists(t, "users"))
}
