package wired_test

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"meryl.moe/internal/modules/wired"
	"meryl.moe/internal/platform/db"
)

// mockRenderer captures the last render call for inspection.
type mockRenderer struct {
	lastData any
	err      error
}

func (mock *mockRenderer) Render(
	writer http.ResponseWriter,
	request *http.Request,
	pageFile, fragment string,
	data any,
) error {
	mock.lastData = data

	return mock.err
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	t.Cleanup(func() { database.Close() })

	return database
}

func insertTestUser(t *testing.T, database *sql.DB, username string, password string) string {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	testUserID := "344bab10-0913-4fa0-824c-5ea9d4548d85"
	now := time.Now().Unix()

	if _, err := database.Exec(
		"INSERT INTO users (id, username, password_hash, updated_at, created_at) VALUES (?, ?, ?, ?, ?)",
		testUserID, username, string(hash), now, now,
	); err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	return testUserID
}

func hashToken(raw string) string {
	hash := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(hash[:])
}

func newTestHandler(database *sql.DB, renderer *mockRenderer) *wired.Handler {
	return wired.NewHandler(renderer, database, 168*time.Hour, false)
}

func TestLogin_GET_RendersForm(t *testing.T) {
	renderer := &mockRenderer{}
	handler := newTestHandler(openTestDB(t), renderer)

	recorder := httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodGet, "/wired", nil))

	if recorder.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestAuthenticate_ValidCredentials_RedirectsAndSetsCookie(t *testing.T) {
	database := openTestDB(t)

	insertTestUser(t, database, "lain", "lets all love lain")
	form := url.Values{"username": {"lain"}, "password": {"lets all love lain"}}

	renderer := &mockRenderer{}
	handler := newTestHandler(database, renderer)

	request := httptest.NewRequest(http.MethodPost, "/wired", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()

	handler.Authenticate(recorder, request)

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusSeeOther)
	}

	if location := recorder.Header().Get("Location"); location != "/" {
		t.Errorf("redirect location: got %q, want %q", location, "/")
	}

	var sessionCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "session" {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("session cookie not set")
	}

	if sessionCookie.Value == "" {
		t.Error("session cookie value is empty")
	}

	if !sessionCookie.HttpOnly {
		t.Error("session cookie missing HttpOnly")
	}
}

func TestAuthenticate_WrongPassword_Returns403WithError(t *testing.T) {
	database := openTestDB(t)

	insertTestUser(t, database, "lain", "lets all love lain")
	form := url.Values{"username": {"lain"}, "password": {"uwu"}}

	renderer := &mockRenderer{}
	handler := newTestHandler(database, renderer)

	request := httptest.NewRequest(http.MethodPost, "/wired", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()

	handler.Authenticate(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusForbidden)
	}

	dataMap, ok := renderer.lastData.(map[string]any)
	if !ok {
		t.Fatal("render data is not map[string]any")
	}

	authError, hasError := dataMap["Error"]
	if !hasError {
		t.Error("expected \"Error\" key in render data for wrong password, got none")
	}

	if authError != "invalid credentials" {
		t.Errorf("auth error: got %s, want %s", authError, "invalid credentials")
	}
}

func TestAuthenticate_UnknownUser_Returns403WithError(t *testing.T) {
	form := url.Values{"username": {"baudrillard"}, "password": {"rosetta stoned"}}

	renderer := &mockRenderer{}
	handler := newTestHandler(openTestDB(t), renderer)

	request := httptest.NewRequest(http.MethodPost, "/wired", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()

	handler.Authenticate(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusForbidden)
	}

	dataMap, ok := renderer.lastData.(map[string]any)
	if !ok {
		t.Fatal("render data is not map[string]any")
	}

	authError, hasError := dataMap["Error"]
	if !hasError {
		t.Error("expected \"Error\" key in render data for unknown user, got none")
	}

	if authError != "invalid credentials" {
		t.Errorf("auth error: got %s, want %s", authError, "invalid credentials")
	}
}

func TestAuthenticate_EmptyFields_Returns403WithError(t *testing.T) {
	form := url.Values{"username": {""}, "password": {""}}

	renderer := &mockRenderer{}
	handler := newTestHandler(openTestDB(t), renderer)

	request := httptest.NewRequest(http.MethodPost, "/wired", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()

	handler.Authenticate(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusForbidden)
	}

	dataMap, ok := renderer.lastData.(map[string]any)
	if !ok {
		t.Fatal("render data is not map[string]any")
	}

	authError, hasError := dataMap["Error"]
	if !hasError {
		t.Error("expected \"Error\" key in render data for empty fields, got none")
	}

	if authError != "invalid credentials" {
		t.Errorf("auth error: got %s, want %s", authError, "invalid credentials")
	}
}

func TestLogout_DeletesSessionAndClearsCookie(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database, "lain", "owo")

	// Insert a session with a known raw token so we can verify deletion.
	rawToken := strings.Repeat("letsalllovelain", 6)
	storedHash := hashToken(rawToken)

	now := time.Now().Unix()
	if _, err := database.Exec(
		"INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		storedHash, userID, now, now+3600,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	renderer := &mockRenderer{}
	handler := newTestHandler(database, renderer)

	request := httptest.NewRequest(http.MethodPost, "/wired/logout", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: rawToken})

	recorder := httptest.NewRecorder()

	handler.Logout(recorder, request)

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusSeeOther)
	}

	var count int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM sessions WHERE token_hash = ?", storedHash,
	).Scan(&count); err != nil {
		t.Fatalf("count sessions: %v", err)
	}

	if count != 0 {
		t.Errorf("session not deleted from database after logout")
	}

	var clearedCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "session" {
			clearedCookie = cookie
			break
		}
	}

	if clearedCookie == nil {
		t.Fatal("no session cookie in logout response")
	}

	if clearedCookie.MaxAge != -1 {
		t.Errorf("cookie MaxAge: got %d, want -1", clearedCookie.MaxAge)
	}
}

func TestAuthenticate_RendererError_Returns403(t *testing.T) {
	form := url.Values{"username": {""}, "password": {""}}

	renderer := &mockRenderer{err: errors.New("template failure")}
	handler := newTestHandler(openTestDB(t), renderer)

	request := httptest.NewRequest(http.MethodPost, "/wired", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()

	handler.Authenticate(recorder, request)

	// 403 is written before the renderer runs
	if recorder.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusForbidden)
	}
}
