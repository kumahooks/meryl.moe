// Package internal handles route registration.
package internal

import (
	http "net/http"

	about "meryl.moe/internal/modules/about"
	articles "meryl.moe/internal/modules/articles"
	cyberia "meryl.moe/internal/modules/cyberia"
	home "meryl.moe/internal/modules/home"
	notfound "meryl.moe/internal/modules/notfound"
	tools "meryl.moe/internal/modules/tools"
)

// RegisterRoutes mounts the static file server and all page handlers onto the router.
func (server *Server) RegisterRoutes(
	homeHandler *home.Handler,
	aboutHandler *about.Handler,
	articlesHandler *articles.Handler,
	toolsHandler *tools.Handler,
	cyberiaHandler *cyberia.Handler,
	notFoundHandler *notfound.Handler,
) {
	fileServer := http.FileServer(fileOnlyFS{fileSystem: http.Dir("static")})
	server.router.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	server.router.Get("/", homeHandler.Index)
	server.router.Get("/about", aboutHandler.Index)
	server.router.Get("/articles", articlesHandler.Index)
	server.router.Get("/tools", toolsHandler.Index)
	server.router.Get("/cyberia", cyberiaHandler.Index)

	server.router.NotFound(notFoundHandler.Index)

	// TODO: testing endpoint, remove later
	server.router.Get("/api/lain", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("Shall we love lain? :B"))
	})
}
