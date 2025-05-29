package handlers

import (
	"encoding/json"
	"net/http"

	"amanah/libs/logging"
	"amanah/services/authentication/services"
)

// LoginRequest is the expected payload for login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the response containing the auth token.
type LoginResponse struct {
	Token string `json:"token"`
}

// LoginHandler returns an HTTP handler for logging in.
func LoginHandler(svc *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		token, err := svc.Login(req.Username, req.Password)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		logging.Info("user %s logged in", req.Username)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{Token: token})
	}
}
