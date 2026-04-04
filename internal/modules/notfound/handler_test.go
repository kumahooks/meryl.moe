package notfound_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"meryl.moe/internal/modules/notfound"
)

type mockRenderer struct {
	renderFunc func(http.ResponseWriter, *http.Request, string, string, any) error
}

func (mock *mockRenderer) Render(
	writer http.ResponseWriter,
	request *http.Request,
	pageFile, fragment string,
	data any,
) error {
	return mock.renderFunc(writer, request, pageFile, fragment, data)
}

func TestIndex_Returns404Status(t *testing.T) {
	renderer := &mockRenderer{
		renderFunc: func(w http.ResponseWriter, r *http.Request, _, _ string, _ any) error {
			return nil
		},
	}

	recorder := httptest.NewRecorder()
	notfound.NewHandler(renderer).Index(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestIndex_GifPathInValidRange(t *testing.T) {
	gifPattern := regexp.MustCompile(`^/static/assets/gifs/404/(0[1-9]|10)\.gif$`)
	seen := make(map[string]bool)

	renderer := &mockRenderer{
		renderFunc: func(w http.ResponseWriter, r *http.Request, _, _ string, data any) error {
			dataMap, ok := data.(map[string]any)
			if !ok {
				return fmt.Errorf("data is not map[string]any")
			}

			gif, ok := dataMap["Gif"].(string)
			if !ok {
				return fmt.Errorf("Gif key missing or not a string")
			}

			if !gifPattern.MatchString(gif) {
				return fmt.Errorf("gif path %q outside valid range", gif)
			}

			seen[gif] = true

			return nil
		},
	}

	handler := notfound.NewHandler(renderer)

	for range 100 {
		recorder := httptest.NewRecorder()
		handler.Index(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))
	}

	if len(seen) < 5 {
		t.Errorf("gif selection not random enough: only %d distinct values in 100 calls", len(seen))
	}
}

func TestIndex_RendererError_WritesPlainTextFallback(t *testing.T) {
	renderer := &mockRenderer{
		renderFunc: func(w http.ResponseWriter, r *http.Request, _, _ string, _ any) error {
			return errors.New("template failure")
		},
	}

	recorder := httptest.NewRecorder()
	notfound.NewHandler(renderer).Index(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNotFound)
	}

	if body := recorder.Body.String(); body != "not found" {
		t.Errorf("fallback body: got %q, want %q", body, "not found")
	}
}
