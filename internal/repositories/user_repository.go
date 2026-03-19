package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/drujensen/calorie-count/internal/models"
)

// ErrNotFound is returned when a record cannot be found.
var ErrNotFound = errors.New("not found")

// UserRepository defines data access for users.
type UserRepository interface {
	Create(ctx context.Context, user models.User) (models.User, error)
	GetByEmail(ctx context.Context, email string) (models.User, error)
	GetByID(ctx context.Context, id int) (models.User, error)
	UpdateProfile(ctx context.Context, userID int, calorieGoal int, weightLbs, targetWeightLbs float64, age int, heightIn float64, sex, weightLossRate, activityLevel string) error
}

type userRepository struct {
	db *sql.DB
}

// NewUserRepository returns a UserRepository backed by the given SQLite database.
func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

// Create inserts a new user and returns the created user with its assigned ID.
func (r *userRepository) Create(ctx context.Context, user models.User) (models.User, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO users (email, password_hash, calorie_goal, current_weight_lbs, age, height_in, sex, target_weight_lbs, weight_loss_rate, activity_level)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.Email, user.PasswordHash, user.CalorieGoal, user.CurrentWeightLbs,
		user.Age, user.HeightIn, user.Sex, user.TargetWeightLbs, user.WeightLossRate, user.ActivityLevel,
	)
	if err != nil {
		return models.User{}, fmt.Errorf("inserting user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return models.User{}, fmt.Errorf("getting last insert id: %w", err)
	}

	return r.GetByID(ctx, int(id))
}

// GetByEmail retrieves a user by their email address.
// Returns ErrNotFound if no user with that email exists.
func (r *userRepository) GetByEmail(ctx context.Context, email string) (models.User, error) {
	var user models.User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, calorie_goal, current_weight_lbs, age, height_in, sex, target_weight_lbs, weight_loss_rate, activity_level, created_at
		 FROM users WHERE email = ?`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CalorieGoal, &user.CurrentWeightLbs,
		&user.Age, &user.HeightIn, &user.Sex, &user.TargetWeightLbs, &user.WeightLossRate, &user.ActivityLevel, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("querying user by email: %w", err)
	}
	return user, nil
}

// GetByID retrieves a user by their ID.
// Returns ErrNotFound if no user with that ID exists.
func (r *userRepository) GetByID(ctx context.Context, id int) (models.User, error) {
	var user models.User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, calorie_goal, current_weight_lbs, age, height_in, sex, target_weight_lbs, weight_loss_rate, activity_level, created_at
		 FROM users WHERE id = ?`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CalorieGoal, &user.CurrentWeightLbs,
		&user.Age, &user.HeightIn, &user.Sex, &user.TargetWeightLbs, &user.WeightLossRate, &user.ActivityLevel, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("querying user by id: %w", err)
	}
	return user, nil
}

// UpdateProfile updates calorie goal, weight, and profile fields for a user.
func (r *userRepository) UpdateProfile(ctx context.Context, userID int, calorieGoal int, weightLbs, targetWeightLbs float64, age int, heightIn float64, sex, weightLossRate, activityLevel string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET calorie_goal = ?, current_weight_lbs = ?, target_weight_lbs = ?, age = ?, height_in = ?, sex = ?, weight_loss_rate = ?, activity_level = ? WHERE id = ?`,
		calorieGoal, weightLbs, targetWeightLbs, age, heightIn, sex, weightLossRate, activityLevel, userID,
	)
	if err != nil {
		return fmt.Errorf("updating user profile: %w", err)
	}
	return nil
}
