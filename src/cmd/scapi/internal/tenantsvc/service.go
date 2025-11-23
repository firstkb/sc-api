package tenantsvc

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
	"github.com/firstkb/sc-api/internal/config"
	"github.com/firstkb/sc-api/internal/postgres"
)

type Config struct {
	Tenants TenantsConfig `json:"tenants"`
}

type TenantsConfig struct {
	CacheTTL string `json:"cachettl"`
}

type cacheEntry struct {
	tenant  *Tenant
	expires time.Time
}

type ServiceTenantProvider struct {
	repository *Repository
	logger     *slog.Logger

	cacheTTL    time.Duration
	cacheMu     sync.RWMutex
	cacheByID   map[string]cacheEntry
	cacheByHost map[string]cacheEntry
}

const defaultTenantsCacheTTL = time.Minute

func NewService(sqlClient *postgres.Client, cfg *config.Config, logger *slog.Logger) (*ServiceTenantProvider, error) {
	var c Config
	_ = cfg.Unmarshal("", &c)

	ttl := defaultTenantsCacheTTL
	if raw := strings.TrimSpace(c.Tenants.CacheTTL); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			ttl = parsed
		} else if logger != nil {
			logger.Warn("invalid tenants cache TTL, using default", "value", raw, "error", err)
		}
	}

	return &ServiceTenantProvider{
		repository:  NewRepository(sqlClient, logger),
		logger:      logger,
		cacheTTL:    ttl,
		cacheByID:   make(map[string]cacheEntry),
		cacheByHost: make(map[string]cacheEntry),
	}, nil
}

func (s *ServiceTenantProvider) GetCurrentTenant(ctx context.Context) (*Tenant, error) {
	tenant, err := s.GetByID(ctx, utils.GetTenantID(ctx))
	if err != nil {
		return nil, err
	}
	return tenant, nil
}

func (s *ServiceTenantProvider) GetByID(ctx context.Context, tenantID string) (*Tenant, error) {
	if tenant := s.getFromCacheByID(tenantID); tenant != nil {
		return tenant, nil
	}

	tenant, err := s.repository.GetByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	s.saveToCache(tenant)
	return tenant, nil
}

func (s *ServiceTenantProvider) GetByHost(ctx context.Context, host string) (*Tenant, error) {
	if tenant := s.getFromCacheByHost(host); tenant != nil {
		return tenant, nil
	}

	tenant, err := s.repository.GetByHost(ctx, host)
	if err != nil {
		return nil, err
	}
	s.saveToCache(tenant)
	return tenant, nil
}

func (s *ServiceTenantProvider) CheckTenantByHostAndID(ctx context.Context, host string, tenantID string) (*Tenant, error) {
	return s.repository.CheckTenantByHostAndID(ctx, host, tenantID)
}

func (s *ServiceTenantProvider) saveToCache(tenant *Tenant) {
	if tenant == nil {
		return
	}

	entry := cacheEntry{
		tenant:  tenant,
		expires: time.Now().Add(s.cacheTTL),
	}

	s.cacheMu.Lock()
	s.cacheByID[tenant.ID] = entry
	if tenant.Host != "" {
		s.cacheByHost[normalizeHost(tenant.Host)] = entry
	}
	s.cacheMu.Unlock()
}

func (s *ServiceTenantProvider) getFromCacheByID(tenantID string) *Tenant {
	if tenantID == "" {
		return nil
	}

	s.cacheMu.RLock()
	entry, ok := s.cacheByID[tenantID]
	s.cacheMu.RUnlock()
	if !ok || time.Now().After(entry.expires) {
		if ok {
			s.cacheMu.Lock()
			delete(s.cacheByID, tenantID)
			s.cacheMu.Unlock()
		}
		return nil
	}
	return entry.tenant
}

func (s *ServiceTenantProvider) getFromCacheByHost(host string) *Tenant {
	norm := normalizeHost(host)
	if norm == "" {
		return nil
	}

	s.cacheMu.RLock()
	entry, ok := s.cacheByHost[norm]
	s.cacheMu.RUnlock()
	if !ok || time.Now().After(entry.expires) {
		if ok {
			s.cacheMu.Lock()
			delete(s.cacheByHost, norm)
			s.cacheMu.Unlock()
		}
		return nil
	}
	return entry.tenant
}

func normalizeHost(host string) string {
	return strings.TrimSpace(strings.ToLower(host))
}
