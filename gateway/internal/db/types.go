// Package db handles SQLite initialization, connection pooling, and migrations.
package db

// Config controls how the SQLite database is opened and migrated.
type Config struct {
	// Path is the filesystem path to the SQLite database file.
	Path string
	// MaxConnections caps the open connection pool size.
	MaxConnections int
	// MigrationsDir is the directory containing golang-migrate SQL files.
	MigrationsDir string
}
