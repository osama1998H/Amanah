package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"amanah/services/account/models"
	"amanah/services/account/repositories"
	"amanah/services/account/services"
)

// AccountHandler handles HTTP requests for accounts
type AccountHandler struct {
	service *services.AccountService
}

// NewAccountHandler creates a new handler
func NewAccountHandler(service *services.AccountService) *AccountHandler {
	return &AccountHandler{service: service}
}

// RegisterRoutes registers all account routes
func (h *AccountHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/accounts", h.handleAccounts)
	mux.HandleFunc("/accounts/", h.handleAccount)
}

// handleAccounts handles POST /accounts and GET /accounts
func (h *AccountHandler) handleAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createAccount(w, r)
	case http.MethodGet:
		h.listAccounts(w, r)
	default:
		h.methodNotAllowed(w)
	}
}

// handleAccount handles requests to /accounts/{id}
func (h *AccountHandler) handleAccount(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/accounts/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		h.badRequest(w, "account ID required")
		return
	}

	id := parts[0]

	// Check for actions
	if len(parts) > 1 {
		switch parts[1] {
		case "activate":
			h.activateAccount(w, r, id)
		case "freeze":
			h.freezeAccount(w, r, id)
		case "unfreeze":
			h.unfreezeAccount(w, r, id)
		case "close":
			h.closeAccount(w, r, id)
		case "kyc":
			h.updateKYC(w, r, id)
		case "payment-instruments":
			if len(parts) > 2 {
				h.removePaymentInstrument(w, r, id, parts[2])
			} else {
				h.addPaymentInstrument(w, r, id)
			}
		case "credit":
			h.creditBalance(w, r, id)
		case "debit":
			h.debitBalance(w, r, id)
		default:
			h.notFound(w)
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getAccount(w, r, id)
	case http.MethodPut, http.MethodPatch:
		h.updateAccount(w, r, id)
	default:
		h.methodNotAllowed(w)
	}
}

// createAccount handles POST /accounts
func (h *AccountHandler) createAccount(w http.ResponseWriter, r *http.Request) {
	var req models.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.badRequest(w, "invalid request body")
		return
	}

	account, err := h.service.CreateAccount(&req)
	if err != nil {
		status := http.StatusBadRequest
		if err == repositories.ErrEmailAlreadyExists {
			status = http.StatusConflict
		}
		h.errorResponse(w, status, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusCreated, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// getAccount handles GET /accounts/{id}
func (h *AccountHandler) getAccount(w http.ResponseWriter, r *http.Request, id string) {
	account, err := h.service.GetAccount(id)
	if err != nil {
		if err == repositories.ErrAccountNotFound {
			h.notFound(w)
			return
		}
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// updateAccount handles PUT /accounts/{id}
func (h *AccountHandler) updateAccount(w http.ResponseWriter, r *http.Request, id string) {
	var req models.UpdateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.badRequest(w, "invalid request body")
		return
	}

	account, err := h.service.UpdateAccount(id, &req)
	if err != nil {
		if err == repositories.ErrAccountNotFound {
			h.notFound(w)
			return
		}
		h.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// listAccounts handles GET /accounts
func (h *AccountHandler) listAccounts(w http.ResponseWriter, r *http.Request) {
	accountType := models.AccountType(r.URL.Query().Get("type"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = 20
	}

	accounts, total, err := h.service.ListAccounts(accountType, limit, offset)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, models.AccountListResponse{
		Success:  true,
		Accounts: accounts,
		Total:    total,
	})
}

// activateAccount handles POST /accounts/{id}/activate
func (h *AccountHandler) activateAccount(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	account, err := h.service.ActivateAccount(id)
	if err != nil {
		h.handleAccountError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// freezeAccount handles POST /accounts/{id}/freeze
func (h *AccountHandler) freezeAccount(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	account, err := h.service.FreezeAccount(id, body.Reason)
	if err != nil {
		h.handleAccountError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// unfreezeAccount handles POST /accounts/{id}/unfreeze
func (h *AccountHandler) unfreezeAccount(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	account, err := h.service.UnfreezeAccount(id)
	if err != nil {
		h.handleAccountError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// closeAccount handles POST /accounts/{id}/close
func (h *AccountHandler) closeAccount(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	account, err := h.service.CloseAccount(id)
	if err != nil {
		h.handleAccountError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// updateKYC handles POST /accounts/{id}/kyc
func (h *AccountHandler) updateKYC(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	var body struct {
		Status models.KYCStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.badRequest(w, "invalid request body")
		return
	}

	account, err := h.service.UpdateKYCStatus(id, body.Status)
	if err != nil {
		h.handleAccountError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// addPaymentInstrument handles POST /accounts/{id}/payment-instruments
func (h *AccountHandler) addPaymentInstrument(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	var req models.AddPaymentInstrumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.badRequest(w, "invalid request body")
		return
	}

	account, err := h.service.AddPaymentInstrument(id, &req)
	if err != nil {
		h.handleAccountError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusCreated, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// removePaymentInstrument handles DELETE /accounts/{id}/payment-instruments/{instrumentId}
func (h *AccountHandler) removePaymentInstrument(w http.ResponseWriter, r *http.Request, accountID, instrumentID string) {
	if r.Method != http.MethodDelete {
		h.methodNotAllowed(w)
		return
	}

	account, err := h.service.RemovePaymentInstrument(accountID, instrumentID)
	if err != nil {
		h.handleAccountError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// creditBalance handles POST /accounts/{id}/credit
func (h *AccountHandler) creditBalance(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	var body struct {
		Amount int64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.badRequest(w, "invalid request body")
		return
	}

	account, err := h.service.CreditBalance(id, body.Amount)
	if err != nil {
		h.handleAccountError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// debitBalance handles POST /accounts/{id}/debit
func (h *AccountHandler) debitBalance(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		h.methodNotAllowed(w)
		return
	}

	var body struct {
		Amount int64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.badRequest(w, "invalid request body")
		return
	}

	account, err := h.service.DebitBalance(id, body.Amount)
	if err != nil {
		h.handleAccountError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, models.AccountResponse{
		Success: true,
		Account: account,
	})
}

// Helper methods

func (h *AccountHandler) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *AccountHandler) errorResponse(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, models.AccountResponse{
		Success: false,
		Error:   message,
	})
}

func (h *AccountHandler) badRequest(w http.ResponseWriter, message string) {
	h.errorResponse(w, http.StatusBadRequest, message)
}

func (h *AccountHandler) notFound(w http.ResponseWriter) {
	h.errorResponse(w, http.StatusNotFound, "account not found")
}

func (h *AccountHandler) methodNotAllowed(w http.ResponseWriter) {
	h.errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (h *AccountHandler) handleAccountError(w http.ResponseWriter, err error) {
	if err == repositories.ErrAccountNotFound {
		h.notFound(w)
		return
	}
	h.errorResponse(w, http.StatusBadRequest, err.Error())
}
