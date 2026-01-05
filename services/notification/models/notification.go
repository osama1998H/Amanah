package models

import (
	"time"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	TypeEmail   NotificationType = "email"
	TypeSMS     NotificationType = "sms"
	TypePush    NotificationType = "push"
	TypeWebhook NotificationType = "webhook"
)

// NotificationStatus represents the delivery status
type NotificationStatus string

const (
	StatusQueued    NotificationStatus = "queued"
	StatusSending   NotificationStatus = "sending"
	StatusDelivered NotificationStatus = "delivered"
	StatusFailed    NotificationStatus = "failed"
	StatusRetrying  NotificationStatus = "retrying"
)

// EventType represents the type of event that triggered the notification
type EventType string

const (
	EventTransactionCreated   EventType = "transaction.created"
	EventTransactionCompleted EventType = "transaction.completed"
	EventTransactionFailed    EventType = "transaction.failed"
	EventTransactionRefunded  EventType = "transaction.refunded"
	EventAccountCreated       EventType = "account.created"
	EventAccountVerified      EventType = "account.verified"
	EventAccountFrozen        EventType = "account.frozen"
	EventPaymentReceived      EventType = "payment.received"
	EventPaymentSent          EventType = "payment.sent"
)

// Notification represents a notification to be sent
type Notification struct {
	ID          string             `json:"id"`
	Type        NotificationType   `json:"type"`
	Status      NotificationStatus `json:"status"`
	Event       EventType          `json:"event"`
	Recipient   string             `json:"recipient"` // Email, phone, URL, or device token
	Subject     string             `json:"subject,omitempty"`
	Content     string             `json:"content"`
	Metadata    map[string]string  `json:"metadata,omitempty"`
	RetryCount  int                `json:"retry_count"`
	MaxRetries  int                `json:"max_retries"`
	LastError   string             `json:"last_error,omitempty"`
	ScheduledAt *time.Time         `json:"scheduled_at,omitempty"`
	SentAt      *time.Time         `json:"sent_at,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// NotificationTemplate represents a message template
type NotificationTemplate struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Type     NotificationType `json:"type"`
	Event    EventType        `json:"event"`
	Subject  string           `json:"subject,omitempty"`
	Body     string           `json:"body"`
	Locale   string           `json:"locale"`
	IsActive bool             `json:"is_active"`
}

// WebhookConfig represents webhook endpoint configuration
type WebhookConfig struct {
	ID        string            `json:"id"`
	URL       string            `json:"url"`
	Events    []EventType       `json:"events"`
	Secret    string            `json:"secret"`
	Headers   map[string]string `json:"headers,omitempty"`
	IsActive  bool              `json:"is_active"`
	CreatedAt time.Time         `json:"created_at"`
}

// SendNotificationRequest represents a request to send a notification
type SendNotificationRequest struct {
	Type      NotificationType  `json:"type"`
	Event     EventType         `json:"event"`
	Recipient string            `json:"recipient"`
	Subject   string            `json:"subject,omitempty"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// NotificationResponse wraps a notification for API responses
type NotificationResponse struct {
	Success      bool          `json:"success"`
	Notification *Notification `json:"notification,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// NotificationListResponse wraps a list of notifications
type NotificationListResponse struct {
	Success       bool            `json:"success"`
	Notifications []*Notification `json:"notifications"`
	Total         int             `json:"total"`
}
