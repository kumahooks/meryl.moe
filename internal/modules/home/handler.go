// Package home implements the home page handler.
package home

import (
	http "net/http"

	templates "meryl.moe/internal/platform/templates"
)

type Handler struct {
	templates *templates.TemplatesManager
}

func NewHandler(templatesManager *templates.TemplatesManager) *Handler {
	return &Handler{templates: templatesManager}
}

// TODO: this will not scale very well as this will be repeated every module pmuch
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	data := map[string]interface{}{"Title": "Home"}

	if request.Header.Get("HX-Request") == "true" {
		err := handler.templates.Render(writer, "home-content", data)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	err := handler.templates.Render(writer, "base", data)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
