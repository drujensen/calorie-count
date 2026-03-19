package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/drujensen/calorie-count/internal/models"
)

// seedUser inserts a user into the test DB and returns it.
func seedUser(t *testing.T, repo UserRepository) models.User {
	t.Helper()
	user, err := repo.Create(context.Background(), models.User{
		Email:        "session-user@example.com",
		PasswordHash: "hash",
		CalorieGoal:  2000,
	})
	if err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	return user
}

func TestSessionRepository_Create(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	sessionRepo := NewSessionRepository(db)

	user := seedUser(t, userRepo)

	session := models.Session{
		Token:     "tok-create",
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := sessionRepo.Create(context.Background(), session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionRepository_GetByToken(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	sessionRepo := NewSessionRepository(db)

	user := seedUser(t, userRepo)
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	session := models.Session{Token: "tok-get", UserID: user.ID, ExpiresAt: expires}
	if err := sessionRepo.Create(context.Background(), session); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	found, err := sessionRepo.GetByToken(context.Background(), "tok-get")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.UserID != user.ID {
		t.Errorf("expected userID %d, got %d", user.ID, found.UserID)
	}
	if found.Token != "tok-get" {
		t.Errorf("expected token tok-get, got %s", found.Token)
	}
}

func TestSessionRepository_GetByToken_NotFound(t *testing.T) {
	db := openTestSQLDB(t)
	sessionRepo := NewSessionRepository(db)

	_, err := sessionRepo.GetByToken(context.Background(), "does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestSessionRepository_Delete(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	sessionRepo := NewSessionRepository(db)

	user := seedUser(t, userRepo)
	session := models.Session{Token: "tok-del", UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}

	if err := sessionRepo.Create(context.Background(), session); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if err := sessionRepo.Delete(context.Background(), "tok-del"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err := sessionRepo.GetByToken(context.Background(), "tok-del")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got: %v", err)
	}
}

func TestSessionRepository_DeleteExpired(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	sessionRepo := NewSessionRepository(db)

	user := seedUser(t, userRepo)

	expired := models.Session{Token: "tok-expired", UserID: user.ID, ExpiresAt: time.Now().Add(-time.Hour)}
	active := models.Session{Token: "tok-active", UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}

	if err := sessionRepo.Create(context.Background(), expired); err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	if err := sessionRepo.Create(context.Background(), active); err != nil {
		t.Fatalf("create active session: %v", err)
	}

	if err := sessionRepo.DeleteExpired(context.Background()); err != nil {
		t.Fatalf("delete expired: %v", err)
	}

	_, err := sessionRepo.GetByToken(context.Background(), "tok-expired")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected expired session to be deleted, got: %v", err)
	}

	_, err = sessionRepo.GetByToken(context.Background(), "tok-active")
	if err != nil {
		t.Errorf("expected active session to remain, got: %v", err)
	}
}
