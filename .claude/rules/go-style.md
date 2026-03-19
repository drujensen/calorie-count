---
paths:
  - "**/*.go"
---

# Go Style Rules

## Error Handling

- Handle every error — never assign to `_` without a comment explaining why
- Wrap errors with context: `fmt.Errorf("creating user: %w", err)`
- Return errors to callers; don't log and return (choose one)
- Use sentinel errors for expected conditions: `var ErrNotFound = errors.New("not found")`

## Function Design

- Keep functions under 50 lines
- One responsibility per function
- Accept interfaces, return concrete types
- Prefer early returns over deeply nested conditionals

## Naming

- Exported names: `PascalCase`; unexported: `camelCase`
- Acronyms stay uppercase: `userID`, `httpClient`, `parseURL`
- Boolean variables: descriptive predicates (`isActive`, `hasCalorieGoal`)
- Avoid generic names: `data`, `info`, `tmp` — be specific

## Packages

- Package names are lowercase, single words, no underscores
- Don't repeat the package name in exported identifiers: `models.Food` not `models.FoodModel`

## HTTP Handlers

- Signature: `func (h *Handler) Name(w http.ResponseWriter, r *http.Request)`
- Decode request body; check for errors; return 400 on bad input
- Use `http.StatusXxx` constants — never raw integers
- Set `Content-Type` header before writing body

## Testing

- Test files in same package: `package foo` (white-box) or `package foo_test` (black-box)
- Table-driven tests using `[]struct{ name, input, expected }`
- Use `t.Run(tt.name, ...)` for subtests
- Use `httptest.NewRecorder()` and `httptest.NewRequest()` for handler tests
