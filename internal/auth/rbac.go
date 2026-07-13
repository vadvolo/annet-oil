package auth

import (
	"path/filepath"
	"strings"

	"annet-oil/internal/config"
)

type Role struct {
	Name         string
	DeviceScopes []string
	Methods      []string
	Commands     []string
}

type User struct {
	Name string
	Role *Role
}

type RBAC struct {
	roles       map[string]*Role
	users       map[string]*User // token -> user
	legacyToken string
}

func NewRBAC(cfg config.AuthConfig, legacyToken string) *RBAC {
	rbac := &RBAC{
		roles:       make(map[string]*Role),
		users:       make(map[string]*User),
		legacyToken: legacyToken,
	}

	rbac.roles["admin"] = &Role{
		Name:         "admin",
		DeviceScopes: []string{"*"},
		Methods:      []string{"*"},
		Commands:     []string{"*"},
	}

	for _, r := range cfg.Roles {
		rbac.roles[r.Name] = &Role{
			Name:         r.Name,
			DeviceScopes: r.DeviceScopes,
			Methods:      r.Methods,
			Commands:     r.Commands,
		}
	}

	userRoles := make(map[string]string)
	for _, g := range cfg.Groups {
		for _, member := range g.Members {
			if _, exists := userRoles[member]; !exists {
				userRoles[member] = g.Role
			}
		}
	}

	for _, u := range cfg.Users {
		roleName := u.Role
		if roleName == "" {
			if gr, ok := userRoles[u.Name]; ok {
				roleName = gr
			}
		}

		role := rbac.roles[roleName]
		if role == nil {
			role = rbac.roles["admin"]
		}

		rbac.users[u.Token] = &User{
			Name: u.Name,
			Role: role,
		}
	}

	return rbac
}

func (r *RBAC) GetUser(token string) *User {
	if user, ok := r.users[token]; ok {
		return user
	}

	if r.legacyToken != "" && token == r.legacyToken {
		return &User{
			Name: "legacy",
			Role: r.roles["admin"],
		}
	}

	return nil
}

func (r *RBAC) CanAccess(user *User, endpoint string) bool {
	if user == nil || user.Role == nil {
		return false
	}

	if containsWildcard(user.Role.Methods) {
		return true
	}

	for _, method := range user.Role.Methods {
		if strings.Contains(endpoint, method) {
			return true
		}
	}

	return false
}

func (r *RBAC) CanExecuteCommand(user *User, command string) bool {
	if user == nil || user.Role == nil {
		return false
	}

	if containsWildcard(user.Role.Commands) {
		return true
	}

	cmd := strings.ToLower(strings.TrimSpace(command))
	for _, allowed := range user.Role.Commands {
		pattern := strings.ToLower(allowed)
		if strings.HasPrefix(cmd, pattern) {
			return true
		}
		if strings.Contains(pattern, "*") {
			if matched, _ := filepath.Match(pattern, cmd); matched {
				return true
			}
		}
	}

	return false
}

func (r *RBAC) FilterDevices(user *User, hostnames []string) []string {
	if user == nil || user.Role == nil {
		return nil
	}

	if containsWildcard(user.Role.DeviceScopes) {
		return hostnames
	}

	var allowed []string
	for _, host := range hostnames {
		for _, scope := range user.Role.DeviceScopes {
			if matchPattern(scope, host) {
				allowed = append(allowed, host)
				break
			}
		}
	}

	return allowed
}

func containsWildcard(items []string) bool {
	for _, s := range items {
		if s == "*" {
			return true
		}
	}
	return false
}

func matchPattern(pattern, hostname string) bool {
	if pattern == "*" {
		return true
	}
	matched, _ := filepath.Match(pattern, hostname)
	return matched
}
