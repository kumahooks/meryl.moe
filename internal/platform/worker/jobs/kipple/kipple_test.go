package kipple_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"meryl.moe/internal/platform/db"
	kippleJob "meryl.moe/internal/platform/worker/jobs/kipple"
)

const (
	testUserID = "a2ccf831-0d18-4d77-b153-18cdc2334586"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := db.OpenCore(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	t.Cleanup(func() { database.Close() })

	return database
}

func insertTestUser(t *testing.T, database *sql.DB) {
	t.Helper()

	now := time.Now().Unix()

	if _, err := database.Exec(
		"INSERT INTO users (id, username, password_hash, updated_at, created_at) VALUES (?, ?, ?, ?, ?)",
		testUserID, "lain", "hash", now, now,
	); err != nil {
		t.Fatalf("insert test user: %v", err)
	}
}

// fixtureGIF returns the path to the small test GIF in the repo's static directory.
func fixtureGIF() string {
	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "..")

	return filepath.Join(repoRoot, "static", "assets", "gifs", "404", "03.gif")
}

// copyFixture copies the test GIF into dir and returns the destination path.
func copyFixture(t *testing.T, dir string) string {
	t.Helper()

	src, err := os.ReadFile(fixtureGIF())
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	dest := filepath.Join(dir, uuid.New().String())
	if err := os.WriteFile(dest, src, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	return dest
}

// insertFile inserts a kipple_files row with the given status, expireAt, and createdAt.
// Returns the inserted ID.
func insertFile(t *testing.T, database *sql.DB, dir string, status string, expireAt, createdAt int64) string {
	t.Helper()

	id := uuid.New().String()
	path := copyFixture(t, dir)

	if _, err := database.Exec(
		`INSERT INTO kipple_files (id, user_id, filename, size, offset, status, visibility, expire_at, path, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, testUserID, "03.gif", 1024, 1024, status, "link", expireAt, path, createdAt,
	); err != nil {
		t.Fatalf("insert file: %v", err)
	}

	return id
}

func countFiles(t *testing.T, database *sql.DB, id string) int {
	t.Helper()

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM kipple_files WHERE id = ?", id).Scan(&count); err != nil {
		t.Fatalf("count files: %v", err)
	}

	return count
}

func now() int64 { return time.Now().Unix() }

func TestCleanup_DeletesExpiredCompleteFile_DBRow(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)

	dir := t.TempDir()

	id := insertFile(t, database, dir, "complete", now()-3600, now()-86400)

	if err := kippleJob.Cleanup(context.Background(), database); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if countFiles(t, database, id) != 0 {
		t.Error("expired complete file row must be deleted")
	}
}

func TestCleanup_DeletesExpiredCompleteFile_DiskFile(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)

	dir := t.TempDir()

	id := insertFile(t, database, dir, "complete", now()-3600, now()-86400)

	var path string
	if err := database.QueryRow("SELECT path FROM kipple_files WHERE id = ?", id).Scan(&path); err != nil {
		t.Fatalf("scan path: %v", err)
	}

	if err := kippleJob.Cleanup(context.Background(), database); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("disk file must be removed after cleanup of expired file")
	}
}

func TestCleanup_PreservesActiveCompleteFile(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)

	dir := t.TempDir()

	id := insertFile(t, database, dir, "complete", now()+86400, now()-86400)

	if err := kippleJob.Cleanup(context.Background(), database); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if countFiles(t, database, id) != 1 {
		t.Error("active complete file must not be deleted by cleanup")
	}
}

func TestCleanup_DeletesStalePendingUpload(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)

	dir := t.TempDir()

	// created_at more than 1 hour ago = stale
	staleCreatedAt := now() - 3601
	id := insertFile(t, database, dir, "pending", now()+86400, staleCreatedAt)

	if err := kippleJob.Cleanup(context.Background(), database); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if countFiles(t, database, id) != 0 {
		t.Error("stale pending upload must be deleted by cleanup")
	}
}

func TestCleanup_DeletesStalePendingUpload_DiskFile(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)

	dir := t.TempDir()

	staleCreatedAt := now() - 3601
	id := insertFile(t, database, dir, "pending", now()+86400, staleCreatedAt)

	var path string
	if err := database.QueryRow("SELECT path FROM kipple_files WHERE id = ?", id).Scan(&path); err != nil {
		t.Fatalf("scan path: %v", err)
	}

	if err := kippleJob.Cleanup(context.Background(), database); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("disk file of stale pending upload must be removed")
	}
}

func TestCleanup_PreservesRecentPendingUpload(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)

	dir := t.TempDir()

	// created_at within the last hour = still in progress
	recentCreatedAt := now() - 60
	id := insertFile(t, database, dir, "pending", now()+86400, recentCreatedAt)

	if err := kippleJob.Cleanup(context.Background(), database); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if countFiles(t, database, id) != 1 {
		t.Error("recent pending upload must not be deleted by cleanup")
	}
}

func TestCleanup_MissingDiskFile_StillDeletesRow(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)

	dir := t.TempDir()

	id := insertFile(t, database, dir, "complete", now()-3600, now()-86400)

	// Remove the disk file before cleanup runs.
	var path string
	if err := database.QueryRow("SELECT path FROM kipple_files WHERE id = ?", id).Scan(&path); err != nil {
		t.Fatalf("scan path: %v", err)
	}

	os.Remove(path)

	if err := kippleJob.Cleanup(context.Background(), database); err != nil {
		t.Fatalf("Cleanup with missing disk file: %v", err)
	}

	if countFiles(t, database, id) != 0 {
		t.Error("row must be deleted even when disk file is already gone")
	}
}

func TestCleanup_OnlyDeletesExpired_WhenMixed(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)

	dir := t.TempDir()

	expiredID := insertFile(t, database, dir, "complete", now()-3600, now()-86400)
	activeID := insertFile(t, database, dir, "complete", now()+86400, now()-86400)

	if err := kippleJob.Cleanup(context.Background(), database); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if countFiles(t, database, expiredID) != 0 {
		t.Error("expired file must be deleted")
	}

	if countFiles(t, database, activeID) != 1 {
		t.Error("active file must not be deleted")
	}
}

func TestCleanup_EmptyTable_Succeeds(t *testing.T) {
	database := openTestDB(t)

	if err := kippleJob.Cleanup(context.Background(), database); err != nil {
		t.Errorf("Cleanup on empty table: %v", err)
	}
}

func TestCleanupOrphans_RemovesRowWithMissingDiskFile(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)

	dir := t.TempDir()

	id := insertFile(t, database, dir, "complete", now()+86400, now()-86400)

	var path string
	if err := database.QueryRow("SELECT path FROM kipple_files WHERE id = ?", id).Scan(&path); err != nil {
		t.Fatalf("scan path: %v", err)
	}

	os.Remove(path)

	if err := kippleJob.CleanupOrphans(context.Background(), database, dir); err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}

	if countFiles(t, database, id) != 0 {
		t.Error("row with missing disk file must be deleted by orphan cleanup")
	}
}

func TestCleanupOrphans_RemovesDiskFileWithNoRow(t *testing.T) {
	database := openTestDB(t)
	dir := t.TempDir()

	orphanPath := copyFixture(t, dir)

	if err := kippleJob.CleanupOrphans(context.Background(), database, dir); err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}

	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Error("disk file with no DB row must be removed by orphan cleanup")
	}
}

func TestCleanupOrphans_PreservesMatchedFiles(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)

	dir := t.TempDir()

	id := insertFile(t, database, dir, "complete", now()+86400, now()-86400)

	var path string
	if err := database.QueryRow("SELECT path FROM kipple_files WHERE id = ?", id).Scan(&path); err != nil {
		t.Fatalf("scan path: %v", err)
	}

	if err := kippleJob.CleanupOrphans(context.Background(), database, dir); err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}

	if countFiles(t, database, id) != 1 {
		t.Error("row with matching disk file must not be deleted")
	}

	if _, err := os.Stat(path); err != nil {
		t.Error("disk file with matching row must not be removed")
	}
}

func TestCleanupOrphans_NonexistentDir_Succeeds(t *testing.T) {
	database := openTestDB(t)

	nonexistentDir := filepath.Join(t.TempDir(), "does-not-exist")

	if err := kippleJob.CleanupOrphans(context.Background(), database, nonexistentDir); err != nil {
		t.Errorf("CleanupOrphans on nonexistent dir: %v", err)
	}
}

func TestCleanupOrphans_EmptyDir_Succeeds(t *testing.T) {
	database := openTestDB(t)

	if err := kippleJob.CleanupOrphans(context.Background(), database, t.TempDir()); err != nil {
		t.Errorf("CleanupOrphans on empty dir: %v", err)
	}
}

func TestCleanupOrphans_SkipsSubdirectories(t *testing.T) {
	database := openTestDB(t)
	dir := t.TempDir()

	// Create a subdirectory inside the kipple dir, orphan cleanup must not try to remove it.
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := kippleJob.CleanupOrphans(context.Background(), database, dir); err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}

	if _, err := os.Stat(subdir); err != nil {
		t.Error("subdirectory must not be removed by orphan cleanup")
	}
}

func TestCleanupOrphans_MixedOrphans_CleansAll(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)

	dir := t.TempDir()

	// One row with missing disk file.
	rowOrphanID := insertFile(t, database, dir, "complete", now()+86400, now()-86400)
	var rowOrphanPath string
	if err := database.QueryRow("SELECT path FROM kipple_files WHERE id = ?", rowOrphanID).
		Scan(&rowOrphanPath); err != nil {
		t.Fatalf("scan path: %v", err)
	}

	os.Remove(rowOrphanPath)

	// One disk file with no row.
	diskOrphanPath := copyFixture(t, dir)

	if err := kippleJob.CleanupOrphans(context.Background(), database, dir); err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}

	if countFiles(t, database, rowOrphanID) != 0 {
		t.Error("row orphan must be removed")
	}

	if _, err := os.Stat(diskOrphanPath); !os.IsNotExist(err) {
		t.Error("disk orphan must be removed")
	}
}
