package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/drujensen/calorie-count/internal/models"
	"github.com/drujensen/calorie-count/internal/repositories"
)

// --- mock log repository ---

type mockLogRepo struct {
	entries []models.LogEntry
	nextID  int
	sumErr  error
}

func newMockLogRepo() *mockLogRepo {
	return &mockLogRepo{nextID: 1}
}

func (m *mockLogRepo) Create(_ context.Context, entry models.LogEntry) (models.LogEntry, error) {
	entry.ID = m.nextID
	m.nextID++
	entry.LoggedAt = time.Now()
	m.entries = append(m.entries, entry)
	return entry, nil
}

func (m *mockLogRepo) ListByUserAndDate(_ context.Context, userID int, date time.Time) ([]models.LogEntry, error) {
	var result []models.LogEntry
	for _, e := range m.entries {
		if e.UserID == userID && sameDate(e.LoggedAt, date) {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockLogRepo) Delete(_ context.Context, entryID int, userID int) error {
	for i, e := range m.entries {
		if e.ID == entryID && e.UserID == userID {
			m.entries = append(m.entries[:i], m.entries[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockLogRepo) GetByID(_ context.Context, id int, userID int) (models.LogEntry, error) {
	for _, e := range m.entries {
		if e.ID == id && e.UserID == userID {
			return e, nil
		}
	}
	return models.LogEntry{}, errors.New("not found")
}

func (m *mockLogRepo) Update(_ context.Context, entry models.LogEntry) (models.LogEntry, error) {
	for i, e := range m.entries {
		if e.ID == entry.ID && e.UserID == entry.UserID {
			m.entries[i] = entry
			return entry, nil
		}
	}
	return models.LogEntry{}, errors.New("not found")
}

func (m *mockLogRepo) SumByPeriod(_ context.Context, userID int, from, to time.Time, tzOffsetMin int) (models.MacroSummary, error) {
	if m.sumErr != nil {
		return models.MacroSummary{}, m.sumErr
	}
	var summary models.MacroSummary
	days := make(map[string]bool)
	for _, e := range m.entries {
		if e.UserID == userID && !e.LoggedAt.Before(from) && !e.LoggedAt.After(to) {
			summary.TotalCalories += e.Calories
			summary.TotalProteinG += e.ProteinG
			summary.TotalFatG += e.FatG
			summary.TotalCarbsG += e.CarbsG
			days[e.LoggedAt.UTC().Format("2006-01-02")] = true
		}
	}
	summary.Days = len(days)
	return summary, nil
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

// --- helpers ---

// mockWeightRepo is a no-op weight repository for tests that don't need weight data.
type mockWeightRepo struct{}

func (m *mockWeightRepo) AddEntry(_ context.Context, _ int, _ float64) (models.WeightEntry, error) {
	return models.WeightEntry{}, nil
}
func (m *mockWeightRepo) ListByUser(_ context.Context, _ int, _ int) ([]models.WeightEntry, error) {
	return nil, nil
}
func (m *mockWeightRepo) GetLatest(_ context.Context, _ int) (models.WeightEntry, error) {
	return models.WeightEntry{}, repositories.ErrNotFound
}

func newTestLogService() (LogService, *mockLogRepo, *mockUserRepo) {
	logRepo := newMockLogRepo()
	userRepo := newMockUserRepo()
	svc := NewLogService(logRepo, userRepo, &mockWeightRepo{})
	return svc, logRepo, userRepo
}

func seedUser(t *testing.T, repo *mockUserRepo) models.User {
	t.Helper()
	u, err := repo.Create(context.Background(), models.User{
		Email:              "logtest@example.com",
		PasswordHash:       "hash",
		CalorieGoal:        2000,
		CurrentWeightLbs:   150,
		HeightIn:           68,
		Age:                30,
		Sex:                "male",
		ActivityLevel:      "sedentary",
	})
	if err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	return u
}

// --- AddEntry validation tests ---

func TestLogService_AddEntry_Success(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	entry := models.LogEntry{FoodName: "Apple", Calories: 95, ProteinG: 0.5, FatG: 0.3, CarbsG: 25}
	created, err := svc.AddEntry(context.Background(), user.ID, entry)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if created.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if created.UserID != user.ID {
		t.Errorf("expected UserID %d, got %d", user.ID, created.UserID)
	}
}

func TestLogService_AddEntry_EmptyName(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{FoodName: "", Calories: 100})
	if !errors.Is(err, ErrInvalidEntry) {
		t.Errorf("expected ErrInvalidEntry, got: %v", err)
	}
}

func TestLogService_AddEntry_WhitespaceName(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{FoodName: "   ", Calories: 100})
	if !errors.Is(err, ErrInvalidEntry) {
		t.Errorf("expected ErrInvalidEntry for whitespace name, got: %v", err)
	}
}

func TestLogService_AddEntry_NegativeCalories(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{FoodName: "Food", Calories: -1})
	if !errors.Is(err, ErrInvalidEntry) {
		t.Errorf("expected ErrInvalidEntry, got: %v", err)
	}
}

func TestLogService_AddEntry_CaloriesTooHigh(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{FoodName: "Food", Calories: 10001})
	if !errors.Is(err, ErrInvalidEntry) {
		t.Errorf("expected ErrInvalidEntry for calories > 10000, got: %v", err)
	}
}

func TestLogService_AddEntry_ZeroCaloriesAllowed(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{FoodName: "Water", Calories: 0})
	if err != nil {
		t.Errorf("expected no error for 0 calories, got: %v", err)
	}
}

func TestLogService_AddEntry_NegativeProtein(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{FoodName: "Food", Calories: 100, ProteinG: -1})
	if !errors.Is(err, ErrInvalidEntry) {
		t.Errorf("expected ErrInvalidEntry, got: %v", err)
	}
}

func TestLogService_AddEntry_NegativeFat(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{FoodName: "Food", Calories: 100, FatG: -1})
	if !errors.Is(err, ErrInvalidEntry) {
		t.Errorf("expected ErrInvalidEntry, got: %v", err)
	}
}

