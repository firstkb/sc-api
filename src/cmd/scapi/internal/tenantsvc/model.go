package tenantsvc

// Tenant описывает атрибуты тенанта для Origin-проверок и подключения к БД.
type Tenant struct {
	ID              string `json:"tenant_id"`
	Name            string `json:"tenant_name"`
	Plan            string `json:"plan"`
	Isolation       string `json:"isolation"`
	Status          string `json:"status"`
	DBName          string `json:"db_name"`
	Host            string `json:"host"`
	TenantUpdatedAt string `json:"tenant_updated_at"`
	DBInstanceID    string `json:"db_instance_id"`
	DBInstanceCode  string `json:"db_instance_code"`
	UpdatedAt       string `json:"updated_at"`
}
