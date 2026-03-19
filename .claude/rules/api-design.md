---
paths:
  - "internal/handlers/api/**/*.go"
---

# API Design Rules

## URL Conventions

- Plural nouns for collections: `/api/foods`, `/api/meals`
- Resource by ID: `/api/foods/{id}`
- Lowercase with hyphens: `/api/calorie-goals`
- No trailing slashes

## Response Format

All API responses use this envelope:

```json
{
  "data": { },
  "error": null
}
```

On error:
```json
{
  "data": null,
  "error": {
    "code": "INVALID_INPUT",
    "message": "calories must be a positive integer"
  }
}
```

## HTTP Status Codes

| Situation | Code |
|-----------|------|
| Success (read) | 200 |
| Success (created) | 201 |
| Bad input | 400 |
| Unauthorized | 401 |
| Forbidden | 403 |
| Not found | 404 |
| Server error | 500 |

## Input Validation

- Validate all fields before processing
- Return 400 with a specific `error.message` explaining what is wrong
- Numeric values: check for negative numbers and unrealistic ranges (calories: 0–10000)
- Strings: trim whitespace; reject empty required fields

## Content Type

- Always set `Content-Type: application/json` before writing response body
- Decode request bodies with `json.NewDecoder(r.Body).Decode(&input)`
- Limit request body: `r.Body = http.MaxBytesReader(w, r.Body, 1<<20)` (1 MB)
