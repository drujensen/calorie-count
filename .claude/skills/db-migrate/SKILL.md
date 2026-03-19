---
name: db-migrate
description: Create a database migration for a schema change. Produces both an UP and DOWN migration file. Invoke with a description of the change.
argument-hint: <migration description>
---

Create a database migration for: $ARGUMENTS

## Migration Requirements

1. **Naming**: Use timestamp format — `migrations/YYYYMMDDHHMMSS_description.sql`
2. **Both directions**: Every migration needs an UP and a DOWN
3. **Backward compatible**: UP must not break the running app before the code deploy
4. **Idempotent**: Safe to run multiple times (use `IF NOT EXISTS`, `IF EXISTS`)
5. **Data preservation**: Never DROP a column with data — deprecate first

## File Format

Create two files:

**`migrations/YYYYMMDDHHMMSS_description.up.sql`**
```sql
-- Migration: <description>
-- Created: <date>

-- UP: apply the change
BEGIN;

-- your SQL here

COMMIT;
```

**`migrations/YYYYMMDDHHMMSS_description.down.sql`**
```sql
-- Migration: <description> (rollback)
-- Created: <date>

-- DOWN: reverse the change
BEGIN;

-- your SQL here

COMMIT;
```

## Common Patterns

**Add table:**
```sql
CREATE TABLE IF NOT EXISTS foods (
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    calories   INTEGER NOT NULL CHECK (calories >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Add column (safe — nullable or with default):**
```sql
ALTER TABLE meals ADD COLUMN IF NOT EXISTS notes TEXT;
```

**Add index:**
```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_meals_date ON meals(date);
```

## After Creating Files

Note what changes are needed in:
- `internal/models/models.go` — struct updates
- `internal/repositories/` — new repository methods
- Any existing queries that need updating
