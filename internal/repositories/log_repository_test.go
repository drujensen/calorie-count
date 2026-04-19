package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/drujensen/calorie-count/internal/models"
)

// createTestUser inserts a user and returns it with its assigned ID.
func createTestUser(t *testing.T, repo UserRepository, email string) models.User {
	t.Helper()
	u, err := repo.Create(context.Background(), models.User{
		Email:        email,
		PasswordHash: "hash",
		CalorieGoal:  2000,
	})
	if err != nil {
		t.Fatalf("creating test user: %v", err)
	}
	return u
}

func TestLogRepository_Create(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	logRepo := NewLogRepository(db)

	user := createTestUser(t, userRepo, "log-create@example.com")

	entry := models.LogEntry{
		UserID:   user.ID,
		FoodName: "Apple",
		Calories: 95,
		ProteinG: 0.5,
		FatG:     0.3,
		CarbsG:   25.0,
	}

	created, err := logRepo.Create(context.Background(), entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if created.FoodName != "Apple" {
		t.Errorf("expected FoodName Apple, got %s", created.FoodName)
	}
	if created.Calories != 95 {
		t.Errorf("expected Calories 95, got %d", created.Calories)
	}
	if created.LoggedAt.IsZero() {
		t.Error("expected non-zero LoggedAt")
	}
}

func TestLogRepository_ListByUserAndDate_Today(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	logRepo := NewLogRepository(db)

	user1 := createTestUser(t, userRepo, "list-u1@example.com")
	user2 := createTestUser(t, userRepo, "list-u2@example.com")

	// Create entries for user1 today
	for _, name := range []string{"Banana", "Oatmeal"} {
		_, err := logRepo.Create(context.Background(), models.LogEntry{
			UserID:   user1.ID,
			FoodName: name,
			Calories: 100,
		})
		if err != nil {
			t.Fatalf("creating entry %s: %v", name, err)
		}
	}

	// Create an entry for user2 (should not appear in user1's list)
	_, err := logRepo.Create(context.Background(), models.LogEntry{
		UserID:   user2.ID,
		FoodName: "Other User Food",
		Calories: 200,
	})
	if err != nil {
		t.Fatalf("creating user2 entry: %v", err)
	}

	entries, err := logRepo.ListByUserAndDate(context.Background(), user1.ID, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for user1 today, got %d", len(entries))
	}
	for _, e := range entries {
		if e.UserID != user1.ID {
			t.Errorf("got entry with wrong user_id: %d", e.UserID)
		}
	}
}

func TestLogRepository_ListByUserAndDate_WrongDate(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	logRepo := NewLogRepository(db)

	user := createTestUser(t, userRepo, "list-date@example.com")

	_, err := logRepo.Create(context.Background(), models.LogEntry{
		UserID:   user.ID,
		FoodName: "Bread",
		Calories: 80,
	})
	if err != nil {
		t.Fatalf("creating entry: %v", err)
	}

	// Query for yesterday — should return nothing
	yesterday := time.Now().AddDate(0, 0, -1)
	entries, err := logRepo.ListByUserAndDate(context.Background(), user.ID, yesterday)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for yesterday, got %d", len(entries))
	}
}

func TestLogRepository_Delete_OwnEntry(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	logRepo := NewLogRepository(db)

	user := createTestUser(t, userRepo, "delete-own@example.com")

	created, err := logRepo.Create(context.Background(), models.LogEntry{
		UserID:   user.ID,
		FoodName: "Salad",
		Calories: 150,
	})
	if err != nil {
		t.Fatalf("creating entry: %v", err)
	}

	if err := logRepo.Delete(context.Background(), created.ID, user.ID); err != nil {
		t.Fatalf("unexpected error deleting: %v", err)
	}

	// Verify gone
	entries, err := logRepo.ListByUserAndDate(context.Background(), user.ID, time.Now())
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after delete, got %d", len(entries))
	}
}

func TestLogRepository_Delete_EnforcesOwnership(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	logRepo := NewLogRepository(db)

	owner := createTestUser(t, userRepo, "owner@example.com")
	attacker := createTestUser(t, userRepo, "attacker@example.com")

	created, err := logRepo.Create(context.Background(), models.LogEntry{
		UserID:   owner.ID,
		FoodName: "Pizza",
		Calories: 300,
	})
	if err != nil {
		t.Fatalf("creating entry: %v", err)
	}

	// attacker tries to delete owner's entry — should succeed without error
	// but the row must NOT be deleted because user_id doesn't match
	if err := logRepo.Delete(context.Background(), created.ID, attacker.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Owner's entry should still exist
	entries, err := logRepo.ListByUserAndDate(context.Background(), owner.ID, time.Now())
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected entry to still exist (ownership enforced), got %d entries", len(entries))
	}
}

