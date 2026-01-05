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

	"amanah/services/account/handlers"
	"amanah/services/account/repositories"
	"amanah/services/account/services"
)

func main() {
	// Initialize repository
	repo := repositories.NewInMemoryRepository()

	// Initialize service
	accountService := services.NewAccountService(repo)

	// Initialize handler
	handler := handlers.NewAccountHandler(accountService)

	// Setup routes
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"healthy","service":"account"}`)
	})

	// Register account routes
	handler.RegisterRoutes(mux)

	// Start server
	addr := ":8082"
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
		log.Printf("Account service starting on %s", addr)
		log.Printf("Endpoints:")
		log.Printf("  POST   /accounts                    - Create account")
		log.Printf("  GET    /accounts                    - List accounts")
		log.Printf("  GET    /accounts/{id}               - Get account")
		log.Printf("  PUT    /accounts/{id}               - Update account")
		log.Printf("  POST   /accounts/{id}/activate      - Activate account")
		log.Printf("  POST   /accounts/{id}/freeze        - Freeze account")
		log.Printf("  POST   /accounts/{id}/unfreeze      - Unfreeze account")
		log.Printf("  POST   /accounts/{id}/close         - Close account")
		log.Printf("  POST   /accounts/{id}/kyc           - Update KYC status")
		log.Printf("  POST   /accounts/{id}/payment-instruments - Add payment method")
		log.Printf("  DELETE /accounts/{id}/payment-instruments/{pid} - Remove payment method")
		log.Printf("  POST   /accounts/{id}/credit        - Credit balance")
		log.Printf("  POST   /accounts/{id}/debit         - Debit balance")
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
