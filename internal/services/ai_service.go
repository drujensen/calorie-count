package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/drujensen/calorie-count/internal/config"
	"github.com/drujensen/calorie-count/internal/models"
)

// AIService manages AI-driven food photo logging sessions via Ollama.
type AIService interface {
	// StartSession begins a logging session. imageBase64 may be empty for
	// text-only descriptions. description may be empty for photo-only.
	// At least one of imageBase64 or description must be non-empty.
	// logDate overrides the LoggedAt timestamp on the resulting entry; zero means now.
	StartSession(ctx context.Context, userID int, imageBase64, mimeType, description string, logDate time.Time) (sessionID string, message string, done bool, err error)
	Continue(ctx context.Context, sessionID string, userAnswer string) (message string, done bool, entry *models.LogEntry, err error)
	// GetResult returns the LogEntry for a completed session, or nil if not done.
	GetResult(sessionID string) *models.LogEntry
	// CorrectEntry sends a natural-language correction for an existing entry
	// and returns the revised LogEntry.
	CorrectEntry(ctx context.Context, original models.LogEntry, correction string) (*models.LogEntry, error)
	IsAvailable(ctx context.Context) bool
}

// conversation holds the in-memory state for one AI session.
type conversation struct {
	userID    int
	logDate   time.Time // override logged_at; zero means use current time
	messages  []ollamaMessage
	createdAt time.Time
	result    *models.LogEntry
	done      bool
}

type aiServiceImpl struct {
	cfg        *config.Config
	httpClient *http.Client
	availClt   *http.Client // short-timeout client for availability checks
	sessions   map[string]*conversation
	mu         sync.RWMutex
}

// --- Ollama API structures ---

type ollamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaChatResponse struct {
	Model   string        `json:"model"`
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

const systemPrompt = `You are a nutrition assistant helping track calorie intake. When shown a food photo, identify the food and estimate its nutritional content.

If you need clarification about preparation method or specific ingredients, ask ONE short friendly question at a time. Do not ask more than 2 clarifying questions total. Always make a reasonable estimate for portion size rather than asking about it.

When you have enough information, respond with ONLY a JSON object in this exact format (no other text):
{"log":{"food_name":"...","calories":0,"protein_g":0.0,"fat_g":0.0,"carbs_g":0.0,"amount":1.0,"unit":"serving"}}

The amount field is a number and unit is a string such as "cup", "oz", "g", "ml", "slice", "piece", "tbsp", "tsp", or "serving".

IMPORTANT: Always provide realistic estimates for ALL of protein_g, fat_g, and carbs_g — never leave them at 0 unless the food truly has none of that macro. The calories should approximately equal (protein_g × 4) + (fat_g × 9) + (carbs_g × 4). Use standard nutrition references to guide your estimates.
Until you are ready to log, respond with only a plain conversational question or statement. Never mix JSON with other text.`

// NewAIService creates an AIService and starts a background cleanup goroutine.
// The goroutine stops when ctx is cancelled.
func NewAIService(cfg *config.Config, ctx context.Context) AIService {
	svc := &aiServiceImpl{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
		availClt: &http.Client{
			Timeout: 2 * time.Second,
		},
		sessions: make(map[string]*conversation),
	}

	go svc.cleanupLoop(ctx)
	return svc
}

// cleanupLoop removes sessions older than 30 minutes every 5 minutes.
func (s *aiServiceImpl) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.removeExpiredSessions()
		}
	}
}

func (s *aiServiceImpl) removeExpiredSessions() {
	cutoff := time.Now().Add(-30 * time.Minute)
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, conv := range s.sessions {
		if conv.createdAt.Before(cutoff) {
			delete(s.sessions, id)
		}
	}
}

// IsAvailable checks whether the Ollama server is reachable.
func (s *aiServiceImpl) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.OllamaAPIURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := s.availClt.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close() //nolint:errcheck
	return resp.StatusCode == http.StatusOK
}

// StartSession begins a food logging session with an optional image and/or text description.
// At least one of imageBase64 or description must be non-empty.
// logDate, if non-zero, is stored on the session and stamped onto the resulting entry.
func (s *aiServiceImpl) StartSession(ctx context.Context, userID int, imageBase64, mimeType, description string, logDate time.Time) (string, string, bool, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return "", "", false, fmt.Errorf("generating session ID: %w", err)
	}

	userMsg := ollamaMessage{Role: "user"}
	if imageBase64 != "" && description != "" {
		userMsg.Content = "Please identify the food in this image and help me log it. " + description
		userMsg.Images = []string{imageBase64}
	} else if imageBase64 != "" {
		userMsg.Content = "Please identify the food in this image and help me log it."
		userMsg.Images = []string{imageBase64}
	} else {
		userMsg.Content = description
	}

	msgs := []ollamaMessage{
		{Role: "system", Content: systemPrompt},
		userMsg,
	}

	resp, err := s.chat(ctx, msgs)
	if err != nil {
		return "", "", false, fmt.Errorf("calling Ollama: %w", err)
	}

	// Append the assistant response to history.
	msgs = append(msgs, resp.Message)

	conv := &conversation{
		userID:    userID,
		logDate:   logDate,
		messages:  msgs,
		createdAt: time.Now(),
	}

	// Check if the model responded with a JSON log entry.
	if entry, confirmMsg, ok := parseLogJSON(resp.Message.Content, userID); ok {
		if !logDate.IsZero() {
			entry.LoggedAt = logDate
		}
		conv.result = entry
		conv.done = true
		s.mu.Lock()
		s.sessions[sessionID] = conv
		s.mu.Unlock()
		return sessionID, confirmMsg, true, nil
	}

	s.mu.Lock()
	s.sessions[sessionID] = conv
	s.mu.Unlock()

	return sessionID, resp.Message.Content, false, nil
}

