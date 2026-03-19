//go:build e2e

package e2e

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/drujensen/calorie-count/internal/config"
	"github.com/drujensen/calorie-count/internal/db"
	"github.com/drujensen/calorie-count/internal/handlers"
	"github.com/drujensen/calorie-count/internal/middleware"
	"github.com/drujensen/calorie-count/internal/migrations"
	"github.com/drujensen/calorie-count/internal/repositories"
	"github.com/drujensen/calorie-count/internal/services"
)

// newTestServer spins up the full application with an in-memory SQLite database
// and returns an *httptest.Server. The caller is responsible for calling
// server.Close().
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	cfg := &config.Config{
		Port:         "0",
		Env:          "test",
		DatabasePath: ":memory:",
		OllamaAPIURL: "http://127.0.0.1:0", // unreachable — AI unavailable
		OllamaModel:  "test",
	}

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := db.RunMigrations(database, migrations.FS); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	userRepo := repositories.NewUserRepository(database)
	sessionRepo := repositories.NewSessionRepository(database)
	logRepo := repositories.NewLogRepository(database)
	authSvc := services.NewAuthService(userRepo, sessionRepo)
	logSvc := services.NewLogService(logRepo, userRepo)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	aiSvc := services.NewAIService(cfg, ctx)
	rateLimiter := middleware.NewRateLimiter(ctx, 100) // high limit for tests

	mux := http.NewServeMux()
	handlers.SetupRoutes(mux, authSvc, logSvc, aiSvc, userRepo, false /* not production */, rateLimiter)
	httpHandler := middleware.ApplyMiddleware(mux)

	srv := httptest.NewServer(httpHandler)
	t.Cleanup(srv.Close)
	return srv
}

// newClient returns an http.Client with a cookie jar that does NOT follow
// redirects automatically, so we can inspect each redirect response.
func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// getCSRFToken fetches the CSRF token from the "csrf" cookie set by the server.
func getCSRFToken(client *http.Client, serverURL string) string {
	u, _ := url.Parse(serverURL)
	for _, c := range client.Jar.Cookies(u) {
		if c.Name == "csrf" {
			return c.Value
		}
	}
	return ""
}

// bodyContains reads an HTTP response body and checks whether it contains s.
func bodyContains(t *testing.T, resp *http.Response, s string) bool {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return strings.Contains(string(b), s)
}

// readBody reads and returns the response body as a string.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(b)
}

