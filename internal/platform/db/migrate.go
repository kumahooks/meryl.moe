package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"meryl.moe/internal/platform/db/migrations"
)

func migrate(database *sql.DB) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id         INTEGER PRIMARY KEY,
			name       TEXT    NOT NULL,
			applied_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied := 0

	for _, m := range migrations.All() {
		var count int
		if err := database.QueryRow(
			"SELECT COUNT(*) FROM schema_migrations WHERE id = ?", m.ID,
		).Scan(&count); err != nil {
			return fmt.Errorf("check migration %d: %w", m.ID, err)
		}

		if count > 0 {
			continue
		}

		transaction, err := database.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.ID, err)
		}

		if _, err := transaction.Exec(m.SQL); err != nil {
			transaction.Rollback()
			return fmt.Errorf("run migration %d: %w", m.ID, err)
		}

		if _, err := transaction.Exec(
			"INSERT INTO schema_migrations (id, name, applied_at) VALUES (?, ?, ?)",
			m.ID, m.Name, time.Now().Unix(),
		); err != nil {
			transaction.Rollback()
			return fmt.Errorf("record migration %d: %w", m.ID, err)
		}

		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.ID, err)
		}

		log.Printf("database: applied migration %d (%s)", m.ID, m.Name)
		applied++
	}

	if applied == 0 {
		log.Printf("database: migrations up to date")
	} else {
		log.Printf("database: %d migration(s) applied", applied)
	}

	return nil
}
