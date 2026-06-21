package grrt

import "fmt"

// permissions defines which roles are allowed to perform which actions per entity type.
var permissions = map[string]map[string][]string{
	"system": {
		"group:create":   {"admin"},
		"group:edit":     {"admin"},
		"group:delete":   {"admin"},
		"group:list":     {"admin"},
		"user:create":    {"admin"},
		"user:edit":      {"admin"},
		"user:delete":    {"admin"},
		"user:list":      {"admin"},
		"member:add":     {"admin"},
		"member:remove":  {"admin"},
		"depot:delete":   {"admin"},
		"depot:transfer": {"admin"},
	},
	"group": {
		"group:view":                     {"admin"},
		"group:edit":                     {"admin"},
		"group:delete":                   {"admin"},
		"group:list:accessible":          {"admin", "member"},
		"currency:list":                  {"admin", "member"},
		"currency:add":                   {"admin", "member"},
		"currency:edit":                  {"admin", "member"},
		"currency:delete":                {"admin"},
		"security:list":                  {"admin", "member"},
		"security:add":                   {"admin", "member"},
		"security:edit":                  {"admin", "member"},
		"security:delete":                {"admin"},
		"withholding-tax-default:list":   {"admin", "member"},
		"withholding-tax-default:add":    {"admin", "member"},
		"withholding-tax-default:edit":   {"admin", "member"},
		"withholding-tax-default:delete": {"admin"},
		"member:remove":                  {"admin"},
		"member:list":                    {"admin", "member"},
		"user:create":                    {"admin"},
		"user:edit":                      {"admin"},
		"user:delete":                    {"admin"},
		"depot:create":                   {"admin"},
		"depot:list":                     {"admin"},
	},
	"depot": {
		"depot:rename":                 {"owner"},
		"depot:delete":                 {"owner"},
		"depot:deactivate":             {"owner"},
		"depot:reactivate":             {"owner"},
		"depot:export":                 {"owner", "editor"},
		"depot:access:add":             {"owner"},
		"depot:access:remove":          {"owner"},
		"depot:access:change":          {"owner"},
		"depot:access:list":            {"owner", "editor", "viewer"},
		"entries:list":                 {"owner", "editor", "viewer"},
		"entries:create":               {"owner", "editor"},
		"entries:edit":                 {"owner", "editor"},
		"entries:delete":               {"owner", "editor"},
		"withholding-tax-default:edit": {"owner", "editor"},
	},
}

// AllowedRoles returns a copy of the roles that are allowed for the action.
func AllowedRoles(entityType, action string) ([]string, error) {
	entityPerms, ok := permissions[entityType]
	if !ok {
		return nil, fmt.Errorf("unknown entity type %q", entityType)
	}

	roles, ok := entityPerms[action]
	if !ok {
		return nil, fmt.Errorf("unknown action %q for entity type %q", action, entityType)
	}

	return append([]string(nil), roles...), nil
}
