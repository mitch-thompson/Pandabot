package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"PandaBot/internal/gui"
	"PandaBot/internal/server"
)

func main() {
	config := server.DefaultConfig()
	srv := server.NewServer(config)

	go func() {
		err := srv.Start()
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	g := gui.NewGUI(srv)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down server...")
		err := srv.Stop()
		if err != nil {
			log.Printf("Error stopping server: %v", err)
		}
		os.Exit(0)
	}()

	log.Println("PandaBot server started. GUI launching...")
	g.Show()
}
