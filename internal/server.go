// Package internal provides the core HTTP server functionality using Chi router.
// It handles server initialization, route registration, static file serving,
// and coordinates between the routing layer and application handlers.
package internal

import (
	"context"
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
	"meryl.moe/internal/platform/middleware"
	"meryl.moe/internal/platform/templates"
)

// Server holds the Chi router and application configuration.
type Server struct {
	router  *chi.Mux
	config  *config.Config
	logFile io.Closer
}

// NewServer creates a Server with global middleware applied.
func NewServer(configuration *config.Config) *Server {
	router := chi.NewRouter()

	router.Use(chiMiddleware.Logger)
	router.Use(chiMiddleware.Recoverer)

	router.Use(middleware.Security)

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

	homeHandler := home.NewHandler(templateManager)
	whoamiHandler := whoami.NewHandler(templateManager)
	logsHandler := logs.NewHandler(templateManager)
	noiseHandler := noise.NewHandler(templateManager)
	binHandler := bin.NewHandler(templateManager)
	cyberiaHandler := cyberia.NewHandler(templateManager)
	relayHandler := relay.NewHandler(templateManager)
	notFoundHandler := notfound.NewHandler(templateManager)

	server.RegisterRoutes(
		home.Routes(homeHandler),
		whoami.Routes(whoamiHandler),
		logs.Routes(logsHandler),
		noise.Routes(noiseHandler),
		bin.Routes(binHandler),
		cyberia.Routes(cyberiaHandler),
		relay.Routes(relayHandler),
		notfound.Routes(notFoundHandler),
	)

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
		return err
	case <-shutdownContext.Done():
		stop()

		log.Printf("Shutting down...")

		drainContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := httpServer.Shutdown(drainContext)

		server.logFile.Close()

		return err
	}
}