// Continue sends a user answer back to Ollama and returns the next message.
// When done=true, entry is the LogEntry that the caller should persist.
func (s *aiServiceImpl) Continue(ctx context.Context, sessionID string, userAnswer string) (string, bool, *models.LogEntry, error) {
	s.mu.RLock()
	conv, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return "", false, nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Session already completed (tool already called in StartSession).
	if conv.done {
		return "Entry already logged.", true, conv.result, nil
	}

	// Append the user reply.
	conv.messages = append(conv.messages, ollamaMessage{
		Role:    "user",
		Content: userAnswer,
	})

	resp, err := s.chat(ctx, conv.messages)
	if err != nil {
		return "", false, nil, fmt.Errorf("calling Ollama: %w", err)
	}

	conv.messages = append(conv.messages, resp.Message)

	if entry, confirmMsg, ok := parseLogJSON(resp.Message.Content, conv.userID); ok {
		if !conv.logDate.IsZero() {
			entry.LoggedAt = conv.logDate
		}
		conv.result = entry
		conv.done = true
		return confirmMsg, true, entry, nil
	}

	return resp.Message.Content, false, nil, nil
}

// GetResult returns the stored LogEntry for a completed session, or nil.
func (s *aiServiceImpl) GetResult(sessionID string) *models.LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conv, ok := s.sessions[sessionID]
	if !ok || !conv.done {
		return nil
	}
	return conv.result
}

// CorrectEntry sends a natural-language correction for an existing log entry
// and returns the revised LogEntry from the AI.
func (s *aiServiceImpl) CorrectEntry(ctx context.Context, original models.LogEntry, correction string) (*models.LogEntry, error) {
	amountStr := ""
	if original.Amount > 0 && original.Unit != "" {
		amountStr = fmt.Sprintf(", %.4g %s", original.Amount, original.Unit)
	}
	userMsg := fmt.Sprintf(
		`The user previously logged this food entry:
- Food: %s%s
- Calories: %d
- Protein: %.1fg
- Fat: %.1fg
- Carbs: %.1fg

The user wants to correct it with this note: %s

Apply the correction and return the updated entry as JSON.`,
		original.FoodName, amountStr,
		original.Calories, original.ProteinG, original.FatG, original.CarbsG,
		correction,
	)

	const correctPrompt = `You are a nutrition assistant. The user is correcting a previously logged food entry. Apply their correction and immediately return updated nutritional data as JSON. Do NOT ask clarifying questions — make your best estimate and return the JSON now.

IMPORTANT: Always provide realistic estimates for ALL of protein_g, fat_g, and carbs_g — never leave them at 0 unless the food truly has none of that macro. The calories should approximately equal (protein_g × 4) + (fat_g × 9) + (carbs_g × 4).

Respond with only this JSON, filling in the correct values:
{"log": {"food_name": "string", "calories": integer, "protein_g": float, "fat_g": float, "carbs_g": float, "amount": float, "unit": "string"}}`

	msgs := []ollamaMessage{
		{Role: "system", Content: correctPrompt},
		{Role: "user", Content: userMsg},
	}

	resp, err := s.chat(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("calling Ollama: %w", err)
	}

	entry, _, ok := parseLogJSON(resp.Message.Content, original.UserID)
	if !ok {
		return nil, fmt.Errorf("AI did not return valid nutritional JSON: %s", resp.Message.Content)
	}
	return entry, nil
}

// parseLogJSON checks if the model response contains a JSON log entry.
// Returns the entry, a confirmation message, and true if found.
func parseLogJSON(content string, userID int) (*models.LogEntry, string, bool) {
	var wrapper struct {
		Log struct {
			FoodName string  `json:"food_name"`
			Calories int     `json:"calories"`
			ProteinG float64 `json:"protein_g"`
			FatG     float64 `json:"fat_g"`
			CarbsG   float64 `json:"carbs_g"`
			Amount   float64 `json:"amount"`
			Unit     string  `json:"unit"`
		} `json:"log"`
	}
	// Find the first '{' to handle any accidental leading text.
	start := strings.Index(content, "{")
	if start == -1 {
		return nil, "", false
	}
	if err := json.Unmarshal([]byte(content[start:]), &wrapper); err != nil {
		return nil, "", false
	}
	if wrapper.Log.FoodName == "" {
		return nil, "", false
	}
	entry := &models.LogEntry{
		UserID:   userID,
		FoodName: wrapper.Log.FoodName,
		Calories: wrapper.Log.Calories,
		ProteinG: wrapper.Log.ProteinG,
		FatG:     wrapper.Log.FatG,
		CarbsG:   wrapper.Log.CarbsG,
		Amount:   wrapper.Log.Amount,
		Unit:     wrapper.Log.Unit,
	}
	msg := fmt.Sprintf("Got it! Logging %s (%d cal).", wrapper.Log.FoodName, wrapper.Log.Calories)
	return entry, msg, true
}

// chat sends a request to the Ollama /api/chat endpoint and returns the response.
func (s *aiServiceImpl) chat(ctx context.Context, messages []ollamaMessage) (*ollamaChatResponse, error) {
	reqBody := ollamaChatRequest{
		Model:    s.cfg.OllamaModel,
		Messages: messages,
		Stream:   false,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.OllamaAPIURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		slog.Error("Ollama error", "status", resp.StatusCode, "body", string(body), "model", s.cfg.OllamaModel, "url", s.cfg.OllamaAPIURL)
		return nil, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &chatResp, nil
}

// generateSessionID returns a hex-encoded 8-byte random session ID.
func generateSessionID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