func TestLogService_AddEntry_NegativeCarbs(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{FoodName: "Food", Calories: 100, CarbsG: -1})
	if !errors.Is(err, ErrInvalidEntry) {
		t.Errorf("expected ErrInvalidEntry, got: %v", err)
	}
}

// --- DeleteEntry tests ---

func TestLogService_DeleteEntry_Success(t *testing.T) {
	svc, logRepo, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	created, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{FoodName: "Salad", Calories: 150})
	if err != nil {
		t.Fatalf("add entry: %v", err)
	}

	if err := svc.DeleteEntry(context.Background(), user.ID, created.ID); err != nil {
		t.Fatalf("delete entry: %v", err)
	}

	// Verify deletion
	entries, _ := logRepo.ListByUserAndDate(context.Background(), user.ID, time.Now())
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after delete, got %d", len(entries))
	}
}

// --- GetSummary macro percentage tests ---

func TestLogService_GetSummary_MacroPercentages(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	// 100 cal protein (25g * 4), 90 cal fat (10g * 9), 100 cal carbs (25g * 4) = 290 cal total
	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{
		FoodName: "Mixed Meal",
		Calories: 290,
		ProteinG: 25,
		FatG:     10,
		CarbsG:   25,
	})
	if err != nil {
		t.Fatalf("add entry: %v", err)
	}

	ps, err := svc.GetSummary(context.Background(), user.ID, "daily", time.Now())
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}

	// Protein: 25*4/290*100 ≈ 34.48%
	expectedProteinPct := 25.0 * 4 / 290 * 100
	if abs(ps.ProteinPct-expectedProteinPct) > 0.01 {
		t.Errorf("expected ProteinPct %.2f, got %.2f", expectedProteinPct, ps.ProteinPct)
	}

	// Fat: 10*9/290*100 ≈ 31.03%
	expectedFatPct := 10.0 * 9 / 290 * 100
	if abs(ps.FatPct-expectedFatPct) > 0.01 {
		t.Errorf("expected FatPct %.2f, got %.2f", expectedFatPct, ps.FatPct)
	}

	// Carbs: 25*4/290*100 ≈ 34.48%
	expectedCarbsPct := 25.0 * 4 / 290 * 100
	if abs(ps.CarbsPct-expectedCarbsPct) > 0.01 {
		t.Errorf("expected CarbsPct %.2f, got %.2f", expectedCarbsPct, ps.CarbsPct)
	}
}

func TestLogService_GetSummary_ZeroCaloriesNoPercentages(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	ps, err := svc.GetSummary(context.Background(), user.ID, "daily", time.Now())
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}

	if ps.ProteinPct != 0 || ps.FatPct != 0 || ps.CarbsPct != 0 {
		t.Errorf("expected 0%% percentages when no calories, got P:%.2f F:%.2f C:%.2f",
			ps.ProteinPct, ps.FatPct, ps.CarbsPct)
	}
}

