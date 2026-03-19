package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/drujensen/calorie-count/internal/models"
)

// WeightRepository defines data access for weight entries.
type WeightRepository interface {
	AddEntry(ctx context.Context, userID int, weightLbs float64) (models.WeightEntry, error)
	ListByUser(ctx context.Context, userID int, limit int) ([]models.WeightEntry, error)
	GetLatest(ctx context.Context, userID int) (models.WeightEntry, error)
}

type weightRepository struct {
	db *sql.DB
}

// NewWeightRepository returns a WeightRepository backed by the given SQLite database.
func NewWeightRepository(db *sql.DB) WeightRepository {
	return &weightRepository{db: db}
}

// AddEntry inserts a new weight measurement and returns it with its assigned ID.
func (r *weightRepository) AddEntry(ctx context.Context, userID int, weightLbs float64) (models.WeightEntry, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO weight_entries (user_id, weight_lbs) VALUES (?, ?)`,
		userID, weightLbs,
	)
	if err != nil {
		return models.WeightEntry{}, fmt.Errorf("inserting weight entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return models.WeightEntry{}, fmt.Errorf("getting last insert id: %w", err)
	}

	var entry models.WeightEntry
	err = r.db.QueryRowContext(ctx,
		`SELECT id, user_id, weight_lbs, logged_at FROM weight_entries WHERE id = ?`, id,
	).Scan(&entry.ID, &entry.UserID, &entry.WeightLbs, &entry.LoggedAt)
	if err != nil {
		return models.WeightEntry{}, fmt.Errorf("fetching new weight entry: %w", err)
	}
	return entry, nil
}

// ListByUser returns the most recent `limit` weight entries for a user, newest first.
func (r *weightRepository) ListByUser(ctx context.Context, userID int, limit int) ([]models.WeightEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, weight_lbs, logged_at
		 FROM weight_entries WHERE user_id = ?
		 ORDER BY logged_at DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing weight entries: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var entries []models.WeightEntry
	for rows.Next() {
		var e models.WeightEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.WeightLbs, &e.LoggedAt); err != nil {
			return nil, fmt.Errorf("scanning weight entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetLatest returns the most recent weight entry for a user.
// Returns ErrNotFound if no entries exist.
func (r *weightRepository) GetLatest(ctx context.Context, userID int) (models.WeightEntry, error) {
	var entry models.WeightEntry
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, weight_lbs, logged_at
		 FROM weight_entries WHERE user_id = ?
		 ORDER BY logged_at DESC LIMIT 1`,
		userID,
	).Scan(&entry.ID, &entry.UserID, &entry.WeightLbs, &entry.LoggedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.WeightEntry{}, ErrNotFound
	}
	if err != nil {
		return models.WeightEntry{}, fmt.Errorf("getting latest weight entry: %w", err)
	}
	return entry, nil
}
