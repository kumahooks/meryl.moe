// Package db manages the SQLite database connection and schema migrations.
package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite database at path, applies configuration
// pragmas (WAL mode, foreign keys), and runs schema migrations.
func Open(path string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// TODO: validate if this is fine
	// tests were failing because multiple connections were being opened,
	// and FK wasn't being enforced in some of them. This fixes it, whcih ensures
	// only one valid connection with all the PRAGMAs are set, but still need to
	// make sure this is fine
	database.SetMaxOpenConns(1)

	log.Printf("database: opened %s", path)

	if err := configure(database); err != nil {
		database.Close()
		return nil, err
	}

	log.Printf("database: configured (WAL, foreign keys)")

	if err := migrate(database); err != nil {
		database.Close()
		return nil, err
	}

	return database, nil
}

// configure applies session-level PRAGMAs that must be set on every connection.
//
// journal_mode=WAL: switches from the default rollback journal to write-ahead-log.
// In default mode a write locks the entire file, blocking all readers.
// In WAL mode readers do not block writers and a writer does not block readers.
// ref: https://sqlite.org/wal.html
//
// foreign_keys=ON: SQLite does not enforce foreign key constraints by default.
// ref: https://sqlite.org/foreignkeys.html
func configure(database *sql.DB) error {
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := database.Exec(pragma); err != nil {
			return fmt.Errorf("configure: %s: %w", pragma, err)
		}
	}

	return nil
}
