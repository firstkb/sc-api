package pingsvc

import "github.com/firstkb/sc-api/internal/httpx/requestctx"

type Ping struct {
	Message  string                  `json:"message"`
	Users    []map[string]any        `json:"users,omitempty"`
	TenantId string                  `json:"tenant_id,omitempty"`
	Tenant   requestctx.TenantInfo   `json:"tenant,omitempty"`
	Identity requestctx.IdentityInfo `json:"identity,omitempty"`
	Claims   requestctx.ClaimsInfo   `json:"claims,omitempty"`
	Plan     string                  `json:"plan,omitempty"`
	User     requestctx.UserInfo     `json:"user,omitempty"`
}
