package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/drujensen/calorie-count/internal/models"
	"github.com/drujensen/calorie-count/internal/repositories"
)

// WeightService handles business logic for weight tracking and BMI.
type WeightService interface {
	LogWeight(ctx context.Context, userID int, weightLbs float64) (models.WeightEntry, error)
	GetBMIHistory(ctx context.Context, userID int) (points []models.BMIPoint, currentBMI float64, category string, err error)
	GetGoalData(ctx context.Context, userID int, calorieGoal int) (models.GoalData, error)
}

type weightService struct {
	weights repositories.WeightRepository
	users   repositories.UserRepository
}

// NewWeightService creates a WeightService with the given repositories.
func NewWeightService(weights repositories.WeightRepository, users repositories.UserRepository) WeightService {
	return &weightService{weights: weights, users: users}
}

// LogWeight records a new weight entry. It also updates the user's current_weight_lbs
// so calorie impact projections stay current.
func (s *weightService) LogWeight(ctx context.Context, userID int, weightLbs float64) (models.WeightEntry, error) {
	if weightLbs <= 0 || weightLbs > 1500 {
		return models.WeightEntry{}, fmt.Errorf("weight must be between 0 and 1500 lbs")
	}

	entry, err := s.weights.AddEntry(ctx, userID, weightLbs)
	if err != nil {
		return models.WeightEntry{}, fmt.Errorf("logging weight: %w", err)
	}

	// Sync current_weight_lbs on the user profile.
	user, err := s.users.GetByID(ctx, userID)
	if err == nil {
		_ = s.users.UpdateProfile(ctx, userID, user.CalorieGoal, weightLbs, user.TargetWeightLbs, user.Age, user.HeightIn, user.Sex, user.WeightLossRate, user.ActivityLevel)
	}

	return entry, nil
}

// GetBMIHistory returns BMI data points for the last 90 entries, the current BMI,
// and the BMI category label.
func (s *weightService) GetBMIHistory(ctx context.Context, userID int) ([]models.BMIPoint, float64, string, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, 0, "", fmt.Errorf("getting user: %w", err)
	}

	entries, err := s.weights.ListByUser(ctx, userID, 90)
	if err != nil {
		return nil, 0, "", fmt.Errorf("listing weight entries: %w", err)
	}

	// Reverse so points are oldest → newest for the chart.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	points := make([]models.BMIPoint, 0, len(entries))
	for _, e := range entries {
		bmi := calcBMI(e.WeightLbs, user.HeightIn)
		if bmi > 0 {
			points = append(points, models.BMIPoint{
				Date:      e.LoggedAt,
				WeightLbs: e.WeightLbs,
				BMI:       bmi,
			})
		}
	}

	// Current BMI: prefer the latest logged weight, fall back to profile weight.
	var currentBMI float64
	latest, err := s.weights.GetLatest(ctx, userID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, 0, "", fmt.Errorf("getting latest weight: %w", err)
	}
	if err == nil {
		currentBMI = calcBMI(latest.WeightLbs, user.HeightIn)
	} else if user.CurrentWeightLbs > 0 {
		currentBMI = calcBMI(user.CurrentWeightLbs, user.HeightIn)
	}

	return points, math.Round(currentBMI*10) / 10, bmiCategory(currentBMI), nil
}

