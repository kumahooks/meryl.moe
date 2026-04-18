package kipple_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"meryl.moe/internal/modules/kipple"
	"meryl.moe/internal/platform/auth"
)

// mockRenderer satisfies templates.Renderer for handler tests.
type mockRenderer struct {
	lastFragment string
	lastData     any
	err          error
}

func (mock *mockRenderer) Render(
	writer http.ResponseWriter,
	request *http.Request,
	pageFile, fragment string,
	data any,
) error {
	mock.lastFragment = fragment
	mock.lastData = data

	return mock.err
}

func withKippleRouteID(request *http.Request, id string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id)

	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeCtx))
}

func withAuth(request *http.Request, userID string) *http.Request {
	return request.WithContext(auth.WithUser(request.Context(), auth.User{ID: userID, Username: "lain"}))
}

// tusMetadata builds an Upload-Metadata header value matching the tus base64 encoding.
func tusMetadata(pairs ...string) string {
	if len(pairs)%2 != 0 {
		panic("tusMetadata: odd number of args")
	}

	parts := make([]string, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		encoded := base64.StdEncoding.EncodeToString([]byte(pairs[i+1]))
		parts = append(parts, pairs[i]+" "+encoded)
	}

	return strings.Join(parts, ", ")
}

func newHandlerWithRenderer(t *testing.T, renderer *mockRenderer) (*kipple.Handler, *kipple.Service) {
	t.Helper()

	database := openTestDB(t)
	service, _ := newService(t, database)

	return kipple.NewHandler(renderer, service), service
}

func TestHandler_Index_RendersPage(t *testing.T) {
	renderer := &mockRenderer{}
	handler, _ := newHandlerWithRenderer(t, renderer)

	recorder := httptest.NewRecorder()
	handler.Index(recorder, httptest.NewRequest(http.MethodGet, "/kipple", nil))

	if recorder.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	if renderer.lastFragment != "page-content" {
		t.Errorf("fragment: got %q, want page-content", renderer.lastFragment)
	}
}

func TestHandler_Index_WithAuthUser_SetsUserInData(t *testing.T) {
	renderer := &mockRenderer{}
	database := openTestDB(t)

	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(renderer, service)

	request := withAuth(httptest.NewRequest(http.MethodGet, "/kipple", nil), userID)

	handler.Index(httptest.NewRecorder(), request)

	dataMap, ok := renderer.lastData.(map[string]any)
	if !ok {
		t.Fatal("render data is not map[string]any")
	}

	if _, hasUser := dataMap["User"]; !hasUser {
		t.Error("expected User key for authenticated request")
	}
}

