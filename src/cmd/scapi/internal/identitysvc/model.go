package identitysvc

import "time"

type SubjectType string

const (
	SubjectTypeEmail SubjectType = "email"
	SubjectTypePhone SubjectType = "phone"
)

type Subject struct {
	ID        int64       `json:"id"`
	Type      SubjectType `json:"type"`
	HMAC      []byte      `json:"hmac"`
	HMACKID   int16       `json:"hmac_kid"`
	IsPrimary bool        `json:"is_primary"`
	Status    string      `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type Membership struct {
	TenantID     int64     `json:"tenant_id"`
	IdentityID   int64     `json:"identity_id"`
	TenantUserID string    `json:"tenant_user_id"`
	Role         string    `json:"role"`
	Level        int       `json:"level"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
