// Package internal handles route registration.
package internal

import (
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// RouteRegistrar is a function that registers routes on a chi.Router.
// Each module exposes a Routes() function returning this type so that
// route paths are owned by the module rather than the central wiring layer.
//
// Type alias (not a new type) so callers can return func(chi.Router)
// without importing this package.
type RouteRegistrar = func(chi.Router)

// RegisterRoutes mounts the static file server and delegates all page routes
// to the provided registrars. Each module registers its own paths via Routes().
func (server *Server) RegisterRoutes(registrars ...RouteRegistrar) {
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
