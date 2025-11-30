package notify

import "context"

// MessageID represents a unique identifier for a sent message.
type MessageID string

// Sender describes low-level transport that actually delivers notifications.
type Sender interface {
	SendEmail(ctx context.Context, in EmailInput) (MessageID, error)
	SendSMS(ctx context.Context, in SMSInput) (MessageID, error)
}

// EmailInput contains already rendered email contents.
type EmailInput struct {
	To       []string
	Subject  string
	HTMLBody string
	TextBody string
	Metadata map[string]string
}

// SMSInput contains rendered SMS payload.
type SMSInput struct {
	To       string
	Body     string
	Metadata map[string]string
}
