// Package internal handles route registration.
package internal

import (
	http "net/http"

	home "meryl.moe/internal/modules/home"
)

func (server *Server) RegisterRoutes(homeHandler *home.Handler) {
	fileServer := http.FileServer(fileOnlyFS{fileSystem: http.Dir("static")})
	server.router.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	server.router.Get("/", homeHandler.Index)

	// TODO: testing endpoint, remove later
	server.router.Get("/api/lain", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("Shall we love lain? :B"))
	})
}
