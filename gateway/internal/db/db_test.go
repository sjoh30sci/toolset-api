package db

import (
	"path/filepath"
	"testing"
)

// TestMigration001 verifies that opening a fresh SQLite database applies
// migration 001 and creates all expected tables.
func TestMigration001(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	// Migrations live at gateway/migrations relative to this package.
	migrationsDir := filepath.Join("..", "..", "migrations")

	d, err := Open(Config{
		Path:           dbPath,
		MaxConnections: 4,
		MigrationsDir:  migrationsDir,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	wantTables := []string{"tools", "api_keys", "executions", "rate_limits"}
	for _, tbl := range wantTables {
		var name string
		err := d.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&name)
		if err != nil {
			t.Errorf("expected table %q to exist: %v", tbl, err)
		}
	}

	// Applying migrations again should be a no-op (idempotent).
	if err := d.Migrate(migrationsDir); err != nil {
		t.Fatalf("re-run migrate: %v", err)
	}
}

// TestOpenEmptyPath verifies validation of an empty path.
func TestOpenEmptyPath(t *testing.T) {
	if _, err := Open(Config{Path: ""}); err == nil {
		t.Fatal("expected error for empty path")
	}
}
