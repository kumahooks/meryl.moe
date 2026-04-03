// Package whoami implements the whoami page handler.
package whoami

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"meryl.moe/internal/platform/templates"
)

// Handler handles requests for the whoami page.
type Handler struct {
	renderer templates.Renderer
}

// NewHandler returns a Handler backed by the given renderer.
func NewHandler(renderer templates.Renderer) *Handler {
	return &Handler{renderer: renderer}
}

// Routes registers the whoami page route on the given router.
func Routes(handler *Handler) func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/whoami", handler.Index)
	}
}

// Index renders the whoami page.
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/whoami/whoami.html"
	data := map[string]any{"Page": "whoami", "Title": "whoami - meryl.moe"}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
