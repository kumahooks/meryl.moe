package relay

import (
	"bytes"
	"compress/zlib"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a relay with the given ID does not exist.
var ErrNotFound = errors.New("relay not found")

const (
	PrivateModeLink = "link"
	PrivateModeUser = "user"
)

// Relay represents a stored relay with its metadata.
type Relay struct {
	ID          string
	UserID      string
	Content     string
	PrivateMode string
	ExpiresAt   time.Time
}

// Service handles relay persistence.
type Service struct {
	database *sql.DB
}

// NewService returns a Service backed by the given database.
func NewService(database *sql.DB) *Service {
	return &Service{database: database}
}

// Save compresses text and stores it with the given options. Returns the new relay ID.
func (service *Service) Save(userID string, text string, privateMode string, expiresAt time.Time) (string, error) {
	compressed, err := compressText(text)
	if err != nil {
		return "", fmt.Errorf("compress relay content: %w", err)
	}

	relayID := uuid.New().String()
	now := time.Now().Unix()

	if _, err := service.database.Exec(
		"INSERT INTO relays (id, user_id, content, private_mode, expire_at, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		relayID, userID, compressed, privateMode, expiresAt.Unix(), now,
	); err != nil {
		return "", fmt.Errorf("insert relay: %w", err)
	}

	return relayID, nil
}

// Get retrieves a relay by ID. Returns ErrNotFound if no relay with that ID exists.
func (service *Service) Get(id string) (*Relay, error) {
	var compressed []byte
	var expiresAtUnix int64

	savedRelay := &Relay{ID: id}

	err := service.database.QueryRow(
		"SELECT user_id, content, private_mode, expire_at FROM relays WHERE id = ?", id,
	).Scan(&savedRelay.UserID, &compressed, &savedRelay.PrivateMode, &expiresAtUnix)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("query relay: %w", err)
	}

	content, err := decompressText(compressed)
	if err != nil {
		return nil, err
	}

	savedRelay.Content = content
	savedRelay.ExpiresAt = time.Unix(expiresAtUnix, 0)

	return savedRelay, nil
}

func compressText(text string) ([]byte, error) {
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)

	if _, err := writer.Write([]byte(text)); err != nil {
		writer.Close()
		return nil, fmt.Errorf("write: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close: %w", err)
	}

	return buf.Bytes(), nil
}

func decompressText(data []byte) (string, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}

	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}

	return string(decompressed), nil
}
