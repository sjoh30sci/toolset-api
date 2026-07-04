package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite3 "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps a *sql.DB with helpers for the gateway.
type DB struct {
	*sql.DB
	path string
}

// Open opens (creating if necessary) the SQLite database described by cfg,
// configures the pool, and runs any pending migrations.
func Open(cfg Config) (*DB, error) {
	if cfg.Path == "" {
		return nil, errors.New("db: path must not be empty")
	}

	// Ensure the parent directory exists.
	if dir := filepath.Dir(cfg.Path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("db: creating data dir %s: %w", dir, err)
		}
	}

	// WAL mode + busy timeout make concurrent local access much friendlier.
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", cfg.Path)
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}

	if cfg.MaxConnections > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxConnections)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	d := &DB{DB: sqlDB, path: cfg.Path}

	if cfg.MigrationsDir != "" {
		if err := d.Migrate(cfg.MigrationsDir); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	}

	return d, nil
}

// Migrate applies all pending "up" migrations from migrationsDir. It is a no-op
// if the schema is already current.
func (d *DB) Migrate(migrationsDir string) error {
	abs, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("db: resolving migrations dir: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("db: migrations dir %s: %w", abs, err)
	}

	driver, err := migratesqlite3.WithInstance(d.DB, &migratesqlite3.Config{})
	if err != nil {
		return fmt.Errorf("db: migrate driver: %w", err)
	}

	src, err := (&file.File{}).Open("file://" + filepath.ToSlash(abs))
	if err != nil {
		return fmt.Errorf("db: migrate source: %w", err)
	}

	m, err := migrate.NewWithInstance("file", src, "sqlite3", driver)
	if err != nil {
		return fmt.Errorf("db: migrate init: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: migrate up: %w", err)
	}
	return nil
}

// Path returns the underlying database file path.
func (d *DB) Path() string { return d.path }
