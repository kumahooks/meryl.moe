package relay_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"meryl.moe/internal/modules/relay"
	"meryl.moe/internal/platform/auth"
)

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

func withRelayRouteID(request *http.Request, relayID string) *http.Request {
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", relayID)

	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func TestIndex_RendersPage(t *testing.T) {
	renderer := &mockRenderer{}
	handler := relay.NewHandler(renderer, relay.NewService(openTestDB(t)))

	recorder := httptest.NewRecorder()
	handler.Index(recorder, httptest.NewRequest(http.MethodGet, "/relay", nil))

	if recorder.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestIndex_WithAuthenticatedUser_SetsUserInData(t *testing.T) {
	renderer := &mockRenderer{}
	handler := relay.NewHandler(renderer, relay.NewService(openTestDB(t)))

	request := httptest.NewRequest(http.MethodGet, "/relay", nil)
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: "123", Username: "lain"}))

	handler.Index(httptest.NewRecorder(), request)

	dataMap, ok := renderer.lastData.(map[string]any)
	if !ok {
		t.Fatal("render data is not map[string]any")
	}

	if _, hasUser := dataMap["User"]; !hasUser {
		t.Error("expected User key in render data for authenticated request")
	}
}

func TestIndex_RendererError_Returns500(t *testing.T) {
	renderer := &mockRenderer{err: errors.New("template failure")}
	handler := relay.NewHandler(renderer, relay.NewService(openTestDB(t)))

	recorder := httptest.NewRecorder()
	handler.Index(recorder, httptest.NewRequest(http.MethodGet, "/relay", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestSave_ValidContent_ReturnsLinkFragment(t *testing.T) {
	form := url.Values{"text": {"hello, wired"}}

	database := openTestDB(t)
	userID := insertTestUser(t, database)

	handler := relay.NewHandler(&mockRenderer{}, relay.NewService(database))

	request := httptest.NewRequest(http.MethodPost, "/relay", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: userID, Username: "lain"}))

	recorder := httptest.NewRecorder()
	handler.Save(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	if body := recorder.Body.String(); !strings.Contains(body, "/relay/") {
		t.Errorf("response does not contain relay link: %q", body)
	}
}

func TestSave_EmptyContent_Returns400(t *testing.T) {
	form := url.Values{"text": {""}}

	database := openTestDB(t)
	userID := insertTestUser(t, database)

	handler := relay.NewHandler(&mockRenderer{}, relay.NewService(database))

	request := httptest.NewRequest(http.MethodPost, "/relay", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: userID, Username: "lain"}))

	recorder := httptest.NewRecorder()
	handler.Save(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestSave_Unauthenticated_Returns500(t *testing.T) {
	// RequireAuth middleware blocks this in production, but we verify the handler
	// fails safely rather than persisting a relay with no owner.
	form := url.Values{"text": {"some content"}}

	handler := relay.NewHandler(&mockRenderer{}, relay.NewService(openTestDB(t)))

	request := httptest.NewRequest(http.MethodPost, "/relay", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()
	handler.Save(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestSave_OversizedBody_Returns400(t *testing.T) {
	// 1MB + 1 byte exceeds MaxBytesReader limit of 1<<20.
	largeContent := strings.Repeat("x", 1<<20+1)
	form := url.Values{"text": {largeContent}}

	database := openTestDB(t)
	userID := insertTestUser(t, database)

	handler := relay.NewHandler(&mockRenderer{}, relay.NewService(database))

	request := httptest.NewRequest(http.MethodPost, "/relay", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: userID, Username: "lain"}))

	recorder := httptest.NewRecorder()
	handler.Save(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestSave_MalformedBody_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	handler := relay.NewHandler(&mockRenderer{}, relay.NewService(database))

	// Malformed body: not valid form encoding, no text field - results in empty text.
	request := httptest.NewRequest(http.MethodPost, "/relay", strings.NewReader(";;;===%%%"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: userID, Username: "lain"}))

	recorder := httptest.NewRecorder()
	handler.Save(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestSave_HTMLContentInLink_DoesNotEscapeRelayID(t *testing.T) {
	// Verify the returned anchor href contains the relay ID, not injected content.
	form := url.Values{"text": {"<script>alert(1)</script>"}}

	database := openTestDB(t)
	userID := insertTestUser(t, database)

	handler := relay.NewHandler(&mockRenderer{}, relay.NewService(database))

	request := httptest.NewRequest(http.MethodPost, "/relay", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: userID, Username: "lain"}))

	recorder := httptest.NewRecorder()
	handler.Save(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	if strings.Contains(body, "<script>") {
		t.Errorf("response contains raw script tag: %q", body)
	}

	if !strings.Contains(body, `href="/relay/`) {
		t.Errorf("response does not contain relay href: %q", body)
	}
}

func TestView_SeededRelay_RendersContent(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	relayID, err := service.Save(userID, "let's all love lain")
	if err != nil {
		t.Fatalf("seed relay: %v", err)
	}

	renderer := &mockRenderer{}
	handler := relay.NewHandler(renderer, service)

	recorder := httptest.NewRecorder()
	handler.View(recorder, withRelayRouteID(httptest.NewRequest(http.MethodGet, "/relay/"+relayID, nil), relayID))

	if recorder.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	dataMap, ok := renderer.lastData.(map[string]any)
	if !ok {
		t.Fatal("render data is not map[string]any")
	}

	content, ok := dataMap["Content"].(string)
	if !ok {
		t.Fatal("Content key missing or not a string")
	}

	if content != "let's all love lain" {
		t.Errorf("content: got %q, want %q", content, "let's all love lain")
	}
}

func TestView_SeededRelay_IsReadOnly(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	relayID, err := service.Save(userID, "archived")
	if err != nil {
		t.Fatalf("seed relay: %v", err)
	}

	renderer := &mockRenderer{}
	handler := relay.NewHandler(renderer, service)

	handler.View(
		httptest.NewRecorder(),
		withRelayRouteID(httptest.NewRequest(http.MethodGet, "/relay/"+relayID, nil), relayID),
	)

	dataMap, ok := renderer.lastData.(map[string]any)
	if !ok {
		t.Fatal("render data is not map[string]any")
	}

	readOnly, _ := dataMap["ReadOnly"].(bool)
	if !readOnly {
		t.Error("expected ReadOnly to be true for /relay/:id view")
	}
}

func TestView_UnknownID_RedirectsToRelay(t *testing.T) {
	handler := relay.NewHandler(&mockRenderer{}, relay.NewService(openTestDB(t)))

	recorder := httptest.NewRecorder()
	handler.View(recorder, withRelayRouteID(httptest.NewRequest(http.MethodGet, "/relay/owo", nil), "owo"))

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusSeeOther)
	}

	if location := recorder.Header().Get("Location"); location != "/relay" {
		t.Errorf("redirect location: got %q, want %q", location, "/relay")
	}
}

func TestView_SQLInjectionID_RedirectsToRelay(t *testing.T) {
	handler := relay.NewHandler(&mockRenderer{}, relay.NewService(openTestDB(t)))

	maliciousIDs := []string{
		"' OR '1'='1",
		"'; DROP TABLE relays; --",
	}

	for _, maliciousID := range maliciousIDs {
		recorder := httptest.NewRecorder()
		handler.View(recorder, withRelayRouteID(httptest.NewRequest(http.MethodGet, "/relay/x", nil), maliciousID))

		if recorder.Code != http.StatusSeeOther {
			t.Errorf("SQL injection ID %q: got status %d, want %d", maliciousID, recorder.Code, http.StatusSeeOther)
		}
	}
}

func TestView_AuthenticatedUser_DoesNotSetUserInData(t *testing.T) {
	// Save button must not appear on read-only views; User is intentionally omitted.
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	relayID, err := service.Save(userID, "text")
	if err != nil {
		t.Fatalf("seed relay: %v", err)
	}

	renderer := &mockRenderer{}
	handler := relay.NewHandler(renderer, service)

	request := withRelayRouteID(httptest.NewRequest(http.MethodGet, "/relay/"+relayID, nil), relayID)
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: userID, Username: "lain"}))

	handler.View(httptest.NewRecorder(), request)

	dataMap, ok := renderer.lastData.(map[string]any)
	if !ok {
		t.Fatal("render data is not map[string]any")
	}

	if _, hasUser := dataMap["User"]; hasUser {
		t.Error("User must not be set in render data for read-only view")
	}
}

func TestView_RendererError_Returns500(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	relayID, err := service.Save(userID, "text")
	if err != nil {
		t.Fatalf("seed relay: %v", err)
	}

	renderer := &mockRenderer{err: errors.New("template failure")}
	handler := relay.NewHandler(renderer, service)

	recorder := httptest.NewRecorder()
	handler.View(recorder, withRelayRouteID(httptest.NewRequest(http.MethodGet, "/relay/"+relayID, nil), relayID))

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
