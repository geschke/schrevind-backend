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
		"currency:delete":                {"admin"},
		"security:delete":                {"admin"},
		"withholding-tax-default:delete": {"admin"},
		"member:add":                     {"admin"},
		"member:remove":                  {"admin"},
		"member:list":                    {"admin"},
		"user:create":                    {"admin"},
		"user:edit":                      {"admin"},
		"user:delete":                    {"admin"},
		"user:list":                      {"admin"},
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
