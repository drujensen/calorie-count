# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make install   # go mod tidy
make build     # compile to bin/calorie-count
make run       # go run ./cmd/server
make test      # go test ./...
make lint      # gofmt + go vet
make clean     # remove bin/ and dist/
```

Run a single test:
```bash
go test ./internal/handlers/... -run TestFunctionName
```

Environment variables:
- `PORT` — server port (default: `8080`)
- `ENV` — environment mode (default: `development`)

## Architecture

This is a calorie-tracking web app with a **hybrid REST API + server-side HTML** architecture using only the Go standard library (`net/http`). No external web framework.

**Request flow:**
```
HTTP → middleware (logging) → ServeMux → api/ or web/ handler
```

**Layer structure:**
- `cmd/server/main.go` — entry point; loads config, wires routes, starts server
- `internal/config/` — reads env vars into a config struct
- `internal/handlers/handlers.go` — registers all routes; delegates to sub-packages
- `internal/handlers/api/` — REST endpoints returning JSON (`/api/foods`, `/api/meals`)
- `internal/handlers/web/` — page handlers rendering HTML templates (`/`, `/dashboard`, `/meals`, `/foods`)
- `internal/middleware/` — HTTP middleware chain (currently: request logging)
- `internal/models/` — `Food`, `Meal`, `User` structs (shared across layers)
- `internal/templates/pages/` — Go HTML templates rendered by web handlers
- `web/` — static assets served at `/static/`

**Planned but empty:**
- `internal/repositories/` — intended data access layer (repository pattern)
- `internal/services/` — intended business logic layer
- `tests/e2e/` and `tests/integration/` — no tests written yet

**Frontend:** HTMX (v1.9.6) for dynamic interactions; no JS build step required.

**Data persistence:** Not yet implemented. All API handlers return hardcoded mock data. TODOs in `internal/handlers/api/api.go` mark where database calls should go.

## SDLC Agent Team

Specialized agents and skills are configured in `.claude/` for autonomous development.

**Agents** (delegate with `use the <name> agent`):
| Agent | Role |
|-------|------|
| `pm` | Scrum master — decomposes features, orchestrates the team |
| `designer` | UX/UI — HTML templates, HTMX patterns, user flows |
| `developer` | Go implementation — handlers, services, repositories |
| `tester` | QA — unit/integration/e2e tests, edge cases |
| `security` | AppSec — vulnerability review, threat modeling |
| `devops` | Infrastructure — CI/CD, Docker, GitHub Actions |

**Skills** (invoke with `/skill-name [args]`):
- `/plan-feature <description>` — full end-to-end implementation plan
- `/review-code <path>` — Go code review checklist
- `/db-migrate <description>` — generate UP/DOWN SQL migration files
- `/audit-security` — full project security audit

**Rules** (auto-applied by file path):
- `go-style.md` — all `*.go` files
- `api-design.md` — `internal/handlers/api/`
- `templates.md` — HTML templates and web handlers
- `security.md` — all `*.go` files
- `testing.md` — all `*_test.go` and `tests/` files

## Running Tests

```bash
make test                              # unit tests (excludes e2e/integration build tags)
go test -tags=e2e ./tests/e2e/         # e2e tests — spins up a real in-process server
go test -v ./internal/...             # verbose unit tests for all internal packages
```

## Module

```
github.com/drujensen/calorie-count
```

Go 1.23.3, no external dependencies currently.