func TestFullUserJourney(t *testing.T) {
	srv := newTestServer(t)
	client := newClient(t)
	base := srv.URL

	const (
		email    = "testuser@example.com"
		password = "securepassword123"
	)

	// -------------------------------------------------------------------------
	// Step 1: GET /register — fetch CSRF token
	// -------------------------------------------------------------------------
	resp, err := client.Get(base + "/register")
	if err != nil {
		t.Fatalf("GET /register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /register: want 200, got %d", resp.StatusCode)
	}

	csrfToken := getCSRFToken(client, base)
	if csrfToken == "" {
		t.Fatal("no csrf cookie after GET /register")
	}

	// -------------------------------------------------------------------------
	// Step 2: POST /register
	// -------------------------------------------------------------------------
	form := url.Values{
		"email":            {email},
		"password":         {password},
		"confirm_password": {password},
		"_csrf":            {csrfToken},
	}
	resp, err = client.PostForm(base+"/register", form)
	if err != nil {
		t.Fatalf("POST /register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /register: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/" {
		t.Fatalf("POST /register redirect: want /, got %q", loc)
	}

	// Follow the redirect manually so the session cookie is stored.
	resp, err = client.Get(base + "/")
	if err != nil {
		t.Fatalf("GET / after register: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("GET / after register: want 200, got %d\nbody: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// -------------------------------------------------------------------------
	// Step 3: POST /logout to reset state
	// -------------------------------------------------------------------------
	csrfToken = getCSRFToken(client, base)
	resp, err = client.PostForm(base+"/logout", url.Values{"_csrf": {csrfToken}})
	if err != nil {
		t.Fatalf("POST /logout: %v", err)
	}
	resp.Body.Close()

	// -------------------------------------------------------------------------
	// Step 4: GET /login — load page (fresh CSRF token)
	// -------------------------------------------------------------------------
	resp, err = client.Get(base + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /login: want 200, got %d", resp.StatusCode)
	}
	if !bodyContains(t, resp, "Login") {
		t.Fatal("GET /login: body does not contain 'Login'")
	}

	csrfToken = getCSRFToken(client, base)

	// -------------------------------------------------------------------------
	// Step 5: POST /login
	// -------------------------------------------------------------------------
	resp, err = client.PostForm(base+"/login", url.Values{
		"email":    {email},
		"password": {password},
		"_csrf":    {csrfToken},
	})
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /login: want 303, got %d", resp.StatusCode)
	}

	// -------------------------------------------------------------------------
	// Step 6: GET / — protected page must load (session cookie present)
	// -------------------------------------------------------------------------
	resp, err = client.Get(base + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("GET /: want 200, got %d\nbody: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// -------------------------------------------------------------------------
	// Step 7: POST /api/log — add a log entry (form submission with CSRF)
	// -------------------------------------------------------------------------
	csrfToken = getCSRFToken(client, base)
	resp, err = client.PostForm(base+"/api/log", url.Values{
		"food_name": {"Test Apple"},
		"calories":  {"95"},
		"protein_g": {"0.5"},
		"fat_g":     {"0.3"},
		"carbs_g":   {"25"},
		"_csrf":     {csrfToken},
	})
	if err != nil {
		t.Fatalf("POST /api/log: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body := readBody(t, resp)
		t.Fatalf("POST /api/log: want 201, got %d\nbody: %s", resp.StatusCode, body)
	}
	if !bodyContains(t, resp, "Test Apple") {
		t.Fatal("POST /api/log: response body does not contain 'Test Apple'")
	}

	// -------------------------------------------------------------------------
	// Step 8: GET /results — must return 200 and contain "Calories"
	// -------------------------------------------------------------------------
	resp, err = client.Get(base + "/results")
	if err != nil {
		t.Fatalf("GET /results: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /results: want 200, got %d", resp.StatusCode)
	}
	if !bodyContains(t, resp, "Calories") {
		t.Fatal("GET /results: body does not contain 'Calories'")
	}

	// -------------------------------------------------------------------------
	// Step 9: GET /settings — must return 200
	// -------------------------------------------------------------------------
	resp, err = client.Get(base + "/settings")
	if err != nil {
		t.Fatalf("GET /settings: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /settings: want 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// -------------------------------------------------------------------------
	// Step 10: POST /settings — update calorie goal
	// -------------------------------------------------------------------------
	csrfToken = getCSRFToken(client, base)
	resp, err = client.PostForm(base+"/settings", url.Values{
		"calorie_goal":       {"1800"},
		"current_weight_lbs": {"170"},
		"_csrf":              {csrfToken},
	})
	if err != nil {
		t.Fatalf("POST /settings: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /settings: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/settings?saved=1" {
		t.Fatalf("POST /settings redirect: want /settings?saved=1, got %q", loc)
	}

	// -------------------------------------------------------------------------
	// Step 11: POST /logout
	// -------------------------------------------------------------------------
	csrfToken = getCSRFToken(client, base)
	resp, err = client.PostForm(base+"/logout", url.Values{"_csrf": {csrfToken}})
	if err != nil {
		t.Fatalf("POST /logout: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /logout: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("POST /logout redirect: want /login, got %q", loc)
	}

	// -------------------------------------------------------------------------
	// Step 12: GET / without session — must redirect to /login
	// -------------------------------------------------------------------------
	resp, err = client.Get(base + "/")
	if err != nil {
		t.Fatalf("GET / unauthenticated: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("GET / unauthenticated: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("GET / unauthenticated redirect: want /login, got %q", loc)
	}
}
