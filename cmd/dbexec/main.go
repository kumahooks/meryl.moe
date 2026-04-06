// Command dbexec executes a one-off SQL operation against the SQLite database.
// Write the target SQL directly in the query constant below, then use
// deploy/dbexec.sh to build, ship, run, and delete this binary on the server.
package main

import (
	"database/sql"
	"flag"
	"log"

	_ "modernc.org/sqlite"
)

const query = ``

func main() {
	dbPath := flag.String("db", "", "path to SQLite database file (required)")
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("flag -db is required")
	}

	database, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	defer database.Close()

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err = database.Exec(pragma); err != nil {
			log.Fatalf("configure pragma: %s: %v", pragma, err)
		}
	}

	transaction, err := database.Begin()
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}

	result, err := transaction.Exec(query)
	if err != nil {
		transaction.Rollback()
		log.Fatalf("execute: %v", err)
	}

	if err := transaction.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("done: %d row(s) affected", rowsAffected)
}
