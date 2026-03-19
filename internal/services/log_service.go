package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/drujensen/calorie-count/internal/models"
	"github.com/drujensen/calorie-count/internal/repositories"
)

// ErrInvalidEntry is returned when a log entry fails validation.
var ErrInvalidEntry = errors.New("invalid log entry")

// LogService handles business logic for food log entries.
type LogService interface {
	AddEntry(ctx context.Context, userID int, entry models.LogEntry) (models.LogEntry, error)
	GetEntry(ctx context.Context, userID int, entryID int) (models.LogEntry, error)
	UpdateEntry(ctx context.Context, userID int, entryID int, entry models.LogEntry) (models.LogEntry, error)
	DeleteEntry(ctx context.Context, userID int, entryID int) error
	GetEntriesForDate(ctx context.Context, userID int, date time.Time) ([]models.LogEntry, error)
	GetSummaryForDate(ctx context.Context, userID int, date time.Time) (models.MacroSummary, error)
	GetSummary(ctx context.Context, userID int, period string) (models.PeriodSummary, error)
}

type logService struct {
	logs  repositories.LogRepository
	users repositories.UserRepository
}

// NewLogService creates a LogService with the given repositories.
func NewLogService(logs repositories.LogRepository, users repositories.UserRepository) LogService {
	return &logService{logs: logs, users: users}
}

// AddEntry validates and inserts a new log entry for the given user.
func (s *logService) AddEntry(ctx context.Context, userID int, entry models.LogEntry) (models.LogEntry, error) {
	if strings.TrimSpace(entry.FoodName) == "" {
		return models.LogEntry{}, fmt.Errorf("%w: food name is required", ErrInvalidEntry)
	}
	if entry.Calories < 0 || entry.Calories > 10000 {
		return models.LogEntry{}, fmt.Errorf("%w: calories must be between 0 and 10000", ErrInvalidEntry)
	}
	if entry.ProteinG < 0 {
		return models.LogEntry{}, fmt.Errorf("%w: protein must be >= 0", ErrInvalidEntry)
	}
	if entry.FatG < 0 {
		return models.LogEntry{}, fmt.Errorf("%w: fat must be >= 0", ErrInvalidEntry)
	}
	if entry.CarbsG < 0 {
		return models.LogEntry{}, fmt.Errorf("%w: carbs must be >= 0", ErrInvalidEntry)
	}

	entry.UserID = userID
	created, err := s.logs.Create(ctx, entry)
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("creating log entry: %w", err)
	}
	return created, nil
}

// GetEntry returns a single log entry, enforcing ownership.
func (s *logService) GetEntry(ctx context.Context, userID int, entryID int) (models.LogEntry, error) {
	entry, err := s.logs.GetByID(ctx, entryID, userID)
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("getting log entry: %w", err)
	}
	return entry, nil
}

// UpdateEntry replaces the nutritional fields of an existing entry, enforcing ownership.
func (s *logService) UpdateEntry(ctx context.Context, userID int, entryID int, entry models.LogEntry) (models.LogEntry, error) {
	if strings.TrimSpace(entry.FoodName) == "" {
		return models.LogEntry{}, fmt.Errorf("%w: food name is required", ErrInvalidEntry)
	}
	if entry.Calories < 0 || entry.Calories > 10000 {
		return models.LogEntry{}, fmt.Errorf("%w: calories must be between 0 and 10000", ErrInvalidEntry)
	}
	entry.ID = entryID
	entry.UserID = userID
	updated, err := s.logs.Update(ctx, entry)
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("updating log entry: %w", err)
	}
	return updated, nil
}

// DeleteEntry removes an entry, enforcing that it belongs to userID.
func (s *logService) DeleteEntry(ctx context.Context, userID int, entryID int) error {
	if err := s.logs.Delete(ctx, entryID, userID); err != nil {
		return fmt.Errorf("deleting log entry: %w", err)
	}
	return nil
}

// GetEntriesForDate returns all entries logged by userID on the given date.
func (s *logService) GetEntriesForDate(ctx context.Context, userID int, date time.Time) ([]models.LogEntry, error) {
	entries, err := s.logs.ListByUserAndDate(ctx, userID, date)
	if err != nil {
		return nil, fmt.Errorf("listing entries for date: %w", err)
	}
	return entries, nil
}

// GetSummaryForDate returns macro totals for the given date.
func (s *logService) GetSummaryForDate(ctx context.Context, userID int, date time.Time) (models.MacroSummary, error) {
	d := date.In(time.Local)
	from := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.Local)
	to := from.Add(24*time.Hour - time.Nanosecond)

	summary, err := s.logs.SumByPeriod(ctx, userID, from, to)
	if err != nil {
		return models.MacroSummary{}, fmt.Errorf("summing entries for date: %w", err)
	}
	return summary, nil
}

// GetSummary returns a PeriodSummary for "daily", "weekly", or "monthly".
func (s *logService) GetSummary(ctx context.Context, userID int, period string) (models.PeriodSummary, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	var from, to time.Time
	switch period {
	case "weekly":
		from = today.AddDate(0, 0, -6)
		to = today.Add(24*time.Hour - time.Nanosecond)
	case "monthly":
		from = today.AddDate(0, 0, -29)
		to = today.Add(24*time.Hour - time.Nanosecond)
	default:
		period = "daily"
		from = today
		to = today.Add(24*time.Hour - time.Nanosecond)
	}

	macro, err := s.logs.SumByPeriod(ctx, userID, from, to)
	if err != nil {
		return models.PeriodSummary{}, fmt.Errorf("summing period entries: %w", err)
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return models.PeriodSummary{}, fmt.Errorf("getting user: %w", err)
	}

	ps := models.PeriodSummary{
		Period:       period,
		MacroSummary: macro,
		CalorieGoal:  user.CalorieGoal,
	}

	if macro.TotalCalories > 0 {
		total := float64(macro.TotalCalories)
		ps.ProteinPct = macro.TotalProteinG * 4 / total * 100
		ps.FatPct = macro.TotalFatG * 9 / total * 100
		ps.CarbsPct = macro.TotalCarbsG * 4 / total * 100
	}

	if macro.Days > 0 && user.CalorieGoal > 0 {
		avgDaily := float64(macro.TotalCalories) / float64(macro.Days)
		surplus := avgDaily - float64(user.CalorieGoal)
		ps.WeightImpactLbs = surplus * float64(macro.Days) / 3500
	}

	return ps, nil
}