func TestHandler_Index_WithoutAuth_NoUserInData(t *testing.T) {
	renderer := &mockRenderer{}
	handler, _ := newHandlerWithRenderer(t, renderer)

	handler.Index(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/kipple", nil))

	dataMap, ok := renderer.lastData.(map[string]any)
	if !ok {
		t.Fatal("render data is not map[string]any")
	}

	if _, hasUser := dataMap["User"]; hasUser {
		t.Error("unexpected User key for unauthenticated request")
	}
}

func TestHandler_Index_RendererError_Returns500(t *testing.T) {
	renderer := &mockRenderer{err: errors.New("template failure")}
	handler, _ := newHandlerWithRenderer(t, renderer)

	recorder := httptest.NewRecorder()
	handler.Index(recorder, httptest.NewRequest(http.MethodGet, "/kipple", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestHandler_CreateUpload_MissingUploadLength_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	handler, _ := newHandlerWithRenderer(t, &mockRenderer{})

	request := withAuth(httptest.NewRequest(http.MethodPost, "/kipple/upload", nil), userID)
	recorder := httptest.NewRecorder()

	handler.CreateUpload(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandler_CreateUpload_ZeroLength_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	handler, _ := newHandlerWithRenderer(t, &mockRenderer{})

	request := withAuth(httptest.NewRequest(http.MethodPost, "/kipple/upload", nil), userID)
	request.Header.Set("Upload-Length", "0")

	recorder := httptest.NewRecorder()
	handler.CreateUpload(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandler_CreateUpload_NegativeLength_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	handler, _ := newHandlerWithRenderer(t, &mockRenderer{})

	request := withAuth(httptest.NewRequest(http.MethodPost, "/kipple/upload", nil), userID)
	request.Header.Set("Upload-Length", "-1")

	recorder := httptest.NewRecorder()
	handler.CreateUpload(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandler_CreateUpload_NonNumericLength_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	handler, _ := newHandlerWithRenderer(t, &mockRenderer{})

	request := withAuth(httptest.NewRequest(http.MethodPost, "/kipple/upload", nil), userID)
	request.Header.Set("Upload-Length", "owo")

	recorder := httptest.NewRecorder()
	handler.CreateUpload(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandler_CreateUpload_MissingFilename_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	handler, _ := newHandlerWithRenderer(t, &mockRenderer{})

	request := withAuth(httptest.NewRequest(http.MethodPost, "/kipple/upload", nil), userID)
	request.Header.Set("Upload-Length", "1024")

	// No Upload-Metadata header at all.
	recorder := httptest.NewRecorder()
	handler.CreateUpload(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandler_CreateUpload_EmptyFilename_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	handler, _ := newHandlerWithRenderer(t, &mockRenderer{})

	request := withAuth(httptest.NewRequest(http.MethodPost, "/kipple/upload", nil), userID)
	request.Header.Set("Upload-Length", "1024")
	request.Header.Set("Upload-Metadata", tusMetadata("filename", "   "))

	recorder := httptest.NewRecorder()
	handler.CreateUpload(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandler_CreateUpload_QuotaExceeded_Returns413(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	request := withAuth(httptest.NewRequest(http.MethodPost, "/kipple/upload", nil), userID)
	request.Header.Set("Upload-Length", fmt.Sprintf("%d", testQuota+1))
	request.Header.Set("Upload-Metadata", tusMetadata("filename", "big.bin"))

	recorder := httptest.NewRecorder()
	handler.CreateUpload(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestHandler_CreateUpload_ValidRequest_Returns201WithLocation(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)

	request := withAuth(httptest.NewRequest(http.MethodPost, "/kipple/upload", nil), userID)
	request.Header.Set("Upload-Length", fmt.Sprintf("%d", len(content)))
	request.Header.Set("Upload-Metadata", tusMetadata("filename", "03.gif", "visibility", "link", "expire_days", "1"))

	recorder := httptest.NewRecorder()
	handler.CreateUpload(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusCreated)
	}

	location := recorder.Header().Get("Location")
	if !strings.HasPrefix(location, "/kipple/upload/") {
		t.Errorf("Location: got %q, want prefix /kipple/upload/", location)
	}
}

func TestHandler_CreateUpload_InvalidVisibility_DefaultsToLink(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)

	request := withAuth(httptest.NewRequest(http.MethodPost, "/kipple/upload", nil), userID)
	request.Header.Set("Upload-Length", fmt.Sprintf("%d", len(content)))
	request.Header.Set("Upload-Metadata", tusMetadata("filename", "f.gif", "visibility", "bogus"))

	recorder := httptest.NewRecorder()
	handler.CreateUpload(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusCreated)
	}

	location := recorder.Header().Get("Location")
	uploadID := strings.TrimPrefix(location, "/kipple/upload/")

	upload, err := service.GetUpload(uploadID)
	if err != nil {
		t.Fatalf("get upload: %v", err)
	}

	if upload.Visibility != kipple.VisibilityLink {
		t.Errorf("visibility: got %q, want %q", upload.Visibility, kipple.VisibilityLink)
	}
}

func TestHandler_CreateUpload_InvalidExpireDays_DefaultsTo1(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)

	request := withAuth(httptest.NewRequest(http.MethodPost, "/kipple/upload", nil), userID)
	request.Header.Set("Upload-Length", fmt.Sprintf("%d", len(content)))
	request.Header.Set("Upload-Metadata", tusMetadata("filename", "f.gif", "expire_days", "bogus"))

	recorder := httptest.NewRecorder()
	handler.CreateUpload(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusCreated)
	}
}

func TestHandler_CreateUpload_ZeroExpireDays_DefaultsTo1(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)

	request := withAuth(httptest.NewRequest(http.MethodPost, "/kipple/upload", nil), userID)
	request.Header.Set("Upload-Length", fmt.Sprintf("%d", len(content)))
	request.Header.Set("Upload-Metadata", tusMetadata("filename", "f.gif", "expire_days", "0"))

	recorder := httptest.NewRecorder()
	handler.CreateUpload(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusCreated)
	}
}

func TestHandler_HeadUpload_NotFound_Returns404(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodHead, "/kipple/upload/uwu", nil), "uwu"),
		userID,
	)

	recorder := httptest.NewRecorder()
	handler.HeadUpload(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestHandler_HeadUpload_WrongUser_Returns404(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	otherID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload, err := service.CreateUpload(userID, "f.gif", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodHead, "/kipple/upload/"+upload.ID, nil), upload.ID),
		otherID,
	)

	recorder := httptest.NewRecorder()
	handler.HeadUpload(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestHandler_HeadUpload_Valid_ReturnsOffsetAndLength(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)

	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodHead, "/kipple/upload/"+upload.ID, nil), upload.ID),
		userID,
	)

	recorder := httptest.NewRecorder()
	handler.HeadUpload(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNoContent)
	}

	if recorder.Header().Get("Upload-Offset") != "0" {
		t.Errorf("Upload-Offset: got %q, want 0", recorder.Header().Get("Upload-Offset"))
	}

	wantLength := fmt.Sprintf("%d", len(content))
	if recorder.Header().Get("Upload-Length") != wantLength {
		t.Errorf("Upload-Length: got %q, want %q", recorder.Header().Get("Upload-Length"), wantLength)
	}
}

