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

func (g *Grrt) checkContextGroup(userID, contextGroupID int64) (sudo bool, allowed bool, err error) {
	if g == nil || g.db == nil {
		return false, false, fmt.Errorf("grrt not initialized")
	}
	if userID <= 0 {
		return false, false, fmt.Errorf("userID must be > 0")
	}
	if contextGroupID <= 0 {
		return false, false, fmt.Errorf("contextGroupID must be > 0")
	}

	if contextGroupID == db.SystemGroupID {
		isSystemAdmin, err := g.isSystemAdmin(userID)
		if err != nil {
			return false, false, err
		}
		return isSystemAdmin, isSystemAdmin, nil
	}

	inContextGroup, err := g.db.IsUserInGroup(contextGroupID, userID)
	if err != nil {
		return false, false, fmt.Errorf("grrt context check: %w", err)
	}
	return false, inContextGroup, nil
}

// ContextGroupAllowed checks whether userID may act in contextGroupID.
// It returns true for system-admin sudo in the system group context, or for members of a normal group context.
func (g *Grrt) ContextGroupAllowed(userID, contextGroupID int64) (bool, error) {
	_, allowed, err := g.checkContextGroup(userID, contextGroupID)
	return allowed, err
}

// CanWithContext checks whether userID may perform a system-wide action in the active context.
func (g *Grrt) CanWithContext(userID, contextGroupID int64, action string) (bool, error) {
	return g.CanDoWithContext(userID, contextGroupID, db.EntityTypeSystem, action, db.SystemGroupID)
}

// CanDoWithContext checks whether userID may perform action on (entityType, entityID)
// while acting in contextGroupID. System-admin sudo rights only apply in the system context.
func (g *Grrt) CanDoWithContext(userID, contextGroupID int64, entityType string, action string, entityID int64) (bool, error) {
	if entityID <= 0 {
		return false, fmt.Errorf("entityID must be > 0")
	}

	sudo, contextAllowed, err := g.checkContextGroup(userID, contextGroupID)
	if err != nil {
		return false, err
	}
	if !contextAllowed {
		return false, nil
	}
	if sudo {
		return true, nil
	}

	m, found, err := g.db.GetMembership(entityType, entityID, userID)
	if err != nil {
		return false, fmt.Errorf("grrt entity check: %w", err)
	}
	if !found {
		return false, nil
	}

	return roleAllowed(entityType, action, m.Role), nil
}

// CanDoAnyWithContext checks whether userID may perform action on at least one entity
// of entityType while acting in contextGroupID.
func (g *Grrt) CanDoAnyWithContext(userID, contextGroupID int64, entityType string, action string) (bool, error) {
	sudo, contextAllowed, err := g.checkContextGroup(userID, contextGroupID)
	if err != nil {
		return false, err
	}
	if !contextAllowed {
		return false, nil
	}
	if sudo {
		return true, nil
	}
	if entityType == db.EntityTypeSystem {
		return g.CanDoWithContext(userID, contextGroupID, entityType, action, db.SystemGroupID)
	}

	roles, err := AllowedRoles(entityType, action)
	if err != nil {
		return false, err
	}

	return g.db.UserHasAnyMembershipWithRoles(userID, entityType, roles)
}

// ScopeForActionWithContext returns the effective scope for a many-entity action
// while acting in contextGroupID.
func (g *Grrt) ScopeForActionWithContext(userID, contextGroupID int64, entityType string, action string) (ActionScope, error) {
	sudo, contextAllowed, err := g.checkContextGroup(userID, contextGroupID)
	if err != nil {
		return ActionScope{}, err
	}
	if !contextAllowed {
		return ActionScope{}, nil
	}
	if sudo && entityType != db.EntityTypeSystem {
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
