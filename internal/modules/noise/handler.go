// Package noise implements the noise page handler.
package noise

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"meryl.moe/internal/platform/templates"
)

// Handler handles requests for the noise page.
type Handler struct {
	renderer templates.Renderer
}

// NewHandler returns a Handler backed by the given renderer.
func NewHandler(renderer templates.Renderer) *Handler {
	return &Handler{renderer: renderer}
}

// Routes registers the noise page route on the given router.
func Routes(handler *Handler) func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/noise", handler.Index)
	}
}

// Index renders the noise page.
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/noise/noise.html"
	data := map[string]any{"Page": "noise", "Title": "noise - meryl.moe"}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
