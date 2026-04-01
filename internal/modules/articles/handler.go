// Package articles implements the articles page handler.
package articles

import (
	http "net/http"

	templates "meryl.moe/internal/platform/templates"
)

// Handler handles requests for the articles page.
type Handler struct {
	templates *templates.Manager
}

// NewHandler returns a Handler backed by the given template manager.
func NewHandler(templateManager *templates.Manager) *Handler {
	return &Handler{templates: templateManager}
}

// Index renders the articles page.
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/articles/articles.html"
	data := map[string]any{"Page": "articles"}

	if err := handler.templates.Render(writer, request, pageFile, "page-content", data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
