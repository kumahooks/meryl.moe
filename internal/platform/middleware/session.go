package middleware

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"time"

	"meryl.moe/internal/platform/auth"
)

// LoadAuth validates the session cookie against the database and injects the
// user into the request context if the session is valid and not expired.
// Always calls next regardless of auth state (use RequireAuth to enforce auth).
func LoadAuth(database *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			cookie, err := request.Cookie("session")
			if err != nil {
				next.ServeHTTP(writer, request)
				return
			}

			tokenHash := hashSessionToken(cookie.Value)

			// TODO: I should at some point think about cache here,
			// otherwise every auth request will do this and it's kinda... eh
			var userID, username string
			err = database.QueryRow(
				`SELECT s.user_id, u.username
				 FROM sessions s
				 JOIN users u ON u.id = s.user_id
				 WHERE s.token_hash = ? AND s.expires_at > ?`,
				tokenHash, time.Now().Unix(),
			).Scan(&userID, &username)
			if err != nil {
				next.ServeHTTP(writer, request)
				return
			}

			ctx := auth.WithUser(
				request.Context(),
				auth.User{
					ID:       userID,
					Username: username,
				})

			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	}
}

// RequireAuth wraps LoadAuth and redirects to / if no valid session is
// present.
func RequireAuth(database *sql.DB) func(http.Handler) http.Handler {
	load := LoadAuth(database)

	return func(next http.Handler) http.Handler {
		return load(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if _, ok := auth.AuthUser(request.Context()); !ok {
				http.Redirect(writer, request, "/", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(writer, request)
		}))
	}
}

// hashSessionToken returns the hex-encoded SHA-256 hash of the raw token.
func hashSessionToken(raw string) string {
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}
