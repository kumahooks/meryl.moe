package relay_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"meryl.moe/internal/modules/relay"
	"meryl.moe/internal/platform/db"
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

func insertTestRelay(t *testing.T, database *sql.DB, userID string, createdAt int64) string {
	t.Helper()

	relayID := uuid.New().String()
	// represents "old" in bytes
	compressedPlaceholder := []byte{120, 156, 202, 207, 73, 1, 4, 0, 0, 255, 255, 2, 140, 1, 64}

	if _, err := database.Exec(
		"INSERT INTO relays (id, user_id, content, private_mode, expire_at, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		relayID, userID, compressedPlaceholder, relay.PrivateModeLink, futureExpiry().Unix(), createdAt,
	); err != nil {
		t.Fatalf("insert test relay: %v", err)
	}

	return relayID
}

func TestService_SaveAndGet_RoundTrip(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	original := "hello, wired\n\nlet's all love lain"

	relayID, err := service.Save(userID, original, relay.PrivateModeLink, futureExpiry())
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	savedRelay, err := service.Get(relayID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if savedRelay.Content != original {
		t.Errorf("round-trip content: got %q, want %q", savedRelay.Content, original)
	}
}

func TestService_SaveAndGet_Unicode(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	service := relay.NewService(database)

	original := "你好，小可爱\n<script>alert('xss')</script>\n\u0000null\naccénts & êntités"

	relayID, err := service.Save(userID, original, relay.PrivateModeLink, futureExpiry())
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	savedRelay, err := service.Get(relayID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if savedRelay.Content != original {
		t.Errorf("unicode round-trip failed: got %q, want %q", savedRelay.Content, original)
	}
}

func TestService_SaveAndGet_LargeContent(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	original := strings.Repeat("<lain wired owouwu o7>", 10_000)

	relayID, err := service.Save(userID, original, relay.PrivateModeLink, futureExpiry())
	if err != nil {
		t.Fatalf("save large content: %v", err)
	}

	savedRelay, err := service.Get(relayID)
	if err != nil {
		t.Fatalf("get large content: %v", err)
	}

	if savedRelay.Content != original {
		t.Errorf("large content round-trip failed: lengths got %d, want %d", len(savedRelay.Content), len(original))
	}
}

func TestService_Get_UnknownID_ReturnsNotFound(t *testing.T) {
	service := relay.NewService(openTestDB(t))

	_, err := service.Get("does-not-exist")
	if !errors.Is(err, relay.ErrNotFound) {
		t.Errorf("error: got %v, want ErrNotFound", err)
	}
}

func TestService_Get_SQLInjectionID_ReturnsNotFound(t *testing.T) {
	service := relay.NewService(openTestDB(t))

	maliciousIDs := []string{
		"' OR '1'='1",
		"'; DROP TABLE relays; --",
		"1 UNION SELECT content FROM relays--",
	}

	for _, maliciousID := range maliciousIDs {
		_, err := service.Get(maliciousID)
		if !errors.Is(err, relay.ErrNotFound) {
			t.Errorf("SQL injection ID %q: got %v, want ErrNotFound", maliciousID, err)
		}
	}
}

func TestService_Save_ReturnsDistinctIDs(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	firstID, err := service.Save(userID, "first", relay.PrivateModeLink, futureExpiry())
	if err != nil {
		t.Fatalf("first save: %v", err)
	}

	secondID, err := service.Save(userID, "second", relay.PrivateModeLink, futureExpiry())
	if err != nil {
		t.Fatalf("second save: %v", err)
	}

	if firstID == secondID {
		t.Errorf("expected distinct IDs, got %q for both", firstID)
	}
}

func TestService_Save_SameContentProducesDistinctIDs(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	service := relay.NewService(database)

	firstID, err := service.Save(userID, "lain", relay.PrivateModeLink, futureExpiry())
	if err != nil {
		t.Fatalf("first save: %v", err)
	}

	secondID, err := service.Save(userID, "lain", relay.PrivateModeLink, futureExpiry())
	if err != nil {
		t.Fatalf("second save: %v", err)
	}

	if firstID == secondID {
		t.Errorf("same content produced the same ID; IDs must be unique")
	}
}

func TestService_Get_ReturnsCorrectUserID(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	service := relay.NewService(database)

	relayID, err := service.Save(userID, "wired", relay.PrivateModeLink, futureExpiry())
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	savedRelay, err := service.Get(relayID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if savedRelay.UserID != userID {
		t.Errorf("user ID: got %q, want %q", savedRelay.UserID, userID)
	}
}

func TestService_Save_PrivateMode_StoresMode(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	relayID, err := service.Save(userID, "secret", relay.PrivateModeUser, futureExpiry())
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	savedRelay, err := service.Get(relayID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if savedRelay.PrivateMode != relay.PrivateModeUser {
		t.Errorf("private mode: got %q, want %q", savedRelay.PrivateMode, relay.PrivateModeUser)
	}
}

func TestService_Save_PublicMode_StoresMode(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	service := relay.NewService(database)

	relayID, err := service.Save(userID, "public content", relay.PrivateModeLink, futureExpiry())
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	savedRelay, err := service.Get(relayID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if savedRelay.PrivateMode != relay.PrivateModeLink {
		t.Errorf("private mode: got %q, want %q", savedRelay.PrivateMode, relay.PrivateModeLink)
	}
}

func TestService_Save_ExpiresAt_IsStoredAndRetrieved(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)
	service := relay.NewService(database)

	expiresAt := time.Now().Add(7 * 24 * time.Hour).Truncate(time.Second)

	relayID, err := service.Save(userID, "text", relay.PrivateModeLink, expiresAt)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	savedRelay, err := service.Get(relayID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if !savedRelay.ExpiresAt.Equal(expiresAt) {
		t.Errorf("expires at: got %v, want %v", savedRelay.ExpiresAt, expiresAt)
	}
}

func TestService_List_Empty_ReturnsNil(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	items, err := service.List(userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if items != nil {
		t.Errorf("expected nil for user with no relays, got %v", items)
	}
}

func TestService_List_ReturnsNewestFirst(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	oldRelayID := insertTestRelay(t, database, userID, 1000)
	newRelayID, err := service.Save(userID, "new", relay.PrivateModeLink, futureExpiry())
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	items, err := service.List(userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("list length: got %d, want 2", len(items))
	}

	if items[0].ID != newRelayID {
		t.Errorf("first item: got %q, want %q (newest)", items[0].ID, newRelayID)
	}

	if items[1].ID != oldRelayID {
		t.Errorf("second item: got %q, want %q (oldest)", items[1].ID, oldRelayID)
	}
}

func TestService_List_PreviewTruncatedTo15Runes(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	if _, err := service.Save(userID, "lainlainlainlainlainlain", relay.PrivateModeLink, futureExpiry()); err != nil {
		t.Fatalf("save: %v", err)
	}

	items, err := service.List(userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if items[0].Preview != "lainlainlainlai..." {
		t.Errorf("preview: got %q, want %q", items[0].Preview, "lainlainlainlai...")
	}
}

func TestService_List_PreviewShortContentUnchanged(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	if _, err := service.Save(userID, "owo?", relay.PrivateModeLink, futureExpiry()); err != nil {
		t.Fatalf("save: %v", err)
	}

	items, err := service.List(userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if items[0].Preview != "owo?" {
		t.Errorf("preview: got %q, want %q", items[0].Preview, "owo?")
	}
}

func TestService_List_PreviewUnicodeTruncatesOnRuneBoundary(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	// Each hanzi is one rune but multiple bytes
	content := "你你你你lainlainlainlainlain"
	if _, err := service.Save(userID, content, relay.PrivateModeLink, futureExpiry()); err != nil {
		t.Fatalf("save: %v", err)
	}

	items, err := service.List(userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := "你你你你lainlainlai..."
	if items[0].Preview != want {
		t.Errorf("preview: got %q, want %q", items[0].Preview, want)
	}
}

func TestService_List_PrivateModeIsReturned(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	if _, err := service.Save(userID, "secret", relay.PrivateModeUser, futureExpiry()); err != nil {
		t.Fatalf("save: %v", err)
	}

	items, err := service.List(userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if items[0].PrivateMode != relay.PrivateModeUser {
		t.Errorf("private mode: got %q, want %q", items[0].PrivateMode, relay.PrivateModeUser)
	}
}

func TestService_List_DateFormattedAsYYYYMMDD(t *testing.T) {
	database := openTestDB(t)
	userID := insertTestUser(t, database)

	service := relay.NewService(database)

	if _, err := service.Save(userID, "dated", relay.PrivateModeLink, futureExpiry()); err != nil {
		t.Fatalf("save: %v", err)
	}

	items, err := service.List(userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := time.Now().Format("2006-01-02")
	if items[0].Date != want {
		t.Errorf("date: got %q, want %q", items[0].Date, want)
	}
}

func TestService_List_OnlyReturnsOwnRelays(t *testing.T) {
	database := openTestDB(t)
	firstUserID := insertTestUser(t, database)
	secondUserID := insertTestUser(t, database)

	service := relay.NewService(database)

	if _, err := service.Save(firstUserID, "mine", relay.PrivateModeLink, futureExpiry()); err != nil {
		t.Fatalf("save own relay: %v", err)
	}

	if _, err := service.Save(secondUserID, "theirs", relay.PrivateModeLink, futureExpiry()); err != nil {
		t.Fatalf("save other relay: %v", err)
	}

	items, err := service.List(firstUserID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("list length: got %d, want 1", len(items))
	}

	if items[0].Preview != "mine" {
		t.Errorf("preview: got %q, want %q", items[0].Preview, "mine")
	}
}
