// Package security provides RBAC capabilities, risk assessment, and security
// policy enforcement for the SecOps agent.
package security

import (
	"slices"
	"strings"
	"sync"
)

// Role represents a user's operational role within the SecOps agent.
type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
	RoleAnalyst  Role = "analyst"
	RoleResponder Role = "responder"
)

// Capability represents a single permission grant in the form "resource:action"
// or "resource:action:scope".
type Capability string

// CapabilityPolicy defines a set of allowed or blocked capabilities.
type CapabilityPolicy struct {
	Mode  string       `json:"mode"` // "allowlist" or "blocklist"
	Rules []Capability `json:"rules"`
}

// Matches returns true if the given capability string is covered by this
// individual capability rule. Supports wildcard matching with "*".
func (c Capability) Matches(requested string) bool {
	rule := string(c)
	if rule == "*" {
		return true
	}
	if rule == requested {
		return true
	}
	// Support wildcard suffix: "file:read:*" matches "file:read:/var/log/syslog"
	if strings.HasSuffix(rule, ":*") {
		prefix := strings.TrimSuffix(rule, "*")
		if strings.HasPrefix(requested, prefix) {
			return true
		}
	}
	// Support wildcard at resource level: "file:*" matches "file:read" and "file:write:/etc"
	parts := strings.SplitN(rule, ":", 2)
	reqParts := strings.SplitN(requested, ":", 2)
	if len(parts) == 2 && len(reqParts) >= 2 && parts[0] == reqParts[0] && parts[1] == "*" {
		return true
	}
	return false
}

// OpsRoleCapabilities maps operational roles to their allowed capabilities.
var OpsRoleCapabilities = map[Role][]Capability{
	RoleViewer: {
		"file:read:/var/log/*",
		"file:read:/etc/*",
		"monitoring:query",
		"log:analyze",
		"compliance:check",
		"process:read",
		"network:read",
	},
	RoleOperator: {
		"file:read:*",
		"file:write:/var/log/*",
		"shell:read-only",
		"shell:safe",
		"network:diagnostic",
		"process:read",
		"process:signal",
		"database:query",
		"monitoring:query",
		"log:analyze",
		"compliance:check",
		"security:scan",
		"certificate:audit",
	},
	RoleAdmin: {
		"shell:*",
		"file:*",
		"network:*",
		"process:*",
		"database:*",
		"monitoring:*",
		"log:*",
		"compliance:*",
		"security:*",
		"certificate:*",
		"container:*",
	},
}

// SecurityRoleCapabilities maps security-specific roles to their allowed capabilities.
var SecurityRoleCapabilities = map[Role][]Capability{
	RoleAnalyst: {
		"security:scan",
		"security:vulnerability",
		"compliance:check",
		"compliance:report",
		"file:read:*",
		"log:analyze",
		"network:read",
		"certificate:audit",
	},
	RoleResponder: {
		"security:*",
		"compliance:*",
		"file:*",
		"log:*",
		"container:exec",
		"process:kill",
		"network:*",
		"certificate:*",
	},
}

// CapabilityManager provides thread-safe capability checking and management.
type CapabilityManager struct {
	mu             sync.RWMutex
	roleMap        map[Role][]Capability
	userRoles      map[string]Role // userID -> role
	customPolicies map[string]*CapabilityPolicy // userID -> custom overrides
}

// NewCapabilityManager creates a new manager with the default ops and security
// role definitions merged.
func NewCapabilityManager() *CapabilityManager {
	merged := make(map[Role][]Capability)
	for role, caps := range OpsRoleCapabilities {
		merged[role] = append(merged[role], caps...)
	}
	for role, caps := range SecurityRoleCapabilities {
		merged[role] = append(merged[role], caps...)
	}
	return &CapabilityManager{
		roleMap:        merged,
		userRoles:      make(map[string]Role),
		customPolicies: make(map[string]*CapabilityPolicy),
	}
}

// SetUserRole assigns a role to a user.
func (cm *CapabilityManager) SetUserRole(userID string, role Role) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.userRoles[userID] = role
}

// GetUserRole returns the role assigned to a user, defaulting to RoleViewer.
func (cm *CapabilityManager) GetUserRole(userID string) Role {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if role, ok := cm.userRoles[userID]; ok {
		return role
	}
	return RoleViewer
}

// SetCustomPolicy assigns a custom capability policy to a user that overrides
// the role-based policy.
func (cm *CapabilityManager) SetCustomPolicy(userID string, policy *CapabilityPolicy) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.customPolicies[userID] = policy
}

// CheckCapability returns true if the user has the requested capability.
func (cm *CapabilityManager) CheckCapability(userID, capability string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Check custom policy first
	if policy, ok := cm.customPolicies[userID]; ok {
		return cm.checkPolicy(policy, capability)
	}

	// Fall back to role-based check
	role, ok := cm.userRoles[userID]
	if !ok {
		role = RoleViewer
	}

	caps, ok := cm.roleMap[role]
	if !ok {
		return false
	}

	for _, cap := range caps {
		if cap.Matches(capability) {
			return true
		}
	}
	return false
}

// GetCapabilities returns all capabilities for a given role.
func (cm *CapabilityManager) GetCapabilities(role Role) []Capability {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return slices.Clone(cm.roleMap[role])
}

func (cm *CapabilityManager) checkPolicy(policy *CapabilityPolicy, capability string) bool {
	matched := false
	for _, rule := range policy.Rules {
		if rule.Matches(capability) {
			matched = true
			break
		}
	}
	if policy.Mode == "blocklist" {
		return !matched
	}
	return matched // allowlist mode
}
