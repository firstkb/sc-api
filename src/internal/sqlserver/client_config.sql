CREATE TABLE tenant_config (
   tenant_id NVARCHAR(255) PRIMARY KEY, -- need chagefrom client_id to tenant_id
   [database_name] NVARCHAR(255) NOT NULL,
   [rowversion] ROWVERSION
);