// Package notfound implements the 404 not found page handler.
package notfound

import (
	fmt "fmt"
	rand "math/rand"
	http "net/http"

	templates "meryl.moe/internal/platform/templates"
)

// Handler handles requests for the 404 not found page.
type Handler struct {
	templates *templates.Manager
}

// NewHandler returns a Handler backed by the given template manager.
func NewHandler(templateManager *templates.Manager) *Handler {
	return &Handler{templates: templateManager}
}

// Index renders the 404 page with a randomly selected gif and writes a 404 status.
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/notfound/notfound.html"
	data := map[string]any{
		"Gif": fmt.Sprintf("/static/assets/gifs/404/%02d.gif", rand.Intn(10)+1),
	}

	writer.WriteHeader(http.StatusNotFound)
	if err := handler.templates.Render(writer, request, pageFile, "page-content", data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
