package db_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/drujensen/calorie-count/internal/db"
)

func TestOpen_InMemory(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open(':memory:') returned unexpected error: %v", err)
	}
	defer database.Close()

	if err := database.Ping(); err != nil {
		t.Fatalf("Ping() after Open() failed: %v", err)
	}
}

func TestOpen_ForeignKeysEnabled(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer database.Close()

	// WAL mode is not supported for in-memory databases; SQLite uses "memory"
	// journal mode instead. We only verify that foreign keys are enabled, which
	// is a valid check for both in-memory and file-based databases.
	var fkEnabled int
	if err := database.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		t.Fatalf("querying foreign_keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Errorf("foreign_keys = %d, want 1", fkEnabled)
	}
}

func TestRunMigrations_AppliesSQLFiles(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer database.Close()

	migFS := fstest.MapFS{
		"001_create_test.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY);`),
		},
	}

	if err := db.RunMigrations(database, migFS); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}

	// Verify test_table was created.
	var count int
	err = database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_table'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("querying sqlite_master: %v", err)
	}
	if count != 1 {
		t.Errorf("test_table not found after migration")
	}

	// Verify migration was recorded.
	var recorded int
	err = database.QueryRow(
		`SELECT COUNT(*) FROM schema_migrations WHERE filename='001_create_test.sql'`,
	).Scan(&recorded)
	if err != nil {
		t.Fatalf("querying schema_migrations: %v", err)
	}
	if recorded != 1 {
		t.Errorf("migration not recorded in schema_migrations")
	}
}

func TestRunMigrations_IdempotentOnRerun(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer database.Close()

	migFS := fstest.MapFS{
		"001_idempotent.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE IF NOT EXISTS idempotent_table (id INTEGER PRIMARY KEY);`),
		},
	}

	// Run twice — should not error on second run.
	if err := db.RunMigrations(database, migFS); err != nil {
		t.Fatalf("first RunMigrations() error: %v", err)
	}
	if err := db.RunMigrations(database, migFS); err != nil {
		t.Fatalf("second RunMigrations() error: %v", err)
	}
}

func TestRunMigrations_AlphabeticalOrder(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer database.Close()

	migFS := fstest.MapFS{
		"002_add_col.sql": &fstest.MapFile{
			Data: []byte(`ALTER TABLE order_test ADD COLUMN name TEXT;`),
		},
		"001_create.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE IF NOT EXISTS order_test (id INTEGER PRIMARY KEY);`),
		},
	}

	// Should succeed because 001 runs before 002.
	if err := db.RunMigrations(database, migFS); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}
}

func TestRunMigrations_EmptyFS(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database, fstest.MapFS{}); err != nil {
		t.Fatalf("RunMigrations() with empty FS error: %v", err)
	}
}

// Compile-time check: db.RunMigrations accepts fs.FS interface.
var _ fs.FS = fstest.MapFS{}