func TestHandler_AppendChunk_WrongContentType_Returns415(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload, err := service.CreateUpload(userID, "f.gif", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodPatch, "/kipple/upload/"+upload.ID, nil), upload.ID),
		userID,
	)
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("Upload-Offset", "0")

	recorder := httptest.NewRecorder()
	handler.AppendChunk(recorder, request)

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusUnsupportedMediaType)
	}
}

func TestHandler_AppendChunk_MissingUploadOffset_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload, err := service.CreateUpload(userID, "f.gif", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodPatch, "/kipple/upload/"+upload.ID, nil), upload.ID),
		userID,
	)
	request.Header.Set("Content-Type", "application/offset+octet-stream")
	recorder := httptest.NewRecorder()

	handler.AppendChunk(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandler_AppendChunk_NegativeOffset_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload, err := service.CreateUpload(userID, "f.gif", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodPatch, "/kipple/upload/"+upload.ID, nil), upload.ID),
		userID,
	)
	request.Header.Set("Content-Type", "application/offset+octet-stream")
	request.Header.Set("Upload-Offset", "-1")

	recorder := httptest.NewRecorder()
	handler.AppendChunk(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandler_AppendChunk_NonNumericOffset_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload, err := service.CreateUpload(userID, "f.gif", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodPatch, "/kipple/upload/"+upload.ID, nil), upload.ID),
		userID,
	)
	request.Header.Set("Content-Type", "application/offset+octet-stream")
	request.Header.Set("Upload-Offset", "abc")

	recorder := httptest.NewRecorder()
	handler.AppendChunk(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandler_AppendChunk_NotFound_Returns404(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)

	request := withAuth(
		withKippleRouteID(
			httptest.NewRequest(http.MethodPatch, "/kipple/upload/wire", bytes.NewReader(content)),
			"wire",
		),
		userID,
	)
	request.Header.Set("Content-Type", "application/offset+octet-stream")
	request.Header.Set("Upload-Offset", "0")
	request.Header.Set("Upload-Checksum", sha1Header(content))

	recorder := httptest.NewRecorder()
	handler.AppendChunk(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestHandler_AppendChunk_WrongUser_Returns403(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	otherID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)

	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(
			httptest.NewRequest(http.MethodPatch, "/kipple/upload/"+upload.ID, bytes.NewReader(content)),
			upload.ID,
		),
		otherID,
	)
	request.Header.Set("Content-Type", "application/offset+octet-stream")
	request.Header.Set("Upload-Offset", "0")
	request.Header.Set("Upload-Checksum", sha1Header(content))

	recorder := httptest.NewRecorder()
	handler.AppendChunk(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestHandler_AppendChunk_OffsetMismatch_Returns409(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)

	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	mid := len(content) / 2
	skippedChunk := content[mid:]

	request := withAuth(
		withKippleRouteID(
			httptest.NewRequest(http.MethodPatch, "/kipple/upload/"+upload.ID, bytes.NewReader(skippedChunk)),
			upload.ID,
		),
		userID,
	)
	request.Header.Set("Content-Type", "application/offset+octet-stream")
	request.Header.Set("Upload-Offset", fmt.Sprintf("%d", mid))
	request.Header.Set("Upload-Checksum", sha1Header(skippedChunk))

	recorder := httptest.NewRecorder()
	handler.AppendChunk(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func TestHandler_AppendChunk_UnsupportedChecksum_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)

	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(
			httptest.NewRequest(http.MethodPatch, "/kipple/upload/"+upload.ID, bytes.NewReader(content)),
			upload.ID,
		),
		userID,
	)
	request.Header.Set("Content-Type", "application/offset+octet-stream")
	request.Header.Set("Upload-Offset", "0")
	request.Header.Set("Upload-Checksum", "md5 uwu=")

	recorder := httptest.NewRecorder()
	handler.AppendChunk(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandler_AppendChunk_ChecksumMismatch_Returns400(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)

	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(
			httptest.NewRequest(http.MethodPatch, "/kipple/upload/"+upload.ID, bytes.NewReader(content)),
			upload.ID,
		),
		userID,
	)
	request.Header.Set("Content-Type", "application/offset+octet-stream")
	request.Header.Set("Upload-Offset", "0")
	request.Header.Set("Upload-Checksum", sha1Header([]byte("wrong data entirely")))

	recorder := httptest.NewRecorder()
	handler.AppendChunk(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandler_AppendChunk_Valid_Returns204WithUpdatedOffset(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)

	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(
			httptest.NewRequest(http.MethodPatch, "/kipple/upload/"+upload.ID, bytes.NewReader(content)),
			upload.ID,
		),
		userID,
	)
	request.Header.Set("Content-Type", "application/offset+octet-stream")
	request.Header.Set("Upload-Offset", "0")
	request.Header.Set("Upload-Checksum", sha1Header(content))

	recorder := httptest.NewRecorder()
	handler.AppendChunk(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNoContent)
	}

	wantOffset := fmt.Sprintf("%d", len(content))
	if recorder.Header().Get("Upload-Offset") != wantOffset {
		t.Errorf("Upload-Offset: got %q, want %q", recorder.Header().Get("Upload-Offset"), wantOffset)
	}
}

