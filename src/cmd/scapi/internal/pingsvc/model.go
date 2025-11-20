package pingsvc

type Ping struct {
	Message  string                      `json:"message"`
	TenantId string                      `json:"tenant_id"`
	Plan     string                      `json:"plan"`
	Users    map[string][]map[string]any `json:"users"`
}
