package kipple_test

import (
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"meryl.moe/internal/modules/kipple"
	"meryl.moe/internal/platform/db"
)

const testQuota = int64(10 * 1024 * 1024) // 10 MB

// testFixturePath returns the absolute path to the small 404 GIF in static/.
// Go test working directory is the package dir, so we walk up to the repo root.
func testFixturePath() string {
	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..")

	return filepath.Join(repoRoot, "static", "assets", "gifs", "404", "03.gif")
}

// loadTestFixture reads the real GIF file used as upload content in tests.
func loadTestFixture(t *testing.T) []byte {
	t.Helper()

	data, err := os.ReadFile(testFixturePath())
	if err != nil {
		t.Fatalf("load test fixture: %v", err)
	}

	return data
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := db.OpenCore(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	t.Cleanup(func() { database.Close() })

	return database
}

func newService(t *testing.T, database *sql.DB) (*kipple.Service, string) {
	t.Helper()

	dir := t.TempDir()

	return kipple.NewService(database, dir, testQuota), dir
}

func insertTestUser(t *testing.T, database *sql.DB) string {
	t.Helper()

	userID := uuid.New().String()
	now := time.Now().Unix()

	if _, err := database.Exec(
		"INSERT INTO users (id, username, password_hash, updated_at, created_at) VALUES (?, ?, ?, ?, ?)",
		userID, userID, "irrelevant", now, now,
	); err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	return userID
}

func futureExpiry() time.Time {
	return time.Now().Add(24 * time.Hour)
}

func pastExpiry() time.Time {
	return time.Now().Add(-time.Hour)
}

func sha1Header(data []byte) string {
	h := sha1.New()
	h.Write(data)

	return "sha1 " + base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func createCompleteUpload(t *testing.T, service *kipple.Service, userID string) *kipple.Upload {
	t.Helper()

	content := loadTestFixture(t)

	upload, err := service.CreateUpload(userID, "03.gif", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create upload: %v", err)
	}

	if _, err := service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content)); err != nil {
		t.Fatalf("append chunk: %v", err)
	}

	return upload
}

func TestService_CreateUpload_Succeeds(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload, err := service.CreateUpload(userID, "file.txt", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create upload: %v", err)
	}

	if upload.ID == "" {
		t.Error("expected non-empty upload ID")
	}

	if upload.Status != "pending" {
		t.Errorf("status: got %q, want %q", upload.Status, "pending")
	}

	if upload.Offset != 0 {
		t.Errorf("offset: got %d, want 0", upload.Offset)
	}
}

func TestService_CreateUpload_CreatesFileOnDisk(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload, err := service.CreateUpload(userID, "file.txt", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create upload: %v", err)
	}

	if _, err := os.Stat(upload.Path); err != nil {
		t.Errorf("file not on disk at %s: %v", upload.Path, err)
	}
}

func TestService_CreateUpload_QuotaExceeded(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	_, err := service.CreateUpload(userID, "big.bin", testQuota+1, kipple.VisibilityLink, futureExpiry())
	if !errors.Is(err, kipple.ErrQuotaExceeded) {
		t.Errorf("error: got %v, want ErrQuotaExceeded", err)
	}
}

func TestService_CreateUpload_QuotaEnforcedAcrossFiles(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	half := testQuota / 2

	if _, err := service.CreateUpload(userID, "a.bin", half, kipple.VisibilityLink, futureExpiry()); err != nil {
		t.Fatalf("first upload: %v", err)
	}

	_, err := service.CreateUpload(userID, "b.bin", half+1, kipple.VisibilityLink, futureExpiry())
	if !errors.Is(err, kipple.ErrQuotaExceeded) {
		t.Errorf("error: got %v, want ErrQuotaExceeded", err)
	}
}

