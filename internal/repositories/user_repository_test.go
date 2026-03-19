package repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/drujensen/calorie-count/internal/models"
)

func TestUserRepository_Create(t *testing.T) {
	db := openTestSQLDB(t)
	repo := NewUserRepository(db)

	user := models.User{
		Email:        "create@example.com",
		PasswordHash: "hashedpw",
		CalorieGoal:  2000,
	}

	created, err := repo.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if created.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, created.Email)
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := openTestSQLDB(t)
	repo := NewUserRepository(db)

	user := models.User{Email: "byemail@example.com", PasswordHash: "hash", CalorieGoal: 1800}
	created, err := repo.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	found, err := repo.GetByEmail(context.Background(), "byemail@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, found.ID)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	db := openTestSQLDB(t)
	repo := NewUserRepository(db)

	_, err := repo.GetByEmail(context.Background(), "nobody@example.com")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	db := openTestSQLDB(t)
	repo := NewUserRepository(db)

	user := models.User{Email: "byid@example.com", PasswordHash: "hash", CalorieGoal: 2200}
	created, err := repo.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	found, err := repo.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.Email != "byid@example.com" {
		t.Errorf("expected email byid@example.com, got %s", found.Email)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db := openTestSQLDB(t)
	repo := NewUserRepository(db)

	_, err := repo.GetByID(context.Background(), 99999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestUserRepository_UpdateGoalAndWeight(t *testing.T) {
	db := openTestSQLDB(t)
	repo := NewUserRepository(db)

	user := models.User{Email: "update@example.com", PasswordHash: "hash", CalorieGoal: 2000}
	created, err := repo.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := repo.UpdateProfile(context.Background(), created.ID, 1800, 175.5, 160.0, 30, 70.0, "male", "lose_1", "moderate"); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	found, err := repo.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if found.CalorieGoal != 1800 {
		t.Errorf("expected calorie goal 1800, got %d", found.CalorieGoal)
	}
	if found.CurrentWeightLbs != 175.5 {
		t.Errorf("expected weight 175.5, got %f", found.CurrentWeightLbs)
	}
}
