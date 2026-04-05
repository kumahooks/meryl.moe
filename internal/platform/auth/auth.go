// Package auth defines the authenticated user type, context accessors,
// and the Service that manages session lifecycle.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials is returned by Authenticate when the username/password
// pair is not valid.
var ErrInvalidCredentials = errors.New("invalid credentials")

type authContextKey struct{}

// User holds the authenticated user's identity injected into the request context.
type User struct {
	ID       string
	Username string
}

// WithUser returns a new context with the given User stored under the auth key.
func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, authContextKey{}, user)
}

// AuthUser returns the authenticated User from the request context.
// The second return value is false if no authenticated user is present.
func AuthUser(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(authContextKey{}).(User)

	return user, ok
}

// Service manages session lifecycle: credential validation, session creation,
// and session deletion.
type Service struct {
	database          *sql.DB
	sessionTTL        time.Duration
	dummyPasswordHash []byte
}

// NewService creates a Service backed by the given database.
// It pre-computes a dummy bcrypt hash at startup.
func NewService(database *sql.DB, sessionTTL time.Duration) (*Service, error) {
	dummyPasswordHash, err := bcrypt.GenerateFromPassword([]byte("owo7"), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("generate dummy password hash: %w", err)
	}

	return &Service{
		database:          database,
		sessionTTL:        sessionTTL,
		dummyPasswordHash: dummyPasswordHash,
	}, nil
}

// Authenticate validates the username/password pair, creates a session, and
// returns the raw session token and its expiry time.
// Returns ErrInvalidCredentials if the credentials are wrong.
func (service *Service) Authenticate(
	username string,
	password string,
) (rawToken string, expiresAt time.Time, err error) {
	var userID string
	var storedHash string

	userFound := true

	queryErr := service.database.QueryRow(
		"SELECT id, password_hash FROM users WHERE username = ?", username,
	).Scan(&userID, &storedHash)
	if queryErr != nil {
		if !errors.Is(queryErr, sql.ErrNoRows) {
			return "", time.Time{}, fmt.Errorf("query user: %w", queryErr)
		}

		// If we don't find the user we simply proceed with the dummy hash
		userFound = false
		storedHash = string(service.dummyPasswordHash)
	}

	compareErr := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
	if !userFound || compareErr != nil {
		return "", time.Time{}, ErrInvalidCredentials
	}

	rawToken, tokenHash, err := generateSessionToken()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate token: %w", err)
	}

	now := time.Now()
	expiresAt = now.Add(service.sessionTTL)

	if _, err := service.database.Exec(
		"INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		tokenHash, userID, now.Unix(), expiresAt.Unix(),
	); err != nil {
		return "", time.Time{}, fmt.Errorf("insert session: %w", err)
	}

	if _, err := service.database.Exec(
		"UPDATE users SET last_login_at = ? WHERE id = ?",
		now.Unix(), userID,
	); err != nil {
		// Non-fatal: session was created successfully.
		log.Printf("auth: update last_login_at: %v", err)
	}

	return rawToken, expiresAt, nil
}

// Logout deletes the session identified by rawToken from the database.
func (service *Service) Logout(rawToken string) error {
	tokenHash := HashToken(rawToken)

	if _, err := service.database.Exec(
		"DELETE FROM sessions WHERE token_hash = ?", tokenHash,
	); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// HashToken returns the hex-encoded SHA-256 hash of the given raw token string.
// TODO: is this safe?
func HashToken(raw string) string {
	hash := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(hash[:])
}

// generateSessionToken creates a random 32-byte session token.
// Returns the raw token (sent to the client) and its SHA-256 hash (stored in DB).
// TODO: small collision chance. Absurdly small chance. Do we care?
func generateSessionToken() (raw string, tokenHash string, err error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", fmt.Errorf("read random bytes: %w", err)
	}

	raw = hex.EncodeToString(tokenBytes)
	tokenHash = HashToken(raw)

	return raw, tokenHash, nil
}
