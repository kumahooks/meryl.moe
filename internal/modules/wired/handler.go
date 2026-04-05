// Package wired implements the login, logout, and session management handlers.
package wired

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
	"meryl.moe/internal/platform/auth"
	"meryl.moe/internal/platform/middleware"
	"meryl.moe/internal/platform/templates"
)

// dummyPasswordHash is computed once at startup and used when a login attempt
// references a username that does not exist.
var dummyPasswordHash []byte

func init() {
	var err error

	dummyPasswordHash, err = bcrypt.GenerateFromPassword([]byte("owo7"), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("wired: generate dummy password hash: %v", err))
	}
}

// Handler handles login, logout, and session creation.
type Handler struct {
	database     *sql.DB
	renderer     templates.Renderer
	sessionTTL   time.Duration
	secureCookie bool
}

// NewHandler returns a Handler backed by the given database and renderer.
func NewHandler(renderer templates.Renderer, database *sql.DB, sessionTTL time.Duration, isDevelopment bool) *Handler {
	return &Handler{
		database:     database,
		renderer:     renderer,
		sessionTTL:   sessionTTL,
		secureCookie: !isDevelopment,
	}
}

// Routes registers the wired (auth) routes on the given router.
func Routes(handler *Handler) func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/wired", handler.Login)
		router.Post("/wired", handler.Authenticate)
		router.Post("/wired/logout", handler.Logout)
		router.With(middleware.RequireAuth(handler.database)).Get("/wired/me", handler.Me)
	}
}

// Login renders the login form.
func (handler *Handler) Login(writer http.ResponseWriter, request *http.Request) {
	handler.renderLogin(writer, request, "")
}

// Authenticate validates credentials, creates a session, and redirects to /relay.
func (handler *Handler) Authenticate(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1024)
	if err := request.ParseForm(); err != nil {
		writer.WriteHeader(http.StatusForbidden)
		handler.renderLogin(writer, request, "invalid credentials")

		return
	}

	username := request.FormValue("username")
	password := request.FormValue("password")

	if username == "" || password == "" {
		writer.WriteHeader(http.StatusForbidden)
		handler.renderLogin(writer, request, "invalid credentials")

		return
	}

	var userID string
	var storedHash string
	userFound := true

	err := handler.database.QueryRow(
		"SELECT id, password_hash FROM users WHERE username = ?", username,
	).Scan(&userID, &storedHash)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("wired: query user: %v", err)
			writer.WriteHeader(http.StatusForbidden)
			handler.renderLogin(writer, request, "invalid credentials")

			return
		}

		// User not found, we then use dummy hash
		userFound = false
		storedHash = string(dummyPasswordHash)
	}

	compareErr := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))

	if !userFound || compareErr != nil {
		writer.WriteHeader(http.StatusForbidden)
		handler.renderLogin(writer, request, "invalid credentials")

		return
	}

	rawToken, tokenHash, err := generateSessionToken()
	if err != nil {
		log.Printf("wired: generate session token: %v", err)
		writer.WriteHeader(http.StatusInternalServerError)
		handler.renderLogin(writer, request, "internal server error")

		return
	}

	now := time.Now()
	expiresAt := now.Add(handler.sessionTTL)

	if _, err := handler.database.Exec(
		"INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		tokenHash, userID, now.Unix(), expiresAt.Unix(),
	); err != nil {
		log.Printf("wired: insert session: %v", err)
		writer.WriteHeader(http.StatusInternalServerError)
		handler.renderLogin(writer, request, "internal server error")

		return
	}

	if _, err := handler.database.Exec(
		"UPDATE users SET last_login_at = ? WHERE id = ?",
		now.Unix(), userID,
	); err != nil {
		// Non-fatal: session was created successfully.
		log.Printf("wired: update last_login_at: %v", err)
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     "session",
		Value:    rawToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   handler.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(writer, request, "/wired/me", http.StatusSeeOther)
}

// Logout deletes the session from the database, clears the cookie, and redirects to /.
func (handler *Handler) Logout(writer http.ResponseWriter, request *http.Request) {
	cookie, err := request.Cookie("session")
	if err == nil {
		tokenHash := hashToken(cookie.Value)

		if _, err := handler.database.Exec(
			"DELETE FROM sessions WHERE token_hash = ?", tokenHash,
		); err != nil {
			log.Printf("wired: delete session: %v", err)
		}
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   handler.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

// Me renders the authenticated user status page.
func (handler *Handler) Me(writer http.ResponseWriter, request *http.Request) {
	user, _ := auth.AuthUser(request.Context())

	pageFile := "modules/wired/me.html"
	data := map[string]any{"Title": "wired/me", "User": user}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		log.Printf("wired: render me: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func (handler *Handler) renderLogin(writer http.ResponseWriter, request *http.Request, errorMessage string) {
	pageFile := "modules/wired/wired.html"
	data := map[string]any{"Title": "wired"}

	if errorMessage != "" {
		data["Error"] = errorMessage
	}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		log.Printf("wired: render login: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// generateSessionToken creates a random session token.
// Returns the raw token (sent to the client in the cookie) and its SHA-256
// hash (stored in the database).
func generateSessionToken() (raw string, tokenHash string, err error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", fmt.Errorf("read random bytes: %w", err)
	}

	raw = hex.EncodeToString(tokenBytes)
	tokenHash = hashToken(raw)

	return raw, tokenHash, nil
}

// hashToken returns the hex-encoded SHA-256 hash of the given raw token string.
func hashToken(raw string) string {
	hash := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(hash[:])
}
