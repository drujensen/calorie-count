package repositories

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

const testSchema = `
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    calorie_goal INTEGER NOT NULL DEFAULT 2000,
    current_weight_lbs REAL NOT NULL DEFAULT 0,
    age INTEGER NOT NULL DEFAULT 0,
    height_in REAL NOT NULL DEFAULT 0,
    sex TEXT NOT NULL DEFAULT '',
    target_weight_lbs REAL NOT NULL DEFAULT 0,
    weight_loss_rate TEXT NOT NULL DEFAULT 'maintain',
    activity_level TEXT NOT NULL DEFAULT 'sedentary',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sessions (
    token TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS log_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    food_name TEXT NOT NULL,
    calories INTEGER NOT NULL DEFAULT 0,
    protein_g REAL NOT NULL DEFAULT 0,
    fat_g REAL NOT NULL DEFAULT 0,
    carbs_g REAL NOT NULL DEFAULT 0,
    image_path TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    logged_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS weight_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    weight_lbs REAL NOT NULL,
    logged_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
`

func openTestSQLDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("opening in-memory db: %v", err)
	}
	if _, err := db.Exec(testSchema); err != nil {
		t.Fatalf("applying schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
