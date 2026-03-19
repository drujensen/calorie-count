package models

import "time"

// User represents an application user
type User struct {
	ID               int
	Email            string
	PasswordHash     string
	CalorieGoal      int
	CurrentWeightLbs float64
	Age              int
	HeightIn         float64 // total inches
	Sex              string  // "male", "female"
	TargetWeightLbs  float64
	WeightLossRate   string  // "maintain", "lose_1", "lose_2"
	ActivityLevel    string  // "sedentary", "light", "moderate", "very", "extra"
	CreatedAt        time.Time
}

// WeightEntry represents a single logged weight measurement.
type WeightEntry struct {
	ID        int
	UserID    int
	WeightLbs float64
	LoggedAt  time.Time
}

// BMIPoint is one data point on the BMI history chart.
type BMIPoint struct {
	Date      time.Time
	WeightLbs float64
	BMI       float64
}

// WeightPoint is a single weight measurement for the weight/goal chart.
type WeightPoint struct {
	Date      time.Time
	WeightLbs float64
}

// GoalData holds weight history and goal projection for the goal page.
type GoalData struct {
	WeightPoints     []WeightPoint
	CurrentWeightLbs float64
	TargetWeightLbs  float64
	HasTarget        bool
	EstimatedDate    *time.Time // nil if not calculable
	BMR              int        // basal metabolic rate (kcal/day)
	TDEE             int        // total daily energy expenditure (BMR × 1.2 sedentary)
	DailyDeficit     int        // TDEE - CalorieGoal (positive = losing)
	WeeklyLossLbs    float64    // projected weekly weight change (positive = losing)
}

// LogEntry represents a single food log entry
type LogEntry struct {
	ID        int
	UserID    int
	FoodName  string
	Calories  int
	ProteinG  float64
	FatG      float64
	CarbsG    float64
	Amount    float64 // serving size quantity (e.g. 1.5)
	Unit      string  // serving size unit (e.g. "cup", "oz", "g")
	ImagePath string
	Notes     string
	LoggedAt  time.Time
}

// Session represents an authenticated user session
type Session struct {
	Token     string
	UserID    int
	ExpiresAt time.Time
}

// MacroSummary holds aggregated macro totals for a period.
type MacroSummary struct {
	TotalCalories int
	TotalProteinG float64
	TotalFatG     float64
	TotalCarbsG   float64
	Days          int // number of days in the period
}

// PeriodSummary extends MacroSummary with goal and derived metrics.
type PeriodSummary struct {
	Period string // "daily", "weekly", "monthly"
	MacroSummary
	CalorieGoal     int
	ProteinPct      float64 // protein_g * 4 / calories * 100
	FatPct          float64 // fat_g * 9 / calories * 100
	CarbsPct        float64 // carbs_g * 4 / calories * 100
	WeightImpactLbs float64 // (avg_daily_calories - goal) * days / 3500
}
