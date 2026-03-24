package app

import (
	log "log"
	http "net/http"
	chi "github.com/go-chi/chi/v5"
	middleware "github.com/go-chi/chi/v5/middleware"

	handlers "meryl.moe/internal/app/handlers"
	templates "meryl.moe/internal/app/templates"
)

type Server struct {
	router *chi.Mux
}

func NewServer() *Server {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	return &Server{
		router: r,
	}
}

func (server *Server) ServeStatic(path string, dir string) {
	server.router.Handle(path, http.StripPrefix(path, http.FileServer(http.Dir(dir))))
}

func (server *Server) Start(addr string) error {
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, server.router)
}

func (server *Server) SetupRoutes() error {
	templateManager, err := templates.NewManager()
	if err != nil {
		return err
	}

	server.router.Handle("/static/*",
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)

	server.router.Handle("/favicon.ico",
		http.FileServer(http.Dir("static")),
	)

	homeHandler, err := handlers.NewPageHandler(templateManager, "home", "Home")
	if err != nil {
		return err
	}

	server.router.Get("/", homeHandler.ServePage)
	server.router.Get("/api/lain", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Shall we love lain? :B"))
	})

	return nil
}
