package notify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// DebugSender is a debug implementation that logs messages instead of sending them.
type DebugSender struct {
	logger *slog.Logger
}

// NewDebugSender creates a new debug sender.
func NewDebugSender(logger *slog.Logger) *DebugSender {
	if logger == nil {
		logger = slog.Default()
	}
	return &DebugSender{logger: logger}
}

func (d *DebugSender) SendEmail(ctx context.Context, in EmailInput) (MessageID, error) {
	maskedTo := make([]string, len(in.To))
	for i, email := range in.To {
		maskedTo[i] = maskEmail(email)
	}

	d.logger.Info("notify email (debug)",
		"to", maskedTo,
		"subject", in.Subject,
		"html_len", len(in.HTMLBody),
		"text_len", len(in.TextBody),
		"text", in.TextBody,
	)

	return MessageID(fmt.Sprintf("debug-email-%d", len(maskedTo))), nil
}

func (d *DebugSender) SendSMS(ctx context.Context, in SMSInput) (MessageID, error) {
	maskedTo := maskPhone(in.To)

	d.logger.Info("notify sms (debug)",
		"to", maskedTo,
		"body_len", len(in.Body),
		"body", in.Body,
	)

	return MessageID(fmt.Sprintf("debug-sms-%s", maskedTo)), nil
}

func maskEmail(email string) string {
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}

	local := parts[0]
	domain := parts[1]

	if len(local) == 0 {
		return "***@" + domain
	}

	if len(local) == 1 {
		return local[0:1] + "***@" + domain
	}

	return local[0:1] + "***@" + domain
}

func maskPhone(phone string) string {
	if phone == "" {
		return ""
	}

	if len(phone) <= 4 {
		return "***"
	}

	return phone[:len(phone)-4] + "****"
}
