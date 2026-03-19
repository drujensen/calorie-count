package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/drujensen/calorie-count/internal/models"
	"github.com/drujensen/calorie-count/internal/repositories"
)

// --- mock repositories ---

type mockUserRepo struct {
	users  map[string]models.User
	nextID int
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[string]models.User), nextID: 1}
}

func (m *mockUserRepo) Create(_ context.Context, user models.User) (models.User, error) {
	if _, exists := m.users[user.Email]; exists {
		return models.User{}, errors.New("email already exists")
	}
	user.ID = m.nextID
	m.nextID++
	m.users[user.Email] = user
	return user, nil
}

func (m *mockUserRepo) GetByEmail(_ context.Context, email string) (models.User, error) {
	user, ok := m.users[email]
	if !ok {
		return models.User{}, repositories.ErrNotFound
	}
	return user, nil
}

func (m *mockUserRepo) GetByID(_ context.Context, id int) (models.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return models.User{}, repositories.ErrNotFound
}

func (m *mockUserRepo) UpdateProfile(_ context.Context, userID int, calorieGoal int, weightLbs float64, targetWeightLbs float64, age int, heightIn float64, sex string, weightLossRate string, activityLevel string) error {
	for email, u := range m.users {
		if u.ID == userID {
			u.CalorieGoal = calorieGoal
			u.CurrentWeightLbs = weightLbs
			m.users[email] = u
			return nil
		}
	}
	return repositories.ErrNotFound
}

type mockSessionRepo struct {
	sessions map[string]models.Session
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{sessions: make(map[string]models.Session)}
}

func (m *mockSessionRepo) Create(_ context.Context, session models.Session) error {
	m.sessions[session.Token] = session
	return nil
}

func (m *mockSessionRepo) GetByToken(_ context.Context, token string) (models.Session, error) {
	s, ok := m.sessions[token]
	if !ok {
		return models.Session{}, repositories.ErrNotFound
	}
	return s, nil
}

func (m *mockSessionRepo) Delete(_ context.Context, token string) error {
	delete(m.sessions, token)
	return nil
}

func (m *mockSessionRepo) DeleteExpired(_ context.Context) error {
	for token, s := range m.sessions {
		if time.Now().After(s.ExpiresAt) {
			delete(m.sessions, token)
		}
	}
	return nil
}

// --- helpers ---

func newTestAuthService() (AuthService, *mockUserRepo, *mockSessionRepo) {
	userRepo := newMockUserRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(userRepo, sessionRepo)
	return svc, userRepo, sessionRepo
}

// --- tests ---

func TestRegister_Success(t *testing.T) {
	svc, _, _ := newTestAuthService()
	user, err := svc.Register(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}
	if user.ID == 0 {
		t.Error("expected non-zero user ID")
	}
	if user.PasswordHash == "" {
		t.Error("expected non-empty password hash")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, _, _ := newTestAuthService()
	_, err := svc.Register(context.Background(), "dup@example.com", "password123")
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	_, err = svc.Register(context.Background(), "dup@example.com", "password123")
	if err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}
}


func TestLogin_Success(t *testing.T) {
	svc, _, _ := newTestAuthService()
	_, err := svc.Register(context.Background(), "login@example.com", "password123")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	session, err := svc.Login(context.Background(), "login@example.com", "password123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if session.Token == "" {
		t.Error("expected non-empty session token")
	}
	if session.ExpiresAt.Before(time.Now()) {
		t.Error("expected session to expire in the future")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _, _ := newTestAuthService()
	_, err := svc.Register(context.Background(), "wp@example.com", "password123")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	_, err = svc.Login(context.Background(), "wp@example.com", "wrongpassword")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	svc, _, _ := newTestAuthService()
	_, err := svc.Login(context.Background(), "nobody@example.com", "password123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestAuthenticate_ValidSession(t *testing.T) {
	svc, _, _ := newTestAuthService()
	_, err := svc.Register(context.Background(), "auth@example.com", "password123")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	session, err := svc.Login(context.Background(), "auth@example.com", "password123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	user, err := svc.Authenticate(context.Background(), session.Token)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if user.Email != "auth@example.com" {
		t.Errorf("expected email auth@example.com, got %s", user.Email)
	}
}

func TestAuthenticate_ExpiredSession(t *testing.T) {
	svc, userRepo, sessionRepo := newTestAuthService()
	_, err := svc.Register(context.Background(), "exp@example.com", "password123")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// Look up the user to get its ID
	user, err := userRepo.GetByEmail(context.Background(), "exp@example.com")
	if err != nil {
		t.Fatalf("get user failed: %v", err)
	}

	// Insert an already-expired session directly
	expiredSession := models.Session{
		Token:     "expired-token-abc123",
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if err := sessionRepo.Create(context.Background(), expiredSession); err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	_, err = svc.Authenticate(context.Background(), "expired-token-abc123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for expired session, got: %v", err)
	}
}
