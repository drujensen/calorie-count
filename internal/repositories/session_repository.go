package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/drujensen/calorie-count/internal/models"
)

// SessionRepository defines data access for sessions.
type SessionRepository interface {
	Create(ctx context.Context, session models.Session) error
	GetByToken(ctx context.Context, token string) (models.Session, error)
	Delete(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context) error
}

type sessionRepository struct {
	db *sql.DB
}

// NewSessionRepository returns a SessionRepository backed by the given SQLite database.
func NewSessionRepository(db *sql.DB) SessionRepository {
	return &sessionRepository{db: db}
}

// Create inserts a new session record, storing expires_at as UTC RFC3339.
func (r *sessionRepository) Create(ctx context.Context, session models.Session) error {
	expiresAt := session.ExpiresAt.UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		session.Token, session.UserID, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("inserting session: %w", err)
	}
	return nil
}

// GetByToken retrieves a session by its token.
// Returns ErrNotFound if no session with that token exists.
func (r *sessionRepository) GetByToken(ctx context.Context, token string) (models.Session, error) {
	var session models.Session
	var expiresAt string

	err := r.db.QueryRowContext(ctx,
		`SELECT token, user_id, expires_at FROM sessions WHERE token = ?`,
		token,
	).Scan(&session.Token, &session.UserID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Session{}, ErrNotFound
	}
	if err != nil {
		return models.Session{}, fmt.Errorf("querying session by token: %w", err)
	}

	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return models.Session{}, fmt.Errorf("parsing expires_at: %w", err)
	}
	session.ExpiresAt = t
	return session, nil
}

// Delete removes a session by its token.
func (r *sessionRepository) Delete(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// DeleteExpired removes all sessions whose expiry time has passed.
// Comparison is done via unixepoch() so timezone differences do not affect the result.
func (r *sessionRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE unixepoch(expires_at) <= unixepoch('now')`)
	if err != nil {
		return fmt.Errorf("deleting expired sessions: %w", err)
	}
	return nil
}
