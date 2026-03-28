package main

import (
	log "log"

	app "meryl.moe/internal"
)

func main() {
	server := app.NewServer()

	if err := server.SetupRoutes(); err != nil {
		log.Fatal(err)
	}

	if err := server.Start(":3000"); err != nil {
		log.Fatal(err)
	}
}