func TestHandler_AppendChunk_AlreadyComplete_Returns403(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload := createCompleteUpload(t, service, userID)
	extra := []byte("extra")

	request := withAuth(
		withKippleRouteID(
			httptest.NewRequest(http.MethodPatch, "/kipple/upload/"+upload.ID, bytes.NewReader(extra)),
			upload.ID,
		),
		userID,
	)
	request.Header.Set("Content-Type", "application/offset+octet-stream")
	request.Header.Set("Upload-Offset", fmt.Sprintf("%d", upload.Size))
	request.Header.Set("Upload-Checksum", sha1Header(extra))

	recorder := httptest.NewRecorder()
	handler.AppendChunk(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestHandler_TerminateUpload_NotFound_Returns404(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodDelete, "/kipple/upload/uwu", nil), "uwu"),
		userID,
	)

	recorder := httptest.NewRecorder()
	handler.TerminateUpload(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestHandler_TerminateUpload_WrongUser_Returns403(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	otherID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload, err := service.CreateUpload(userID, "f.gif", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodDelete, "/kipple/upload/"+upload.ID, nil), upload.ID),
		otherID,
	)

	recorder := httptest.NewRecorder()
	handler.TerminateUpload(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestHandler_TerminateUpload_Valid_Returns204(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload, err := service.CreateUpload(userID, "f.gif", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodDelete, "/kipple/upload/"+upload.ID, nil), upload.ID),
		userID,
	)

	recorder := httptest.NewRecorder()
	handler.TerminateUpload(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestHandler_TerminateUpload_RemovesDBRow(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload, err := service.CreateUpload(userID, "f.gif", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodDelete, "/kipple/upload/"+upload.ID, nil), upload.ID),
		userID,
	)

	handler.TerminateUpload(httptest.NewRecorder(), request)

	if _, err := service.GetUpload(upload.ID); !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("DB row must be deleted after terminate; GetUpload returned %v, want ErrNotFound", err)
	}
}

