// Package internal provides the core HTTP server functionality using Chi router.
// It handles server initialization, route registration, static file serving,
// and coordinates between the routing layer and application handlers.
package internal

import (
	fs "io/fs"
	log "log"
	http "net/http"
	os "os"
	time "time"

	chi "github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	config "meryl.moe/internal/config"
	about "meryl.moe/internal/modules/about"
	articles "meryl.moe/internal/modules/articles"
	cyberia "meryl.moe/internal/modules/cyberia"
	home "meryl.moe/internal/modules/home"
	notfound "meryl.moe/internal/modules/notfound"
	tools "meryl.moe/internal/modules/tools"
	appMiddleware "meryl.moe/internal/platform/middleware"
	templates "meryl.moe/internal/platform/templates"
)

// Server holds the Chi router and application configuration.
type Server struct {
	router *chi.Mux
	config *config.Config
}

// NewServer creates a Server with global middleware applied.
func NewServer(configuration *config.Config) *Server {
	router := chi.NewRouter()

	router.Use(chiMiddleware.Logger)
	router.Use(chiMiddleware.Recoverer)
	router.Use(appMiddleware.Security)

	return &Server{
		router: router,
		config: configuration,
	}
}

// fileOnlyFS wraps http.FileSystem and rejects directory requests,
// preventing directory listing on the static file server.
type fileOnlyFS struct {
	fileSystem http.FileSystem
}

func (fs fileOnlyFS) Open(name string) (http.File, error) {
	file, err := fs.fileSystem.Open(name)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		file.Close()
		return nil, os.ErrNotExist
	}

	return file, nil
}

// Initialize builds the template manager and wires all handlers and routes.
func (server *Server) Initialize() error {
	var fileSystem fs.FS
	if server.config.App.Dev {
		fileSystem = os.DirFS("internal")
	} else {
		fileSystem = assetsFS
	}

	templateManager, err := templates.NewManager(server.config.App.Dev, fileSystem)
	if err != nil {
		return err
	}

	homeHandler := home.NewHandler(templateManager)
	aboutHandler := about.NewHandler(templateManager)
	articlesHandler := articles.NewHandler(templateManager)
	toolsHandler := tools.NewHandler(templateManager)
	cyberiaHandler := cyberia.NewHandler(templateManager)
	notFoundHandler := notfound.NewHandler(templateManager)

	server.RegisterRoutes(
		homeHandler,
		aboutHandler,
		articlesHandler,
		toolsHandler,
		cyberiaHandler,
		notFoundHandler,
	)

	return nil
}

// Start binds the server to addr and begins serving requests.
func (server *Server) Start(addr string) error {
	log.Printf("Starting server on %s", addr)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return httpServer.ListenAndServe()
}
