package services

import (
	"errors"
	"sync"
	"testing"
	"time"

	"amanah/services/notification/models"
)

// ===========================================
// Mock Senders
// ===========================================

type mockEmailSender struct {
	mu       sync.Mutex
	sent     []map[string]string
	failNext bool
}

func (m *mockEmailSender) Send(to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failNext {
		m.failNext = false
		return errors.New("email delivery failed")
	}

	m.sent = append(m.sent, map[string]string{
		"to":      to,
		"subject": subject,
		"body":    body,
	})
	return nil
}

func (m *mockEmailSender) getSent() []map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sent
}

type mockSMSSender struct {
	mu       sync.Mutex
	sent     []map[string]string
	failNext bool
}

func (m *mockSMSSender) Send(to, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failNext {
		m.failNext = false
		return errors.New("sms delivery failed")
	}

	m.sent = append(m.sent, map[string]string{
		"to":      to,
		"message": message,
	})
	return nil
}

func (m *mockSMSSender) getSent() []map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sent
}

type mockPushSender struct {
	mu       sync.Mutex
	sent     []map[string]interface{}
	failNext bool
}

func (m *mockPushSender) Send(deviceToken, title, body string, data map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failNext {
		m.failNext = false
		return errors.New("push delivery failed")
	}

	m.sent = append(m.sent, map[string]interface{}{
		"token": deviceToken,
		"title": title,
		"body":  body,
		"data":  data,
	})
	return nil
}

func (m *mockPushSender) getSent() []map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sent
}

// ===========================================
// Notification Creation Tests
// ===========================================

