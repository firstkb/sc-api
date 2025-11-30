package auth

import (
	"time"

	"github.com/google/uuid"
)

// AccessLevel represents user access level
type AccessLevel int

const (
	LevelRoot    AccessLevel = 100 // root - manages all tenants
	LevelAdmin   AccessLevel = 80  // admin - full access within tenant
	LevelManager AccessLevel = 60  // manager - manages surveys/contacts/exports
	LevelUser    AccessLevel = 40  // user - basic access
	LevelViewer  AccessLevel = 20  // viewer - read-only access
)

// Role represents user role name
type Role string

const (
	RoleRoot    Role = "root"
	RoleAdmin   Role = "admin"
	RoleManager Role = "manager"
	RoleUser    Role = "user"
	RoleViewer  Role = "viewer"
)

// LevelToRole maps access level to role name
func LevelToRole(level AccessLevel) Role {
	switch {
	case level >= LevelRoot:
		return RoleRoot
	case level >= LevelAdmin:
		return RoleAdmin
	case level >= LevelManager:
		return RoleManager
	case level >= LevelUser:
		return RoleUser
	default:
		return RoleViewer
	}
}

// RoleToLevel maps role name to access level
func RoleToLevel(role Role) AccessLevel {
	switch role {
	case RoleRoot:
		return LevelRoot
	case RoleAdmin:
		return LevelAdmin
	case RoleManager:
		return LevelManager
	case RoleUser:
		return LevelUser
	case RoleViewer:
		return LevelViewer
	default:
		return LevelUser
	}
}

// Membership represents a user's membership in a tenant
type Membership struct {
	TenantUserID string
	TenantID     int64
	IdentityID   int64
	Role         Role
	Level        AccessLevel
	Status       string // "active", "inactive", "suspended"
}

// AdminUser represents a root-level administrator in the master database
type AdminUser struct {
	ID        uuid.UUID
	Email     string
	Name      *string
	Level     AccessLevel
	Status    string // "active", "inactive", "suspended"
	CreatedAt time.Time
}

// CheckLevel checks if user level meets the required level
func CheckLevel(requiredLevel AccessLevel, userLevel AccessLevel) bool {
	return userLevel >= requiredLevel
}

// RequireLevel returns true if user has at least the required level
func RequireLevel(userLevel AccessLevel, requiredLevel AccessLevel) bool {
	return userLevel >= requiredLevel
}
