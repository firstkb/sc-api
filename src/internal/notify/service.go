package notify

import (
	"context"
	"errors"
	"log/slog"

	"github.com/firstkb/sc-api/internal/config"
)

// Service provides notification sending functionality.
type Service interface {
	SendEmail(ctx context.Context, in EmailInput) (MessageID, error)
	SendSMS(ctx context.Context, in SMSInput) (MessageID, error)
}

type NotifyProvider struct {
	sender Sender
	logger *slog.Logger
}

// Config holds configuration for the notification service.
type Config struct {
	ProviderEmail string // "debug", "ses", ...
	ProviderSMS   string // "debug", "sns", ...
	Logger        *slog.Logger
}

// NewService creates a new notification service.
func NewService(config *config.Config, logger *slog.Logger) (*NotifyProvider, error) {
	// TODO: select providers based on config.ProviderEmail / ProviderSMS.
	sender := NewDebugSender(logger)

	return &NotifyProvider{
		sender: sender,
		logger: logger,
	}, nil
}

func (s *NotifyProvider) SendEmail(ctx context.Context, in EmailInput) (MessageID, error) {
	if len(in.To) == 0 {
		return "", errors.New("notify: email recipients required")
	}
	return s.sender.SendEmail(ctx, in)
}

func (s *NotifyProvider) SendSMS(ctx context.Context, in SMSInput) (MessageID, error) {
	if in.To == "" {
		return "", errors.New("notify: sms recipient required")
	}
	return s.sender.SendSMS(ctx, in)
}