func TestLogService_GetSummary_WeightImpactLbs(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	// Log 2500 cal today
	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{
		FoodName: "Big Meal",
		Calories: 2500,
		ProteinG: 50,
	})
	if err != nil {
		t.Fatalf("add entry: %v", err)
	}

	ps, err := svc.GetSummary(context.Background(), user.ID, "daily", time.Now())
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}

	_, tdee := calcBMRTDEE(user)
	expected := (2500.0 - float64(tdee)) / 3500.0
	if abs(ps.WeightImpactLbs-expected) > 0.001 {
		t.Errorf("expected WeightImpactLbs %.4f, got %.4f", expected, ps.WeightImpactLbs)
	}
}

func TestLogService_GetSummary_PeriodNormalization(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	// Unknown period should default to "daily"
	ps, err := svc.GetSummary(context.Background(), user.ID, "unknown", time.Now())
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	if ps.Period != "daily" {
		t.Errorf("expected period 'daily', got '%s'", ps.Period)
	}
}

func TestGetSummary_DailyPeriod(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	ps, err := svc.GetSummary(context.Background(), user.ID, "daily", time.Now())
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	if ps.Period != "daily" {
		t.Errorf("expected period 'daily', got '%s'", ps.Period)
	}
	if ps.CalorieGoal != user.CalorieGoal {
		t.Errorf("expected CalorieGoal %d from user, got %d", user.CalorieGoal, ps.CalorieGoal)
	}
}

func TestGetSummary_WeeklyPeriod(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	ps, err := svc.GetSummary(context.Background(), user.ID, "weekly", time.Now())
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	if ps.Period != "weekly" {
		t.Errorf("expected period 'weekly', got '%s'", ps.Period)
	}
}

func TestGetSummary_WeightImpact_Surplus(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	_, tdee := calcBMRTDEE(user)
	// Log more than TDEE to guarantee a surplus
	surplus := tdee + 500
	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{
		FoodName: "Surplus Meal", Calories: surplus, ProteinG: 50,
	})
	if err != nil {
		t.Fatalf("add entry: %v", err)
	}

	ps, err := svc.GetSummary(context.Background(), user.ID, "daily", time.Now())
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	if ps.WeightImpactLbs <= 0 {
		t.Errorf("expected positive WeightImpactLbs for surplus, got %.4f", ps.WeightImpactLbs)
	}
	expected := 500.0 / 3500.0
	if abs(ps.WeightImpactLbs-expected) > 0.001 {
		t.Errorf("expected WeightImpactLbs %.4f, got %.4f", expected, ps.WeightImpactLbs)
	}
}

func TestGetSummary_WeightImpact_Deficit(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	_, tdee := calcBMRTDEE(user)
	// Log less than TDEE to guarantee a deficit
	deficit := tdee - 500
	_, err := svc.AddEntry(context.Background(), user.ID, models.LogEntry{
		FoodName: "Light Meal", Calories: deficit, ProteinG: 30,
	})
	if err != nil {
		t.Fatalf("add entry: %v", err)
	}

	ps, err := svc.GetSummary(context.Background(), user.ID, "daily", time.Now())
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	if ps.WeightImpactLbs >= 0 {
		t.Errorf("expected negative WeightImpactLbs for deficit, got %.4f", ps.WeightImpactLbs)
	}
	expected := -500.0 / 3500.0
	if abs(ps.WeightImpactLbs-expected) > 0.001 {
		t.Errorf("expected WeightImpactLbs %.4f, got %.4f", expected, ps.WeightImpactLbs)
	}
}

func TestGetSummary_NoDivisionByZero(t *testing.T) {
	svc, _, userRepo := newTestLogService()
	user := seedUser(t, userRepo)

	// No entries — calories = 0
	ps, err := svc.GetSummary(context.Background(), user.ID, "daily", time.Now())
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	if ps.ProteinPct != 0 || ps.FatPct != 0 || ps.CarbsPct != 0 {
		t.Errorf("expected all 0%% when calories=0, got P:%.2f F:%.2f C:%.2f",
			ps.ProteinPct, ps.FatPct, ps.CarbsPct)
	}
	if ps.WeightImpactLbs != 0 {
		t.Errorf("expected WeightImpactLbs=0 when no entries, got %.4f", ps.WeightImpactLbs)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