func TestHandler_Delete_NotFound_Returns404(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodDelete, "/kipple/owo", nil), "owo"),
		userID,
	)

	recorder := httptest.NewRecorder()
	handler.Delete(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestHandler_Delete_WrongUser_Returns403(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	otherID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload := createCompleteUpload(t, service, userID)

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodDelete, "/kipple/"+upload.ID, nil), upload.ID),
		otherID,
	)

	recorder := httptest.NewRecorder()
	handler.Delete(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestHandler_Delete_Valid_RemovesDBRow(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload := createCompleteUpload(t, service, userID)

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodDelete, "/kipple/"+upload.ID, nil), upload.ID),
		userID,
	)

	handler.Delete(httptest.NewRecorder(), request)

	if _, err := service.GetUpload(upload.ID); !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("DB row must be deleted after delete; GetUpload returned %v, want ErrNotFound", err)
	}
}

func TestHandler_Delete_Valid_RendersQuotaFragment(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	renderer := &mockRenderer{}
	handler := kipple.NewHandler(renderer, service)

	upload := createCompleteUpload(t, service, userID)

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodDelete, "/kipple/"+upload.ID, nil), upload.ID),
		userID,
	)

	recorder := httptest.NewRecorder()
	handler.Delete(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	if renderer.lastFragment != "kipple-quota" {
		t.Errorf("fragment: got %q, want kipple-quota", renderer.lastFragment)
	}
}

func TestHandler_Delete_RendererError_Returns500(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	renderer := &mockRenderer{err: errors.New("template failure")}
	handler := kipple.NewHandler(renderer, service)

	upload := createCompleteUpload(t, service, userID)

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodDelete, "/kipple/"+upload.ID, nil), upload.ID),
		userID,
	)

	recorder := httptest.NewRecorder()
	handler.Delete(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestHandler_View_NotFound_Redirects(t *testing.T) {
	database := openTestDB(t)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	request := withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/lain", nil), "lain")
	recorder := httptest.NewRecorder()
	handler.View(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusFound)
	}
}

func TestHandler_View_ValidFile_Renders(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	renderer := &mockRenderer{}
	handler := kipple.NewHandler(renderer, service)

	upload := createCompleteUpload(t, service, userID)

	request := withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID, nil), upload.ID)
	recorder := httptest.NewRecorder()
	handler.View(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	if renderer.lastFragment != "page-content" {
		t.Errorf("fragment: got %q, want page-content", renderer.lastFragment)
	}
}

func TestHandler_View_PrivateFile_WithoutAuth_Redirects(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)
	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityUser, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))

	request := withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID, nil), upload.ID)
	recorder := httptest.NewRecorder()
	handler.View(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusFound)
	}
}

func TestHandler_View_PrivateFile_WithNonOwner_Redirects(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	otherID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)
	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityUser, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID, nil), upload.ID),
		otherID,
	)
	recorder := httptest.NewRecorder()
	handler.View(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusFound)
	}
}

func TestHandler_View_PrivateFile_WithOwner_Renders(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	renderer := &mockRenderer{}
	handler := kipple.NewHandler(renderer, service)

	content := loadTestFixture(t)
	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityUser, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID, nil), upload.ID),
		userID,
	)
	recorder := httptest.NewRecorder()
	handler.View(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestHandler_View_SQLInjectionID_Redirects(t *testing.T) {
	database := openTestDB(t)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	for _, id := range []string{"' OR '1'='1", "'; DROP TABLE kipple_files; --"} {
		request := withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/x", nil), id)
		recorder := httptest.NewRecorder()
		handler.View(recorder, request)

		if recorder.Code != http.StatusFound {
			t.Errorf("SQL injection ID %q: got status %d, want %d", id, recorder.Code, http.StatusFound)
		}
	}
}

func TestHandler_View_RendererError_Returns500(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	renderer := &mockRenderer{err: errors.New("template failure")}
	handler := kipple.NewHandler(renderer, service)

	upload := createCompleteUpload(t, service, userID)

	request := withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID, nil), upload.ID)
	recorder := httptest.NewRecorder()
	handler.View(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestHandler_Download_NotFound_Redirects(t *testing.T) {
	database := openTestDB(t)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	request := withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/swans/download", nil), "swans")
	recorder := httptest.NewRecorder()
	handler.Download(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusFound)
	}
}