func TestLogRepository_SumByPeriod(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	logRepo := NewLogRepository(db)

	user := createTestUser(t, userRepo, "sum@example.com")

	entries := []models.LogEntry{
		{UserID: user.ID, FoodName: "Food A", Calories: 300, ProteinG: 10, FatG: 5, CarbsG: 40},
		{UserID: user.ID, FoodName: "Food B", Calories: 200, ProteinG: 15, FatG: 8, CarbsG: 20},
	}
	for _, e := range entries {
		if _, err := logRepo.Create(context.Background(), e); err != nil {
			t.Fatalf("creating entry: %v", err)
		}
	}

	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	to := from.Add(24*time.Hour - time.Nanosecond)

	summary, err := logRepo.SumByPeriod(context.Background(), user.ID, from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalCalories != 500 {
		t.Errorf("expected TotalCalories 500, got %d", summary.TotalCalories)
	}
	if summary.TotalProteinG != 25 {
		t.Errorf("expected TotalProteinG 25, got %.1f", summary.TotalProteinG)
	}
	if summary.TotalFatG != 13 {
		t.Errorf("expected TotalFatG 13, got %.1f", summary.TotalFatG)
	}
	if summary.TotalCarbsG != 60 {
		t.Errorf("expected TotalCarbsG 60, got %.1f", summary.TotalCarbsG)
	}
	if summary.Days != 1 {
		t.Errorf("expected Days 1, got %d", summary.Days)
	}
}

func TestLogRepository_SumByPeriod_Empty(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	logRepo := NewLogRepository(db)

	user := createTestUser(t, userRepo, "sum-empty@example.com")

	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	to := from.Add(24*time.Hour - time.Nanosecond)

	summary, err := logRepo.SumByPeriod(context.Background(), user.ID, from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalCalories != 0 {
		t.Errorf("expected TotalCalories 0, got %d", summary.TotalCalories)
	}
	if summary.Days != 0 {
		t.Errorf("expected Days 0, got %d", summary.Days)
	}
}

// TestSumByPeriod_MultipleEntries verifies that multiple entries are summed
// correctly and the distinct day count is accurate.
func TestSumByPeriod_MultipleEntries(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	logRepo := NewLogRepository(db)

	user := createTestUser(t, userRepo, "multi-sum@example.com")

	entries := []struct {
		name     string
		calories int
		protein  float64
		fat      float64
		carbs    float64
	}{
		{"Breakfast", 400, 20, 10, 50},
		{"Lunch", 600, 30, 15, 70},
		{"Dinner", 700, 40, 20, 80},
	}

	for _, e := range entries {
		_, err := logRepo.Create(context.Background(), models.LogEntry{
			UserID:   user.ID,
			FoodName: e.name,
			Calories: e.calories,
			ProteinG: e.protein,
			FatG:     e.fat,
			CarbsG:   e.carbs,
		})
		if err != nil {
			t.Fatalf("creating entry %s: %v", e.name, err)
		}
	}

	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	to := from.Add(24*time.Hour - time.Nanosecond)

	summary, err := logRepo.SumByPeriod(context.Background(), user.ID, from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalCalories != 1700 {
		t.Errorf("expected TotalCalories 1700, got %d", summary.TotalCalories)
	}
	if summary.TotalProteinG != 90 {
		t.Errorf("expected TotalProteinG 90, got %.1f", summary.TotalProteinG)
	}
	if summary.TotalFatG != 45 {
		t.Errorf("expected TotalFatG 45, got %.1f", summary.TotalFatG)
	}
	if summary.TotalCarbsG != 200 {
		t.Errorf("expected TotalCarbsG 200, got %.1f", summary.TotalCarbsG)
	}
	if summary.Days != 1 {
		t.Errorf("expected Days 1, got %d", summary.Days)
	}
}

// TestSumByPeriod_EmptyPeriod verifies that querying a period with no entries
// returns all zeros and Days == 0.
func TestSumByPeriod_EmptyPeriod(t *testing.T) {
	db := openTestSQLDB(t)
	userRepo := NewUserRepository(db)
	logRepo := NewLogRepository(db)

	user := createTestUser(t, userRepo, "empty-period@example.com")

	// Query a past range that has no data
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	to := past.Add(24*time.Hour - time.Nanosecond)

	summary, err := logRepo.SumByPeriod(context.Background(), user.ID, past, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalCalories != 0 {
		t.Errorf("expected TotalCalories 0, got %d", summary.TotalCalories)
	}
	if summary.TotalProteinG != 0 {
		t.Errorf("expected TotalProteinG 0, got %.1f", summary.TotalProteinG)
	}
	if summary.Days != 0 {
		t.Errorf("expected Days 0, got %d", summary.Days)
	}
}
