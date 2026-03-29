// Package main is the entry point for the web server application.
// It initializes the server, configures routes, and starts listening for HTTP requests.
package main

import (
	fmt "fmt"
	log "log"

	internal "meryl.moe/internal"
	config "meryl.moe/internal/config"
)

func main() {
	configuration, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	server := internal.NewServer(configuration)

	if err := server.Initialize(); err != nil {
		log.Fatal("Failed to initialize server:", err)
	}

	addr := fmt.Sprintf("%s:%d", configuration.Server.Host, configuration.Server.Port)
	if err := server.Start(addr); err != nil {
		log.Fatal("Server failed:", err)
	}
}