func TestHandler_Download_ValidFile_ServesContent(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	originalContent := loadTestFixture(t)
	upload := createCompleteUpload(t, service, userID)

	request := withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID+"/download", nil), upload.ID)
	recorder := httptest.NewRecorder()
	handler.Download(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	if !bytes.Equal(recorder.Body.Bytes(), originalContent) {
		t.Errorf("body length mismatch: got %d bytes, want %d bytes", recorder.Body.Len(), len(originalContent))
	}

	contentDisposition := recorder.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "attachment") {
		t.Errorf("Content-Disposition must indicate attachment; got %q", contentDisposition)
	}
}

func TestHandler_Download_ValidFile_ContentDispositionHasFilename(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	upload := createCompleteUpload(t, service, userID)

	request := withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID+"/download", nil), upload.ID)
	recorder := httptest.NewRecorder()
	handler.Download(recorder, request)

	contentDisposition := recorder.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "03.gif") {
		t.Errorf("Content-Disposition must contain filename; got %q", contentDisposition)
	}
}

func TestHandler_Download_ExpiredFile_Redirects(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)
	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityLink, pastExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))

	request := withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID+"/download", nil), upload.ID)
	recorder := httptest.NewRecorder()
	handler.Download(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusFound)
	}
}

func TestHandler_Download_PrivateFile_WithoutAuth_Redirects(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)
	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityUser, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))

	request := withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID+"/download", nil), upload.ID)
	recorder := httptest.NewRecorder()
	handler.Download(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusFound)
	}
}

func TestHandler_Download_PrivateFile_WithNonOwner_Redirects(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	otherID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)
	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityUser, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID+"/download", nil), upload.ID),
		otherID,
	)
	recorder := httptest.NewRecorder()
	handler.Download(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusFound)
	}
}

func TestHandler_Download_PrivateFile_WithOwner_ServesContent(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	originalContent := loadTestFixture(t)
	upload, err := service.CreateUpload(
		userID,
		"f.gif",
		int64(len(originalContent)),
		kipple.VisibilityUser,
		futureExpiry(),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(originalContent), sha1Header(originalContent))

	request := withAuth(
		withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID+"/download", nil), upload.ID),
		userID,
	)
	recorder := httptest.NewRecorder()
	handler.Download(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	if !bytes.Equal(recorder.Body.Bytes(), originalContent) {
		t.Errorf("body mismatch: got %d bytes, want %d bytes", recorder.Body.Len(), len(originalContent))
	}
}

func TestHandler_Download_PendingFile_Redirects(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	service, _ := newService(t, database)
	handler := kipple.NewHandler(&mockRenderer{}, service)

	content := loadTestFixture(t)
	upload, err := service.CreateUpload(userID, "f.gif", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	request := withKippleRouteID(httptest.NewRequest(http.MethodGet, "/kipple/"+upload.ID+"/download", nil), upload.ID)
	recorder := httptest.NewRecorder()
	handler.Download(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Errorf("pending file must not be downloadable; status got %d, want %d", recorder.Code, http.StatusFound)
	}
}

func TestHandler_List_ReturnsRowsFragment(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	renderer := &mockRenderer{}
	handler := kipple.NewHandler(renderer, service)

	createCompleteUpload(t, service, userID)

	request := withAuth(httptest.NewRequest(http.MethodGet, "/kipple/list", nil), userID)
	recorder := httptest.NewRecorder()
	handler.List(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	if renderer.lastFragment != "kipple-rows" {
		t.Errorf("fragment: got %q, want kipple-rows", renderer.lastFragment)
	}
}

func TestHandler_List_RendererError_Returns500(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)
	renderer := &mockRenderer{err: errors.New("template failure")}
	handler := kipple.NewHandler(renderer, service)

	request := withAuth(httptest.NewRequest(http.MethodGet, "/kipple/list", nil), userID)
	recorder := httptest.NewRecorder()
	handler.List(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
