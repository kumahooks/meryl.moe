// Package kipple implements the file sharing tool page handler
package kipple

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"meryl.moe/internal/platform/auth"
	"meryl.moe/internal/platform/templates"
)

// Handler handles requests for the kipple tool page.
type Handler struct {
	renderer templates.Renderer
}

// NewHandler returns a Handler backed by the given renderer and service.
func NewHandler(renderer templates.Renderer) *Handler {
	return &Handler{renderer: renderer}
}

// Routes registers the kipple tool routes on the given router.
func Routes(handler *Handler) func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/kipple", handler.Index)
	}
}

func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/kipple/kipple.html"
	data := map[string]any{"Page": "kipple", "Title": "kipple - meryl.moe"}

	if user, ok := auth.AuthUser(request.Context()); ok {
		data["User"] = user
	}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		log.Printf("kipple: render index: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
