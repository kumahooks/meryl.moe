// Package relay implements the relay tool page handler.
package relay

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

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
		router.With(middleware.RequireAuth(database)).Get("/relay/panel", handler.Panel)
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
		log.Printf("relay: render index: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// Panel renders the relay panel items fragment for the authenticated user.
func (handler *Handler) Panel(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/relay/relay.html"
	user, _ := auth.AuthUser(request.Context())

	relays, err := handler.service.List(user.ID)
	if err != nil {
		log.Printf("relay: panel: list: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)

		return
	}

	data := map[string]any{"Relays": relays}

	if err := handler.renderer.Render(writer, request, pageFile, "relay-panel", data); err != nil {
		log.Printf("relay: render panel: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// Save stores the submitted text and fires a notify event with the relay link.
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

	visibility := request.FormValue("visibility")
	if visibility != PrivateModeUser && visibility != PrivateModeLink {
		visibility = PrivateModeLink
	}

	var expiresAt time.Time
	switch request.FormValue("expire_at") {
	case "7d":
		expiresAt = time.Now().Add(7 * 24 * time.Hour)
	case "30d":
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
	default:
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	user, _ := auth.AuthUser(request.Context())

	relayID, err := handler.service.Save(user.ID, text, visibility, expiresAt)
	if err != nil {
		http.Error(writer, "internal server error", http.StatusInternalServerError)

		return
	}

	writer.Header().
		Set("HX-Trigger", `{
			"notify": {
				"message": "relay saved",
				"link": "/relay/`+relayID+`",
				"linkDescription": "open here"
			}
		}`)
}

// View loads a saved relay and renders the editor pre-populated with its content.
func (handler *Handler) View(writer http.ResponseWriter, request *http.Request) {
	relayID := chi.URLParam(request, "id")

	savedRelay, err := handler.service.Get(relayID)
	if errors.Is(err, ErrNotFound) {
		http.Redirect(writer, request, "/relay", http.StatusSeeOther)
		return
	}

	if err != nil {
		log.Printf("relay: view: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)

		return
	}

	if savedRelay.PrivateMode == PrivateModeUser {
		viewer, ok := auth.AuthUser(request.Context())
		if !ok || viewer.ID != savedRelay.UserID {
			http.Redirect(writer, request, "/relay", http.StatusSeeOther)
			return
		}
	}

	pageFile := "modules/relay/relay.html"
	data := map[string]any{
		"Page":     "relay",
		"Title":    "relay - meryl.moe",
		"Content":  savedRelay.Content,
		"ReadOnly": true,
	}

	if user, ok := auth.AuthUser(request.Context()); ok {
		data["User"] = user
	}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		log.Printf("relay: render view: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
