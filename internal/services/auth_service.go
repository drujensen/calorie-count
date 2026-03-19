package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/drujensen/calorie-count/internal/models"
	"github.com/drujensen/calorie-count/internal/repositories"
	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials is returned when login credentials are wrong.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// AuthService handles registration, login, logout, and session authentication.
type AuthService interface {
	Register(ctx context.Context, email, password string) (models.User, error)
	Login(ctx context.Context, email, password string) (models.Session, error)
	Logout(ctx context.Context, token string) error
	Authenticate(ctx context.Context, token string) (models.User, error)
}

type authService struct {
	users    repositories.UserRepository
	sessions repositories.SessionRepository
}

// NewAuthService creates an AuthService with the given repositories.
func NewAuthService(users repositories.UserRepository, sessions repositories.SessionRepository) AuthService {
	return &authService{users: users, sessions: sessions}
}

// Register creates a new user account after validating input and hashing the password.
func (s *authService) Register(ctx context.Context, email, password string) (models.User, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return models.User{}, errors.New("email is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return models.User{}, fmt.Errorf("hashing password: %w", err)
	}

	user := models.User{
		Email:        email,
		PasswordHash: string(hash),
		CalorieGoal:  2000,
	}

	created, err := s.users.Create(ctx, user)
	if err != nil {
		return models.User{}, fmt.Errorf("creating user: %w", err)
	}
	return created, nil
}

// Login verifies credentials and creates a session with a 30-day expiry.
func (s *authService) Login(ctx context.Context, email, password string) (models.Session, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, repositories.ErrNotFound) {
		return models.Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return models.Session{}, fmt.Errorf("looking up user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return models.Session{}, ErrInvalidCredentials
	}

	token, err := generateToken()
	if err != nil {
		return models.Session{}, fmt.Errorf("generating session token: %w", err)
	}

	session := models.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return models.Session{}, fmt.Errorf("creating session: %w", err)
	}
	return session, nil
}

// Logout deletes the session identified by the given token.
func (s *authService) Logout(ctx context.Context, token string) error {
	if err := s.sessions.Delete(ctx, token); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// Authenticate looks up the session token and returns the owning user.
// Returns ErrInvalidCredentials if the session is missing or expired.
func (s *authService) Authenticate(ctx context.Context, token string) (models.User, error) {
	session, err := s.sessions.GetByToken(ctx, token)
	if errors.Is(err, repositories.ErrNotFound) {
		return models.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return models.User{}, fmt.Errorf("getting session: %w", err)
	}

	if time.Now().After(session.ExpiresAt) {
		return models.User{}, ErrInvalidCredentials
	}

	user, err := s.users.GetByID(ctx, session.UserID)
	if errors.Is(err, repositories.ErrNotFound) {
		return models.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return models.User{}, fmt.Errorf("getting user: %w", err)
	}
	return user, nil
}

// generateToken produces a 32-byte crypto/rand token, hex-encoded.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
