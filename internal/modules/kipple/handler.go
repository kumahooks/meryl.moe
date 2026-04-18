// Package kipple implements the file sharing tool page handler
package kipple

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"meryl.moe/internal/platform/auth"
	"meryl.moe/internal/platform/middleware"
	"meryl.moe/internal/platform/templates"
)

// Handler handles requests for the kipple file sharing tool.
type Handler struct {
	renderer templates.Renderer
	service  *Service
}

// NewHandler returns a Handler backed by the given renderer and service.
func NewHandler(renderer templates.Renderer, service *Service) *Handler {
	return &Handler{renderer: renderer, service: service}
}

// Routes registers kipple routes on the given router.
func Routes(handler *Handler, database *sql.DB) func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/kipple", handler.Index)
		router.With(middleware.RequireAuth(database)).Post("/kipple/upload", handler.CreateUpload)
		router.With(middleware.RequireAuth(database)).Head("/kipple/upload/{id}", handler.HeadUpload)
		router.With(middleware.RequireAuth(database)).Patch("/kipple/upload/{id}", handler.AppendChunk)
		router.With(middleware.RequireAuth(database)).Delete("/kipple/upload/{id}", handler.TerminateUpload)
		router.With(middleware.RequireAuth(database)).Get("/kipple/list", handler.List)
		router.With(middleware.RequireAuth(database)).Delete("/kipple/{id}", handler.Delete)
		router.Get("/kipple/{id}", handler.View)
		router.Get("/kipple/{id}/download", handler.Download)
	}
}