func TestService_CreateUpload_QuotaIsolatedPerUser(t *testing.T) {
	database := openTestDB(t)
	userA := insertTestUser(t, database)
	userB := insertTestUser(t, database)

	service, _ := newService(t, database)

	if _, err := service.CreateUpload(userA, "a.bin", testQuota-1, kipple.VisibilityLink, futureExpiry()); err != nil {
		t.Fatalf("user A upload: %v", err)
	}

	if _, err := service.CreateUpload(userB, "b.bin", testQuota-1, kipple.VisibilityLink, futureExpiry()); err != nil {
		t.Errorf("user B quota must be independent of user A; got %v", err)
	}
}

func TestService_GetUpload_NotFound(t *testing.T) {
	database := openTestDB(t)
	service, _ := newService(t, database)

	_, err := service.GetUpload("uwuowouwu")
	if !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("error: got %v, want ErrNotFound", err)
	}
}

func TestService_GetUpload_Found(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload, err := service.CreateUpload(userID, "file.txt", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	fetched, err := service.GetUpload(upload.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if fetched.ID != upload.ID {
		t.Errorf("ID: got %q, want %q", fetched.ID, upload.ID)
	}

	if fetched.UserID != userID {
		t.Errorf("UserID: got %q, want %q", fetched.UserID, userID)
	}
}

func TestService_GetUpload_SQLInjectionID_ReturnsNotFound(t *testing.T) {
	database := openTestDB(t)
	service, _ := newService(t, database)

	for _, id := range []string{"' OR '1'='1", "'; DROP TABLE kipple_files; --"} {
		_, err := service.GetUpload(id)

		if !errors.Is(err, kipple.ErrNotFound) {
			t.Errorf("SQL injection ID %q: got %v, want ErrNotFound", id, err)
		}
	}
}

func TestService_AppendChunk_SingleChunk_CompletesUpload(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("hewo kipple")

	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newOffset, err := service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))
	if err != nil {
		t.Fatalf("append: %v", err)
	}

	if newOffset != int64(len(content)) {
		t.Errorf("offset: got %d, want %d", newOffset, len(content))
	}

	fetched, err := service.GetUpload(upload.ID)
	if err != nil {
		t.Fatalf("get after complete: %v", err)
	}

	if fetched.Status != "complete" {
		t.Errorf("status: got %q, want complete", fetched.Status)
	}
}

func TestService_AppendChunk_MultipleChunks_CompletesUpload(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	fullChunk := loadTestFixture(t)
	midChunk := len(fullChunk) / 2
	chunkPartI := fullChunk[:midChunk]
	chunkPartII := fullChunk[midChunk:]
	fullChunkSize := int64(len(fullChunk))

	upload, err := service.CreateUpload(userID, "f.txt", fullChunkSize, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	offset, err := service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(chunkPartI), sha1Header(chunkPartI))
	if err != nil {
		t.Fatalf("append chunk 1: %v", err)
	}

	if offset != int64(len(chunkPartI)) {
		t.Errorf("offset after chunk 1: got %d, want %d", offset, len(chunkPartI))
	}

	offset, err = service.AppendChunk(upload.ID, userID, offset, bytes.NewReader(chunkPartII), sha1Header(chunkPartII))
	if err != nil {
		t.Fatalf("append chunk 2: %v", err)
	}

	if offset != fullChunkSize {
		t.Errorf("offset after chunk 2: got %d, want %d", offset, fullChunkSize)
	}

	fetched, err := service.GetUpload(upload.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if fetched.Status != "complete" {
		t.Errorf("status: got %q, want complete", fetched.Status)
	}
}

func TestService_AppendChunk_SkippedChunk_OffsetMismatch(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("hewu kipple")

	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	chunk := content[6:]

	_, err = service.AppendChunk(upload.ID, userID, 6, bytes.NewReader(chunk), sha1Header(chunk))
	if !errors.Is(err, kipple.ErrOffsetMismatch) {
		t.Errorf("skipped chunk: got %v, want ErrOffsetMismatch", err)
	}
}

func TestService_AppendChunk_WrongUser_Forbidden(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	otherID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("data")

	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = service.AppendChunk(upload.ID, otherID, 0, bytes.NewReader(content), sha1Header(content))
	if !errors.Is(err, kipple.ErrForbidden) {
		t.Errorf("error: got %v, want ErrForbidden", err)
	}
}

func TestService_AppendChunk_AlreadyComplete_Errors(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload := createCompleteUpload(t, service, userID)

	extra := []byte("extra data")

	_, err := service.AppendChunk(upload.ID, userID, upload.Size, bytes.NewReader(extra), sha1Header(extra))
	if !errors.Is(err, kipple.ErrUploadComplete) {
		t.Errorf("error: got %v, want ErrUploadComplete", err)
	}
}

func TestService_AppendChunk_ChecksumMismatch_Errors(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("real data")

	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	badChecksum := sha1Header([]byte("wrong data"))

	_, err = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), badChecksum)
	if !errors.Is(err, kipple.ErrChecksumMismatch) {
		t.Errorf("error: got %v, want ErrChecksumMismatch", err)
	}
}

