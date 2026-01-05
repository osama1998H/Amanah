package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"amanah/services/ledger/models"
	"amanah/services/ledger/services"
)

func main() {
	// Initialize service
	ledgerService := services.NewLedgerService()

	// Setup routes
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"healthy","service":"ledger"}`)
	})

	// Accounts
	mux.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			accounts := ledgerService.ListAccounts()
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"success":  true,
				"accounts": accounts,
			})
		case http.MethodPost:
			var req struct {
				Name     string             `json:"name"`
				Type     models.AccountType `json:"type"`
				Currency string             `json:"currency"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			account, err := ledgerService.CreateAccount(req.Name, req.Type, req.Currency)
			if err != nil {
				jsonError(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusCreated, models.AccountResponse{
				Success: true,
				Account: account,
			})
		default:
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	// Account by ID
	mux.HandleFunc("/accounts/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/accounts/")
		parts := strings.Split(id, "/")
		accountID := parts[0]

		if len(parts) > 1 && parts[1] == "entries" {
			// GET /accounts/{id}/entries
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			if limit == 0 {
				limit = 20
			}
			entries, total := ledgerService.GetEntriesForAccount(accountID, limit, offset)
			jsonResponse(w, http.StatusOK, models.EntriesResponse{
				Success: true,
				Entries: entries,
				Total:   total,
			})
			return
		}

		account, err := ledgerService.GetAccount(accountID)
		if err != nil {
			jsonError(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, models.AccountResponse{
			Success: true,
			Account: account,
		})
	})

	// Journal entries
	mux.HandleFunc("/journals", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req models.CreateJournalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		journal, err := ledgerService.CreateJournalEntry(&req)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}

		jsonResponse(w, http.StatusCreated, models.LedgerResponse{
			Success: true,
			Journal: journal,
		})
	})

	// Get journal by ID
	mux.HandleFunc("/journals/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/journals/")
		journal, err := ledgerService.GetJournal(id)
		if err != nil {
			jsonError(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, models.LedgerResponse{
			Success: true,
			Journal: journal,
		})
	})

	// Trial balance
	mux.HandleFunc("/trial-balance", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		balances := ledgerService.GetTrialBalance()
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success":  true,
			"balances": balances,
		})
	})

	// Audit logs
	mux.HandleFunc("/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		entityType := r.URL.Query().Get("entity_type")
		entityID := r.URL.Query().Get("entity_id")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit == 0 {
			limit = 50
		}

		logs, total := ledgerService.GetAuditLogs(entityType, entityID, limit, offset)
		jsonResponse(w, http.StatusOK, models.AuditLogsResponse{
			Success: true,
			Logs:    logs,
			Total:   total,
		})
	})

	// Convenience endpoints for common operations
	mux.HandleFunc("/record-payment", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			TransactionID     string `json:"transaction_id"`
			Amount            int64  `json:"amount"`
			MerchantAccountID string `json:"merchant_account_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		journal, err := ledgerService.RecordPayment(req.TransactionID, req.Amount, req.MerchantAccountID)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}

		jsonResponse(w, http.StatusCreated, models.LedgerResponse{
			Success: true,
			Journal: journal,
		})
	})

	mux.HandleFunc("/record-refund", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			TransactionID string `json:"transaction_id"`
			Amount        int64  `json:"amount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		journal, err := ledgerService.RecordRefund(req.TransactionID, req.Amount)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}

		jsonResponse(w, http.StatusCreated, models.LedgerResponse{
			Success: true,
			Journal: journal,
		})
	})

	// Start server
	addr := ":8084"
	log.Printf("Ledger service starting on %s", addr)
	log.Printf("Endpoints:")
	log.Printf("  GET    /accounts         - List accounts")
	log.Printf("  POST   /accounts         - Create account")
	log.Printf("  GET    /accounts/{id}    - Get account")
	log.Printf("  GET    /accounts/{id}/entries - Get account entries")
	log.Printf("  POST   /journals         - Create journal entry")
	log.Printf("  GET    /journals/{id}    - Get journal")
	log.Printf("  GET    /trial-balance    - Get trial balance")
	log.Printf("  GET    /audit-logs       - Get audit logs")
	log.Printf("  POST   /record-payment   - Record payment")
	log.Printf("  POST   /record-refund    - Record refund")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
