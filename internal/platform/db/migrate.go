package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"meryl.moe/internal/platform/db/migrations"
	coreMigrations "meryl.moe/internal/platform/db/migrations/core"
	workerMigrations "meryl.moe/internal/platform/db/migrations/worker"
)

func migrateCore(database *sql.DB) error {
	return runMigrations(database, "core", "schema_migrations", coreMigrations.All())
}

func migrateWorker(database *sql.DB) error {
	return runMigrations(database, "worker", "worker_schema_migrations", workerMigrations.All())
}

func runMigrations(database *sql.DB, label string, trackingTable string, migs []migrations.Migration) error {
	_, err := database.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id         INTEGER PRIMARY KEY,
			name       TEXT    NOT NULL,
			applied_at INTEGER NOT NULL
		)
	`, trackingTable))
	if err != nil {
		return fmt.Errorf("create %s: %w", trackingTable, err)
	}

	applied := 0

	for _, m := range migs {
		var count int
		if err := database.QueryRow(
			fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", trackingTable), m.ID,
		).Scan(&count); err != nil {
			return fmt.Errorf("%s: check migration %d: %w", label, m.ID, err)
		}

		if count > 0 {
			continue
		}

		transaction, err := database.Begin()
		if err != nil {
			return fmt.Errorf("%s: begin migration %d: %w", label, m.ID, err)
		}

		if _, err := transaction.Exec(m.SQL); err != nil {
			transaction.Rollback()
			return fmt.Errorf("%s: run migration %d: %w", label, m.ID, err)
		}

		if _, err := transaction.Exec(
			fmt.Sprintf("INSERT INTO %s (id, name, applied_at) VALUES (?, ?, ?)", trackingTable),
			m.ID, m.Name, time.Now().Unix(),
		); err != nil {
			transaction.Rollback()
			return fmt.Errorf("%s: record migration %d: %w", label, m.ID, err)
		}

		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("%s: commit migration %d: %w", label, m.ID, err)
		}

		log.Printf("database: %s: applied migration %d (%s)", label, m.ID, m.Name)
		applied++
	}

	if applied == 0 {
		log.Printf("database: %s: migrations up to date", label)
	} else {
		log.Printf("database: %s: %d migration(s) applied", label, applied)
	}

	return nil
}
