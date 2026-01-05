package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"amanah/libs/logging"
	"amanah/services/notification/models"
)

var (
	ErrInvalidRecipient  = errors.New("invalid recipient")
	ErrInvalidType       = errors.New("invalid notification type")
	ErrNotificationFailed = errors.New("notification delivery failed")
	ErrMaxRetriesReached = errors.New("max retries reached")
)

// EmailSender interface for email delivery
type EmailSender interface {
	Send(to, subject, body string) error
}

// SMSSender interface for SMS delivery
type SMSSender interface {
	Send(to, message string) error
}

// PushSender interface for push notification delivery
type PushSender interface {
	Send(deviceToken, title, body string, data map[string]string) error
}

// NotificationService handles notification delivery
type NotificationService struct {
	mu sync.RWMutex

	notifications map[string]*models.Notification
	webhooks      map[string]*models.WebhookConfig
	templates     map[string]*models.NotificationTemplate

	emailSender EmailSender
	smsSender   SMSSender
	pushSender  PushSender
	httpClient  *http.Client

	// Queue for async processing
	queue chan *models.Notification
}

// NewNotificationService creates a new notification service
func NewNotificationService() *NotificationService {
	svc := &NotificationService{
		notifications: make(map[string]*models.Notification),
		webhooks:      make(map[string]*models.WebhookConfig),
		templates:     make(map[string]*models.NotificationTemplate),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		queue: make(chan *models.Notification, 1000),
	}

	// Start worker
	go svc.processQueue()

	return svc
}

// SetEmailSender sets the email sender implementation
func (s *NotificationService) SetEmailSender(sender EmailSender) {
	s.emailSender = sender
}

// SetSMSSender sets the SMS sender implementation
func (s *NotificationService) SetSMSSender(sender SMSSender) {
	s.smsSender = sender
}

// SetPushSender sets the push notification sender implementation
func (s *NotificationService) SetPushSender(sender PushSender) {
	s.pushSender = sender
}

