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

// Service handles relay persistence.
type Service struct {
	database *sql.DB
}

// NewService returns a Service backed by the given database.
func NewService(database *sql.DB) *Service {
	return &Service{database: database}
}

// Save compresses text and stores it for the given user. Returns the new relay ID.
func (service *Service) Save(userID string, text string) (string, error) {
	compressed, err := compressText(text)
	if err != nil {
		return "", fmt.Errorf("compress relay content: %w", err)
	}

	relayID := uuid.New().String()
	now := time.Now().Unix()

	if _, err := service.database.Exec(
		"INSERT INTO relays (id, user_id, content, private, created_at) VALUES (?, ?, ?, 0, ?)",
		relayID, userID, compressed, now,
	); err != nil {
		return "", fmt.Errorf("insert relay: %w", err)
	}

	return relayID, nil
}

// Get retrieves and decompresses the text content of the relay with the given ID.
// Returns ErrNotFound if no relay with that ID exists.
func (service *Service) Get(id string) (string, error) {
	var compressed []byte

	err := service.database.QueryRow(
		"SELECT content FROM relays WHERE id = ?", id,
	).Scan(&compressed)

	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}

	if err != nil {
		return "", fmt.Errorf("query relay: %w", err)
	}

	return decompressText(compressed)
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
