package middleware

import "net/http"

// SecurityHeaders adds security-related HTTP headers to every response.
// - X-Content-Type-Options prevents MIME sniffing.
// - X-Frame-Options blocks clickjacking via iframes.
// - Referrer-Policy limits referrer information sent cross-origin.
// - Content-Security-Policy restricts resource loading origins.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}
