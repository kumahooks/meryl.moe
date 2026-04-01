// Package middleware provides HTTP middlewares for the platform layer.
package middleware

import (
	http "net/http"
)

func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self'",
		)

		next.ServeHTTP(writer, request)
	})
}
