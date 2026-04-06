// Package relay implements the relay tool page handler.
package relay

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"meryl.moe/internal/platform/auth"
	"meryl.moe/internal/platform/middleware"
	"meryl.moe/internal/platform/templates"
)

// Handler handles requests for the relay tool page.
type Handler struct {
	renderer templates.Renderer
	service  *Service
}

// NewHandler returns a Handler backed by the given renderer and service.
func NewHandler(renderer templates.Renderer, service *Service) *Handler {
	return &Handler{renderer: renderer, service: service}
}

// Routes registers the relay tool routes on the given router.
func Routes(handler *Handler, database *sql.DB) func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/relay", handler.Index)
		router.With(middleware.RequireAuth(database)).Post("/relay", handler.Save)
		router.Get("/relay/{id}", handler.View)
	}
}

// Index renders the relay tool page.
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/relay/relay.html"
	data := map[string]any{"Page": "relay", "Title": "relay - meryl.moe"}

	if user, ok := auth.AuthUser(request.Context()); ok {
		data["User"] = user
	}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// Save stores the submitted text and returns a link fragment pointing to the new relay.
func (handler *Handler) Save(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "bad request", http.StatusBadRequest)
		return
	}

	text := request.FormValue("text")
	if text == "" {
		http.Error(writer, "no content", http.StatusBadRequest)
		return
	}

	user, _ := auth.AuthUser(request.Context())

	relayID, err := handler.service.Save(user.ID, text)
	if err != nil {
		log.Printf("relay: save: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)

		return
	}

	relayURL := "/relay/" + relayID
	fmt.Fprintf(
		writer,
		`<a href="%s" target="_blank" rel="noopener noreferrer" class="relay-link">%s</a>`,
		relayURL,
		relayURL,
	)
}

// View loads a saved relay and renders the editor pre-populated with its content.
func (handler *Handler) View(writer http.ResponseWriter, request *http.Request) {
	relayID := chi.URLParam(request, "id")

	text, err := handler.service.Get(relayID)
	if errors.Is(err, ErrNotFound) {
		http.Redirect(writer, request, "/relay", http.StatusSeeOther)
		return
	}

	if err != nil {
		log.Printf("relay: view: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)

		return
	}

	pageFile := "modules/relay/relay.html"
	data := map[string]any{
		"Page":     "relay",
		"Title":    "relay - meryl.moe",
		"Content":  text,
		"ReadOnly": true,
	}

	if user, ok := auth.AuthUser(request.Context()); ok {
		data["User"] = user
	}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