func TestService_AppendChunk_ChecksumMismatch_OffsetNotAdvanced(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("real data")

	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header([]byte("wrong")))

	fetched, err := service.GetUpload(upload.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if fetched.Offset != 0 {
		t.Errorf("offset must not advance on checksum mismatch; got %d", fetched.Offset)
	}
}

func TestService_AppendChunk_UnsupportedChecksum_NoHeader(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("data")

	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), "")
	if !errors.Is(err, kipple.ErrUnsupportedChecksum) {
		t.Errorf("error: got %v, want ErrUnsupportedChecksum", err)
	}
}

func TestService_AppendChunk_UnsupportedChecksum_WrongAlgorithm(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("data")

	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), "md5 uwu=")
	if !errors.Is(err, kipple.ErrUnsupportedChecksum) {
		t.Errorf("error: got %v, want ErrUnsupportedChecksum", err)
	}
}

func TestService_AppendChunk_NotFound(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("data")

	_, err := service.AppendChunk("uwuuuuu", userID, 0, bytes.NewReader(content), sha1Header(content))
	if !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("error: got %v, want ErrNotFound", err)
	}
}

func TestService_AppendChunk_WritesToDisk(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := loadTestFixture(t)

	upload, err := service.CreateUpload(userID, "03.gif", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content)); err != nil {
		t.Fatalf("append: %v", err)
	}

	onDisk, err := os.ReadFile(upload.Path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if !bytes.Equal(onDisk, content) {
		t.Errorf("disk content mismatch: lengths got %d, want %d", len(onDisk), len(content))
	}
}

func TestService_DeleteFile_RemovesDBRow(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload := createCompleteUpload(t, service, userID)

	if err := service.DeleteFile(upload.ID, userID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := service.GetUpload(upload.ID)
	if !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("DB row must be deleted; GetUpload returned %v, want ErrNotFound", err)
	}
}

func TestService_DeleteFile_RemovesDiskFile(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload := createCompleteUpload(t, service, userID)
	path := upload.Path

	if err := service.DeleteFile(upload.ID, userID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("disk file must be removed after delete")
	}
}

func TestService_DeleteFile_NotFound(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	err := service.DeleteFile("owo7", userID)
	if !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("error: got %v, want ErrNotFound", err)
	}
}

func TestService_DeleteFile_WrongUser_Forbidden(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	otherID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload := createCompleteUpload(t, service, userID)

	err := service.DeleteFile(upload.ID, otherID)
	if !errors.Is(err, kipple.ErrForbidden) {
		t.Errorf("error: got %v, want ErrForbidden", err)
	}
}

func TestService_DeleteFile_WrongUser_DoesNotDeleteRow(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	otherID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload := createCompleteUpload(t, service, userID)

	_ = service.DeleteFile(upload.ID, otherID)

	if _, err := service.GetUpload(upload.ID); err != nil {
		t.Errorf("row must survive forbidden delete attempt; got %v", err)
	}
}

