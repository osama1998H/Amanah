package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"amanah/services/transaction/models"
	"amanah/services/transaction/repositories"
	"amanah/services/transaction/services"
)

// TransactionHandler handles HTTP requests for transactions
type TransactionHandler struct {
	service *services.TransactionService
}

// NewTransactionHandler creates a new handler
func NewTransactionHandler(service *services.TransactionService) *TransactionHandler {
	return &TransactionHandler{service: service}
}

// RegisterRoutes registers all transaction routes
func (h *TransactionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/transactions", h.handleTransactions)
	mux.HandleFunc("/transactions/", h.handleTransaction)
}

// handleTransactions handles POST /transactions and GET /transactions
func (h *TransactionHandler) handleTransactions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createTransaction(w, r)
	case http.MethodGet:
		h.listTransactions(w, r)
	default:
		h.methodNotAllowed(w)
	}
}

// handleTransaction handles requests to /transactions/{id}
func (h *TransactionHandler) handleTransaction(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/transactions/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		h.badRequest(w, "transaction ID required")
		return
	}

	id := parts[0]

	// Check for action
	if len(parts) > 1 {
		switch parts[1] {
		case "cancel":
			h.cancelTransaction(w, r, id)
		case "refund":
			h.refundTransaction(w, r, id)
		case "authorize":
			h.authorizeTransaction(w, r, id)
		case "complete":
			h.completeTransaction(w, r, id)
		default:
			h.notFound(w)
		}
		return
	}

	// Handle GET /transactions/{id}
	if r.Method == http.MethodGet {
		h.getTransaction(w, r, id)
		return
	}

	h.methodNotAllowed(w)
}

// createTransaction handles POST /transactions
func (h *TransactionHandler) createTransaction(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.badRequest(w, "invalid request body")
		return
	}

	tx, err := h.service.CreateTransaction(&req)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusCreated, models.TransactionResponse{
		Success:     true,
		Transaction: tx,
	})
}

// getTransaction handles GET /transactions/{id}
func (h *TransactionHandler) getTransaction(w http.ResponseWriter, r *http.Request, id string) {
	tx, err := h.service.GetTransaction(id)
	if err != nil {
		if err == repositories.ErrTransactionNotFound {
			h.notFound(w)
			return
		}
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, models.TransactionResponse{
		Success:     true,
		Transaction: tx,
	})
}

// listTransactions handles GET /transactions
func (h *TransactionHandler) listTransactions(w http.ResponseWriter, r *http.Request) {
	merchantID := r.URL.Query().Get("merchant_id")

	// Parse and validate limit
	limit := 20 // default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			h.badRequest(w, "invalid limit parameter")
			return
		}
		if parsedLimit < 0 {
			h.badRequest(w, "limit cannot be negative")
			return
		}
		if parsedLimit > 0 {
			limit = parsedLimit
		}
		// Cap limit to prevent abuse
		if limit > 100 {
			limit = 100
		}
	}

	// Parse and validate offset
	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil {
			h.badRequest(w, "invalid offset parameter")
			return
		}
		if parsedOffset < 0 {
			h.badRequest(w, "offset cannot be negative")
			return
		}
		offset = parsedOffset
	}

	txs, total, err := h.service.ListTransactions(merchantID, limit, offset)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, models.TransactionListResponse{
		Success:      true,
		Transactions: txs,
		Total:        total,
	})
}

// cancelTransaction handles POST /transactions/{id}/cancel
func (h *TransactionHandler) cancelTransaction(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	tx, err := h.service.CancelTransaction(id, body.Reason)
	if err != nil {
		if err == repositories.ErrTransactionNotFound {
			h.notFound(w)
			return
		}
		h.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, models.TransactionResponse{
		Success:     true,
		Transaction: tx,
	})
}

// refundTransaction handles POST /transactions/{id}/refund
func (h *TransactionHandler) refundTransaction(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	var req models.RefundRequest
	json.NewDecoder(r.Body).Decode(&req)

	tx, err := h.service.RefundTransaction(id, req.Amount, req.Reason)
	if err != nil {
		if err == repositories.ErrTransactionNotFound {
			h.notFound(w)
			return
		}
		h.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, models.TransactionResponse{
		Success:     true,
		Transaction: tx,
	})
}

// authorizeTransaction handles POST /transactions/{id}/authorize
func (h *TransactionHandler) authorizeTransaction(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	var body struct {
		ProviderRef string `json:"provider_ref"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	tx, err := h.service.AuthorizeTransaction(id, body.ProviderRef)
	if err != nil {
		if err == repositories.ErrTransactionNotFound {
			h.notFound(w)
			return
		}
		h.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, models.TransactionResponse{
		Success:     true,
		Transaction: tx,
	})
}

// completeTransaction handles POST /transactions/{id}/complete
func (h *TransactionHandler) completeTransaction(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	tx, err := h.service.CompleteTransaction(id)
	if err != nil {
		if err == repositories.ErrTransactionNotFound {
			h.notFound(w)
			return
		}
		h.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, models.TransactionResponse{
		Success:     true,
		Transaction: tx,
	})
}

// Helper methods

func (h *TransactionHandler) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *TransactionHandler) errorResponse(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, models.TransactionResponse{
		Success: false,
		Error:   message,
	})
}

func (h *TransactionHandler) badRequest(w http.ResponseWriter, message string) {
	h.errorResponse(w, http.StatusBadRequest, message)
}

func (h *TransactionHandler) notFound(w http.ResponseWriter) {
	h.errorResponse(w, http.StatusNotFound, "transaction not found")
}

func (h *TransactionHandler) methodNotAllowed(w http.ResponseWriter) {
	h.errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
}
