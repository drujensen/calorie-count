package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/drujensen/calorie-count/internal/models"
)

// LogRepository defines data access for log entries.
type LogRepository interface {
	Create(ctx context.Context, entry models.LogEntry) (models.LogEntry, error)
	GetByID(ctx context.Context, id int, userID int) (models.LogEntry, error)
	Update(ctx context.Context, entry models.LogEntry) (models.LogEntry, error)
	ListByUserAndDate(ctx context.Context, userID int, date time.Time) ([]models.LogEntry, error)
	Delete(ctx context.Context, entryID int, userID int) error
	// SumByPeriod sums macros for entries between from and to (inclusive).
	// tzOffsetMin is the client's timezone offset in minutes west of UTC
	// (matches JavaScript's getTimezoneOffset(); PDT = 420, JST = -540).
	// It is used to group entries by local date rather than UTC date.
	SumByPeriod(ctx context.Context, userID int, from, to time.Time, tzOffsetMin int) (models.MacroSummary, error)
}

type logRepository struct {
	db *sql.DB
}

// NewLogRepository returns a LogRepository backed by the given SQLite database.
func NewLogRepository(db *sql.DB) LogRepository {
	return &logRepository{db: db}
}

// Create inserts a new log entry and returns it with its assigned ID.
// If entry.LoggedAt is zero, logged_at defaults to the current time.
func (r *logRepository) Create(ctx context.Context, entry models.LogEntry) (models.LogEntry, error) {
	loggedAt := entry.LoggedAt
	if loggedAt.IsZero() {
		loggedAt = time.Now()
	}
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO log_entries (user_id, food_name, calories, protein_g, fat_g, carbs_g, amount, unit, image_path, notes, logged_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.UserID, entry.FoodName, entry.Calories, entry.ProteinG, entry.FatG, entry.CarbsG,
		entry.Amount, entry.Unit, entry.ImagePath, entry.Notes, loggedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("inserting log entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("getting last insert id: %w", err)
	}

	return r.scanByID(ctx, int(id))
}

// GetByID retrieves a log entry by ID, enforcing ownership via userID.
func (r *logRepository) GetByID(ctx context.Context, id int, userID int) (models.LogEntry, error) {
	var entry models.LogEntry
	var loggedAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, food_name, calories, protein_g, fat_g, carbs_g, amount, unit, image_path, notes, logged_at
		 FROM log_entries WHERE id = ? AND user_id = ?`,
		id, userID,
	).Scan(
		&entry.ID, &entry.UserID, &entry.FoodName, &entry.Calories,
		&entry.ProteinG, &entry.FatG, &entry.CarbsG,
		&entry.Amount, &entry.Unit, &entry.ImagePath, &entry.Notes, &loggedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.LogEntry{}, ErrNotFound
	}
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("querying log entry: %w", err)
	}
	t, err := parseDateTime(loggedAt)
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("parsing logged_at: %w", err)
	}
	entry.LoggedAt = t
	return entry, nil
}

// Update replaces the nutritional fields of an existing log entry, enforcing ownership.
func (r *logRepository) Update(ctx context.Context, entry models.LogEntry) (models.LogEntry, error) {
	_, err := r.db.ExecContext(ctx,
		`UPDATE log_entries
		 SET food_name=?, calories=?, protein_g=?, fat_g=?, carbs_g=?, amount=?, unit=?, notes=?
		 WHERE id=? AND user_id=?`,
		entry.FoodName, entry.Calories, entry.ProteinG, entry.FatG, entry.CarbsG,
		entry.Amount, entry.Unit, entry.Notes, entry.ID, entry.UserID,
	)
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("updating log entry: %w", err)
	}
	return r.scanByID(ctx, entry.ID)
}

// ListByUserAndDate returns all entries for a user on the given date, newest first.
// Uses local-time day boundaries to correctly handle timezone offsets.
func (r *logRepository) ListByUserAndDate(ctx context.Context, userID int, date time.Time) ([]models.LogEntry, error) {
	d := date
	from := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
	to := from.Add(24*time.Hour - time.Nanosecond)
	fromStr := from.UTC().Format(time.RFC3339)
	toStr := to.UTC().Format(time.RFC3339)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, food_name, calories, protein_g, fat_g, carbs_g, amount, unit, image_path, notes, logged_at
		 FROM log_entries
		 WHERE user_id = ? AND logged_at >= ? AND logged_at <= ?
		 ORDER BY logged_at DESC`,
		userID, fromStr, toStr,
	)
	if err != nil {
		return nil, fmt.Errorf("listing log entries: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var entries []models.LogEntry
	for rows.Next() {
		var entry models.LogEntry
		var loggedAt string
		if err := rows.Scan(
			&entry.ID, &entry.UserID, &entry.FoodName, &entry.Calories,
			&entry.ProteinG, &entry.FatG, &entry.CarbsG,
			&entry.Amount, &entry.Unit, &entry.ImagePath, &entry.Notes, &loggedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning log entry: %w", err)
		}
		t, err := parseDateTime(loggedAt)
		if err != nil {
			return nil, fmt.Errorf("parsing logged_at: %w", err)
		}
		entry.LoggedAt = t
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating log entries: %w", err)
	}
	return entries, nil
}

// Delete removes a log entry by ID, enforcing ownership via user_id.
func (r *logRepository) Delete(ctx context.Context, entryID int, userID int) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM log_entries WHERE id = ? AND user_id = ?`,
		entryID, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting log entry: %w", err)
	}
	return nil
}

// SumByPeriod returns macro totals and distinct local-day count for the given period.
// tzOffsetMin follows the JavaScript getTimezoneOffset() convention: positive = west of UTC
// (PDT = 420, JST = -540). It is applied to logged_at before grouping by date so that
// entries spanning UTC midnight within a single local day are counted as one day.
func (r *logRepository) SumByPeriod(ctx context.Context, userID int, from, to time.Time, tzOffsetMin int) (models.MacroSummary, error) {
	fromStr := from.UTC().Format(time.RFC3339)
	toStr := to.UTC().Format(time.RFC3339)

	// Build a SQLite datetime modifier that converts UTC → local time.
	// getTimezoneOffset() is positive for zones west of UTC, so negate to get
	// the number of minutes to ADD to UTC to reach local time.
	// e.g. PDT (420) → "-420 minutes"; JST (-540) → "540 minutes"
	tzMod := fmt.Sprintf("%d minutes", -tzOffsetMin)

	var summary models.MacroSummary
	err := r.db.QueryRowContext(ctx,
		`SELECT
			COALESCE(SUM(calories), 0),
			COALESCE(SUM(protein_g), 0.0),
			COALESCE(SUM(fat_g), 0.0),
			COALESCE(SUM(carbs_g), 0.0),
			COUNT(DISTINCT date(logged_at, ?))
		 FROM log_entries
		 WHERE user_id = ? AND logged_at >= ? AND logged_at <= ?`,
		tzMod, userID, fromStr, toStr,
	).Scan(
		&summary.TotalCalories,
		&summary.TotalProteinG,
		&summary.TotalFatG,
		&summary.TotalCarbsG,
		&summary.Days,
	)
	if err != nil {
		return models.MacroSummary{}, fmt.Errorf("summing log entries: %w", err)
	}
	return summary, nil
}

// scanByID is an internal helper that fetches a log entry by primary key only.
func (r *logRepository) scanByID(ctx context.Context, id int) (models.LogEntry, error) {
	var entry models.LogEntry
	var loggedAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, food_name, calories, protein_g, fat_g, carbs_g, amount, unit, image_path, notes, logged_at
		 FROM log_entries WHERE id = ?`,
		id,
	).Scan(
		&entry.ID, &entry.UserID, &entry.FoodName, &entry.Calories,
		&entry.ProteinG, &entry.FatG, &entry.CarbsG,
		&entry.Amount, &entry.Unit, &entry.ImagePath, &entry.Notes, &loggedAt,
	)
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("querying log entry by id: %w", err)
	}
	t, err := parseDateTime(loggedAt)
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("parsing logged_at: %w", err)
	}
	entry.LoggedAt = t
	return entry, nil
}

// parseDateTime parses SQLite datetime strings in multiple formats.
func parseDateTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized datetime format: %s", s)
}
