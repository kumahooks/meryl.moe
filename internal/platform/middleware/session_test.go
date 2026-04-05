package middleware_test

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"meryl.moe/internal/platform/auth"
	"meryl.moe/internal/platform/db"
	"meryl.moe/internal/platform/middleware"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	t.Cleanup(func() { database.Close() })

	return database
}

func insertTestSession(t *testing.T, database *sql.DB, rawToken string, expiresAt int64) {
	t.Helper()

	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	now := time.Now().Unix()

	// Insert a minimal user first to satisfy the foreign key.
	userID := "a2ccf831-0d18-4d77-b153-18cdc2334586"
	if _, err := database.Exec(
		"INSERT INTO users (id, username, password_hash, updated_at, created_at) VALUES (?, ?, ?, ?, ?)",
		userID, "lain", "PASSWORD_HASH", now, now,
	); err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	if _, err := database.Exec(
		"INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		tokenHash, userID, now, expiresAt,
	); err != nil {
		t.Fatalf("insert test session: %v", err)
	}
}

func TestLoadAuth_ValidSession_InjectsUser(t *testing.T) {
	database := openTestDB(t)
	rawToken := strings.Repeat("lain", 12)

	insertTestSession(t, database, rawToken, time.Now().Add(time.Hour).Unix())

	var capturedUser auth.User
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		user, ok := auth.AuthUser(request.Context())
		if !ok {
			t.Error("expected user in context, got none")
			return
		}

		capturedUser = user
	})

	handler := middleware.LoadAuth(database)(next)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: rawToken})

	handler.ServeHTTP(httptest.NewRecorder(), request)

	if capturedUser.ID == "" {
		t.Error("user ID in context is empty")
	}

	if capturedUser.Username == "" {
		t.Error("username in context is empty")
	}
}

func TestLoadAuth_NoCookie_NoUserInContext(t *testing.T) {
	database := openTestDB(t)

	nextCalled := false
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		nextCalled = true

		if _, ok := auth.AuthUser(request.Context()); ok {
			t.Error("expected no user in context, got one")
		}
	})

	handler := middleware.LoadAuth(database)(next)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !nextCalled {
		t.Error("no cookie: next handler was not called")
	}
}

func TestLoadAuth_ExpiredSession_NoUserInContext(t *testing.T) {
	database := openTestDB(t)
	rawToken := strings.Repeat("lain42", 8)

	insertTestSession(t, database, rawToken, time.Now().Add(-time.Hour).Unix())

	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, ok := auth.AuthUser(request.Context()); ok {
			t.Error("expected no user for expired session, got one")
		}
	})

	handler := middleware.LoadAuth(database)(next)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: rawToken})

	handler.ServeHTTP(httptest.NewRecorder(), request)
}

func TestLoadAuth_InvalidToken_NoUserInContext(t *testing.T) {
	database := openTestDB(t)

	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, ok := auth.AuthUser(request.Context()); ok {
			t.Error("expected no user for invalid token, got one")
		}
	})

	handler := middleware.LoadAuth(database)(next)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: "notavalidtoken"})

	handler.ServeHTTP(httptest.NewRecorder(), request)
}

func TestRequireAuth_ValidSession_CallsNext(t *testing.T) {
	database := openTestDB(t)
	rawToken := strings.Repeat("lainuwu", 8)

	insertTestSession(t, database, rawToken, time.Now().Add(time.Hour).Unix())

	nextCalled := false
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		nextCalled = true
	})

	handler := middleware.RequireAuth(database)(next)

	request := httptest.NewRequest(http.MethodGet, "/relay", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: rawToken})

	handler.ServeHTTP(httptest.NewRecorder(), request)

	if !nextCalled {
		t.Error("next handler was not called for valid session")
	}
}

func TestRequireAuth_NoSession_RedirectsToRoot(t *testing.T) {
	database := openTestDB(t)

	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Error("next handler must not be called when session is absent")
	})

	handler := middleware.RequireAuth(database)(next)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusSeeOther)
	}

	if location := recorder.Header().Get("Location"); location != "/" {
		t.Errorf("redirect location: got %q, want %q", location, "/")
	}
}
