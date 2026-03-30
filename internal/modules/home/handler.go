// Package home implements the home page handler.
package home

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
	pageFile := "pages/home/home.html"
	data := map[string]any{}

	if err := handler.templates.Render(writer, request, pageFile, data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
