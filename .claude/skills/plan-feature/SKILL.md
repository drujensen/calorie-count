---
name: plan-feature
description: Plan the implementation of a new feature end-to-end — produces a task breakdown covering design, backend, tests, security, and devops concerns. Use before starting any significant feature.
argument-hint: <feature description>
---

Create a detailed implementation plan for: $ARGUMENTS

## Planning Process

1. Read `CLAUDE.md` for project conventions
2. Read the relevant existing code to understand current patterns
3. Identify all layers that need to change

## Plan Template

### Feature: $ARGUMENTS

**Summary**
One paragraph describing what this feature does and why.

**User Story**
As a [user], I want to [action] so that [benefit].

**Acceptance Criteria**
- [ ] Criterion 1
- [ ] Criterion 2
- [ ] ...

---

### Design (designer agent)
- New pages or UI components needed
- HTMX interactions required
- Template files to create/modify

### Models (developer agent)
- New or updated structs in `internal/models/models.go`
- Fields, types, validation rules

### Data Layer (developer agent)
- Repository interfaces to add in `internal/repositories/`
- SQL schema / migration needed

### Business Logic (developer agent)
- Service functions in `internal/services/`
- Logic and validation rules

### HTTP Handlers (developer agent)
- New routes to register in `internal/handlers/handlers.go`
- API endpoints (`internal/handlers/api/api.go`)
- Web endpoints (`internal/handlers/web/web.go`)

### Tests (tester agent)
- Unit tests needed
- Integration tests needed
- Edge cases to cover

### Security (security agent)
- Input validation points
- Authorization checks needed
- Potential vulnerabilities to review

### DevOps (devops agent)
- Environment variables or config changes
- Any deployment or infra impact

---

### Task Order (with dependencies)

1. [ ] Task 1 (no deps)
2. [ ] Task 2 (no deps) — can run in parallel with 1
3. [ ] Task 3 (depends on 1, 2)
4. [ ] ...

### Open Questions

List anything that needs a decision before or during implementation.