func TestService_DeleteFile_WorksForPendingUpload(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload, err := service.CreateUpload(userID, "f.txt", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err = service.DeleteFile(upload.ID, userID); err != nil {
		t.Fatalf("delete pending: %v", err)
	}

	_, err = service.GetUpload(upload.ID)
	if !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("pending row must be deleted; got %v, want ErrNotFound", err)
	}
}

func TestService_Get_ReturnsCompleteFile(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload := createCompleteUpload(t, service, userID)

	file, err := service.Get(upload.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if file.ID != upload.ID {
		t.Errorf("ID: got %q, want %q", file.ID, upload.ID)
	}
}

func TestService_Get_PendingFile_NotFound(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload, err := service.CreateUpload(userID, "f.txt", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = service.Get(upload.ID)
	if !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("pending file must not be accessible via Get; got %v, want ErrNotFound", err)
	}
}

func TestService_Get_ExpiredFile_NotFound(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("expired")
	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, pastExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))

	_, err = service.Get(upload.ID)
	if !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("expired file must not be accessible via Get; got %v, want ErrNotFound", err)
	}
}

func TestService_Get_UnknownID_NotFound(t *testing.T) {
	database := openTestDB(t)
	service, _ := newService(t, database)

	_, err := service.Get("no-such-id")
	if !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("error: got %v, want ErrNotFound", err)
	}
}

func TestService_Get_SQLInjectionID_NotFound(t *testing.T) {
	database := openTestDB(t)
	service, _ := newService(t, database)

	for _, id := range []string{"' OR '1'='1", "'; DROP TABLE kipple_files; --"} {
		_, err := service.Get(id)
		if !errors.Is(err, kipple.ErrNotFound) {
			t.Errorf("SQL injection ID %q: got %v, want ErrNotFound", id, err)
		}
	}
}

func TestService_GetInfo_ReturnsFormattedData(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload := createCompleteUpload(t, service, userID)

	info, err := service.GetInfo(upload.ID)
	if err != nil {
		t.Fatalf("get info: %v", err)
	}

	if info.ID != upload.ID {
		t.Errorf("ID: got %q, want %q", info.ID, upload.ID)
	}

	if info.Size == "" {
		t.Error("Size must not be empty")
	}

	if info.ExpiresIn == "" {
		t.Error("ExpiresIn must not be empty")
	}

	if info.CreatedAt == "" {
		t.Error("CreatedAt must not be empty")
	}
}

func TestService_GetInfo_PendingFile_NotFound(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload, err := service.CreateUpload(userID, "f.txt", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = service.GetInfo(upload.ID)
	if !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("pending file must not be visible via GetInfo; got %v, want ErrNotFound", err)
	}
}

func TestService_GetInfo_ExpiredFile_NotFound(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("expired")
	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, pastExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))

	_, err = service.GetInfo(upload.ID)
	if !errors.Is(err, kipple.ErrNotFound) {
		t.Errorf("expired file must not be visible via GetInfo; got %v, want ErrNotFound", err)
	}
}

func TestService_List_Empty_ReturnsNil(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	items, _, err := service.List(userID, 1, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if items != nil {
		t.Errorf("expected nil for empty list, got %v", items)
	}
}

func TestService_List_ExcludesPendingFiles(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	if _, err := service.CreateUpload(userID, "pending.txt", 1024, kipple.VisibilityLink, futureExpiry()); err != nil {
		t.Fatalf("create: %v", err)
	}

	items, _, err := service.List(userID, 1, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("pending files must not appear in list; got %d item(s)", len(items))
	}
}

func TestService_List_ExcludesExpiredFiles(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("expired")
	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, pastExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))

	items, _, err := service.List(userID, 1, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("expired files must not appear in list; got %d item(s)", len(items))
	}
}

