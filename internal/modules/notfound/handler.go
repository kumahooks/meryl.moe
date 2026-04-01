// Package notfound implements the 404 not found page handler.
package notfound

import (
	fmt "fmt"
	log "log"
	rand "math/rand"
	http "net/http"

	chi "github.com/go-chi/chi/v5"
	templates "meryl.moe/internal/platform/templates"
)

// Handler handles requests for the 404 not found page.
type Handler struct {
	renderer templates.Renderer
}

// NewHandler returns a Handler backed by the given renderer.
func NewHandler(renderer templates.Renderer) *Handler {
	return &Handler{renderer: renderer}
}

// Routes registers the 404 handler on the given router.
func Routes(handler *Handler) func(chi.Router) {
	return func(router chi.Router) {
		router.NotFound(handler.Index)
	}
}

// Index renders the 404 page with a randomly selected gif and writes a 404 status.
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/notfound/notfound.html"
	data := map[string]any{
		"Title": "404 - meryl.moe",
		"Gif":   fmt.Sprintf("/static/assets/gifs/404/%02d.gif", rand.Intn(10)+1),
	}

	writer.WriteHeader(http.StatusNotFound)
	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		log.Printf("failed to render 404 page: %v", err)
		writer.Write([]byte("not found"))
	}
}
