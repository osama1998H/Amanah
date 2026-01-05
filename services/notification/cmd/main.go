package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"amanah/services/notification/models"
	"amanah/services/notification/services"
)

func main() {
	// Initialize service
	notificationService := services.NewNotificationService()

	// Setup routes
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"healthy","service":"notification"}`)
	})

	// Send notification
	mux.HandleFunc("/notifications", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req models.SendNotificationRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, http.StatusBadRequest, "invalid request body")
				return
			}

			notification, err := notificationService.Send(&req)
			if err != nil {
				jsonError(w, http.StatusBadRequest, err.Error())
				return
			}

			jsonResponse(w, http.StatusCreated, models.NotificationResponse{
				Success:      true,
				Notification: notification,
			})

		case http.MethodGet:
			notifications, total := notificationService.ListNotifications(20, 0)
			jsonResponse(w, http.StatusOK, models.NotificationListResponse{
				Success:       true,
				Notifications: notifications,
				Total:         total,
			})

		default:
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	// Trigger event
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var body struct {
			Event models.EventType       `json:"event"`
			Data  map[string]interface{} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := notificationService.TriggerEvent(body.Event, body.Data); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}

		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "event triggered",
		})
	})

	// Webhooks
	mux.HandleFunc("/webhooks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var config models.WebhookConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := notificationService.RegisterWebhook(&config); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}

		jsonResponse(w, http.StatusCreated, map[string]interface{}{
			"success": true,
			"webhook": config,
		})
	})

	// Start server
	addr := ":8083"
	log.Printf("Notification service starting on %s", addr)
	log.Printf("Endpoints:")
	log.Printf("  POST   /notifications    - Send notification")
	log.Printf("  GET    /notifications    - List notifications")
	log.Printf("  POST   /events           - Trigger event")
	log.Printf("  POST   /webhooks         - Register webhook")

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
