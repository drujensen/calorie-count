# Calorie-Count: Implementation Plan (Simplified)

## Vision

**One job: make it effortless to log what you eat and see how it's affecting you.**

Take a photo. AI identifies the food, asks one or two clarifying questions, logs the macros. Two tabs: log and results. That's the whole app.

---

## What We're Building

### Tab 1 — Log
- Big camera button at the top
- Upload a photo of your food
- AI (Ollama vision model) identifies the food and asks clarifying questions (portion size, preparation, etc.)
- User answers in a simple chat interface
- AI logs the entry: name, calories, protein, fat, carbs
- Log shows all entries for today in reverse-chronological order
- Each entry shows: food name, calories, and macro breakdown
- Running totals at the top: calories, protein, fat, carbs for today
- Delete an entry (mistakes happen)

### Tab 2 — Results
- Toggle between Daily / Weekly / Monthly
- Total calories consumed vs goal
- Macro breakdown: protein / fat / carbs in grams and as a % of total calories
- Visual bar or percentage indicators (pure CSS)
- Estimated weight impact: "+0.4 lbs this week at current pace"
- No charts, no complex graphs — just clean numbers

### Settings (minimal, not a tab — just a link)
- Set daily calorie goal
- Set current weight (for weight estimate calculations)
- Change email/password

---

## What We Are NOT Building

- ~~Meal slots (breakfast/lunch/dinner/snack)~~
- ~~Food library / CRUD~~
- ~~Manual food search / typeahead~~
- ~~History page~~
- ~~Nutrition database~~
- ~~Exercise tracking~~
- ~~Social features~~

