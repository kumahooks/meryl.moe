// Package templates manages HTML template parsing, caching, and rendering.
// It provides a centralized template manager that loads all HTML templates
// at startup and efficiently renders them with dynamic data.
package templates

import (
	template "html/template"
	log "log"
	http "net/http"
)

type TemplatesManager struct {
	templates *template.Template
}

func (templatesManager *TemplatesManager) Render(writer http.ResponseWriter, name string, data interface{}) error {
	log.Printf("Rendering template: %s with data: %+v\n", name, data)
	return templatesManager.templates.ExecuteTemplate(writer, name, data)
}

func NewManager() (*TemplatesManager, error) {
	parsedTemplates := template.New("")

	parsedTemplates, err := parsedTemplates.ParseGlob("internal/platform/templates/layouts/*.html")
	if err != nil {
		return nil, err
	}

	parsedTemplates, err = parsedTemplates.ParseGlob("internal/platform/templates/components/*.html")
	if err != nil {
		return nil, err
	}

	parsedTemplates, err = parsedTemplates.ParseGlob("internal/modules/*/*.html")
	if err != nil {
		return nil, err
	}

	return &TemplatesManager{templates: parsedTemplates}, nil
}
