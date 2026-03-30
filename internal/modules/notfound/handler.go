// Package notfound implements the 404 not found page handler.
package notfound

import (
	"fmt"
	"math/rand"
	"net/http"

	templates "meryl.moe/internal/platform/templates"
)

type Handler struct {
	templates *templates.Manager
}

func NewHandler(templateManager *templates.Manager) *Handler {
	return &Handler{templates: templateManager}
}

func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "pages/notfound/notfound.html"
	data := map[string]any{
		"Gif": fmt.Sprintf("/static/assets/gifs/404/%02d.gif", rand.Intn(10)+1),
	}

	writer.WriteHeader(http.StatusNotFound)
	if err := handler.templates.Render(writer, request, pageFile, data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