// Send queues a notification for delivery
func (s *NotificationService) Send(req *models.SendNotificationRequest) (*models.Notification, error) {
	if err := s.validateRequest(req); err != nil {
		return nil, err
	}

	id, _ := generateID("ntf")
	notification := &models.Notification{
		ID:         id,
		Type:       req.Type,
		Status:     models.StatusQueued,
		Event:      req.Event,
		Recipient:  req.Recipient,
		Subject:    req.Subject,
		Content:    req.Content,
		Metadata:   req.Metadata,
		MaxRetries: 3,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	s.mu.Lock()
	s.notifications[notification.ID] = notification
	s.mu.Unlock()

	// Queue for async processing
	select {
	case s.queue <- notification:
		// Queued successfully
	default:
		// Queue full, process synchronously
		s.deliver(notification)
	}

	return notification, nil
}

// SendSync sends a notification synchronously
func (s *NotificationService) SendSync(req *models.SendNotificationRequest) (*models.Notification, error) {
	notification, err := s.Send(req)
	if err != nil {
		return nil, err
	}

	// Wait for delivery
	for notification.Status == models.StatusQueued || notification.Status == models.StatusSending {
		time.Sleep(100 * time.Millisecond)
		s.mu.RLock()
		notification = s.notifications[notification.ID]
		s.mu.RUnlock()
	}

	return notification, nil
}

// GetNotification retrieves a notification by ID
func (s *NotificationService) GetNotification(id string) (*models.Notification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	notification, exists := s.notifications[id]
	if !exists {
		return nil, errors.New("notification not found")
	}
	return notification, nil
}

// TriggerEvent sends notifications for an event
func (s *NotificationService) TriggerEvent(event models.EventType, data map[string]interface{}) error {
	s.mu.RLock()
	webhooksToTrigger := make([]*models.WebhookConfig, 0)
	for _, webhook := range s.webhooks {
		if webhook.IsActive && s.webhookSubscribedToEvent(webhook, event) {
			webhooksToTrigger = append(webhooksToTrigger, webhook)
		}
	}
	s.mu.RUnlock()

	// Send to all subscribed webhooks
	for _, webhook := range webhooksToTrigger {
		go s.sendWebhook(webhook, event, data)
	}

	return nil
}

// RegisterWebhook registers a webhook endpoint
func (s *NotificationService) RegisterWebhook(config *models.WebhookConfig) error {
	id, _ := generateID("whk")
	config.ID = id
	config.CreatedAt = time.Now().UTC()

	s.mu.Lock()
	s.webhooks[config.ID] = config
	s.mu.Unlock()

	return nil
}

// UnregisterWebhook removes a webhook endpoint
func (s *NotificationService) UnregisterWebhook(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.webhooks[id]; !exists {
		return errors.New("webhook not found")
	}
	delete(s.webhooks, id)
	return nil
}

// ListNotifications returns notifications with pagination
func (s *NotificationService) ListNotifications(limit, offset int) ([]*models.Notification, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	notifications := make([]*models.Notification, 0, len(s.notifications))
	for _, n := range s.notifications {
		notifications = append(notifications, n)
	}

	total := len(notifications)
	if offset >= total {
		return []*models.Notification{}, total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return notifications[offset:end], total
}

// processQueue processes notifications from the queue
func (s *NotificationService) processQueue() {
	for notification := range s.queue {
		s.deliver(notification)
	}
}

// deliver attempts to deliver a notification
func (s *NotificationService) deliver(notification *models.Notification) {
	s.updateStatus(notification, models.StatusSending)

	var err error
	switch notification.Type {
	case models.TypeEmail:
		err = s.sendEmail(notification)
	case models.TypeSMS:
		err = s.sendSMS(notification)
	case models.TypePush:
		err = s.sendPush(notification)
	case models.TypeWebhook:
		err = s.sendWebhookNotification(notification)
	default:
		err = ErrInvalidType
	}

	if err != nil {
		notification.LastError = err.Error()
		notification.RetryCount++

		if notification.RetryCount >= notification.MaxRetries {
			s.updateStatus(notification, models.StatusFailed)
		} else {
			s.updateStatus(notification, models.StatusRetrying)
			// Retry with exponential backoff
			go func(n *models.Notification) {
				delay := time.Duration(1<<uint(n.RetryCount)) * time.Second
				time.Sleep(delay)
				s.deliver(n)
			}(notification)
		}
	} else {
		now := time.Now().UTC()
		notification.SentAt = &now
		s.updateStatus(notification, models.StatusDelivered)
	}
}

// sendEmail sends an email notification
func (s *NotificationService) sendEmail(notification *models.Notification) error {
	if s.emailSender == nil {
		// Log-only mode - use structured logging with masked data
		logging.WithFields(map[string]interface{}{
			"notification_id": notification.ID,
			"type":            "email",
			"recipient":       maskEmail(notification.Recipient),
		}).Info("email notification sent (mock mode)")
		return nil
	}
	return s.emailSender.Send(notification.Recipient, notification.Subject, notification.Content)
}

// sendSMS sends an SMS notification
func (s *NotificationService) sendSMS(notification *models.Notification) error {
	if s.smsSender == nil {
		// Log-only mode - use structured logging with masked data
		logging.WithFields(map[string]interface{}{
			"notification_id": notification.ID,
			"type":            "sms",
			"recipient":       maskPhone(notification.Recipient),
		}).Info("sms notification sent (mock mode)")
		return nil
	}
	return s.smsSender.Send(notification.Recipient, notification.Content)
}

// sendPush sends a push notification
func (s *NotificationService) sendPush(notification *models.Notification) error {
	if s.pushSender == nil {
		// Log-only mode - use structured logging with masked data
		logging.WithFields(map[string]interface{}{
			"notification_id": notification.ID,
			"type":            "push",
			"token":           maskToken(notification.Recipient),
		}).Info("push notification sent (mock mode)")
		return nil
	}
	return s.pushSender.Send(notification.Recipient, notification.Subject, notification.Content, notification.Metadata)
}

// sendWebhookNotification sends a webhook notification
func (s *NotificationService) sendWebhookNotification(notification *models.Notification) error {
	payload := map[string]interface{}{
		"id":        notification.ID,
		"event":     notification.Event,
		"content":   notification.Content,
		"metadata":  notification.Metadata,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequest("POST", notification.Recipient, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// sendWebhook sends an event to a webhook endpoint
func (s *NotificationService) sendWebhook(webhook *models.WebhookConfig, event models.EventType, data map[string]interface{}) error {
	payload := map[string]interface{}{
		"event":     event,
		"data":      data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Calculate signature
	signature := s.calculateSignature(body, webhook.Secret)

	req, err := http.NewRequest("POST", webhook.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", string(event))

	for k, v := range webhook.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// calculateSignature calculates HMAC signature for webhook payload
func (s *NotificationService) calculateSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// webhookSubscribedToEvent checks if webhook is subscribed to an event
func (s *NotificationService) webhookSubscribedToEvent(webhook *models.WebhookConfig, event models.EventType) bool {
	for _, e := range webhook.Events {
		if e == event {
			return true
		}
	}
	return false
}

// updateStatus updates a notification's status
func (s *NotificationService) updateStatus(notification *models.Notification, status models.NotificationStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	notification.Status = status
	notification.UpdatedAt = time.Now().UTC()
	s.notifications[notification.ID] = notification
}

// validateRequest validates a send notification request
func (s *NotificationService) validateRequest(req *models.SendNotificationRequest) error {
	if req.Recipient == "" {
		return ErrInvalidRecipient
	}
	validTypes := map[models.NotificationType]bool{
		models.TypeEmail:   true,
		models.TypeSMS:     true,
		models.TypePush:    true,
		models.TypeWebhook: true,
	}
	if !validTypes[req.Type] {
		return ErrInvalidType
	}
	return nil
}

// generateID creates a unique ID with a prefix
func generateID(prefix string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(bytes), nil
}

// maskEmail masks an email address for logging (shows first 2 and last 2 chars before @)
func maskEmail(email string) string {
	if len(email) < 5 {
		return "***"
	}
	atIndex := -1
	for i, c := range email {
		if c == '@' {
			atIndex = i
			break
		}
	}
	if atIndex < 0 {
		return "***"
	}
	local := email[:atIndex]
	domain := email[atIndex:]
	if len(local) <= 4 {
		return "**" + domain
	}
	return local[:2] + "***" + local[len(local)-1:] + domain
}

// maskPhone masks a phone number for logging (shows last 4 digits)
func maskPhone(phone string) string {
	if len(phone) < 4 {
		return "***"
	}
	return "***-***-" + phone[len(phone)-4:]
}

// maskToken masks a token for logging (shows first 4 and last 4 chars)
func maskToken(token string) string {
	if len(token) < 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}
