package api

import (
	"encoding/base64"
	"encoding/json"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/drujensen/calorie-count/internal/middleware"
	"github.com/drujensen/calorie-count/internal/models"
	"github.com/drujensen/calorie-count/internal/services"
	"github.com/drujensen/calorie-count/internal/templates"
)

// APIHandler handles JSON API requests.
type APIHandler struct {
	logSvc    services.LogService
	aiSvc     services.AIService
	weightSvc services.WeightService
	entryTmpl *template.Template
}

// SetupRoutes registers all API routes on the given mux.
func SetupRoutes(mux *http.ServeMux, authSvc services.AuthService, logSvc services.LogService, aiSvc services.AIService, weightSvc services.WeightService) {
	entryTmpl, err := template.ParseFS(templates.Components, "components/log-entry-row.html")
	if err != nil {
		panic("failed to parse log-entry-row template: " + err.Error())
	}

	handler := &APIHandler{logSvc: logSvc, aiSvc: aiSvc, weightSvc: weightSvc, entryTmpl: entryTmpl}
	requireAuth := middleware.RequireAuth(authSvc)
	csrf := middleware.CSRFMiddleware

	mux.HandleFunc("GET /api/health", handler.Health)
	mux.Handle("DELETE /api/log/{id}", requireAuth(http.HandlerFunc(handler.DeleteLogEntry)))
	mux.Handle("POST /api/log/photo", requireAuth(http.HandlerFunc(handler.PhotoLogEntry)))
	mux.Handle("POST /api/log/chat", requireAuth(http.HandlerFunc(handler.ChatLogEntry)))
	mux.Handle("POST /api/log/{id}/edit", requireAuth(http.HandlerFunc(handler.EditLogEntry)))
	mux.Handle("POST /api/weight", requireAuth(csrf(http.HandlerFunc(handler.PostWeight))))
}

// Health returns a JSON health check response.
func (h *APIHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
}

