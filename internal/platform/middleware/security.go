// Package middleware provides HTTP middlewares for the platform layer.
package middleware

import (
	"net/http"
)

// Security sets security-related response headers on every request.
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		writer.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'sha256-faU7yAF8NxuMTNEwVmBz+VcYeIoBQ2EMHW3WaVxCvnk='; style-src-attr 'unsafe-inline'; script-src 'self'; img-src 'self'",
		)

		next.ServeHTTP(writer, request)
	})
}
