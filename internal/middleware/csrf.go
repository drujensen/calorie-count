package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
)

// csrfContextKey is the context key type for the CSRF token.
type csrfContextKey string

const csrfTokenKey csrfContextKey = "csrf_token"

// GenerateCSRFToken produces a 16-byte crypto/rand token, hex-encoded.
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CSRFMiddleware implements the double-submit cookie pattern.
// On every request it ensures a "csrf" cookie is present (generating one if
// needed) and stores the token value in the request context so handlers can
// inject it into forms via GetCSRFToken.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""

		cookie, err := r.Cookie("csrf")
		if err == nil {
			token = cookie.Value
		}

		if token == "" {
			generated, genErr := GenerateCSRFToken()
			if genErr == nil {
				token = generated
			}
		}

		// (Re-)set the cookie on every request to keep it fresh.
		http.SetCookie(w, &http.Cookie{
			Name:     "csrf",
			Value:    token,
			Path:     "/",
			HttpOnly: false, // must be readable by form template injection
			SameSite: http.SameSiteStrictMode,
		})

		ctx := context.WithValue(r.Context(), csrfTokenKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetCSRFToken retrieves the CSRF token from the request context.
// Returns an empty string when CSRFMiddleware has not run.
func GetCSRFToken(r *http.Request) string {
	v, _ := r.Context().Value(csrfTokenKey).(string)
	return v
}

// ValidateCSRF compares the CSRF cookie value to the "_csrf" form field.
// Returns true when both are non-empty and equal.
func ValidateCSRF(r *http.Request) bool {
	cookie, err := r.Cookie("csrf")
	if err != nil || cookie.Value == "" {
		return false
	}
	formToken := r.FormValue("_csrf")
	return formToken != "" && formToken == cookie.Value
}
