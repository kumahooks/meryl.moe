package templates_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"meryl.moe/internal/platform/templates"
)

var testFS = fstest.MapFS{
	"platform/templates/layouts/base.html": {
		Data: []byte(`{{define "base"}}base:{{template "page-content" .}}{{end}}`),
	},
	"platform/templates/components/navbar.html": {
		Data: []byte(`{{define "navbar"}}{{end}}`),
	},
	"modules/page/page.html": {
		Data: []byte(`{{define "page-content"}}content:{{.Title}}{{end}}`),
	},
	"modules/other/other.html": {
		Data: []byte(`{{define "page-content"}}other:{{.Title}}{{end}}`),
	},
}

func TestNewManager_MissingLayouts_ReturnsError(t *testing.T) {
	_, err := templates.NewManager(false, fstest.MapFS{})
	if err == nil {
		t.Fatal("expected error for missing layout templates")
	}
}

func TestRender_DirectRequest_RendersFullPage(t *testing.T) {
	manager, err := templates.NewManager(false, testFS)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	if err := manager.Render(
		recorder,
		request,
		"modules/page/page.html",
		"page-content",
		map[string]any{"Title": "hello"},
	); err != nil {
		t.Fatalf("Render: %v", err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "base:") {
		t.Errorf("expected full page render, got: %q", body)
	}

	if !strings.Contains(body, "content:hello") {
		t.Errorf("expected page content in body, got: %q", body)
	}
}

func TestRender_HTMXFragment_RendersFragmentOnly(t *testing.T) {
	manager, err := templates.NewManager(false, testFS)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("HX-Request", "true")

	if err := manager.Render(
		recorder,
		request,
		"modules/page/page.html",
		"page-content",
		map[string]any{"Title": "hello"},
	); err != nil {
		t.Fatalf("Render: %v", err)
	}

	body := recorder.Body.String()
	if strings.Contains(body, "base:") {
		t.Errorf("expected fragment only, got full page: %q", body)
	}

	if !strings.Contains(body, "content:hello") {
		t.Errorf("expected fragment content in body, got: %q", body)
	}
}

func TestRender_HTMXBoosted_RendersFragmentOnly(t *testing.T) {
	manager, err := templates.NewManager(false, testFS)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("HX-Request", "true")
	request.Header.Set("HX-Boosted", "true")

	if err := manager.Render(
		recorder,
		request,
		"modules/page/page.html",
		"page-content",
		map[string]any{"Title": "hello"},
	); err != nil {
		t.Fatalf("Render: %v", err)
	}

	body := recorder.Body.String()
	if strings.Contains(body, "base:") {
		t.Errorf("expected fragment only for boosted request, got full page: %q", body)
	}

	if !strings.Contains(body, "content:hello") {
		t.Errorf("expected fragment content in body, got: %q", body)
	}
}

func TestRender_MissingPageFile_ReturnsError(t *testing.T) {
	manager, err := templates.NewManager(false, testFS)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	err = manager.Render(recorder, request, "modules/nonexistent/nonexistent.html", "page-content", nil)
	if err == nil {
		t.Fatal("expected error for missing page file")
	}
}

func TestRender_CloneIsolation_PagesDoNotCollide(t *testing.T) {
	manager, err := templates.NewManager(false, testFS)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	recorderA := httptest.NewRecorder()
	if err := manager.Render(
		recorderA,
		httptest.NewRequest(http.MethodGet, "/", nil),
		"modules/page/page.html",
		"page-content",
		map[string]any{"Title": "alpha"},
	); err != nil {
		t.Fatalf("Render page: %v", err)
	}

	recorderB := httptest.NewRecorder()
	if err := manager.Render(
		recorderB,
		httptest.NewRequest(http.MethodGet, "/", nil),
		"modules/other/other.html",
		"page-content",
		map[string]any{"Title": "beta"},
	); err != nil {
		t.Fatalf("Render other: %v", err)
	}

	if bodyA := recorderA.Body.String(); !strings.Contains(bodyA, "content:alpha") ||
		strings.Contains(bodyA, "other:") {
		t.Errorf("page render contaminated: %q", bodyA)
	}

	if bodyB := recorderB.Body.String(); !strings.Contains(bodyB, "other:beta") || strings.Contains(bodyB, "content:") {
		t.Errorf("other render contaminated: %q", bodyB)
	}
}

func TestRender_DevMode_RendersCorrectly(t *testing.T) {
	manager, err := templates.NewManager(true, testFS)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	if err := manager.Render(
		recorder,
		request,
		"modules/page/page.html",
		"page-content",
		map[string]any{"Title": "devmode"},
	); err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(recorder.Body.String(), "content:devmode") {
		t.Errorf("dev mode render failed: %q", recorder.Body.String())
	}
}
