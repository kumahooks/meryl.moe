// Package internal handles route registration.
package internal

import (
	http "net/http"
	filepath "path/filepath"

	appRouter "meryl.moe/internal/platform/router"
)

// RegisterRoutes mounts the static file server and delegates all page routes
// to the provided registrars. Each module registers its own paths via Routes().
//
// Static files are intentionally served from disk (not embedded) so they can be
// updated without redeploying the binary
func (server *Server) RegisterRoutes(registrars ...appRouter.RouteRegistrar) {
	staticDir := filepath.Join(server.config.App.RootDir, "static")
	fileServer := http.FileServer(fileOnlyFS{fileSystem: http.Dir(staticDir)})
	server.router.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	for _, register := range registrars {
		register(server.router)
	}
}
