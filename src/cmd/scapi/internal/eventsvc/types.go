package eventsvc

import (
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of event being logged
type EventType string

const (
	// Auth events
	EventTypeOTPRequest       EventType = "otp_request"
	EventTypeOTPVerify        EventType = "otp_verify"
	EventTypeOTPVerifyFail    EventType = "otp_verify_fail"
	EventTypeLogin            EventType = "login"
	EventTypeLogout           EventType = "logout"
	EventTypeTokenRefresh     EventType = "token_refresh"
	EventTypeTokenRefreshFail EventType = "token_refresh_fail"

	// User events
	EventTypeUserCreate     EventType = "user_create"
	EventTypeUserUpdate     EventType = "user_update"
	EventTypeUserDeactivate EventType = "user_deactivate"
	EventTypeUserActivate   EventType = "user_activate"
	EventTypeUserDelete     EventType = "user_delete"
)

// EventData represents additional event data as a map
type EventData map[string]interface{}

// Event represents a single event to be logged
type Event struct {
	UserID    *uuid.UUID // nullable
	EventType EventType
	EventData EventData
	IPAddress string
	UserAgent string
	CreatedAt time.Time
}