// GetGoalData returns weight history, goal projection, and metabolic estimates.
func (s *weightService) GetGoalData(ctx context.Context, userID int, calorieGoal int) (models.GoalData, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return models.GoalData{}, fmt.Errorf("getting user: %w", err)
	}

	entries, err := s.weights.ListByUser(ctx, userID, 90)
	if err != nil {
		return models.GoalData{}, fmt.Errorf("listing weights: %w", err)
	}

	// Reverse to oldest→newest
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	points := make([]models.WeightPoint, 0, len(entries))
	for _, e := range entries {
		points = append(points, models.WeightPoint{Date: e.LoggedAt, WeightLbs: e.WeightLbs})
	}

	// Current weight: prefer latest logged, fall back to profile
	currentWeight := user.CurrentWeightLbs
	latest, latErr := s.weights.GetLatest(ctx, userID)
	if latErr == nil {
		currentWeight = latest.WeightLbs
	}

	// Use the resolved current weight for BMR/TDEE so the calculation reflects
	// actual body weight, not a potentially stale value in the profile.
	userForCalc := user
	userForCalc.CurrentWeightLbs = currentWeight
	bmr, tdee := calcBMRTDEE(userForCalc)
	dailyDeficit := tdee - calorieGoal
	weeklyLoss := float64(dailyDeficit) * 7.0 / 3500.0

	gd := models.GoalData{
		WeightPoints:     points,
		CurrentWeightLbs: currentWeight,
		TargetWeightLbs:  user.TargetWeightLbs,
		HasTarget:        user.TargetWeightLbs > 0,
		BMR:              bmr,
		TDEE:             tdee,
		DailyDeficit:     dailyDeficit,
		WeeklyLossLbs:    math.Round(weeklyLoss*10) / 10,
	}

	if gd.HasTarget && weeklyLoss > 0 && currentWeight > user.TargetWeightLbs {
		lbsToLose := currentWeight - user.TargetWeightLbs
		daysToGoal := lbsToLose / weeklyLoss * 7
		est := time.Now().AddDate(0, 0, int(math.Ceil(daysToGoal)))
		gd.EstimatedDate = &est
	} else if gd.HasTarget && weeklyLoss < 0 && currentWeight < user.TargetWeightLbs {
		// Gaining toward target
		lbsToGain := user.TargetWeightLbs - currentWeight
		daysToGoal := lbsToGain / math.Abs(weeklyLoss) * 7
		est := time.Now().AddDate(0, 0, int(math.Ceil(daysToGoal)))
		gd.EstimatedDate = &est
	}

	return gd, nil
}

// activityMultiplier returns the TDEE multiplier for the given activity level.
func activityMultiplier(level string) float64 {
	switch level {
	case "light":
		return 1.375
	case "moderate":
		return 1.55
	case "very":
		return 1.725
	case "extra":
		return 1.9
	default: // "sedentary" or empty
		return 1.2
	}
}

// calcBMRTDEE computes BMR (Mifflin-St Jeor) and TDEE (BMR × activity multiplier).
// Returns 0 if profile is incomplete.
func calcBMRTDEE(user models.User) (bmr int, tdee int) {
	if user.HeightIn <= 0 || user.CurrentWeightLbs <= 0 || user.Age <= 0 {
		return 0, 0
	}
	weightKg := user.CurrentWeightLbs / 2.2046
	heightCm := user.HeightIn * 2.54
	base := 10*weightKg + 6.25*heightCm - 5*float64(user.Age)
	var raw float64
	if user.Sex == "male" {
		raw = base + 5
	} else {
		raw = base - 161
	}
	bmr = int(math.Round(raw))
	tdee = int(math.Round(raw * activityMultiplier(user.ActivityLevel)))
	return bmr, tdee
}

// calcBMI computes BMI from weight in lbs and height in total inches.
// Returns 0 if height is zero (profile incomplete).
func calcBMI(weightLbs, heightIn float64) float64 {
	if heightIn <= 0 {
		return 0
	}
	return 703 * weightLbs / (heightIn * heightIn)
}

// bmiCategory returns the standard WHO BMI category label.
func bmiCategory(bmi float64) string {
	switch {
	case bmi <= 0:
		return ""
	case bmi < 18.5:
		return "Underweight"
	case bmi < 25:
		return "Normal"
	case bmi < 30:
		return "Overweight"
	case bmi < 35:
		return "Obese (Class I)"
	case bmi < 40:
		return "Obese (Class II)"
	default:
		return "Obese (Class III)"
	}
}
