package main

import (
	"database/sql"
	"fmt"
	"log"
)

var roles = []struct {
	name        string
	permissions int
}{
	{"god", 0},
	{"human", 0},
}

func seedRoles(transaction *sql.Tx) error {
	for _, role := range roles {
		if _, err := transaction.Exec(
			"INSERT OR IGNORE INTO roles (name, permissions) VALUES (?, ?)",
			role.name, role.permissions,
		); err != nil {
			return fmt.Errorf("insert role %q: %w", role.name, err)
		}

		log.Printf("seed: role %q ready", role.name)
	}

	return nil
}
