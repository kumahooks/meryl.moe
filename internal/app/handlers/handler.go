package handlers

import (
	http "net/http"
	templates "meryl.moe/internal/app/templates"
)

type BaseHandler struct {
	template *templates.TemplatesManager
}

func NewBaseHandler(template *templates.TemplatesManager) *BaseHandler {
	return &BaseHandler{template}
}

func (handler *BaseHandler) RenderTemplate(writer http.ResponseWriter, name string, data interface{}) error {
	return handler.template.Render(writer, name, data)
}

type PageHandler struct {
	*BaseHandler
	PageName string
	Title	 string
}

func NewPageHandler(template *templates.TemplatesManager, pageName string, title string) (*PageHandler, error) {
	return &PageHandler{
		BaseHandler: NewBaseHandler(template),
		PageName: 	 pageName,
		Title: 		 title,
	}, nil
}

func (handler *PageHandler) ServePage(writer http.ResponseWriter, request *http.Request) {
	data := map[string]interface{}{"Title": handler.Title}

	if request.Header.Get("HX-Request") == "true" {
		err := handler.RenderTemplate(writer, handler.PageName+"-content", data)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	err := handler.RenderTemplate(writer, "base", data)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
