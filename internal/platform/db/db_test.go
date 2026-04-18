package db_test

import (
	"path/filepath"
	"testing"

	"meryl.moe/internal/platform/db"
)

func TestOpenCore_Succeeds(t *testing.T) {
	database, err := db.OpenCore(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	database.Close()
}

func TestOpenCore_TablesCreated(t *testing.T) {
	database, err := db.OpenCore(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	defer database.Close()

	for _, table := range []string{"schema_migrations", "users", "sessions", "relays", "roles", "users_roles", "kipple_files"} {
		var name string

		if err := database.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name); err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestOpenCore_MigrationsTracked(t *testing.T) {
	database, err := db.OpenCore(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	defer database.Close()

	rows, err := database.Query("SELECT id, name FROM schema_migrations ORDER BY id")
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}

	defer rows.Close()

	type record struct {
		id   int
		name string
	}

	var records []record
	for rows.Next() {
		var rec record
		if err := rows.Scan(&rec.id, &rec.name); err != nil {
			t.Fatalf("scan migration record: %v", err)
		}

		records = append(records, rec)
	}

	want := []record{
		{1, "001_create_users"},
		{2, "002_create_sessions"},
		{3, "003_create_relays"},
		{4, "004_create_roles"},
		{5, "005_create_users_roles"},
		{6, "006_add_relay_private_mode"},
		{7, "007_add_relay_expire_at"},
		{8, "008_drop_relay_private"},
		{9, "009_create_kipple_files"},
	}

	if len(records) != len(want) {
		t.Fatalf("schema_migrations: got %d records, want %d", len(records), len(want))
	}

	for i, rec := range records {
		if rec.id != want[i].id || rec.name != want[i].name {
			t.Errorf("migration %d: got {%d, %q}, want {%d, %q}", i, rec.id, rec.name, want[i].id, want[i].name)
		}
	}
}

func TestOpenCore_MigrationIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	first, err := db.OpenCore(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}

	first.Close()

	second, err := db.OpenCore(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}

	defer second.Close()

	var count int
	if err := second.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}

	if count != 9 {
		t.Errorf("schema_migrations count: got %d, want 9", count)
	}
}

func TestOpenCore_WALMode(t *testing.T) {
	database, err := db.OpenCore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	defer database.Close()

	var journalMode string
	if err := database.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}

	if journalMode != "wal" {
		t.Errorf("journal_mode: got %q, want %q", journalMode, "wal")
	}
}

func TestOpenCore_ForeignKeysEnabled(t *testing.T) {
	database, err := db.OpenCore(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	defer database.Close()

	nonExistentUserID := "05f0e80e-fe1c-4ee6-b4ab-464b9dd2163e"
	_, err = database.Exec(
		"INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		"sometoken", nonExistentUserID, 1000, 2000,
	)

	if err == nil {
		t.Error("expected foreign key violation inserting session with nonexistent user_id, got nil")
	}
}

func TestOpenCore_SchemaConstraints(t *testing.T) {
	database, err := db.OpenCore(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	defer database.Close()

	newUserID := "00c3c585-e16e-44f8-9cd5-30c0ec4e1679"
	_, err = database.Exec(
		"INSERT INTO users (id, username, password_hash, updated_at, created_at) VALUES (?, ?, ?, ?, ?)",
		newUserID, "lain", "hash", 1000, 1000,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Duplicate username must fail
	duplicatedUserID := "010827a2-0d62-4351-89cd-b2c28520ba6c"
	_, err = database.Exec(
		"INSERT INTO users (id, username, password_hash, updated_at, created_at) VALUES (?, ?, ?, ?, ?)",
		duplicatedUserID, "lain", "hash2", 2000, 2000,
	)
	if err == nil {
		t.Error("expected unique constraint violation for duplicate username, got nil")
	}

	// Valid session referencing the user must succeed
	_, err = database.Exec(
		"INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		"token1", newUserID, 1000, 9999,
	)
	if err != nil {
		t.Errorf("insert valid session: %v", err)
	}

	// Valid relay referencing the user must succeed
	newRelayID := "13586f4c-7aa8-4392-9b31-77d8031406b9"
	_, err = database.Exec(
		"INSERT INTO relays (id, user_id, content, private_mode, expire_at, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		newRelayID, newUserID, []byte("compressed"), "link", 9999, 1000,
	)
	if err != nil {
		t.Errorf("insert valid relay: %v", err)
	}
}

func TestOpenWorker_Succeeds(t *testing.T) {
	database, err := db.OpenWorker(":memory:")
	if err != nil {
		t.Fatalf("OpenWorker: %v", err)
	}

	database.Close()
}

func TestOpenWorker_TablesCreated(t *testing.T) {
	database, err := db.OpenWorker(":memory:")
	if err != nil {
		t.Fatalf("OpenWorker: %v", err)
	}

	defer database.Close()

	for _, table := range []string{"worker_schema_migrations", "job_queue", "job_history", "job_graveyard"} {
		var name string

		if err := database.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name); err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestOpenWorker_MigrationsTracked(t *testing.T) {
	database, err := db.OpenWorker(":memory:")
	if err != nil {
		t.Fatalf("OpenWorker: %v", err)
	}

	defer database.Close()

	rows, err := database.Query("SELECT id, name FROM worker_schema_migrations ORDER BY id")
	if err != nil {
		t.Fatalf("query worker_schema_migrations: %v", err)
	}

	defer rows.Close()

	type record struct {
		id   int
		name string
	}

	var records []record
	for rows.Next() {
		var rec record
		if err := rows.Scan(&rec.id, &rec.name); err != nil {
			t.Fatalf("scan migration record: %v", err)
		}

		records = append(records, rec)
	}

	want := []record{
		{1, "001_create_job_queue"},
		{2, "002_create_job_history"},
		{3, "003_create_job_graveyard"},
	}

	if len(records) != len(want) {
		t.Fatalf("worker_schema_migrations: got %d records, want %d", len(records), len(want))
	}

	for i, rec := range records {
		if rec.id != want[i].id || rec.name != want[i].name {
			t.Errorf("migration %d: got {%d, %q}, want {%d, %q}", i, rec.id, rec.name, want[i].id, want[i].name)
		}
	}
}

func TestOpenWorker_MigrationIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.db")

	first, err := db.OpenWorker(path)
	if err != nil {
		t.Fatalf("first OpenWorker: %v", err)
	}

	first.Close()

	second, err := db.OpenWorker(path)
	if err != nil {
		t.Fatalf("second OpenWorker: %v", err)
	}

	defer second.Close()

	var count int
	if err := second.QueryRow("SELECT COUNT(*) FROM worker_schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count worker_schema_migrations: %v", err)
	}

	if count != 3 {
		t.Errorf("worker_schema_migrations count: got %d, want 3", count)
	}
}
