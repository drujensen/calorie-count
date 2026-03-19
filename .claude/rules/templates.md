---
paths:
  - "internal/templates/**/*.html"
  - "internal/handlers/web/**/*.go"
---

# Template & UI Rules

## Template Safety

- Always use `html/template` package — NEVER `text/template` for HTML output
- Never call `template.HTML()` on user-provided content
- All user data passed to templates is auto-escaped by `html/template`

## HTMX Patterns

- Use `hx-get`/`hx-post` for dynamic interactions — no full page reloads for updates
- Every HTMX target must have a stable `id` attribute
- Use `hx-target` to specify where the response goes; use `hx-swap` to control how
- Partial responses (for HTMX swaps) go in `internal/templates/components/`
- Full page templates go in `internal/templates/pages/`

```html
<!-- Trigger a GET and replace a target element -->
<button hx-get="/api/foods" hx-target="#food-list" hx-swap="innerHTML">
  Refresh
</button>

<!-- Form submission -->
<form hx-post="/api/meals" hx-target="#result" hx-swap="outerHTML">
  ...
</form>
```

## HTML Structure

- Use semantic HTML5: `<nav>`, `<main>`, `<section>`, `<article>`, `<header>`, `<footer>`
- Proper heading hierarchy: one `<h1>` per page, then `<h2>`, `<h3>`
- All interactive elements must be keyboard navigable
- Form inputs must have associated `<label>` elements

## Accessibility

- Images need `alt` attributes (empty `alt=""` for decorative images)
- Buttons need descriptive text or `aria-label`
- Color must not be the only way to convey information
- Sufficient color contrast (WCAG AA: 4.5:1 for normal text)
