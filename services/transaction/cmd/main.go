package main

import (
	"fmt"
	"log"
	"net/http"

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
	log.Printf("Transaction service starting on %s", addr)
	log.Printf("Endpoints:")
	log.Printf("  POST   /transactions          - Create transaction")
	log.Printf("  GET    /transactions          - List transactions")
	log.Printf("  GET    /transactions/{id}     - Get transaction")
	log.Printf("  POST   /transactions/{id}/authorize - Authorize")
	log.Printf("  POST   /transactions/{id}/complete  - Complete")
	log.Printf("  POST   /transactions/{id}/cancel    - Cancel")
	log.Printf("  POST   /transactions/{id}/refund    - Refund")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