// Index renders the kipple file sharing page.
func (handler *Handler) Index(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/kipple/kipple.html"
	data := map[string]any{"Page": "kipple", "Title": "kipple - meryl.moe"}

	user, ok := auth.AuthUser(request.Context())
	if ok {
		data["User"] = user
	}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		log.Printf("kipple: render index: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// List handles GET /kipple/list - returns the file list rows and OOB quota update.
func (handler *Handler) List(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/kipple/kipple.html"
	user, _ := auth.AuthUser(request.Context())

	files, err := handler.service.List(user.ID)
	if err != nil {
		log.Printf("kipple: list: list: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	quota, err := handler.service.GetQuota(user.ID)
	if err != nil {
		log.Printf("kipple: list: quota: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{"Files": files, "Quota": quota}

	if err := handler.renderer.Render(writer, request, pageFile, "kipple-rows", data); err != nil {
		log.Printf("kipple: render list: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// CreateUpload handles POST /kipple/upload - creates a new tus upload session.
func (handler *Handler) CreateUpload(writer http.ResponseWriter, request *http.Request) {
	uploadLengthStr := request.Header.Get("Upload-Length")
	if uploadLengthStr == "" {
		http.Error(writer, "Upload-Length required", http.StatusBadRequest)
		return
	}

	uploadLength, err := strconv.ParseInt(uploadLengthStr, 10, 64)
	if err != nil || uploadLength <= 0 {
		http.Error(writer, "invalid Upload-Length", http.StatusBadRequest)
		return
	}

	metadata := parseMetadata(request.Header.Get("Upload-Metadata"))

	filename := strings.TrimSpace(metadata["filename"])
	if filename == "" {
		http.Error(writer, "filename required in Upload-Metadata", http.StatusBadRequest)
		return
	}

	visibility := metadata["visibility"]
	if visibility != VisibilityUser && visibility != VisibilityLink {
		visibility = VisibilityLink
	}

	expireDays := 1
	if days, err := strconv.Atoi(metadata["expire_days"]); err == nil && days > 0 {
		expireDays = days
	}

	expireAt := time.Now().Add(time.Duration(expireDays) * 24 * time.Hour)

	user, _ := auth.AuthUser(request.Context())

	upload, err := handler.service.CreateUpload(user.ID, filename, uploadLength, visibility, expireAt)
	if errors.Is(err, ErrQuotaExceeded) {
		// TODO: maybe we shouldn't reset the upload list when failing here
		http.Error(writer, "quota exceeded", http.StatusRequestEntityTooLarge)
		return
	}

	if err != nil {
		log.Printf("kipple: create upload: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Location", "/kipple/upload/"+upload.ID)
	writer.WriteHeader(http.StatusCreated)
}

// HeadUpload handles HEAD /kipple/upload/:id - returns the current upload offset.
func (handler *Handler) HeadUpload(writer http.ResponseWriter, request *http.Request) {
	id := chi.URLParam(request, "id")

	upload, err := handler.service.GetUpload(id)
	if errors.Is(err, ErrNotFound) {
		http.NotFound(writer, request)
		return
	}

	if err != nil {
		log.Printf("kipple: head upload: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	user, _ := auth.AuthUser(request.Context())
	if upload.UserID != user.ID {
		http.NotFound(writer, request)
		return
	}

	writer.Header().Set("Upload-Offset", strconv.FormatInt(upload.Offset, 10))
	writer.Header().Set("Upload-Length", strconv.FormatInt(upload.Size, 10))
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

// AppendChunk handles PATCH /kipple/upload/:id - appends a chunk at the given offset.
func (handler *Handler) AppendChunk(writer http.ResponseWriter, request *http.Request) {
	if request.Header.Get("Content-Type") != "application/offset+octet-stream" {
		http.Error(writer, "Content-Type must be application/offset+octet-stream", http.StatusUnsupportedMediaType)
		return
	}

	offsetStr := request.Header.Get("Upload-Offset")
	if offsetStr == "" {
		http.Error(writer, "Upload-Offset required", http.StatusBadRequest)
		return
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil || offset < 0 {
		http.Error(writer, "invalid Upload-Offset", http.StatusBadRequest)
		return
	}

	id := chi.URLParam(request, "id")
	checksumHeader := request.Header.Get("Upload-Checksum")
	user, _ := auth.AuthUser(request.Context())

	newOffset, err := handler.service.AppendChunk(id, user.ID, offset, request.Body, checksumHeader)

	if errors.Is(err, ErrNotFound) {
		http.NotFound(writer, request)
		return
	}

	if errors.Is(err, ErrForbidden) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}

	if errors.Is(err, ErrUploadComplete) {
		http.Error(writer, "upload already complete", http.StatusForbidden)
		return
	}

	if errors.Is(err, ErrOffsetMismatch) {
		http.Error(writer, "offset mismatch", http.StatusConflict)
		return
	}

	if errors.Is(err, ErrUnsupportedChecksum) {
		http.Error(writer, "unsupported checksum algorithm", http.StatusBadRequest)
		return
	}

	if errors.Is(err, ErrChecksumMismatch) {
		writer.Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))
		http.Error(writer, "checksum mismatch", 460)
		return
	}

	if err != nil {
		log.Printf("kipple: append chunk: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))
	writer.WriteHeader(http.StatusNoContent)
}

// TerminateUpload handles DELETE /kipple/upload/:id - cancels an in-progress upload.
// TODO: maybe we shouldn't reset the upload list when running here
func (handler *Handler) TerminateUpload(writer http.ResponseWriter, request *http.Request) {
	id := chi.URLParam(request, "id")
	user, _ := auth.AuthUser(request.Context())

	err := handler.service.DeleteFile(id, user.ID)
	if errors.Is(err, ErrNotFound) {
		http.NotFound(writer, request)
		return
	}

	if errors.Is(err, ErrForbidden) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}

	if err != nil {
		log.Printf("kipple: terminate upload: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

// Delete handles DELETE /kipple/:id - deletes a completed file.
func (handler *Handler) Delete(writer http.ResponseWriter, request *http.Request) {
	pageFile := "modules/kipple/kipple.html"

	id := chi.URLParam(request, "id")
	user, _ := auth.AuthUser(request.Context())

	// TODO: validate if this is deleting from db correctly
	err := handler.service.DeleteFile(id, user.ID)
	if errors.Is(err, ErrNotFound) {
		http.NotFound(writer, request)
		return
	}

	if errors.Is(err, ErrForbidden) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}

	if err != nil {
		log.Printf("kipple: delete: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	quota, err := handler.service.GetQuota(user.ID)
	if err != nil {
		log.Printf("kipple: delete: quota: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{"Quota": quota}

	if err := handler.renderer.Render(writer, request, pageFile, "kipple-quota", data); err != nil {
		log.Printf("kipple: render delete: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// View handles GET /kipple/:id - renders the file info page.
func (handler *Handler) View(writer http.ResponseWriter, request *http.Request) {
	id := chi.URLParam(request, "id")

	info, err := handler.service.GetInfo(id)
	if errors.Is(err, ErrNotFound) {
		http.Redirect(writer, request, "/kipple", http.StatusFound)
		return
	}

	if err != nil {
		log.Printf("kipple: view: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	if info.Visibility == VisibilityUser {
		viewer, ok := auth.AuthUser(request.Context())
		if !ok || viewer.ID != info.UserID {
			http.Redirect(writer, request, "/kipple", http.StatusFound)
			return
		}
	}

	pageFile := "modules/kipple/kipple-file.html"
	data := map[string]any{
		"Page":  "kipple",
		"Title": info.Filename + " - kipple - meryl.moe",
		"File":  info,
	}

	user, ok := auth.AuthUser(request.Context())
	if ok {
		data["User"] = user
	}

	if err := handler.renderer.Render(writer, request, pageFile, "page-content", data); err != nil {
		log.Printf("kipple: render view: %v", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// Download handles GET /kipple/:id/download - serves a completed file for download.
func (handler *Handler) Download(writer http.ResponseWriter, request *http.Request) {
	id := chi.URLParam(request, "id")

	file, err := handler.service.Get(id)
	if errors.Is(err, ErrNotFound) {
		http.Redirect(writer, request, "/kipple", http.StatusFound)
		return
	}

	if err != nil {
		log.Printf("kipple: download: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	if file.Visibility == VisibilityUser {
		viewer, ok := auth.AuthUser(request.Context())
		if !ok || viewer.ID != file.UserID {
			http.Redirect(writer, request, "/kipple", http.StatusFound)
			return
		}
	}

	osFile, err := os.Open(file.Path)
	if err != nil {
		log.Printf("kipple: download: open: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}
	defer osFile.Close()

	stat, err := osFile.Stat()
	if err != nil {
		log.Printf("kipple: download: stat: %v", err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	writer.Header().
		Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.Filename}))
	http.ServeContent(writer, request, file.Filename, stat.ModTime(), osFile)
}

func parseMetadata(header string) map[string]string {
	result := make(map[string]string)

	for _, pair := range strings.Split(header, ",") {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, " ", 2)

		if len(parts) != 2 {
			continue
		}

		// tus metadata values are base64-encoded
		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			continue
		}

		result[parts[0]] = string(decoded)
	}

	return result
}
