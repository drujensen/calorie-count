---
paths:
  - "**/*.go"
---

# Security Rules

## Secrets

- No credentials, tokens, or keys in source code — use `os.Getenv()`
- `.env` must be in `.gitignore`
- `.env.example` contains only placeholder values

## HTML Output

- Use `html/template` for all HTML rendering — it auto-escapes user data
- Never cast user input to `template.HTML`

## HTTP Server

Set timeouts on every `http.Server`:
```go
srv := &http.Server{
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
    IdleTimeout:  120 * time.Second,
}
```

Limit request body size in handlers:
```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
```

## Randomness

- Use `crypto/rand` for tokens, session IDs, nonces
- Never use `math/rand` for security-sensitive values

## SQL (when database is added)

- Always use parameterized queries — never string-concatenate user input into SQL
- ✅ `db.QueryRow("SELECT * FROM foods WHERE id = $1", id)`
- ❌ `db.Query("SELECT * FROM foods WHERE id = " + id)`

## Input Validation

- Validate all user-supplied values before use
- Numeric fields: check range (calories 0–10000, negative not allowed)
- String fields: trim whitespace, enforce max length
- Return HTTP 400 with a clear message for invalid input — do not panic
