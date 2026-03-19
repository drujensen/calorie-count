---
name: audit-security
description: Run a full security audit of the project — checks for hardcoded secrets, insecure patterns, dependency vulnerabilities, and missing security controls. Run before any release.
---

Perform a full security audit of this project.

## Audit Steps

### 1. Secret Scanning
Search for hardcoded credentials:
- Grep for: `password`, `secret`, `api_key`, `token`, `private_key` in source files
- Verify `.gitignore` includes `.env`
- Check that `.env.example` uses placeholder values only

### 2. Dependency Vulnerabilities
```bash
go list -m all
```
If `govulncheck` is available:
```bash
govulncheck ./...
```
Otherwise note that manual CVE checking is needed for each dependency.

### 3. Go Security Patterns
Check for:
- `html/template` used for all HTML output (not `text/template`)
- `http.MaxBytesReader` on request bodies
- Server timeouts set (`ReadTimeout`, `WriteTimeout`, `IdleTimeout`)
- No `math/rand` for security-sensitive randomness (use `crypto/rand`)
- No `fmt.Sprintf` building SQL queries

### 4. HTTP Security Headers
Verify middleware sets:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Content-Security-Policy` (if serving HTML)

### 5. Input Validation
Review all handler inputs for:
- Type validation (JSON decode checks)
- Range checks on numeric inputs (calories must be >= 0)
- String length limits

### 6. Authentication & Authorization (when implemented)
- Password hashing algorithm and cost factor
- Session token entropy and storage
- Per-resource authorization checks

## Output

Produce a report with sections:
1. **Critical** — must fix before release
2. **High** — fix in this sprint
3. **Medium** — schedule soon
4. **Low / Informational** — track in backlog
5. **Passed** — controls verified and working

Each finding includes: location, description, risk, and recommended fix.
