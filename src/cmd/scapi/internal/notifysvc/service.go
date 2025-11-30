package notifysvc

import (
	"context"
	"errors"
	"log/slog"

	"github.com/firstkb/sc-api/internal/config"
	"github.com/firstkb/sc-api/internal/notify"
	"github.com/firstkb/sc-api/internal/postgres"
)

// Service описывает application-слой вокруг инфраструктурного notify.
type Service interface {
	SendEmail(ctx context.Context, tenantID string, input EmailRequest) (notify.MessageID, error)
	SendSMS(ctx context.Context, tenantID string, input SMSRequest) (notify.MessageID, error)
}

type NotifyService struct {
	notify *notify.NotifyProvider
	repo   *Repo
	logger *slog.Logger
}

// EmailRequest contains data for template email sending.
type EmailRequest struct {
	To       []string
	Template string
	Locale   string
	Data     map[string]any
}

// SMSRequest contains data for SMS sending.
type SMSRequest struct {
	To       string
	Template string
	Locale   string
	Data     map[string]any
}

func NewService(sqlClient *postgres.Client, config *config.Config, logger *slog.Logger) (*NotifyService, error) {

	notifyProvider, err := notify.NewService(config, logger)
	if err != nil {
		return nil, err
	}

	return &NotifyService{
		notify: notifyProvider,
		repo:   NewRepo(sqlClient, logger),
		logger: logger,
	}, nil
}

func (s *NotifyService) SendEmail(ctx context.Context, tenantID string, input EmailRequest) (notify.MessageID, error) {
	if len(input.To) == 0 {
		return "", errors.New("notifysvc: email recipients required")
	}

	template, err := s.repo.getTemplate(ctx, tenantID, KindEmail, input.Template, input.Locale)
	if err != nil {
		return "", err
	}

	subject, htmlBody, textBody, err := RenderTemplate(template, input.Data)
	if err != nil {
		return "", err
	}

	return s.notify.SendEmail(ctx, notify.EmailInput{
		To:       input.To,
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
		Metadata: map[string]string{
			"tenant_id": tenantID,
			"template":  input.Template,
			"locale":    input.Locale,
		},
	})
}

func (s *NotifyService) SendSMS(ctx context.Context, tenantID string, input SMSRequest) (notify.MessageID, error) {
	if input.To == "" {
		return "", errors.New("notifysvc: sms recipient required")
	}

	template, err := s.repo.getTemplate(ctx, tenantID, KindSMS, input.Template, input.Locale)
	if err != nil {
		return "", err
	}

	_, _, textBody, err := RenderTemplate(template, input.Data)
	if err != nil {
		return "", err
	}

	return s.notify.SendSMS(ctx, notify.SMSInput{
		To:   input.To,
		Body: textBody,
		Metadata: map[string]string{
			"tenant_id": tenantID,
			"template":  input.Template,
			"locale":    input.Locale,
		},
	})
}

const (
	KindEmail = "email"
	KindSMS   = "sms"
)

type Template struct {
	ID       string
	TenantID *string
	Kind     string
	Key      string
	Locale   string
	Subject  string
	HTML     string
	Text     string
}
