package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thenameisvicky/tokenly-gateway/internal/config"
	"github.com/thenameisvicky/tokenly-gateway/internal/server"
)

func main() {
	config := config.Load()

	srv := server.New(config)

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Gateway listening on %s", config.Address)
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Gateway start failed: %v", err)
		}
	}()

	sig := <-shutdownSig
	log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Close(ctx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
	} else {
		log.Println("Gateway shutdown completed gracefully")
	}
}