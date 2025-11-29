package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mot0x0/goth-api/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config: ", err)
	}

	server, err := InitializeApp()
	if err != nil {
		log.Fatal("Failed to initialize app: ", err)
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.ServerPort)
		if err := server.Run(cfg.ServerPort); err != nil {
			log.Fatal("Server failed: ", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down...")
}
