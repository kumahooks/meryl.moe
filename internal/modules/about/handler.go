// Package about implements the about page handler.
package about

import (
	http "net/http"

	templates "meryl.moe/internal/platform/templates"
)

type Handler struct {
	templates *templates.Manager
}

func NewHandler(templateManager *templates.Manager) *Handler {
	return &Handler{templates: templateManager}
}

func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "internal/modules/about/about.html"
	data := map[string]any{"Page": "about"}

	if err := handler.templates.Render(writer, request, pageFile, data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
