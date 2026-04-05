package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var users = []struct {
	username string
	password string
	role     string
}{
	{"lain", "lain", "god"},
	{"human", "human", "human"},
}

func seedUsers(transaction *sql.Tx) error {
	now := time.Now().Unix()

	for _, user := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte(user.password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password for %q: %w", user.username, err)
		}

		userID := uuid.New().String()
		result, err := transaction.Exec(
			"INSERT OR IGNORE INTO users (id, username, password_hash, updated_at, created_at) VALUES (?, ?, ?, ?, ?)",
			userID, user.username, string(hash), now, now,
		)
		if err != nil {
			return fmt.Errorf("insert user %q: %w", user.username, err)
		}

		if affected, _ := result.RowsAffected(); affected == 0 {
			if err := transaction.QueryRow(
				"SELECT id FROM users WHERE username = ?", user.username,
			).Scan(&userID); err != nil {
				return fmt.Errorf("query existing user %q: %w", user.username, err)
			}

			log.Printf("seed: user %q already exists, skipping", user.username)
		} else {
			log.Printf("seed: user %q created (password: %q)", user.username, user.password)
		}

		var roleID int
		if err := transaction.QueryRow(
			"SELECT id FROM roles WHERE name = ?", user.role,
		).Scan(&roleID); err != nil {
			return fmt.Errorf("query role %q: %w", user.role, err)
		}

		if _, err := transaction.Exec(
			"INSERT OR IGNORE INTO users_roles (user_id, role_id) VALUES (?, ?)",
			userID, roleID,
		); err != nil {
			return fmt.Errorf("assign role %q to user %q: %w", user.role, user.username, err)
		}
	}

	return nil
}
