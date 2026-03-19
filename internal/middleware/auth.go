package middleware

import (
	"context"
	"net/http"

	"github.com/drujensen/calorie-count/internal/models"
	"github.com/drujensen/calorie-count/internal/services"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

// UserContextKey is the key used to store the authenticated user in request context.
const UserContextKey contextKey = "user"

// RequireAuth returns middleware that validates the session cookie and injects the
// authenticated user into the request context. Unauthenticated requests are
// redirected to /login.
func RequireAuth(authSvc services.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			user, err := authSvc.Authenticate(r.Context(), cookie.Value)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext retrieves the authenticated user from the request context.
// The second return value is false if no user is present.
func UserFromContext(ctx context.Context) (models.User, bool) {
	user, ok := ctx.Value(UserContextKey).(models.User)
	return user, ok
}
