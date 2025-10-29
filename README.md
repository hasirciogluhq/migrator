# Migrator

[![Go Report Card](https://goreportcard.com/badge/github.com/hasirciogluhq/migrator)](https://goreportcard.com/report/github.com/hasirciogluhq/migrator)
[![GoDoc](https://godoc.org/github.com/hasirciogluhq/migrator?status.svg)](https://godoc.org/github.com/hasirciogluhq/migrator)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Production-ready database migration tool for Go with shadow database testing and transaction safety.

## Features

- 🔒 **Transaction-Safe**: All migrations run in transactions with automatic rollback on failure
- 🧪 **Shadow Database Testing**: Test migrations on a shadow database before applying to production
- ✅ **Validation**: Ensures migration consistency between database and filesystem
- 📊 **PostgreSQL Support**: Built specifically for PostgreSQL with proper connection handling
- 🎯 **Simple API**: Clean, easy-to-use interface
- 📝 **Detailed Logging**: Clear output showing migration progress
- 🔄 **Idempotent**: Safe to run multiple times
- ⚡ **Context Support**: Proper context handling with timeout support
- 🧩 **Modular Architecture**: Clean internal package structure

## Installation

```bash
# Latest stable version (recommended)
go get github.com/hasirciogluhq/migrator@latest

# Specific version
go get github.com/hasirciogluhq/migrator@v1.0.0
```

**Version Policy:**
- 🤖 **Fully automated releases** - Every push to main triggers CI/CD
- ✅ **Test-protected** - Only releases if all tests pass
- 📦 **Semantic versioning** - Auto-calculated from commit messages
- 🔄 **Always safe** - Failed tests = no release, use previous version
- 📖 See [RELEASE.md](RELEASE.md) for details

## 🚀 Automatic Releases

This package uses **fully automated CI/CD**. Just push to main:

```bash
# Bug fix (v1.0.0 → v1.0.1)
git commit -m "[*] Fixed migration bug"
git push origin main

# New feature (v1.0.0 → v1.1.0)  
git commit -m "[+] Added rollback support"
git push origin main

# Breaking change (v1.0.0 → v2.0.0)
git commit -m "[MAJOR] Changed API signature"
git push origin main
```

**✅ Tests pass** → Automatic release  
**❌ Tests fail** → No release, safe!

See [RELEASE.md](RELEASE.md) for commit message format.

## Quick Start

### 1. Create your migrations directory

```bash
mkdir migrations
```

### 2. Create migration files

Create SQL files in your migrations directory. Files are executed in alphabetical order.

**migrations/001_create_users.sql:**
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
```

**migrations/002_create_posts.sql:**
```sql
CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT,
    published BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_posts_user_id ON posts(user_id);
```

### 3. Run migrations in your application

```go
package main

import (
    "context"
    "database/sql"
    "log"
    
    _ "github.com/lib/pq"
    "github.com/hasirciogluhq/migrator"
)

func main() {
    // Connect to your database
    db, err := sql.Open("postgres", 
        "postgres://user:password@localhost:5432/mydb?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create migrator
    m := migrator.New(db)

    // Run migrations
    if err := m.Migrate(context.Background()); err != nil {
        log.Fatal(err)
    }

    log.Println("Migrations completed successfully!")
}
```

## Configuration

### Environment Variables

- `DATABASE_URL`: PostgreSQL connection string (optional, fallback for shadow database operations)
- `MIGRATIONS_PATH`: Path to migrations directory (default: `./migrations`)

### Custom Configuration

```go
m := migrator.NewWithOptions(db, migrator.Options{
    MigrationsPath: "./db/migrations",
    DatabaseURL:    "postgres://user:pass@localhost:5432/mydb", // Optional, for shadow DB testing
    SkipShadowDB:   false, // Set to true to skip shadow database testing
})
```

**Shadow Database Testing:**
Shadow database testing requires a database URL to create temporary test databases. You can provide it in two ways:
1. **Recommended (Production):** Pass it in `Options.DatabaseURL`
2. **Fallback:** Set `DATABASE_URL` environment variable

If neither is provided, shadow database testing will be skipped with a warning.

## How It Works

The migrator follows a robust, multi-step process:

1. **Ensure Tracking Table**: Creates `_go_migrations` table if it doesn't exist
2. **Validate Existing Migrations**: Verifies all applied migrations still exist in filesystem
3. **Load Migration Files**: Reads all `.sql` files from migrations directory
4. **Shadow Database Testing**: 
   - Creates a temporary shadow database
   - Applies existing migrations to shadow database
   - Tests new migrations on shadow database
   - Drops shadow database after testing
5. **Apply to Production**: Applies pending migrations to production database
6. **Cleanup**: Ensures shadow database is removed

### Transaction Safety

Each migration runs in its own transaction:
- ✅ If successful: Changes are committed and migration is recorded
- ❌ If failed: Changes are rolled back and migration is not recorded

### Shadow Database Testing

Before applying to production, new migrations are tested on a shadow database:

```
your_database           → Production database (untouched during testing)
your_database_gi_mig_shadow_db → Temporary shadow database (created, tested, dropped)
```

This ensures:
- Syntax errors are caught before production
- Migrations are compatible with existing schema
- No surprises in production deployment

## API Reference

### Core Functions

#### `New(db *sql.DB) *Migrator`

Creates a new migrator with default options.

```go
m := migrator.New(db)
```

#### `NewWithOptions(db *sql.DB, opts Options) *Migrator`

Creates a new migrator with custom options.

```go
m := migrator.NewWithOptions(db, migrator.Options{
    MigrationsPath: "./custom/path",
    SkipShadowDB:   false,
})
```

#### `Migrate(ctx context.Context) error`

Runs the complete migration process.

```go
if err := m.Migrate(context.Background()); err != nil {
    log.Fatal(err)
}
```

#### `GetAppliedMigrations(ctx context.Context) ([]string, error)`

Returns a list of all applied migration names.

```go
applied, err := m.GetAppliedMigrations(context.Background())
if err != nil {
    log.Fatal(err)
}
fmt.Println("Applied migrations:", applied)
```

#### `GetPendingMigrations(ctx context.Context) ([]*validator.MigrationFile, error)`

Returns a list of migrations that haven't been applied yet.

```go
pending, err := m.GetPendingMigrations(context.Background())
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Pending migrations: %d\n", len(pending))
```

## Examples

### Basic Usage

See [examples/basic/main.go](examples/basic/main.go) for a complete example.

### Custom Migrations Path

```go
os.Setenv("MIGRATIONS_PATH", "./db/migrations")
m := migrator.New(db)
m.Migrate(context.Background())
```

### With Context Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

if err := m.Migrate(ctx); err != nil {
    log.Fatal(err)
}
```

### Check Migration Status

```go
m := migrator.New(db)

// Get applied migrations
applied, err := m.GetAppliedMigrations(context.Background())
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Applied: %v\n", applied)

// Get pending migrations
pending, err := m.GetPendingMigrations(context.Background())
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Pending: %d migrations\n", len(pending))
```

## Best Practices

### Migration File Naming

Use a consistent naming convention:
```
001_create_users.sql
002_create_posts.sql
003_add_user_avatar.sql
```

### Migration Content

1. **Be Explicit**: Always specify column types, constraints, and defaults
2. **Use Transactions**: Each file runs in a transaction, so keep related changes together
3. **Add Indexes**: Create indexes in the same migration as the table
4. **Handle Data**: You can include INSERT/UPDATE statements for seed data

### Production Deployment

1. **Test Locally**: Always test migrations locally first
2. **Backup**: Take database backups before running migrations in production
3. **Monitor**: Watch logs during migration execution
4. **Rollback Plan**: Have a rollback strategy for each migration

### Do's and Don'ts

✅ **Do:**
- Keep migrations small and focused
- Test migrations thoroughly
- Use descriptive file names
- Version control your migrations
- Run migrations during deployment automation

❌ **Don't:**
- Modify already-applied migrations
- Delete migration files that have been applied
- Include DROP statements without careful consideration
- Skip shadow database testing in production

## Testing

The package includes comprehensive tests with 11 test scenarios covering:
- ✅ Basic migration flow
- ✅ Idempotent migrations  
- ✅ Transaction rollback on failure
- ✅ Incremental migrations
- ✅ Missing file detection
- ✅ Complex multi-statement migrations
- ✅ Concurrent migrations
- ✅ Context cancellation

### Quick Testing with Make

The easiest way to run tests:

```bash
# Run tests with Docker (automatically starts/stops PostgreSQL)
make test-docker

# Run tests with existing PostgreSQL
make test

# Run tests with coverage report
make test-coverage

# See all available commands
make help
```

### Local Testing

**Option 1: Using Docker (Recommended)**

```bash
# Start PostgreSQL
docker run -d --name migrator-test \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_DB=postgres \
  -p 5432:5432 \
  postgres

# Wait for PostgreSQL to be ready
sleep 3

# Set connection string and run tests
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
go test -v ./...

# Cleanup
docker stop migrator-test && docker rm migrator-test
```

**Option 2: Using Existing PostgreSQL**

```bash
# Set your PostgreSQL connection string
export DATABASE_URL="postgres://YOUR_USER:YOUR_PASSWORD@localhost:5432/postgres?sslmode=disable"

# Run tests
go test -v ./...
```

**Run with Coverage:**

```bash
go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
go tool cover -html=coverage.txt
```

### CI/CD Testing

Tests automatically run on GitHub Actions on every push/PR. The workflow:
1. Spins up PostgreSQL 15 service
2. Runs `go vet` for code quality
3. Executes all tests with race detector
4. Uploads coverage to Codecov

See [.github/workflows/test.yml](.github/workflows/test.yml) for details.

## Architecture

```
migrator/
├── migrator.go              # Public API
├── internal/
│   ├── tracker/             # Migration tracking & database operations
│   │   └── tracker.go
│   ├── validator/           # Migration validation & file handling
│   │   └── validator.go
│   └── shadowdb/           # Shadow database management
│       └── shadowdb.go
├── migrator_test.go        # Comprehensive test suite
└── examples/               # Usage examples
```

## Troubleshooting

### "DATABASE_URL environment variable not set" or Shadow DB Warning

If you see a warning about DATABASE_URL not being provided, you have two options:

**Option 1 (Recommended):** Pass the database URL explicitly in options:
```go
m := migrator.NewWithOptions(db, migrator.Options{
    DatabaseURL: "postgres://user:password@localhost:5432/mydb",
})
```

**Option 2:** Set the `DATABASE_URL` environment variable:
```bash
export DATABASE_URL="postgres://user:password@localhost:5432/mydb"
```

Note: Shadow database testing will be skipped if no database URL is provided.

### "Migration validation failed: X migrations missing from filesystem"

This means migrations that were previously applied have been deleted from your migrations directory. This is a safety check to prevent inconsistencies. You need to restore the missing migration files.

### "Failed to drop shadow database"

The shadow database cleanup failed. You can manually drop it:
```sql
DROP DATABASE IF EXISTS your_database_gi_mig_shadow_db;
```

### Migration hangs or times out

Each migration has a 5-minute timeout by default. If your migration needs more time, consider:
1. Breaking it into smaller migrations
2. Optimizing the SQL queries
3. Running heavy operations outside migration system

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by database migration tools like Flyway and golang-migrate
- Built with production reliability and developer experience in mind

## Support

- 📫 Issues: [GitHub Issues](https://github.com/hasirciogluhq/migrator/issues)
- 💬 Discussions: [GitHub Discussions](https://github.com/hasirciogluhq/migrator/discussions)

---

Made with ❤️ for the Go community