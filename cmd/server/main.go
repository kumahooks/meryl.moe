// Package main is the entry point for the web server application.
// It initializes the server, configures routes, and starts listening for HTTP requests.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"meryl.moe/internal"
	"meryl.moe/internal/config"
	"meryl.moe/internal/platform/db"
)

func main() {
	configuration, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	if err := os.MkdirAll(filepath.Dir(configuration.DB.Path), 0o755); err != nil {
		log.Fatalf("create database directory: %v", err)
	}

	database, err := db.Open(configuration.DB.Path)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	defer database.Close()

	server := internal.NewServer(configuration, database)

	if err := server.Initialize(); err != nil {
		log.Fatal("Failed to initialize server:", err)
	}

	addr := fmt.Sprintf("%s:%d", configuration.Server.Host, configuration.Server.Port)
	if err := server.Start(addr); err != nil {
		log.Fatal("Server failed:", err)
	}
}
