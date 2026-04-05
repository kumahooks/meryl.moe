// Package bin implements the bin page handler.
package bin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"meryl.moe/internal/platform/auth"
	"meryl.moe/internal/platform/templates"
)

// Handler handles requests for the bin page.
type Handler struct {
	renderer templates.Renderer
}

// NewHandler returns a Handler backed by the given renderer.
func NewHandler(renderer templates.Renderer) *Handler {
	return &Handler{renderer: renderer}
}

// Routes registers the bin page route on the given router.
func Routes(handler *Handler) func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/bin", handler.Index)
	}
}

// Index renders the bin page.
// TODO: there's a case in production where the content loads before the style
// which renders the page in a very ugly manner
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/bin/bin.html"
	data := map[string]any{"Page": "bin", "Title": "bin - meryl.moe"}

	if user, ok := auth.AuthUser(request.Context()); ok {
		data["User"] = user
	}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
