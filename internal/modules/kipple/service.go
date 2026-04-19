package kipple

import (
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound            = errors.New("kipple: not found")
	ErrOffsetMismatch      = errors.New("kipple: offset mismatch")
	ErrChecksumMismatch    = errors.New("kipple: checksum mismatch")
	ErrUnsupportedChecksum = errors.New("kipple: unsupported checksum algorithm")
	ErrQuotaExceeded       = errors.New("kipple: quota exceeded")
	ErrUploadComplete      = errors.New("kipple: upload already complete")
	ErrForbidden           = errors.New("kipple: forbidden")
)

const (
	VisibilityLink = "link"
	VisibilityUser = "user"
)

// Upload is an in-progress or completed file upload record.
type Upload struct {
	ID         string
	UserID     string
	Filename   string
	Size       int64
	Offset     int64
	Status     string
	Visibility string
	ExpireAt   int64
	Path       string
}

// File is a completed file ready for download.
type File struct {
	ID         string
	UserID     string
	Filename   string
	Size       int64
	Visibility string
	Path       string
}

// FileInfo holds formatted display data for the file info page.
type FileInfo struct {
	ID         string
	UserID     string
	Filename   string
	Size       string
	Visibility string
	CreatedAt  string
	ExpiresIn  string
	UploadedBy string
}

// FileListItem is a summary row for the file table.
type FileListItem struct {
	ID         string
	Filename   string
	Size       string
	ExpiresIn  string
	Visibility string
}

// QuotaInfo holds formatted quota display values.
type QuotaInfo struct {
	UsedStr   string
	TotalStr  string
	Percent   int
	FillStyle template.HTMLAttr
}

// Service handles kipple file persistence and upload lifecycle.
type Service struct {
	database *sql.DB
	dir      string
	quota    int64
}

// NewService returns a Service backed by the given database, storing files in dir.
func NewService(database *sql.DB, dir string, quota int64) *Service {
	return &Service{database: database, dir: dir, quota: quota}
}

// CreateUpload checks quota atomically, creates the file on disk, and inserts a pending DB row.
// expireAt is the desired expiry for the completed file.
func (service *Service) CreateUpload(
	userID, filename string,
	size int64,
	visibility string,
	expireAt time.Time,
) (*Upload, error) {
	if err := os.MkdirAll(service.dir, 0o755); err != nil {
		return nil, fmt.Errorf("create kipple dir: %w", err)
	}

	uploadID := uuid.New().String()
	filePath := filepath.Join(service.dir, uploadID)

	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("create upload file: %w", err)
	}

	file.Close()

	now := time.Now().Unix()

	result, err := service.database.Exec(
		`INSERT INTO kipple_files (id, user_id, filename, size, offset, status, visibility, expire_at, path, created_at)
		 SELECT ?, ?, ?, ?, 0, 'pending', ?, ?, ?, ?
		 WHERE (SELECT COALESCE(SUM(size), 0) FROM kipple_files WHERE user_id = ?) + ? <= ?`,
		uploadID, userID, filename, size, visibility, expireAt.Unix(), filePath, now,
		userID, size, service.quota,
	)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("insert upload: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("check quota: %w", err)
	}

	if rows == 0 {
		os.Remove(filePath)
		return nil, ErrQuotaExceeded
	}

	return &Upload{
		ID:         uploadID,
		UserID:     userID,
		Filename:   filename,
		Size:       size,
		Offset:     0,
		Status:     "pending",
		Visibility: visibility,
		ExpireAt:   expireAt.Unix(),
		Path:       filePath,
	}, nil
}

// GetUpload fetches a kipple_files row by ID regardless of status.
func (service *Service) GetUpload(id string) (*Upload, error) {
	upload := &Upload{ID: id}

	err := service.database.QueryRow(
		`SELECT user_id, filename, size, offset, status, visibility, expire_at, path
		 FROM kipple_files WHERE id = ?`,
		id,
	).Scan(&upload.UserID, &upload.Filename, &upload.Size, &upload.Offset,
		&upload.Status, &upload.Visibility, &upload.ExpireAt, &upload.Path)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("query upload: %w", err)
	}

	return upload, nil
}

// AppendChunk writes reader at offset into the upload file, and advances the stored offset.
// userID must match the upload owner.
func (service *Service) AppendChunk(
	id, userID string,
	offset int64,
	reader io.Reader,
	checksumHeader string,
) (int64, error) {
	upload, err := service.GetUpload(id)
	if err != nil {
		return 0, err
	}

	if upload.UserID != userID {
		return 0, ErrForbidden
	}

	if upload.Status != "pending" {
		return 0, ErrUploadComplete
	}

	if upload.Offset != offset {
		return 0, ErrOffsetMismatch
	}

	parts := strings.SplitN(checksumHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "sha1" {
		return 0, ErrUnsupportedChecksum
	}

	expectedHash := parts[1]

	file, err := os.OpenFile(upload.Path, os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open file: %w", err)
	}

	defer file.Close()

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return 0, fmt.Errorf("seek: %w", err)
	}

	hasher := sha1.New()
	written, copyErr := io.Copy(file, io.TeeReader(reader, hasher))
	newOffset := offset + written

	if copyErr == nil {
		computed := base64.StdEncoding.EncodeToString(hasher.Sum(nil))

		if computed != expectedHash {
			file.Truncate(offset)
			return offset, ErrChecksumMismatch
		}
	}

	if _, dbErr := service.database.Exec(
		"UPDATE kipple_files SET offset = ? WHERE id = ?",
		newOffset, id,
	); dbErr != nil {
		return 0, fmt.Errorf("update offset: %w", dbErr)
	}

	if newOffset == upload.Size {
		if completeErr := service.completeUpload(id); completeErr != nil {
			return newOffset, completeErr
		}
	}

	if copyErr != nil {
		return newOffset, fmt.Errorf("copy: %w", copyErr)
	}

	return newOffset, nil
}

