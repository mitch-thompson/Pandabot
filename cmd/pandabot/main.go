package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"PandaBot/internal/server"
)

func main() {
	// Create server configuration
	config := server.DefaultConfig()
	config.Port = 31337

	// Create and start the server
	srv := server.NewServer(config)
	
	err := srv.Start()
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Wait for interrupt signal to gracefully shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Println("PandaBot server started. Press Ctrl+C to stop.")
	<-sigChan

	log.Println("Shutting down server...")
	err = srv.Stop()
	if err != nil {
		log.Printf("Error stopping server: %v", err)
	}
}