// DeleteLogEntry handles DELETE /api/log/{id}.
func (h *APIHandler) DeleteLogEntry(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	entryID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid entry id", http.StatusBadRequest)
		return
	}

	if err := h.logSvc.DeleteEntry(r.Context(), user.ID, entryID); err != nil {
		http.Error(w, "failed to delete entry", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// PhotoLogEntry handles POST /api/log/photo.
// Accepts multipart/form-data with an optional "photo" field and/or a "description" text field.
// At least one must be present.
func (h *APIHandler) PhotoLogEntry(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 25<<20) // 25MB

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if !h.aiSvc.IsAvailable(r.Context()) {
		slog.Warn("AI service unavailable", "url", r.Header.Get("X-Forwarded-For"))
		writeJSONError(w, "AI service unavailable", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseMultipartForm(25 << 20); err != nil {
		slog.Error("ParseMultipartForm failed", "error", err, "content-type", r.Header.Get("Content-Type"))
		writeJSONError(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	description := r.FormValue("description")

	// Parse optional log date (for past-date logging). Zero value means "now".
	// Only override the timestamp for past dates so today's entries keep their
	// actual wall-clock time instead of being stamped at local midnight (which
	// can fall in yesterday UTC and break the daily summary query).
	var logDate time.Time
	if dateStr := r.FormValue("date"); dateStr != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", dateStr, time.Local); err == nil {
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
			if parsed.Before(today) {
				// Use noon local time to avoid UTC day-boundary issues.
				logDate = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 12, 0, 0, 0, time.Local)
			}
		}
	}

	// Photo is optional — read it only if present.
	var imageBase64, mimeType string
	file, _, fileErr := r.FormFile("photo")
	if fileErr == nil {
		defer file.Close() //nolint:errcheck

		sniff := make([]byte, 512)
		n, err := file.Read(sniff)
		if err != nil && err != io.EOF {
			writeJSONError(w, "reading photo", http.StatusInternalServerError)
			return
		}
		mimeType = http.DetectContentType(sniff[:n])

		allBytes := append(sniff[:n:n], func() []byte { b, _ := io.ReadAll(file); return b }()...)
		imageBase64 = base64.StdEncoding.EncodeToString(allBytes)
	}

	if imageBase64 == "" && description == "" {
		writeJSONError(w, "provide a photo or a description", http.StatusBadRequest)
		return
	}

	sessionID, message, done, err := h.aiSvc.StartSession(r.Context(), user.ID, imageBase64, mimeType, description, logDate)
	if err != nil {
		slog.Error("AI StartSession failed", "error", err)
		writeJSONError(w, "AI session error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var persistedEntry *models.LogEntry
	if done {
		if entry := h.aiSvc.GetResult(sessionID); entry != nil {
			created, saveErr := h.logSvc.AddEntry(r.Context(), user.ID, *entry)
			if saveErr == nil {
				persistedEntry = &created
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	resp := struct {
		Data  interface{} `json:"data"`
		Error *string     `json:"error"`
	}{
		Data: struct {
			SessionID string           `json:"session_id"`
			Message   string           `json:"message"`
			Done      bool             `json:"done"`
			Entry     *models.LogEntry `json:"entry"`
		}{
			SessionID: sessionID,
			Message:   message,
			Done:      done,
			Entry:     persistedEntry,
		},
	}
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// editRequest is the JSON body for POST /api/log/{id}/edit.
type editRequest struct {
	Message string `json:"message"`
}

// EditLogEntry handles POST /api/log/{id}/edit.
// Sends a natural-language correction to the AI and returns the updated HTML row.
func (h *APIHandler) EditLogEntry(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	entryID, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSONError(w, "invalid entry id", http.StatusBadRequest)
		return
	}

	var req editRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		writeJSONError(w, "message is required", http.StatusBadRequest)
		return
	}

	original, err := h.logSvc.GetEntry(r.Context(), user.ID, entryID)
	if err != nil {
		writeJSONError(w, "entry not found", http.StatusNotFound)
		return
	}

	if !h.aiSvc.IsAvailable(r.Context()) {
		writeJSONError(w, "AI service unavailable", http.StatusServiceUnavailable)
		return
	}

	corrected, err := h.aiSvc.CorrectEntry(r.Context(), original, req.Message)
	if err != nil {
		slog.Error("AI CorrectEntry failed", "error", err)
		writeJSONError(w, "AI error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	updated, err := h.logSvc.UpdateEntry(r.Context(), user.ID, entryID, *corrected)
	if err != nil {
		writeJSONError(w, "failed to save correction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.entryTmpl.ExecuteTemplate(w, "log-entry-row", updated); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

type chatRequest struct {
	SessionID string `json:"session_id"`
	Answer    string `json:"answer"`
}

type chatResponse struct {
	Data  *chatData `json:"data"`
	Error *string   `json:"error"`
}

type chatData struct {
	Done    bool             `json:"done"`
	Message string           `json:"message"`
	Entry   *models.LogEntry `json:"entry"`
}

// ChatLogEntry handles POST /api/log/chat.
func (h *APIHandler) ChatLogEntry(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.SessionID == "" {
		writeJSONError(w, "session_id is required", http.StatusBadRequest)
		return
	}

	message, done, entry, err := h.aiSvc.Continue(r.Context(), req.SessionID, req.Answer)
	if err != nil {
		writeJSONError(w, "AI error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var persistedEntry *models.LogEntry
	if done && entry != nil {
		created, err := h.logSvc.AddEntry(r.Context(), user.ID, *entry)
		if err != nil {
			writeJSONError(w, "failed to save entry: "+err.Error(), http.StatusInternalServerError)
			return
		}
		persistedEntry = &created
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResponse{ //nolint:errcheck
		Data: &chatData{
			Done:    done,
			Message: message,
			Entry:   persistedEntry,
		},
	})
}

// PostWeight handles POST /api/weight — logs a weight entry.
func (h *APIHandler) PostWeight(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		writeJSONError(w, "invalid form data", http.StatusBadRequest)
		return
	}

	if !middleware.ValidateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	weightLbs, err := strconv.ParseFloat(r.FormValue("weight_lbs"), 64)
	if err != nil || weightLbs <= 0 {
		writeJSONError(w, "invalid weight value", http.StatusBadRequest)
		return
	}

	entry, err := h.weightSvc.LogWeight(r.Context(), user.ID, weightLbs)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct { //nolint:errcheck
		Data  interface{} `json:"data"`
		Error *string     `json:"error"`
	}{Data: entry})
}

// writeJSONError writes a standard JSON error response.
func writeJSONError(w http.ResponseWriter, msg string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := struct {
		Data  interface{} `json:"data"`
		Error string      `json:"error"`
	}{Data: nil, Error: msg}
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// parseFloatDefault parses a float64 from s, returning def on error.
func parseFloatDefault(s string, def float64) float64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}