func TestService_List_OnlyReturnsOwnFiles(t *testing.T) {
	database := openTestDB(t)
	userA := insertTestUser(t, database)
	userB := insertTestUser(t, database)

	service, _ := newService(t, database)

	createCompleteUpload(t, service, userA)
	createCompleteUpload(t, service, userB)

	items, _, err := service.List(userA, 1, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("list length: got %d, want 1", len(items))
	}
}

func TestService_List_SizeAndExpiresInFormatted(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	createCompleteUpload(t, service, userID)

	items, _, err := service.List(userID, 1, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("expected at least one item")
	}

	if items[0].Size == "" {
		t.Error("Size must not be empty")
	}

	if items[0].ExpiresIn == "" {
		t.Error("ExpiresIn must not be empty")
	}
}

func TestService_List_Pagination_HasNext(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	for range 3 {
		createCompleteUpload(t, service, userID)
	}

	items, hasNext, err := service.List(userID, 1, 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("items: got %d, want 2", len(items))
	}

	if !hasNext {
		t.Error("hasNext must be true when more items exist beyond page size")
	}
}

func TestService_List_Pagination_SecondPage(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	for range 3 {
		createCompleteUpload(t, service, userID)
	}

	items, hasNext, err := service.List(userID, 2, 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("items: got %d, want 1", len(items))
	}

	if hasNext {
		t.Error("hasNext must be false on last page")
	}
}

func TestService_GetQuota_Empty_ReturnsZeroPercent(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	quota, err := service.GetQuota(userID)
	if err != nil {
		t.Fatalf("get quota: %v", err)
	}

	if quota.Percent != 0 {
		t.Errorf("percent: got %d, want 0", quota.Percent)
	}
}

func TestService_GetQuota_CountsCompleteNonExpiredFiles(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	createCompleteUpload(t, service, userID)

	quota, err := service.GetQuota(userID)
	if err != nil {
		t.Fatalf("get quota: %v", err)
	}

	if quota.Percent == 0 {
		t.Error("quota percent must be > 0 after a completed upload")
	}
}

func TestService_GetQuota_ExcludesPendingFiles(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	if _, err := service.CreateUpload(userID, "pending.txt", 1024, kipple.VisibilityLink, futureExpiry()); err != nil {
		t.Fatalf("create: %v", err)
	}

	quota, err := service.GetQuota(userID)
	if err != nil {
		t.Fatalf("get quota: %v", err)
	}

	if quota.Percent != 0 {
		t.Errorf("pending files must not count toward displayed quota; got %d%%", quota.Percent)
	}
}

func TestService_GetQuota_ExcludesExpiredFiles(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("expired")
	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, pastExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))

	quota, err := service.GetQuota(userID)
	if err != nil {
		t.Fatalf("get quota: %v", err)
	}

	if quota.Percent != 0 {
		t.Errorf("expired files must not count toward displayed quota; got %d%%", quota.Percent)
	}
}

