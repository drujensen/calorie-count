package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database at the given path and configures it.
func Open(path string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, pragma := range pragmas {
		if _, err := database.Exec(pragma); err != nil {
			return nil, fmt.Errorf("setting pragma %q: %w", pragma, err)
		}
	}

	return database, nil
}

// RunMigrations reads .sql files from migrationsFS in alphabetical order and
// executes any that have not yet been applied. It tracks applied migrations in
// a schema_migrations table (created on first run).
func RunMigrations(database *sql.DB, migrationsFS fs.FS) error {
	_, err := database.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		filename TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, filename := range files {
		var count int
		err := database.QueryRow(
			"SELECT COUNT(*) FROM schema_migrations WHERE filename = ?", filename,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("checking migration %s: %w", filename, err)
		}
		if count > 0 {
			continue
		}

		content, err := fs.ReadFile(migrationsFS, filename)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", filename, err)
		}

		if _, err := database.Exec(string(content)); err != nil {
			return fmt.Errorf("executing migration %s: %w", filename, err)
		}

		if _, err := database.Exec(
			"INSERT INTO schema_migrations (filename) VALUES (?)", filename,
		); err != nil {
			return fmt.Errorf("recording migration %s: %w", filename, err)
		}
	}

	return nil
}