Manual text entry is a fallback only (when camera isn't available), not a primary feature.

---

## Infrastructure

| Resource | Spec | Purpose |
|----------|------|---------|
| 4x Ubuntu servers | — | Host the web app via Docker Compose |
| Framework Desktop | 128GB VRAM | Run Ollama (`llama3.2-vision`) for food photo analysis |

**AI model:** `llama3.2-vision` via Ollama REST API. Configurable via:
- `OLLAMA_API_URL` — default `http://localhost:11434`
- `OLLAMA_MODEL` — default `llama3.2-vision`

**Inspiration:** [CaLoRAify](https://arxiv.org/abs/2412.09936) — research VLM fine-tuned on 330K food image-text pairs. Same UX, implemented with a standard Ollama vision model.

---

## Technical Decisions

| Decision | Choice |
|----------|--------|
| Database | SQLite (`modernc.org/sqlite`) — zero infra, upgradeable to Postgres via interface |
| Auth | Session-based cookies (`HttpOnly`, `Secure`, `SameSite=Lax`) |
| Passwords | bcrypt cost 12 |
| CSRF | Synchronizer token in all POST forms |
| Templates | `html/template` + base layout + `//go:embed` |
| CSS | Single `web/public/css/app.css` — no build step |
| Migrations | Hand-rolled runner in `scripts/migrations/*.sql` |
| AI | Ollama HTTP client in `internal/services/ai_service.go` — no external SDK |
| HTTP | `net/http` only — no framework |

---

## Data Model

```
User
  id, email, password_hash, calorie_goal, current_weight_lbs, created_at

LogEntry
  id, user_id, food_name, calories, protein_g, fat_g, carbs_g,
  image_path (nullable), notes (nullable), logged_at

Session
  token, user_id, expires_at
```

No `Meal`, no `Food` library, no `MealEntry`. A `LogEntry` is just a timestamped record of something eaten.

---

## Project Structure

```
cmd/server/main.go
internal/
  config/config.go             — PORT, ENV, DatabasePath, OllamaAPIURL, OllamaModel
  db/db.go                     — Open(), RunMigrations()
  models/models.go             — User, LogEntry, Session
  repositories/
    user_repository.go
    log_repository.go          — Create, List(userID, from, to), Delete, SumByPeriod
    session_repository.go
  services/
    auth_service.go            — Register, Login, Logout, Authenticate
    log_service.go             — Add, Delete, GetSummary(daily/weekly/monthly)
    ai_service.go              — StartSession(image), Continue(sessionID, answer) → LogEntry
    weight_service.go          — EstimateWeightImpact(userID, period)
  middleware/
    auth.go                    — RequireAuth
    csrf.go
    headers.go                 — security headers
    ratelimit.go               — on /login, /register
  handlers/
    handlers.go                — route registration
    web/web.go                 — all page handlers
    api/api.go                 — /api/* JSON endpoints (log CRUD, AI chat)
  templates/
    pages/
      base.html                — nav with 2 tabs + settings link
      log.html                 — Tab 1: photo upload + chat + today's entries
      results.html             — Tab 2: daily/weekly/monthly summary
      settings.html            — calorie goal, weight, password
      login.html
      register.html
    components/
      chat-bubble.html         — AI Q&A conversation bubble
      log-entry-row.html       — single log entry (HTMX swap)
      summary-card.html        — daily/weekly/monthly results card
web/
  public/css/app.css
scripts/migrations/
  001_initial_schema.sql
```

---

## AI Logging Flow

```
1. User taps camera button → selects/takes photo
2. POST /api/log/photo  { image: base64 }
   → ai_service.StartSession(): send to Ollama with system prompt +
     tool definition for log_entry(food_name, calories, protein_g, fat_g, carbs_g)
   → Ollama returns first clarifying question (or calls tool directly if confident)
3. HTMX renders question as chat bubble; user types answer
4. POST /api/log/chat   { session_id, answer }
   → ai_service.Continue(): send answer back to Ollama conversation
   → If Ollama calls log_entry tool → insert LogEntry to DB, return entry JSON
   → If Ollama asks another question → return question for next chat bubble
5. On tool call: HTMX swaps in the new log entry row at top of today's list
   Updates running totals inline
6. Fallback: if Ollama unreachable → show manual text entry form with warning
```

**System prompt for Ollama:**
> You are a nutrition assistant. The user will show you a photo of food they are about to eat. Identify the food and estimate its calories and macronutrients. If you need clarification (portion size, cooking method, brand), ask ONE short question at a time. When you have enough information, call the log_entry tool. Be conversational and brief.

---

## Phases

### Phase 1 — Foundation
**Goal:** Compiles, CI green, real templates, SQLite wired, `GET /health` works.

1. Fix `text/template` → `html/template` (security bug)
2. `http.Server` with timeouts (5s read, 10s write, 120s idle)
3. `internal/db/db.go` — `Open()` + `RunMigrations()` with `modernc.org/sqlite`
4. `scripts/migrations/001_initial_schema.sql` — `users`, `log_entries`, `sessions`
5. Base layout template (`base.html`) with 2-tab nav; all pages use `{{define "content"}}`
6. `web/public/css/app.css` — mobile-first, 2-tab layout, form styles, entry list
7. `.github/workflows/ci.yml` — Go 1.23.3, lint + test + build
8. `Dockerfile` (multi-stage, distroless) + `docker-compose.yml`
9. `GET /health` → `{"status":"ok"}`
10. First unit tests for `config` and `db` packages

**Done when:** `make build && make lint && make test` all pass; CI green; app starts and serves styled pages.

---

### Phase 2 — Authentication
**Goal:** Register, login, logout. All pages protected.

1. `user_repository.go` — `Create`, `GetByEmail`, `GetByID`
2. `session_repository.go` — `Create`, `GetByToken`, `Delete`
3. `auth_service.go` — `Register` (bcrypt), `Login`, `Logout`, `Authenticate`
4. `middleware/auth.go` — `RequireAuth`; injects user into context
5. `middleware/csrf.go` — token generation + POST validation
6. Auth handlers + routes (`/login`, `/register`, `/logout`)
7. `login.html` + `register.html` templates
8. Wire full DI in `main.go`; protect all routes with `RequireAuth`
9. Unit tests: register/login happy path, wrong password, duplicate email, expired session
10. Security review: cookie flags, bcrypt cost, CSRF

**Done when:** Register → login → protected pages work; logout clears session.

---

### Phase 3 — Manual Log Entry (Baseline)
**Goal:** User can manually log a food entry. This validates the full data stack before AI is added.

1. `log_repository.go` — `Create`, `ListByUserAndDate`, `Delete`, `SumByPeriod`
2. `log_service.go` — `Add(userID, entry)`, `Delete(userID, entryID)`, `GetToday(userID)`
3. `POST /api/log` — create entry from JSON `{food_name, calories, protein_g, fat_g, carbs_g}`
4. `DELETE /api/log/{id}` — delete own entry only
5. `GET /` (log page) — shows today's entries + running totals + manual entry form
6. `log-entry-row.html` component — HTMX swap on add/delete
7. Manual entry form: food name, calories, protein, fat, carbs (simple form, not the primary UX)
8. Unit tests: log service validation (calories > 0, name required), ownership on delete
9. Integration test: add entry → verify totals → delete → verify updated

**Done when:** User can add and delete entries; today's totals update inline via HTMX.

---

### Phase 4 — AI Photo Logging (Core Feature)
**Goal:** Photo → AI Q&A → auto-logged entry. This is the whole point of the app.

1. `ai_service.go`:
   - `StartSession(userID int, imageBase64 string) (sessionID, firstMessage string, err error)`
   - `Continue(sessionID, userAnswer string) (reply string, entry *models.LogEntry, err error)`
   - Ollama client: `POST {OLLAMA_API_URL}/api/chat` with `images` array
   - Conversation state: in-memory `map[sessionID]conversation` (TTL 30 min)
   - Tool definition: `log_entry(food_name, calories, protein_g, fat_g, carbs_g)`
   - When tool called: return `*LogEntry` to handler; handler persists via `log_service`
2. Config: `OllamaAPIURL`, `OllamaModel` fields in `internal/config/config.go`
3. `POST /api/log/photo` — accept multipart image, start AI session, return `{session_id, message}`
4. `POST /api/log/chat` — accept `{session_id, answer}`, return `{message}` or `{entry}` on completion
5. Update `log.html` — replace manual form with camera upload as primary action; chat bubble UI below
6. `chat-bubble.html` component — AI message + user reply input (HTMX polling or swap)
7. Graceful fallback: if Ollama returns connection error → show manual form with banner "AI unavailable"
8. Unit tests: mock Ollama HTTP server; test clarifying Q path, tool-call path, fallback path
9. Integration test (build tag `integration`): real Ollama call with a test image

**Done when:** User uploads photo, answers one clarifying question, entry is logged and appears in the list via HTMX with no page reload. Fallback works when Ollama is down.

---

### Phase 5 — Results Tab
**Goal:** Tab 2 shows meaningful nutrition summaries. Weight estimate included.

1. `log_repository.SumByPeriod(userID, from, to)` → `{calories, protein_g, fat_g, carbs_g}`
2. `log_service.GetSummary(userID, period)` — period: `daily` | `weekly` | `monthly`
   - Calculates macro percentages: `protein_pct = (protein_g * 4 / calories) * 100`
   - Calories: protein=4 kcal/g, carbs=4 kcal/g, fat=9 kcal/g
3. `weight_service.EstimateWeightImpact(userID, period)`:
   - avg daily surplus/deficit = avg_calories_consumed - calorie_goal
   - lbs per period = (avg_surplus * days) / 3500
4. `GET /results` — renders results page; `?period=daily|weekly|monthly` param
5. `results.html` — period toggle (HTMX), macro breakdown card, weight estimate
6. `summary-card.html` component — HTMX swap target for period toggle
7. `settings.html` — calorie goal + current weight fields; `POST /settings`
8. Unit tests: macro % calculations, weight estimate formula (edge: no data, no goal set)

**Done when:** Results tab shows real data for all three periods; macros shown in grams and %; weight estimate is sensible.

---

### Phase 6 — Hardening
**Goal:** Secure, deployable, tested end-to-end.

1. Security audit: SQL params, `html/template`, CSRF on all forms, cookie flags, security headers, `MaxBytesReader`, no secrets in logs
2. `middleware/headers.go` — `X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy`
3. `middleware/ratelimit.go` — token bucket on `/login` and `/register`
4. `log/slog` structured logging — JSON in production, text in development
5. Graceful shutdown — `signal.NotifyContext` + `server.Shutdown(ctx)`
6. Session pruning — background goroutine removes expired sessions hourly
7. AI session cleanup — TTL expiry on in-memory conversation map
8. Finalize `Dockerfile` with `//go:embed` (no volume mounts for templates)
9. CI: add `govulncheck` step
10. `.devcontainer/devcontainer.json`
11. E2E test: register → login → upload photo → answer question → verify entry logged → check results tab
12. 80%+ handler coverage, 100% service coverage

**Done when:** `govulncheck` clean; `docker-compose up` cold-starts a working app; E2E test passes.

---

## New Files (All Phases)

| Phase | Files |
|-------|-------|
| 1 | `internal/db/db.go`, `scripts/migrations/001_initial_schema.sql`, `internal/templates/pages/base.html`, `web/public/css/app.css`, `.github/workflows/ci.yml`, `Dockerfile`, `docker-compose.yml` |
| 2 | `internal/repositories/user_repository.go`, `session_repository.go`, `internal/services/auth_service.go`, `internal/middleware/auth.go`, `csrf.go`, `internal/templates/pages/login.html`, `register.html` |
| 3 | `internal/repositories/log_repository.go`, `internal/services/log_service.go`, `internal/templates/pages/log.html`, `components/log-entry-row.html` |
| 4 | `internal/services/ai_service.go`, `internal/templates/components/chat-bubble.html` |
| 5 | `internal/services/weight_service.go`, `internal/services/log_service.go` (extended), `internal/templates/pages/results.html`, `settings.html`, `components/summary-card.html` |
| 6 | `internal/middleware/headers.go`, `ratelimit.go`, `tests/e2e/journey_test.go`, `.devcontainer/devcontainer.json` |

## Existing Files Modified

| File | Change |
|------|--------|
| `internal/models/models.go` | Replace all existing structs with `User`, `LogEntry`, `Session` |
| `internal/handlers/handlers.go` | Accept `authSvc`, `logSvc`, `aiSvc`; register ~10 routes (down from ~20) |
| `internal/handlers/api/api.go` | Rebuild: log CRUD + AI chat endpoints |
| `internal/handlers/web/web.go` | Rebuild: log page, results page, settings, auth pages |
| `internal/config/config.go` | Add `DatabasePath`, `OllamaAPIURL`, `OllamaModel` |
| `cmd/server/main.go` | Full DI wiring, server timeouts, graceful shutdown |
| `go.mod` | Add `modernc.org/sqlite`, `golang.org/x/crypto` |
