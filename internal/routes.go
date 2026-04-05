// Package internal handles route registration.
package internal

import (
	"net/http"
	"path/filepath"

	"meryl.moe/internal/platform/router"
)

// RegisterRoutes mounts the static file server and delegates all page routes
// to the provided registrars. Each module registers its own paths via Routes().
func (server *Server) RegisterRoutes(registrars ...router.RouteRegistrar) {
	staticDir := filepath.Join(server.config.App.RootDir, "static")
	fileServer := http.FileServer(fileOnlyFS{fileSystem: http.Dir(staticDir)})
	server.router.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	server.router.Get("/robots.txt", func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, filepath.Join(staticDir, "robots.txt"))
	})

	server.router.Get("/.well-known/security.txt", func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, filepath.Join(staticDir, ".well-known", "security.txt"))
	})

	for _, register := range registrars {
		register(server.router)
	}
}
