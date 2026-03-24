package templates

import (
	log "log"
	template "html/template"
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
	templates, err := template.ParseGlob("internal/app/templates/**/*.html")
	if err != nil {
		return nil, err
	}

	return &TemplatesManager{templates}, nil
}

