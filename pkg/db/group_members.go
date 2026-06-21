package db

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ErrLastGroupAdmin is returned when an operation would leave a group without an admin.
var ErrLastGroupAdmin = errors.New("last group admin cannot be removed")

// GroupMember represents a user in a group enriched with their group role.
type GroupMember struct {
	ID        int64  `json:"ID"`
	FirstName string `json:"FirstName,omitempty"`
	LastName  string `json:"LastName,omitempty"`
	Email     string `json:"Email,omitempty"`
	Locale    string `json:"Locale,omitempty"`
	Status    string `json:"Status,omitempty"`
	Role      string `json:"Role"`
	CreatedAt int64  `json:"CreatedAt,omitempty"`
	UpdatedAt int64  `json:"UpdatedAt,omitempty"`
}

// AddGroupMember adds a user to a group and sets their group role.
func (d *DB) AddGroupMember(groupID, userID int64, role string) (bool, error) {
	if d == nil || d.SQL == nil {
		return false, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return false, fmt.Errorf("groupID must be > 0")
	}
	if userID <= 0 {
		return false, fmt.Errorf("userID must be > 0")
	}
	role = strings.TrimSpace(role)
	if role == "" {
		return false, fmt.Errorf("role is required")
	}
	if !IsValidGroupRole(role) {
		return false, fmt.Errorf("invalid group role %q", role)
	}

	tx, err := d.SQL.Begin()
	if err != nil {
		return false, fmt.Errorf("add group member: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentRole string
	err = tx.QueryRow(`
SELECT role
  FROM memberships
 WHERE entity_type = ?
   AND entity_id   = ?
   AND user_id     = ?
 LIMIT 1;
`, EntityTypeGroup, groupID, userID).Scan(&currentRole)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("add group member current role: %w", err)
	}
	added := err == sql.ErrNoRows

	if currentRole == RoleGroupAdmin && role == RoleGroupMember {
		count, err := countGroupAdminsTx(tx, groupID)
		if err != nil {
			return false, err
		}
		if count <= 1 {
			return false, ErrLastGroupAdmin
		}
	}

	now := time.Now().Unix()
	if _, err := tx.Exec(`
INSERT INTO memberships (entity_type, entity_id, user_id, role, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(entity_type, entity_id, user_id) DO UPDATE SET
  role       = excluded.role,
  updated_at = excluded.updated_at;
`, EntityTypeGroup, groupID, userID, role, now, now); err != nil {
		return false, fmt.Errorf("add group member grant role: %w", err)
	}

	if role == RoleGroupMember {
		count, err := countGroupAdminsTx(tx, groupID)
		if err != nil {
			return false, err
		}
		if count <= 0 {
			return false, ErrLastGroupAdmin
		}
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("add group member commit: %w", err)
	}

	return added, nil
}

// RemoveUserFromGroup removes a user from a group.
// Returns true if a membership was deleted, false if not found.
func (d *DB) RemoveUserFromGroup(groupID, userID int64) (bool, error) {
	return d.RemoveGroupMember(groupID, userID)
}

// RemoveGroupMember removes a user's group membership.
func (d *DB) RemoveGroupMember(groupID, userID int64) (bool, error) {
	if d == nil || d.SQL == nil {
		return false, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return false, fmt.Errorf("groupID must be > 0")
	}
	if userID <= 0 {
		return false, fmt.Errorf("userID must be > 0")
	}

	tx, err := d.SQL.Begin()
	if err != nil {
		return false, fmt.Errorf("remove group member: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentRole string
	err = tx.QueryRow(`
SELECT role
  FROM memberships
 WHERE entity_type = ?
   AND entity_id   = ?
   AND user_id     = ?
 LIMIT 1;
`, EntityTypeGroup, groupID, userID).Scan(&currentRole)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("remove group member current role: %w", err)
	}
	if currentRole == RoleGroupAdmin {
		count, err := countGroupAdminsTx(tx, groupID)
		if err != nil {
			return false, err
		}
		if count <= 1 {
			return false, ErrLastGroupAdmin
		}
	}

	res, err := tx.Exec(`
DELETE FROM memberships
 WHERE entity_type = ?
   AND entity_id   = ?
   AND user_id     = ?;
`, EntityTypeGroup, groupID, userID)
	if err != nil {
		return false, fmt.Errorf("remove group member role: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("remove group member rows affected: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("remove group member commit: %w", err)
	}

	return affected > 0, nil
}

func countGroupAdminsTx(tx *sql.Tx, groupID int64) (int, error) {
	var count int
	err := tx.QueryRow(`
SELECT COUNT(*)
  FROM memberships
 WHERE entity_type = ?
   AND entity_id   = ?
   AND role        = ?;
`, EntityTypeGroup, groupID, RoleGroupAdmin).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count group admins: %w", err)
	}
	return count, nil
}

// IsUserInGroup checks whether a user is a member of the given group.
func (d *DB) IsUserInGroup(groupID, userID int64) (bool, error) {
	if d == nil || d.SQL == nil {
		return false, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return false, fmt.Errorf("groupID must be > 0")
	}
	if userID <= 0 {
		return false, fmt.Errorf("userID must be > 0")
	}

	var one int
	err := d.SQL.QueryRow(`
SELECT 1
  FROM memberships
 WHERE entity_type = ?
   AND entity_id   = ?
   AND user_id     = ?
 LIMIT 1;
`, EntityTypeGroup, groupID, userID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("is user in group: %w", err)
	}
	return true, nil
}

// ListGroupsByUserID returns all groups the given user belongs to.
func (d *DB) ListGroupsByUserID(userID int64) ([]Group, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("userID must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT g.id, g.name, g.created_at, g.updated_at
  FROM groups g
  JOIN memberships m ON m.entity_type = ?
                    AND m.entity_id   = g.id
 WHERE m.user_id = ?
 ORDER BY g.id ASC;
`, EntityTypeGroup, userID)
	if err != nil {
		return nil, fmt.Errorf("list groups by user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Group, 0)
	for rows.Next() {
		var g Group
		if err := rows.Scan(
			&g.ID,
			&g.Name,
			&g.CreatedAt,
			&g.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan group by user: %w", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate groups by user: %w", err)
	}

	return out, nil
}

// ListGroupMembersByGroupID returns all users belonging to the given group
// enriched with their group role. The reserved system group is backed by
// entity_type='system' memberships, not normal entity_type='group' rows.
func (d *DB) ListGroupMembersByGroupID(groupID int64) ([]GroupMember, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}

	entityType := EntityTypeGroup
	if groupID == SystemGroupID {
		entityType = EntityTypeSystem
	}

	rows, err := d.SQL.Query(`
SELECT u.id,
       u.firstname,
       u.lastname,
       u.email,
       u.locale,
       u.status,
       m.role,
       u.created_at,
       u.updated_at
  FROM users u
  JOIN memberships m ON m.entity_type = ?
                    AND m.entity_id   = ?
                    AND m.user_id     = u.id
 ORDER BY u.id ASC;
`, entityType, groupID)
	if err != nil {
		return nil, fmt.Errorf("list group members by group: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]GroupMember, 0)
	for rows.Next() {
		var member GroupMember
		if err := rows.Scan(
			&member.ID,
			&member.FirstName,
			&member.LastName,
			&member.Email,
			&member.Locale,
			&member.Status,
			&member.Role,
			&member.CreatedAt,
			&member.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan group member: %w", err)
		}
		out = append(out, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group members by group: %w", err)
	}

	return out, nil
}

// GroupWithRole combines group data with the user's role in that group.
type GroupWithRole struct {
	ID        int64  `json:"ID"`
	Name      string `json:"Name"`
	Role      string `json:"Role,omitempty"`
	CreatedAt int64  `json:"CreatedAt,omitempty"`
	UpdatedAt int64  `json:"UpdatedAt,omitempty"`
}

// ListGroupsWithRoleByUserID returns all groups the user belongs to, enriched with
// the user's group role from memberships.
func (d *DB) ListGroupsWithRoleByUserID(userID int64) ([]GroupWithRole, error) {
	groups, err := d.ListGroupsByUserID(userID)
	if err != nil {
		return nil, err
	}

	memberships, err := d.ListMembershipsByUser(userID)
	if err != nil {
		return nil, err
	}

	roleByGroup := make(map[int64]string, len(memberships))
	hasSystemAdmin := false
	for _, m := range memberships {
		switch {
		case m.EntityType == EntityTypeGroup:
			roleByGroup[m.EntityID] = m.Role
		case m.EntityType == EntityTypeSystem && m.EntityID == SystemGroupID:
			// System-admin membership is stored under entity_type='system',
			// but is shown as the role for the reserved system group.
			roleByGroup[SystemGroupID] = m.Role
			hasSystemAdmin = m.Role == RoleSystemAdmin
		}
	}

	if hasSystemAdmin {
		hasSystemGroup := false
		for _, g := range groups {
			if g.ID == SystemGroupID {
				hasSystemGroup = true
				break
			}
		}
		if !hasSystemGroup {
			systemGroup, found, err := d.GetGroupByID(SystemGroupID)
			if err != nil {
				return nil, err
			}
			if !found {
				return nil, fmt.Errorf("system group not found")
			}
			groups = append(groups, systemGroup)
			sort.Slice(groups, func(i, j int) bool {
				return groups[i].ID < groups[j].ID
			})
		}
	}

	out := make([]GroupWithRole, len(groups))
	for i, g := range groups {
		out[i] = GroupWithRole{
			ID:        g.ID,
			Name:      g.Name,
			Role:      roleByGroup[g.ID],
			CreatedAt: g.CreatedAt,
			UpdatedAt: g.UpdatedAt,
		}
	}
	return out, nil
}
