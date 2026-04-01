// Package about implements the about page handler.
package about

import (
	http "net/http"

	templates "meryl.moe/internal/platform/templates"
)

// Handler handles requests for the about page.
type Handler struct {
	templates *templates.Manager
}

// NewHandler returns a Handler backed by the given template manager.
func NewHandler(templateManager *templates.Manager) *Handler {
	return &Handler{templates: templateManager}
}

// Index renders the about page.
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/about/about.html"
	data := map[string]any{"Page": "about"}

	if err := handler.templates.Render(writer, request, pageFile, data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
