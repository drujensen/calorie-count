---
paths:
  - "**/*_test.go"
  - "tests/**/*.go"
---

# Testing Rules

## Test Organization

- **Unit tests**: `*_test.go` next to the file under test, same or `_test` package
- **Integration tests**: `tests/integration/`
- **E2E tests**: `tests/e2e/`

## Test Structure

Use table-driven tests for any function with multiple cases:

```go
func TestAddFood(t *testing.T) {
    tests := []struct {
        name    string
        input   models.Food
        wantErr bool
    }{
        {"valid food", models.Food{Name: "Apple", Calories: 95}, false},
        {"empty name", models.Food{Name: "", Calories: 95}, true},
        {"negative calories", models.Food{Name: "Apple", Calories: -1}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := AddFood(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("AddFood() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## HTTP Handler Tests

Use `net/http/httptest` — no live server:

```go
func TestGetFoodsHandler(t *testing.T) {
    req := httptest.NewRequest(http.MethodGet, "/api/foods", nil)
    w := httptest.NewRecorder()

    handler.GetFoods(w, req)

    res := w.Result()
    if res.StatusCode != http.StatusOK {
        t.Errorf("expected 200, got %d", res.StatusCode)
    }
}
```

## Coverage Targets

- Business logic and validation: 100%
- HTTP handlers: 80%+
- Utility functions: 80%+
- Don't chase coverage on trivial code (simple getters, main())

## Assertions

Use the standard `testing` package — no assertion libraries required.
For helper assertions, write a local `assertEqual(t, got, want)` helper rather than importing a framework.

## Test Naming

- Name describes the scenario: `TestAddFood_EmptyName_ReturnsError`
- Or use `t.Run` with descriptive names: `t.Run("empty name returns error", ...)`
- Avoid: `TestAddFood1`, `TestAddFoodCase2`