func (service *Service) completeUpload(id string) error {
	_, err := service.database.Exec(
		"UPDATE kipple_files SET status = 'complete' WHERE id = ?",
		id,
	)
	if err != nil {
		return fmt.Errorf("complete upload: %w", err)
	}

	return nil
}

// DeleteFile removes the file from disk and its DB row. userID must match the owner.
// Works for both pending uploads and completed files.
func (service *Service) DeleteFile(id, userID string) error {
	upload, err := service.GetUpload(id)
	if err != nil {
		return err
	}

	if upload.UserID != userID {
		return ErrForbidden
	}

	os.Remove(upload.Path)

	if _, err := service.database.Exec("DELETE FROM kipple_files WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	return nil
}

// Get fetches a completed, non-expired file by ID for download.
func (service *Service) Get(id string) (*File, error) {
	file := &File{ID: id}

	err := service.database.QueryRow(
		`SELECT user_id, filename, size, visibility, path
		 FROM kipple_files WHERE id = ? AND status = 'complete' AND expire_at > ?`,
		id, time.Now().Unix(),
	).Scan(&file.UserID, &file.Filename, &file.Size, &file.Visibility, &file.Path)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("query file: %w", err)
	}

	return file, nil
}

// GetInfo fetches display metadata for a completed, non-expired file.
func (service *Service) GetInfo(id string) (*FileInfo, error) {
	info := &FileInfo{ID: id}
	var size, expireAt, createdAt int64

	err := service.database.QueryRow(
		`SELECT kf.user_id, kf.filename, kf.size, kf.visibility, kf.expire_at, kf.created_at, u.username
		 FROM kipple_files kf
		 JOIN users u ON u.id = kf.user_id
		 WHERE kf.id = ? AND kf.status = 'complete' AND kf.expire_at > ?`,
		id, time.Now().Unix(),
	).Scan(&info.UserID, &info.Filename, &size, &info.Visibility, &expireAt, &createdAt, &info.UploadedBy)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("query file info: %w", err)
	}

	info.Size = formatBytes(size)
	info.ExpiresIn = formatExpiresIn(expireAt)
	info.CreatedAt = time.Unix(createdAt, 0).Format("2006-01-02")

	return info, nil
}

// List returns one page of completed, non-expired files for userID, newest first.
// hasNext reports whether more files exist beyond this page.
func (service *Service) List(userID string, page, pageSize int) ([]FileListItem, bool, error) {
	offset := (page - 1) * pageSize

	rows, err := service.database.Query(
		`SELECT id, filename, size, expire_at, visibility
		 FROM kipple_files WHERE user_id = ? AND status = 'complete' AND expire_at > ?
		 ORDER BY filename ASC
		 LIMIT ? OFFSET ?`,
		userID, time.Now().Unix(), pageSize+1, offset,
	)
	if err != nil {
		return nil, false, fmt.Errorf("query files: %w", err)
	}

	defer rows.Close()

	var items []FileListItem

	for rows.Next() {
		var item FileListItem
		var size int64
		var expireAt int64

		if err := rows.Scan(&item.ID, &item.Filename, &size, &expireAt, &item.Visibility); err != nil {
			return nil, false, fmt.Errorf("scan file: %w", err)
		}

		item.Size = formatBytes(size)
		item.ExpiresIn = formatExpiresIn(expireAt)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate files: %w", err)
	}

	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}

	return items, hasNext, nil
}

// GetQuota returns quota display info for the given user.
// Only counts completed, non-expired files.
func (service *Service) GetQuota(userID string) (*QuotaInfo, error) {
	var used int64

	err := service.database.QueryRow(
		`SELECT COALESCE(SUM(size), 0) FROM kipple_files
		 WHERE user_id = ? AND status = 'complete' AND expire_at > ?`,
		userID, time.Now().Unix(),
	).Scan(&used)
	if err != nil {
		return nil, fmt.Errorf("sum quota: %w", err)
	}

	percent := 0
	if service.quota > 0 {
		percent = int(used * 100 / service.quota)

		if percent > 100 {
			percent = 100
		}
	}

	return &QuotaInfo{
		UsedStr:   formatBytes(used),
		TotalStr:  formatBytes(service.quota),
		Percent:   percent,
		FillStyle: template.HTMLAttr(fmt.Sprintf(`style="width: %d%%"`, percent)),
	}, nil
}

func formatBytes(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.2f GB", float64(bytes)/gb)
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/mb)
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/kb)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatExpiresIn(expireAt int64) string {
	remaining := expireAt - time.Now().Unix()
	days := remaining / 86400

	if days <= 0 {
		return "<1d"
	}

	return fmt.Sprintf("%dd", days)
}
