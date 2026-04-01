// Package cyberia implements the cyberia page handler.
package cyberia

import (
	http "net/http"

	chi "github.com/go-chi/chi/v5"
	templates "meryl.moe/internal/platform/templates"
)

// Handler handles requests for the cyberia page.
type Handler struct {
	renderer templates.Renderer
}

// NewHandler returns a Handler backed by the given renderer.
func NewHandler(renderer templates.Renderer) *Handler {
	return &Handler{renderer: renderer}
}

// Routes registers the cyberia page route on the given router.
func Routes(handler *Handler) func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/cyberia", handler.Index)
	}
}

// Index renders the cyberia page.
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/cyberia/cyberia.html"
	data := map[string]any{"Page": "cyberia", "Title": "cyberia - meryl.moe"}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
