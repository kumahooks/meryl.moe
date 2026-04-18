// Package wired implements the login, logout, and session management handlers.
package wired

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"meryl.moe/internal/platform/auth"
	"meryl.moe/internal/platform/dispatch"
	"meryl.moe/internal/platform/middleware"
	"meryl.moe/internal/platform/templates"
)

// Handler handles login, logout, and session creation.
type Handler struct {
	renderer     templates.Renderer
	authService  *auth.Service
	dispatcher   *dispatch.Dispatcher
	secureCookie bool
}

// NewHandler returns a Handler backed by the given auth service, dispatcher, and renderer.
func NewHandler(
	renderer templates.Renderer,
	authService *auth.Service,
	isDevelopment bool,
	dispatcher *dispatch.Dispatcher,
) *Handler {
	return &Handler{
		renderer:     renderer,
		authService:  authService,
		dispatcher:   dispatcher,
		secureCookie: !isDevelopment,
	}
}

// Routes registers the wired (auth) routes on the given router.
// database is required for the RequireAuth middleware on protected routes.
func Routes(handler *Handler, database *sql.DB) func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/wired", handler.Login)
		router.Post("/wired", handler.Authenticate)
		router.Post("/wired/logout", handler.Logout)
		router.With(middleware.RequireAuth(database)).Get("/wired/me", handler.Me)
	}
}

// Login renders the login form.
func (handler *Handler) Login(writer http.ResponseWriter, request *http.Request) {
	handler.renderLogin(writer, request, "")
}

// Authenticate validates credentials, creates a session, and redirects to /wired/me.
func (handler *Handler) Authenticate(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1024)
	if err := request.ParseForm(); err != nil {
		writer.WriteHeader(http.StatusForbidden)
		handler.renderLogin(writer, request, "invalid credentials")

		return
	}

	username := request.FormValue("username")
	password := request.FormValue("password")

	if username == "" || password == "" {
		writer.WriteHeader(http.StatusForbidden)
		handler.renderLogin(writer, request, "invalid credentials")

		return
	}

	rawToken, expiresAt, err := handler.authService.Authenticate(username, password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writer.WriteHeader(http.StatusForbidden)
			handler.renderLogin(writer, request, "invalid credentials")

			return
		}

		log.Printf("wired: authenticate: %v", err)

		writer.WriteHeader(http.StatusInternalServerError)
		handler.renderLogin(writer, request, "internal server error")

		return
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     "session",
		Value:    rawToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   handler.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(writer, request, "/wired/me", http.StatusSeeOther)
}

// Logout deletes the session from the database, clears the cookie, and redirects to /.
func (handler *Handler) Logout(writer http.ResponseWriter, request *http.Request) {
	sessionCookie, err := request.Cookie("session")
	if err == nil {
		if err := handler.authService.Logout(sessionCookie.Value); err != nil {
			log.Printf("wired: logout: %v", err)
		}
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   handler.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

// Me renders the authenticated user status page.
func (handler *Handler) Me(writer http.ResponseWriter, request *http.Request) {
	user, _ := auth.AuthUser(request.Context())

	pageFile := "modules/wired/me.html"
	data := map[string]any{"Title": "wired/me", "User": user}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		log.Printf("wired: render me: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func (handler *Handler) renderLogin(writer http.ResponseWriter, request *http.Request, errorMessage string) {
	pageFile := "modules/wired/wired.html"
	data := map[string]any{"Title": "wired"}

	if errorMessage != "" {
		data["Error"] = errorMessage
	}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		log.Printf("wired: render login: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
