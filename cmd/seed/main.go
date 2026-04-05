// Package main seeds the development database with initial roles and users.
// run with: go run ./cmd/seed
// run against a specific database: go run ./cmd/seed -db ./data/meryl.db
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"meryl.moe/internal/platform/db"
)

func main() {
	dbPath := flag.String("db", "./data/meryl.db", "path to SQLite database")
	skipWipe := flag.Bool("skip-wipe", false, "skip wiping the database before seeding")

	flag.Parse()

	if !*skipWipe {
		if err := os.Remove(*dbPath); err != nil && !os.IsNotExist(err) {
			log.Fatalf("remove database: %v", err)
		}

		log.Printf("seed: wiped %s", *dbPath)
	}

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		log.Fatalf("create database directory: %v", err)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	defer database.Close()

	if err := seed(database); err != nil {
		log.Fatalf("seed: %v", err)
	}

	log.Printf("seed: done")
}
