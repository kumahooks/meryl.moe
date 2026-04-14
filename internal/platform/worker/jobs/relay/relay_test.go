package relay_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"meryl.moe/internal/platform/db"
	relaywork "meryl.moe/internal/platform/worker/jobs/relay"
)

const (
	testUserID     = "a2ccf831-0d18-4d77-b153-18cdc2334586"
	expiredRelayID = "13586f4c-7aa8-4392-9b31-77d8031406b9"
	activeRelayID  = "9f8e7d6c-5b4a-3210-fedc-ba9876543210"
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

func insertRelay(t *testing.T, database *sql.DB, relayID string, expireAt int64) {
	t.Helper()

	now := time.Now().Unix()

	if _, err := database.Exec(
		"INSERT INTO relays (id, user_id, content, private_mode, expire_at, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		relayID, testUserID, []byte("content"), "link", expireAt, now,
	); err != nil {
		t.Fatalf("insert relay: %v", err)
	}
}

func countRelays(t *testing.T, database *sql.DB, relayID string) int {
	t.Helper()

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM relays WHERE id = ?", relayID).Scan(&count); err != nil {
		t.Fatalf("count relays: %v", err)
	}

	return count
}

func TestCleanup_DeletesExpiredRelays(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)
	insertRelay(t, database, expiredRelayID, time.Now().Add(-time.Hour).Unix())

	if err := relaywork.Cleanup(context.Background(), database); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if count := countRelays(t, database, expiredRelayID); count != 0 {
		t.Error("expired relay was not deleted")
	}
}

func TestCleanup_PreservesActiveRelays(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)
	insertRelay(t, database, activeRelayID, time.Now().Add(time.Hour).Unix())

	if err := relaywork.Cleanup(context.Background(), database); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if count := countRelays(t, database, activeRelayID); count != 1 {
		t.Error("active relay was deleted by cleanup")
	}
}

func TestCleanup_OnlyDeletesExpired_WhenMixed(t *testing.T) {
	database := openTestDB(t)
	insertTestUser(t, database)
	insertRelay(t, database, expiredRelayID, time.Now().Add(-time.Hour).Unix())
	insertRelay(t, database, activeRelayID, time.Now().Add(time.Hour).Unix())

	if err := relaywork.Cleanup(context.Background(), database); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if count := countRelays(t, database, expiredRelayID); count != 0 {
		t.Error("expired relay was not deleted")
	}

	if count := countRelays(t, database, activeRelayID); count != 1 {
		t.Error("active relay was incorrectly deleted")
	}
}

func TestCleanup_EmptyTable_Succeeds(t *testing.T) {
	database := openTestDB(t)

	if err := relaywork.Cleanup(context.Background(), database); err != nil {
		t.Errorf("Cleanup on empty table: %v", err)
	}
}
