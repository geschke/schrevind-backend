// Package grrt - Groups, Roles, Rights, Tacken
// "DU kommst hier ned rein."
package grrt

import (
	"fmt"
	"slices"

	"github.com/geschke/schrevind/pkg/db"
)

// Grrt is the permission checker.
type Grrt struct {
	db *db.DB
}

// ActionScope describes the effective scope for a many-entity action.
type ActionScope struct {
	All   bool
	Roles []string
}

// New creates a new Grrt instance.
func New(database *db.DB) *Grrt {
	return &Grrt{db: database}
}

// Can checks whether userID may perform a system-wide action.
// Equivalent to CanDo(userID, "system", action, 1).
func (g *Grrt) Can(userID int64, action string) (bool, error) {
	return g.CanDo(userID, db.EntityTypeSystem, action, db.SystemGroupID)
}

func (g *Grrt) isSystemAdmin(userID int64) (bool, error) {
	if g == nil || g.db == nil {
		return false, fmt.Errorf("grrt not initialized")
	}
	if userID <= 0 {
		return false, fmt.Errorf("userID must be > 0")
	}

	sysM, found, err := g.db.GetMembership(db.EntityTypeSystem, db.SystemGroupID, userID)
	if err != nil {
		return false, fmt.Errorf("grrt system check: %w", err)
	}
	return found && sysM.Role == db.RoleSystemAdmin, nil
}

// CanDo checks whether userID may perform action on (entityType, entityID).
//
// Check order:
//  1. Does the user have a system-admin membership? If yes, allow everything.
//  2. Does the user have a membership for (entityType, entityID)?
//     If yes, check whether their role appears in permissions[entityType][action].
func (g *Grrt) CanDo(userID int64, entityType string, action string, entityID int64) (bool, error) {
	if g == nil || g.db == nil {
		return false, fmt.Errorf("grrt not initialized")
	}
	if userID <= 0 {
		return false, fmt.Errorf("userID must be > 0")
	}

	// Step 1: system-admin bypass (unless we are already checking a system action
	// to avoid infinite recursion).
	if entityType != db.EntityTypeSystem {
		isSystemAdmin, err := g.isSystemAdmin(userID)
		if err != nil {
			return false, err
		}
		if isSystemAdmin {
			return true, nil
		}
	}

	// Step 2: entity-level membership check.
	m, found, err := g.db.GetMembership(entityType, entityID, userID)
	if err != nil {
		return false, fmt.Errorf("grrt entity check: %w", err)
	}
	if !found {
		return false, nil
	}

	return roleAllowed(entityType, action, m.Role), nil
}

// CanDoAny checks whether userID may perform action on at least one entity of entityType.
func (g *Grrt) CanDoAny(userID int64, entityType string, action string) (bool, error) {
	if g == nil || g.db == nil {
		return false, fmt.Errorf("grrt not initialized")
	}
	if userID <= 0 {
		return false, fmt.Errorf("userID must be > 0")
	}
	if entityType == db.EntityTypeSystem {
		return g.Can(userID, action)
	}

	isSystemAdmin, err := g.isSystemAdmin(userID)
	if err != nil {
		return false, err
	}
	if isSystemAdmin {
		return true, nil
	}

	roles, err := AllowedRoles(entityType, action)
	if err != nil {
		return false, err
	}

	return g.db.UserHasAnyMembershipWithRoles(userID, entityType, roles)
}

// ScopeForAction returns the effective scope for a many-entity action.
func (g *Grrt) ScopeForAction(userID int64, entityType string, action string) (ActionScope, error) {
	if g == nil || g.db == nil {
		return ActionScope{}, fmt.Errorf("grrt not initialized")
	}
	if userID <= 0 {
		return ActionScope{}, fmt.Errorf("userID must be > 0")
	}

	isSystemAdmin, err := g.isSystemAdmin(userID)
	if err != nil {
		return ActionScope{}, err
	}
	if isSystemAdmin && entityType != db.EntityTypeSystem {
		return ActionScope{All: true}, nil
	}

	roles, err := AllowedRoles(entityType, action)
	if err != nil {
		return ActionScope{}, err
	}

	return ActionScope{
		All:   false,
		Roles: roles,
	}, nil
}

// roleAllowed returns true if role is listed for action under entityType.
func roleAllowed(entityType, action, role string) bool {
	entityPerms, ok := permissions[entityType]
	if !ok {
		return false
	}
	allowed, ok := entityPerms[action]
	if !ok {
		return false
	}
	return slices.Contains(allowed, role)
}
