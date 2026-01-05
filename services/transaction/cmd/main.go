package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"amanah/services/transaction/handlers"
	"amanah/services/transaction/repositories"
	"amanah/services/transaction/services"
)

func main() {
	// Initialize repository
	repo := repositories.NewInMemoryRepository()

	// Initialize service
	txService := services.NewTransactionService(repo)

	// Initialize handler
	handler := handlers.NewTransactionHandler(txService)

	// Setup routes
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"healthy","service":"transaction"}`)
	})

	// Register transaction routes
	handler.RegisterRoutes(mux)

	// Start server
	addr := ":8081"
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to signal server errors
	serverErrors := make(chan error, 1)

	// Start server in goroutine
	go func() {
		log.Printf("Transaction service starting on %s", addr)
		log.Printf("Endpoints:")
		log.Printf("  POST   /transactions          - Create transaction")
		log.Printf("  GET    /transactions          - List transactions")
		log.Printf("  GET    /transactions/{id}     - Get transaction")
		log.Printf("  POST   /transactions/{id}/authorize - Authorize")
		log.Printf("  POST   /transactions/{id}/complete  - Complete")
		log.Printf("  POST   /transactions/{id}/cancel    - Cancel")
		log.Printf("  POST   /transactions/{id}/refund    - Refund")
		serverErrors <- server.ListenAndServe()
	}()

	// Channel to listen for shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or server error
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("received signal %v, initiating graceful shutdown", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v, forcing close", err)
			if closeErr := server.Close(); closeErr != nil {
				log.Fatalf("forced close failed: %v", closeErr)
			}
		}

		log.Println("server shutdown complete")
	}
}
