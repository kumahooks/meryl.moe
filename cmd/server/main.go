// Package main is the entry point for meryl.moe.
// Loads config, opens the database, wires dependencies, and starts the HTTP server.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"meryl.moe/internal"
	"meryl.moe/internal/config"
	"meryl.moe/internal/platform/db"
	"meryl.moe/internal/platform/dispatch"
	"meryl.moe/internal/platform/worker"
)

func main() {
	configuration, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	if err = os.MkdirAll(filepath.Dir(configuration.DB.CorePath), 0o755); err != nil {
		log.Fatalf("create database directory: %v", err)
	}

	if err = os.MkdirAll(filepath.Dir(configuration.DB.WorkerPath), 0o755); err != nil {
		log.Fatalf("create worker database directory: %v", err)
	}

	coreDatabase, err := db.OpenCore(configuration.DB.CorePath)
	if err != nil {
		log.Fatalf("open core database: %v", err)
	}

	defer coreDatabase.Close()

	workerDatabase, err := db.OpenWorker(configuration.DB.WorkerPath)
	if err != nil {
		log.Fatalf("open worker database: %v", err)
	}

	defer workerDatabase.Close()

	runner := worker.NewRegistrar(coreDatabase, workerDatabase, configuration.Kipple.Dir).JobRunner()
	server := internal.NewServer(configuration, coreDatabase, dispatch.New(runner))

	if err := server.Initialize(); err != nil {
		log.Fatal("Failed to initialize server:", err)
	}

	addr := fmt.Sprintf("%s:%d", configuration.Server.Host, configuration.Server.Port)
	if err := server.Start(addr, runner, workerDatabase); err != nil {
		log.Fatal("Server failed:", err)
	}
}
