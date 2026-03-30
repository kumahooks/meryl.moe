// Package templates manages HTML template parsing and rendering.
// A base set (layouts + components) is parsed once at startup.
// Each Render call clones the base and parses the page-specific
// template into the clone.
package templates

import (
	template "html/template"
	http "net/http"
)

type Manager struct {
	base *template.Template
}

func NewManager() (*Manager, error) {
	base := template.New("")

	if _, err := base.ParseGlob("internal/platform/templates/layouts/*.html"); err != nil {
		return nil, err
	}

	if _, err := base.ParseGlob("internal/platform/templates/components/*.html"); err != nil {
		return nil, err
	}

	return &Manager{base: base}, nil
}

// Render clones the base template set, parses pageFile into the clone,
// then executes:
//   - "page-content" for HTMX fragment swaps (HX-Request: true, not boosted)
//   - "base" for full page loads and hx-boost navigation
//
// Every page template must define {{define "page-content"}}.
// Pages may optionally define {{define "page-head"}} for <head> content
// (loaded on full-page requests) and should also include their CSS <link>
// inside page-content so it loads on HTMX swaps too.
func (templateManager *Manager) Render(
	writer http.ResponseWriter,
	request *http.Request,
	pageFile string,
	data any,
) error {
	clonedTemplate, err := templateManager.base.Clone()
	if err != nil {
		return err
	}

	if _, err = clonedTemplate.ParseFiles(pageFile); err != nil {
		return err
	}

	// hx-boost sends both HX-Request and HX-Boosted, return full page so
	// HTMX can merge <head> (CSS, title) and swap <body>.
	isFragment := request.Header.Get("HX-Request") == "true" && request.Header.Get("HX-Boosted") != "true"
	if isFragment {
		return clonedTemplate.ExecuteTemplate(writer, "page-content", data)
	}

	return clonedTemplate.ExecuteTemplate(writer, "base", data)
}
