// Package internal provides the core HTTP server functionality using Chi router.
// It handles server initialization, route registration, static file serving,
// and coordinates between the routing layer and application handlers.
package internal

import (
	log "log"
	http "net/http"
	os "os"

	chi "github.com/go-chi/chi/v5"
	middleware "github.com/go-chi/chi/v5/middleware"

	config "meryl.moe/internal/config"
	about "meryl.moe/internal/modules/about"
	articles "meryl.moe/internal/modules/articles"
	cyberia "meryl.moe/internal/modules/cyberia"
	home "meryl.moe/internal/modules/home"
	notfound "meryl.moe/internal/modules/notfound"
	tools "meryl.moe/internal/modules/tools"
	templates "meryl.moe/internal/platform/templates"
)

type Server struct {
	router *chi.Mux
	config *config.Config
}

func NewServer(configuration *config.Config) *Server {
	router := chi.NewRouter()

	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	return &Server{
		router: router,
		config: configuration,
	}
}

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

func (server *Server) Initialize() error {
	templateManager, err := templates.NewManager()
	if err != nil {
		return err
	}

	homeHandler := home.NewHandler(templateManager)
	aboutHandler := about.NewHandler(templateManager)
	articlesHandler := articles.NewHandler(templateManager)
	toolsHandler := tools.NewHandler(templateManager)
	cyberiaHandler := cyberia.NewHandler(templateManager)
	notFoundHandler := notfound.NewHandler(templateManager)

	server.RegisterRoutes(homeHandler, aboutHandler, articlesHandler, toolsHandler, cyberiaHandler, notFoundHandler)

	return nil
}

func (server *Server) Start(addr string) error {
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, server.router)
}
