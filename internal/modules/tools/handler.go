// Package tools implements the tools page handler.
package tools

import (
	http "net/http"

	chi "github.com/go-chi/chi/v5"
	templates "meryl.moe/internal/platform/templates"
)

// Handler handles requests for the tools page.
type Handler struct {
	renderer templates.Renderer
}

// NewHandler returns a Handler backed by the given renderer.
func NewHandler(renderer templates.Renderer) *Handler {
	return &Handler{renderer: renderer}
}

// Routes registers the tools page route on the given router.
func Routes(handler *Handler) func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/tools", handler.Index)
	}
}

// Index renders the tools page.
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/tools/tools.html"
	data := map[string]any{"Page": "tools", "Title": "tools - meryl.moe"}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
