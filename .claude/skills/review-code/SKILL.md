---
name: review-code
description: Comprehensive code review for Go code — checks quality, idiomatic patterns, error handling, test coverage, and security. Invoke with a file path or package path.
argument-hint: <file-or-package-path>
---

Perform a thorough code review of: $ARGUMENTS

## Review Checklist

**Correctness**
- [ ] Logic is correct and handles edge cases
- [ ] All errors are handled (no `_` on error returns unless intentional)
- [ ] No nil pointer dereferences possible
- [ ] No data races (shared state protected by mutex or channels)

**Go Idioms**
- [ ] Interfaces used appropriately (accept interfaces, return structs)
- [ ] Error types and wrapping follow `fmt.Errorf("context: %w", err)`
- [ ] No unnecessary use of `init()`
- [ ] Context propagated through call chain where needed
- [ ] `defer` used correctly (not in loops)

**Code Quality**
- [ ] Functions are small and single-purpose
- [ ] Variable names are clear and descriptive
- [ ] No commented-out code left behind
- [ ] No TODO left without a corresponding issue/plan
- [ ] Magic numbers replaced with named constants

**HTTP Handlers** (if applicable)
- [ ] Correct HTTP status codes returned
- [ ] Request body size limited (`http.MaxBytesReader`)
- [ ] Input validated before use
- [ ] `html/template` used for HTML output (not `text/template`)

**Security**
- [ ] No hardcoded secrets
- [ ] SQL queries parameterized (when DB is added)
- [ ] User input sanitized before rendering in templates

**Tests**
- [ ] Unit tests exist for new/changed code
- [ ] Edge cases are tested (empty input, boundaries, errors)
- [ ] Table-driven test style used

## Output Format

For each issue:
**[Severity: High/Medium/Low]** `file.go:line` — description of issue and suggested fix.

End with a one-line summary: "X issues found (Y high, Z medium, W low)" or "LGTM — no issues found."