func TestService_GetQuota_PercentClampedAt100(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	// Quota of 1 byte; insert a larger completed file directly to bypass
	tinyQuota := int64(1)
	dir := t.TempDir()
	service := kipple.NewService(database, dir, tinyQuota)

	id := uuid.New().String()
	filePath := filepath.Join(dir, id)
	if err := os.WriteFile(filePath, loadTestFixture(t), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := database.Exec(
		`INSERT INTO kipple_files (id, user_id, filename, size, offset, status, visibility, expire_at, path, created_at)
		 VALUES (?, ?, ?, ?, ?, 'complete', 'link', ?, ?, ?)`,
		id, userID, "03.gif", 1024, 1024, futureExpiry().Unix(), filePath, time.Now().Unix(),
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	quota, err := service.GetQuota(userID)
	if err != nil {
		t.Fatalf("get quota: %v", err)
	}

	if quota.Percent > 100 {
		t.Errorf("percent must be clamped at 100; got %d", quota.Percent)
	}
}

func TestService_FormatBytes_LargeFile_ShowsGB(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	largeQuota := int64(5 * 1024 * 1024 * 1024)
	dir := t.TempDir()
	service := kipple.NewService(database, dir, largeQuota)

	sizeGB := int64(1024 * 1024 * 1024)

	upload, err := service.CreateUpload(userID, "big.bin", sizeGB, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Bypass actual upload: directly mark as complete with a 1 GiB size to test formatting.
	if _, err = database.Exec(
		"UPDATE kipple_files SET size = ?, offset = ?, status = 'complete' WHERE id = ?",
		sizeGB, sizeGB, upload.ID,
	); err != nil {
		t.Fatalf("update: %v", err)
	}

	items, _, err := service.List(userID, 1, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("expected item")
	}

	if !strings.HasSuffix(items[0].Size, "GB") {
		t.Errorf("expected GB suffix for 1 GiB file; got %q", items[0].Size)
	}
}

func TestService_AppendChunk_EmptyBody_ChecksumMismatch(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("data")

	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Empty body but checksum of real content: should mismatch since nothing written.
	_, err = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(nil), sha1Header(content))
	if !errors.Is(err, kipple.ErrChecksumMismatch) {
		t.Errorf("empty body with wrong checksum: got %v, want ErrChecksumMismatch", err)
	}
}

func TestService_AppendChunk_LargeChunk_Succeeds(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	// Repeat the fixture to reach ~4 MB to exercise chunked I/O paths.
	base := loadTestFixture(t)
	repeats := (4 * 1024 * 1024) / len(base)
	content := bytes.Repeat(base, repeats+1)

	upload, err := service.CreateUpload(userID, "large.bin", int64(len(content)), kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newOffset, err := service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))
	if err != nil {
		t.Fatalf("append large chunk: %v", err)
	}

	if newOffset != int64(len(content)) {
		t.Errorf("offset: got %d, want %d", newOffset, len(content))
	}
}

func TestService_CreateUpload_VisibilityUser_Stored(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload, err := service.CreateUpload(userID, "f.txt", 1024, kipple.VisibilityUser, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	fetched, err := service.GetUpload(upload.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if fetched.Visibility != kipple.VisibilityUser {
		t.Errorf("visibility: got %q, want %q", fetched.Visibility, kipple.VisibilityUser)
	}
}

func TestService_Get_ReturnsVisibility(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	content := []byte("private")
	upload, err := service.CreateUpload(userID, "f.txt", int64(len(content)), kipple.VisibilityUser, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _ = service.AppendChunk(upload.ID, userID, 0, bytes.NewReader(content), sha1Header(content))

	file, err := service.Get(upload.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if file.Visibility != kipple.VisibilityUser {
		t.Errorf("visibility: got %q, want %q", file.Visibility, kipple.VisibilityUser)
	}
}

func TestService_CreateUpload_ExpiryStoredCorrectly(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	expiry := time.Now().Add(7 * 24 * time.Hour).Truncate(time.Second)

	upload, err := service.CreateUpload(userID, "f.txt", 1024, kipple.VisibilityLink, expiry)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	fetched, err := service.GetUpload(upload.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if fetched.ExpireAt != expiry.Unix() {
		t.Errorf("expire_at: got %d, want %d", fetched.ExpireAt, expiry.Unix())
	}
}

func TestService_AppendChunk_ReaderError_ReturnsError(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service, _ := newService(t, database)

	upload, err := service.CreateUpload(userID, "f.txt", 1024, kipple.VisibilityLink, futureExpiry())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// A reader that always errors.
	badReader := &errorReader{}

	_, err = service.AppendChunk(upload.ID, userID, 0, badReader, "sha1 AAAAAAAAgetmeoutofhereAAAAAAAA==")
	if err == nil {
		t.Error("expected error from failing reader, got nil")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
