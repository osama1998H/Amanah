package main

import (
	"fmt"
	"log"
	"net/http"

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

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
