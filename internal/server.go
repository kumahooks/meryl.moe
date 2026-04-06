// Package internal provides the core HTTP server functionality using Chi router.
// It handles server initialization, route registration, static file serving,
// and coordinates between the routing layer and application handlers.
package internal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"meryl.moe/internal/config"
	"meryl.moe/internal/modules/bin"
	"meryl.moe/internal/modules/cyberia"
	"meryl.moe/internal/modules/home"
	"meryl.moe/internal/modules/logs"
	"meryl.moe/internal/modules/noise"
	"meryl.moe/internal/modules/notfound"
	"meryl.moe/internal/modules/relay"
	"meryl.moe/internal/modules/whoami"
	"meryl.moe/internal/modules/wired"
	"meryl.moe/internal/platform/auth"
	"meryl.moe/internal/platform/middleware"
	"meryl.moe/internal/platform/templates"
)

// Server holds the Chi router and application configuration.
type Server struct {
	router   *chi.Mux
	config   *config.Config
	logFile  io.Closer
	database *sql.DB
}

// NewServer creates a Server with global middleware applied.
func NewServer(configuration *config.Config, database *sql.DB) *Server {
	router := chi.NewRouter()

	router.Use(chiMiddleware.Logger)
	router.Use(chiMiddleware.Recoverer)
	router.Use(middleware.Security)
	router.Use(middleware.LoadAuth(database))

	return &Server{
		router:   router,
		config:   configuration,
		database: database,
	}
}

// fileOnlyFS wraps http.FileSystem and rejects directory requests,
// preventing directory listing on the static file server.
type fileOnlyFS struct {
	fileSystem http.FileSystem
}

func (fileOnly fileOnlyFS) Open(name string) (http.File, error) {
	file, err := fileOnly.fileSystem.Open(name)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
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
	logFile, err := server.initLogging()
	if err != nil {
		return err
	}

	server.logFile = logFile
	log.Printf("logging: initialized")

	var fileSystem fs.FS
	if server.config.App.Dev {
		fileSystem = os.DirFS(filepath.Join(server.config.App.RootDir, "internal"))
	} else {
		fileSystem = templatesEmbedFS
	}

	templateManager, err := templates.NewManager(server.config.App.Dev, fileSystem)
	if err != nil {
		return err
	}

	log.Printf("templates: initialized (dev=%v)", server.config.App.Dev)

	// Home/Entry
	homeHandler := home.NewHandler(templateManager)

	// Articles page
	logsHandler := logs.NewHandler(templateManager)

	// Random posts page
	noiseHandler := noise.NewHandler(templateManager)

	// List of apps
	binHandler := bin.NewHandler(templateManager)

	// About page
	whoamiHandler := whoami.NewHandler(templateManager)

	// Radio module
	cyberiaHandler := cyberia.NewHandler(templateManager)

	// Text Sharing app module
	relayService := relay.NewService(server.database)
	relayHandler := relay.NewHandler(templateManager, relayService)

	// 404 Page
	notFoundHandler := notfound.NewHandler(templateManager)

	// Auth/Login/Logout
	authService, err := auth.NewService(server.database, server.config.Session.TTL)
	if err != nil {
		return fmt.Errorf("auth service: %w", err)
	}

	wiredHandler := wired.NewHandler(templateManager, authService, server.config.App.Dev)

	server.RegisterRoutes(
		home.Routes(homeHandler),
		logs.Routes(logsHandler),
		noise.Routes(noiseHandler),
		bin.Routes(binHandler),
		whoami.Routes(whoamiHandler),
		cyberia.Routes(cyberiaHandler),
		relay.Routes(relayHandler, server.database),
		notfound.Routes(notFoundHandler),
		wired.Routes(wiredHandler, server.database),
	)

	log.Printf("routes: registered")

	return nil
}

// initLogging opens a date-prefixed log file in the configured directory and
// routes all log output to both stdout and the file.
// The caller is responsible for closing the returned io.Closer when done.
func (server *Server) initLogging() (io.Closer, error) {
	logDir := server.config.Logging.Dir
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %q: %w", logDir, err)
	}

	date := time.Now().Format("2006-01-02")
	logFilePath := filepath.Join(logDir, date+"_app.log")

	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %q: %w", logFilePath, err)
	}

	log.SetOutput(io.MultiWriter(os.Stdout, logFile))

	return logFile, nil
}

// Start binds the server to addr and begins serving requests.
// Blocks until SIGINT or SIGTERM is received, then drains in-flight requests
// with a 30-second deadline before returning.
func (server *Server) Start(addr string) error {
	log.Printf("Starting server on %s", addr)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case err := <-serverErrors:
		server.logFile.Close()
		server.database.Close()

		return err
	case <-shutdownContext.Done():
		stop()

		log.Printf("Shutting down...")

		drainContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := httpServer.Shutdown(drainContext)

		server.logFile.Close()
		server.database.Close()

		return err
	}
}
