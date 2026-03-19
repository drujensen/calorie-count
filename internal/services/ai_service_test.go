package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/drujensen/calorie-count/internal/config"
)

// newTestAIService creates an AIService pointed at the given mock server URL.
func newTestAIService(serverURL string) AIService {
	cfg := &config.Config{
		OllamaAPIURL: serverURL,
		OllamaModel:  "llama-vision:latest",
	}
	ctx, cancel := context.WithCancel(context.Background())
	_ = cancel // caller responsible for lifetime; we cancel immediately for tests
	cancel()
	return NewAIService(cfg, ctx)
}

// --- helpers ---

func makeTextResponse(content string) ollamaChatResponse {
	return ollamaChatResponse{
		Model:   "llama-vision:latest",
		Message: ollamaMessage{Role: "assistant", Content: content},
		Done:    true,
	}
}

func makeLogJSONResponse(foodName string, calories int) ollamaChatResponse {
	content, _ := json.Marshal(map[string]interface{}{
		"log": map[string]interface{}{
			"food_name": foodName,
			"calories":  calories,
			"protein_g": 10.0,
			"fat_g":     5.0,
			"carbs_g":   30.0,
		},
	})
	return makeTextResponse(string(content))
}

func serveJSON(t *testing.T, v interface{}) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(v); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	}
}

// --- IsAvailable tests ---

func TestAIService_IsAvailable_WhenOllamaUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	svc := newTestAIService(srv.URL)
	if !svc.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable to return true when server is up")
	}
}

func TestAIService_IsAvailable_WhenOllamaDown(t *testing.T) {
	// Use a server that we immediately close so connections fail.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	svc := newTestAIService(url)
	if svc.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable to return false when server is down")
	}
}

// --- StartSession tests ---

func TestAIService_StartSession_ImmediateToolCall(t *testing.T) {
	resp := makeLogJSONResponse("Apple", 95)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.WriteHeader(http.StatusOK)
		case "/api/chat":
			serveJSON(t, resp)(w, r)
		}
	}))
	defer srv.Close()

	svc := newTestAIService(srv.URL)
	sessionID, msg, done, err := svc.StartSession(context.Background(), 1, "base64data", "image/jpeg", "", time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID == "" {
		t.Error("expected non-empty session ID")
	}
	if msg == "" {
		t.Error("expected non-empty confirmation message")
	}
	if !done {
		t.Error("expected done=true after immediate tool call")
	}

	// Session should be marked done and hold the entry.
	svc2 := svc.(*aiServiceImpl)
	svc2.mu.RLock()
	conv := svc2.sessions[sessionID]
	svc2.mu.RUnlock()

	if conv == nil {
		t.Fatal("session not found")
	}
	if !conv.done {
		t.Error("expected session to be done after immediate tool call")
	}
	if conv.result == nil {
		t.Error("expected result to be set")
	}
	if conv.result.FoodName != "Apple" {
		t.Errorf("expected food name 'Apple', got '%s'", conv.result.FoodName)
	}
	if conv.result.Calories != 95 {
		t.Errorf("expected 95 calories, got %d", conv.result.Calories)
	}
}

func TestAIService_StartSession_ClarifyingQuestion(t *testing.T) {
	resp := makeTextResponse("How large is the serving size?")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.WriteHeader(http.StatusOK)
		case "/api/chat":
			serveJSON(t, resp)(w, r)
		}
	}))
	defer srv.Close()

	svc := newTestAIService(srv.URL)
	sessionID, msg, done, err := svc.StartSession(context.Background(), 1, "base64data", "image/jpeg", "", time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID == "" {
		t.Error("expected non-empty session ID")
	}
	if msg != "How large is the serving size?" {
		t.Errorf("expected clarifying question, got: %s", msg)
	}
	if done {
		t.Error("expected done=false after clarifying question")
	}

	// Session should NOT be done yet.
	svc2 := svc.(*aiServiceImpl)
	svc2.mu.RLock()
	conv := svc2.sessions[sessionID]
	svc2.mu.RUnlock()

	if conv == nil {
		t.Fatal("session not found")
	}
	if conv.done {
		t.Error("expected session to NOT be done after clarifying question")
	}
}

// --- Continue tests ---

func TestAIService_Continue_AnswerLeadsToToolCall(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.WriteHeader(http.StatusOK)
		case "/api/chat":
			callCount++
			if callCount == 1 {
				// First call: return a clarifying question
				serveJSON(t, makeTextResponse("How large is the serving size?"))(w, r)
			} else {
				// Second call: return a tool call
				serveJSON(t, makeLogJSONResponse("Banana", 105))(w, r)
			}
		}
	}))
	defer srv.Close()

	svc := newTestAIService(srv.URL)

	// StartSession — gets clarifying question
	sessionID, _, _, err := svc.StartSession(context.Background(), 1, "base64data", "image/jpeg", "", time.Time{})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}

	// Continue — answer leads to tool call
	msg, done, entry, err := svc.Continue(context.Background(), sessionID, "One medium banana")
	if err != nil {
		t.Fatalf("Continue error: %v", err)
	}
	if !done {
		t.Error("expected done=true after tool call")
	}
	if entry == nil {
		t.Fatal("expected entry to be non-nil")
	}
	if entry.FoodName != "Banana" {
		t.Errorf("expected food name 'Banana', got '%s'", entry.FoodName)
	}
	if entry.Calories != 105 {
		t.Errorf("expected 105 calories, got %d", entry.Calories)
	}
	if msg == "" {
		t.Error("expected non-empty confirmation message")
	}
}

func TestAIService_Continue_UnknownSession(t *testing.T) {
	svc := newTestAIService("http://localhost:99999")
	_, _, _, err := svc.Continue(context.Background(), "nonexistent", "answer")
	if err == nil {
		t.Error("expected error for unknown session ID")
	}
}

// --- Session cleanup test ---

func TestAIService_SessionCleanup(t *testing.T) {
	cfg := &config.Config{
		OllamaAPIURL: "http://localhost:11434",
		OllamaModel:  "llama-vision:latest",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := NewAIService(cfg, ctx).(*aiServiceImpl)

	// Manually insert a stale session (older than 30 min).
	svc.mu.Lock()
	svc.sessions["old-session"] = &conversation{
		userID:    1,
		createdAt: time.Now().Add(-31 * time.Minute),
	}
	svc.sessions["new-session"] = &conversation{
		userID:    1,
		createdAt: time.Now(),
	}
	svc.mu.Unlock()

	// Run cleanup directly.
	svc.removeExpiredSessions()

	svc.mu.RLock()
	_, oldExists := svc.sessions["old-session"]
	_, newExists := svc.sessions["new-session"]
	svc.mu.RUnlock()

	if oldExists {
		t.Error("expected old session to be removed")
	}
	if !newExists {
		t.Error("expected new session to remain")
	}
}
