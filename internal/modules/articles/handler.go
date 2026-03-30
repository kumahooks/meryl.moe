// Package articles implements the articles page handler.
package articles

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
	pageFile := "internal/modules/articles/articles.html"
	data := map[string]any{"Page": "articles"}

	if err := handler.templates.Render(writer, request, pageFile, data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
