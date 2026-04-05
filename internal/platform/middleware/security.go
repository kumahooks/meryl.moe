// Package middleware provides HTTP middlewares for the platform layer.
package middleware

import (
	"net/http"
)

// Security sets security-related response headers on every request.
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-hashes' 'sha256-faU7yAF8NxuMTNEwVmBz+VcYeIoBQ2EMHW3WaVxCvnk=' 'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=' 'sha256-Z7wqRMsXrTzenFN+Xlsq0Ot702MqTqY52FZleRnUZkc=' 'sha256-C5nbf+LODEXP/7nHS3i3vf4uP0YK2m0n0es7V3eXNlU=' 'sha256-hp7L9m8vZle+uJzcJz5TLXdNUweRTJfazK7dAfdIDvc='; script-src 'self'; img-src 'self'",
		)

		next.ServeHTTP(writer, request)
	})
}
