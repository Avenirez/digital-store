package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// CORS returns a middleware that sets Cross-Origin Resource Sharing headers.
func CORS(allowedOrigins string) func(http.Handler) http.Handler {
	origins := strings.Split(allowedOrigins, ",")
	originSet := make(map[string]bool, len(origins))
	for _, o := range origins {
		originSet[strings.TrimSpace(o)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed safely
			if originSet[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else if originSet["*"] {
				// If wildcard allowed, do not reflect untrusted origin with credentials=true
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
			w.Header().Set("Access-Control-Max-Age", "86400")

			// Handle preflight
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AdminAuth returns a middleware that checks for a valid admin API key
// in the X-Admin-Key header.
func AdminAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKey == "" {
				// No API key configured, deny all admin access
				http.Error(w, `{"error":"admin access not configured"}`, http.StatusForbidden)
				return
			}

			provided := r.Header.Get("X-Admin-Key")
			if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