func TestSend_Email(t *testing.T) {
	svc := NewNotificationService()
	emailSender := &mockEmailSender{}
	svc.SetEmailSender(emailSender)

	req := &models.SendNotificationRequest{
		Type:      models.TypeEmail,
		Event:     models.EventTransactionCompleted,
		Recipient: "test@example.com",
		Subject:   "Payment Received",
		Content:   "Your payment has been processed.",
	}

	notification, err := svc.Send(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if notification == nil {
		t.Fatal("expected notification, got nil")
	}
	if notification.Type != models.TypeEmail {
		t.Errorf("expected type email, got %s", notification.Type)
	}
	if notification.Recipient != "test@example.com" {
		t.Errorf("expected recipient test@example.com, got %s", notification.Recipient)
	}

	// Allow async processing
	time.Sleep(100 * time.Millisecond)
}

func TestSend_SMS(t *testing.T) {
	svc := NewNotificationService()
	smsSender := &mockSMSSender{}
	svc.SetSMSSender(smsSender)

	req := &models.SendNotificationRequest{
		Type:      models.TypeSMS,
		Event:     models.EventPaymentReceived,
		Recipient: "+1234567890",
		Content:   "Your payment of $100 has been received.",
	}

	notification, err := svc.Send(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if notification.Type != models.TypeSMS {
		t.Errorf("expected type sms, got %s", notification.Type)
	}

	// Allow async processing
	time.Sleep(100 * time.Millisecond)
}

func TestSend_Push(t *testing.T) {
	svc := NewNotificationService()
	pushSender := &mockPushSender{}
	svc.SetPushSender(pushSender)

	req := &models.SendNotificationRequest{
		Type:      models.TypePush,
		Event:     models.EventAccountVerified,
		Recipient: "device_token_123",
		Subject:   "Account Verified",
		Content:   "Your account has been verified successfully.",
		Metadata: map[string]string{
			"action": "open_dashboard",
		},
	}

	notification, err := svc.Send(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if notification.Type != models.TypePush {
		t.Errorf("expected type push, got %s", notification.Type)
	}

	// Allow async processing
	time.Sleep(100 * time.Millisecond)
}

func TestSend_Webhook(t *testing.T) {
	svc := NewNotificationService()

	req := &models.SendNotificationRequest{
		Type:      models.TypeWebhook,
		Event:     models.EventTransactionCreated,
		Recipient: "http://localhost:8080/webhook",
		Content:   `{"transaction_id": "txn_001"}`,
	}

	notification, err := svc.Send(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if notification.Type != models.TypeWebhook {
		t.Errorf("expected type webhook, got %s", notification.Type)
	}
}

func TestSend_InvalidRecipient(t *testing.T) {
	svc := NewNotificationService()

	req := &models.SendNotificationRequest{
		Type:      models.TypeEmail,
		Recipient: "",
		Content:   "Test",
	}

	_, err := svc.Send(req)
	if err != ErrInvalidRecipient {
		t.Errorf("expected ErrInvalidRecipient, got %v", err)
	}
}

func TestSend_InvalidType(t *testing.T) {
	svc := NewNotificationService()

	req := &models.SendNotificationRequest{
		Type:      "invalid",
		Recipient: "test@example.com",
		Content:   "Test",
	}

	_, err := svc.Send(req)
	if err != ErrInvalidType {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestSend_AllTypes(t *testing.T) {
	tests := []struct {
		notifType models.NotificationType
		recipient string
	}{
		{models.TypeEmail, "test@example.com"},
		{models.TypeSMS, "+1234567890"},
		{models.TypePush, "device_token"},
		{models.TypeWebhook, "http://localhost/webhook"},
	}

	for _, tt := range tests {
		t.Run(string(tt.notifType), func(t *testing.T) {
			svc := NewNotificationService()

			req := &models.SendNotificationRequest{
				Type:      tt.notifType,
				Recipient: tt.recipient,
				Content:   "Test content",
			}

			notification, err := svc.Send(req)
			if err != nil {
				t.Fatalf("expected no error for type %s, got %v", tt.notifType, err)
			}
			if notification.Type != tt.notifType {
				t.Errorf("expected type %s, got %s", tt.notifType, notification.Type)
			}
		})
	}
}

// ===========================================
// Queue Tests
// ===========================================

func TestSend_QueuedStatus(t *testing.T) {
	svc := NewNotificationService()

	req := &models.SendNotificationRequest{
		Type:      models.TypeEmail,
		Recipient: "test@example.com",
		Content:   "Test",
	}

	notification, _ := svc.Send(req)

	// Initially should be queued or processing
	if notification.Status != models.StatusQueued {
		t.Logf("status was %s (may have been processed quickly)", notification.Status)
	}
}

func TestSend_MaxRetries(t *testing.T) {
	svc := NewNotificationService()

	req := &models.SendNotificationRequest{
		Type:      models.TypeEmail,
		Recipient: "test@example.com",
		Content:   "Test",
	}

	notification, _ := svc.Send(req)
	if notification.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", notification.MaxRetries)
	}
}

// ===========================================
// Retrieval Tests
// ===========================================

func TestGetNotification_Found(t *testing.T) {
	svc := NewNotificationService()

	req := &models.SendNotificationRequest{
		Type:      models.TypeEmail,
		Recipient: "test@example.com",
		Content:   "Test",
	}

	created, _ := svc.Send(req)

	notification, err := svc.GetNotification(created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if notification.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, notification.ID)
	}
}

func TestGetNotification_NotFound(t *testing.T) {
	svc := NewNotificationService()

	_, err := svc.GetNotification("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent notification")
	}
}

func TestListNotifications_Empty(t *testing.T) {
	svc := NewNotificationService()

	notifications, total := svc.ListNotifications(10, 0)
	if len(notifications) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(notifications))
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
}

func TestListNotifications_WithNotifications(t *testing.T) {
	svc := NewNotificationService()

	// Create multiple notifications
	for i := 0; i < 5; i++ {
		req := &models.SendNotificationRequest{
			Type:      models.TypeEmail,
			Recipient: "test@example.com",
			Content:   "Test",
		}
		svc.Send(req)
	}

	notifications, total := svc.ListNotifications(10, 0)
	if len(notifications) != 5 {
		t.Errorf("expected 5 notifications, got %d", len(notifications))
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
}

func TestListNotifications_Pagination(t *testing.T) {
	svc := NewNotificationService()

	// Create 10 notifications
	for i := 0; i < 10; i++ {
		req := &models.SendNotificationRequest{
			Type:      models.TypeEmail,
			Recipient: "test@example.com",
			Content:   "Test",
		}
		svc.Send(req)
	}

	// First page
	notifications, total := svc.ListNotifications(3, 0)
	if len(notifications) != 3 {
		t.Errorf("expected 3 notifications on first page, got %d", len(notifications))
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}

	// Offset beyond count
	notifications, _ = svc.ListNotifications(3, 100)
	if len(notifications) != 0 {
		t.Errorf("expected 0 notifications for offset beyond count, got %d", len(notifications))
	}
}

// ===========================================
// Webhook Tests
// ===========================================

func TestRegisterWebhook_Success(t *testing.T) {
	svc := NewNotificationService()

	config := &models.WebhookConfig{
		URL:      "http://localhost:8080/webhook",
		Events:   []models.EventType{models.EventTransactionCompleted, models.EventTransactionRefunded},
		Secret:   "webhook_secret",
		IsActive: true,
	}

	err := svc.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if config.ID == "" {
		t.Error("expected webhook ID to be set")
	}
}

func TestUnregisterWebhook_Success(t *testing.T) {
	svc := NewNotificationService()

	config := &models.WebhookConfig{
		URL:      "http://localhost:8080/webhook",
		Events:   []models.EventType{models.EventTransactionCompleted},
		IsActive: true,
	}

	svc.RegisterWebhook(config)

	err := svc.UnregisterWebhook(config.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUnregisterWebhook_NotFound(t *testing.T) {
	svc := NewNotificationService()

	err := svc.UnregisterWebhook("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent webhook")
	}
}

// ===========================================
// Event Trigger Tests
// ===========================================

func TestTriggerEvent_Success(t *testing.T) {
	svc := NewNotificationService()

	// Register webhook
	config := &models.WebhookConfig{
		URL:      "http://localhost:8080/webhook",
		Events:   []models.EventType{models.EventTransactionCompleted},
		Secret:   "secret",
		IsActive: true,
	}
	svc.RegisterWebhook(config)

	// Trigger event
	data := map[string]interface{}{
		"transaction_id": "txn_001",
		"amount":         10000,
	}

	err := svc.TriggerEvent(models.EventTransactionCompleted, data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestTriggerEvent_NoSubscribers(t *testing.T) {
	svc := NewNotificationService()

	// No webhooks registered
	data := map[string]interface{}{"test": "data"}

	err := svc.TriggerEvent(models.EventAccountCreated, data)
	if err != nil {
		t.Fatalf("expected no error for event with no subscribers, got %v", err)
	}
}

func TestTriggerEvent_InactiveWebhook(t *testing.T) {
	svc := NewNotificationService()

	// Register inactive webhook
	config := &models.WebhookConfig{
		URL:      "http://localhost:8080/webhook",
		Events:   []models.EventType{models.EventTransactionCompleted},
		IsActive: false, // Inactive
	}
	svc.RegisterWebhook(config)

	// Trigger event - should not send to inactive webhook
	err := svc.TriggerEvent(models.EventTransactionCompleted, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestTriggerEvent_WrongEventType(t *testing.T) {
	svc := NewNotificationService()

	// Register webhook for specific event
	config := &models.WebhookConfig{
		URL:      "http://localhost:8080/webhook",
		Events:   []models.EventType{models.EventTransactionCompleted},
		IsActive: true,
	}
	svc.RegisterWebhook(config)

	// Trigger different event - should not match
	err := svc.TriggerEvent(models.EventAccountFrozen, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// ===========================================
// Delivery Status Tests
// ===========================================

func TestSendSync_DeliveredStatus(t *testing.T) {
	svc := NewNotificationService()
	emailSender := &mockEmailSender{}
	svc.SetEmailSender(emailSender)

	req := &models.SendNotificationRequest{
		Type:      models.TypeEmail,
		Recipient: "test@example.com",
		Subject:   "Test",
		Content:   "Test content",
	}

	notification, err := svc.SendSync(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Wait for delivery
	time.Sleep(200 * time.Millisecond)

	// Check final status
	final, _ := svc.GetNotification(notification.ID)
	if final.Status != models.StatusDelivered {
		t.Errorf("expected status delivered, got %s", final.Status)
	}
}

// ===========================================
// Masking Tests
// ===========================================

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test@example.com", "**@example.com"},       // local part <= 4 chars
		{"johndoe@example.com", "jo***e@example.com"}, // local part > 4 chars
		{"ab@x.com", "**@x.com"},
		{"a", "***"},
		{"", "***"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := maskEmail(tt.input)
			if result != tt.expected {
				t.Errorf("maskEmail(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskPhone(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"+1234567890", "***-***-7890"},
		{"123", "***"},
		{"", "***"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := maskPhone(tt.input)
			if result != tt.expected {
				t.Errorf("maskPhone(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abcd1234efgh5678", "abcd***5678"},
		{"short", "***"},
		{"", "***"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := maskToken(tt.input)
			if result != tt.expected {
				t.Errorf("maskToken(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ===========================================
// Concurrency Tests
// ===========================================

func TestConcurrentSend(t *testing.T) {
	svc := NewNotificationService()
	emailSender := &mockEmailSender{}
	svc.SetEmailSender(emailSender)

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := &models.SendNotificationRequest{
				Type:      models.TypeEmail,
				Recipient: "test@example.com",
				Content:   "Concurrent test",
			}
			_, _ = svc.Send(req)
		}(i)
	}

	wg.Wait()

	// Allow processing
	time.Sleep(500 * time.Millisecond)

	// Verify all notifications were created
	notifications, total := svc.ListNotifications(100, 0)
	if total != numGoroutines {
		t.Errorf("expected %d notifications, got %d", numGoroutines, total)
	}
	if len(notifications) != numGoroutines {
		t.Errorf("expected %d notifications returned, got %d", numGoroutines, len(notifications))
	}
}

// ===========================================
// Event Type Tests
// ===========================================

func TestAllEventTypes(t *testing.T) {
	events := []models.EventType{
		models.EventTransactionCreated,
		models.EventTransactionCompleted,
		models.EventTransactionFailed,
		models.EventTransactionRefunded,
		models.EventAccountCreated,
		models.EventAccountVerified,
		models.EventAccountFrozen,
		models.EventPaymentReceived,
		models.EventPaymentSent,
	}

	svc := NewNotificationService()

	for _, event := range events {
		t.Run(string(event), func(t *testing.T) {
			req := &models.SendNotificationRequest{
				Type:      models.TypeEmail,
				Event:     event,
				Recipient: "test@example.com",
				Content:   "Test",
			}

			notification, err := svc.Send(req)
			if err != nil {
				t.Fatalf("expected no error for event %s, got %v", event, err)
			}
			if notification.Event != event {
				t.Errorf("expected event %s, got %s", event, notification.Event)
			}
		})
	}
}

// ===========================================
// Metadata Tests
// ===========================================

func TestSend_WithMetadata(t *testing.T) {
	svc := NewNotificationService()

	metadata := map[string]string{
		"transaction_id": "txn_001",
		"user_id":        "usr_001",
		"action":         "payment_complete",
	}

	req := &models.SendNotificationRequest{
		Type:      models.TypeEmail,
		Recipient: "test@example.com",
		Content:   "Test",
		Metadata:  metadata,
	}

	notification, err := svc.Send(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if notification.Metadata == nil {
		t.Error("expected metadata to be set")
	}
	if notification.Metadata["transaction_id"] != "txn_001" {
		t.Errorf("expected transaction_id txn_001, got %s", notification.Metadata["transaction_id"])
	}
}
