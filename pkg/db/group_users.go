package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrLastGroupAdmin is returned when an operation would leave a group without an admin.
var ErrLastGroupAdmin = errors.New("last group admin cannot be removed")

// GroupUser represents a single row in the group_users join table.
type GroupUser struct {
	GroupID int64 `json:"GroupID"`
	UserID  int64 `json:"UserID"`
}

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

// AddUserToGroup adds a user to a group.
// Returns true if a new row was inserted, false if already a member.
func (d *DB) AddUserToGroup(groupID, userID int64) (bool, error) {
	if d == nil || d.SQL == nil {
		return false, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return false, fmt.Errorf("groupID must be > 0")
	}
	if userID <= 0 {
		return false, fmt.Errorf("userID must be > 0")
	}

	res, err := d.SQL.Exec(`
INSERT INTO group_users (group_id, user_id)
VALUES (?, ?)
ON CONFLICT(group_id, user_id) DO NOTHING;
`, groupID, userID)
	if err != nil {
		return false, fmt.Errorf("add user to group: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("add user to group rows affected: %w", err)
	}
	return affected > 0, nil
}

// AddGroupMember adds a user to a group and sets or clears their explicit group role.
// Empty role means plain group membership without a memberships row.
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
	if role != "" && !IsValidGroupRole(role) {
		return false, fmt.Errorf("invalid group role %q", role)
	}

	tx, err := d.SQL.Begin()
	if err != nil {
		return false, fmt.Errorf("add group member: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`
INSERT INTO group_users (group_id, user_id)
VALUES (?, ?)
ON CONFLICT(group_id, user_id) DO NOTHING;
`, groupID, userID)
	if err != nil {
		return false, fmt.Errorf("add group member: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("add group member rows affected: %w", err)
	}

	if role == "" {
		var currentRole string
		err := tx.QueryRow(`
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
		if currentRole == RoleGroupAdmin {
			count, err := countGroupAdminsTx(tx, groupID)
			if err != nil {
				return false, err
			}
			if count <= 1 {
				return false, ErrLastGroupAdmin
			}
		}

		if _, err := tx.Exec(`
DELETE FROM memberships
 WHERE entity_type = ?
   AND entity_id   = ?
   AND user_id     = ?;
`, EntityTypeGroup, groupID, userID); err != nil {
			return false, fmt.Errorf("add group member clear role: %w", err)
		}
		count, err := countGroupAdminsTx(tx, groupID)
		if err != nil {
			return false, err
		}
		if count <= 0 {
			return false, ErrLastGroupAdmin
		}
	} else {
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
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("add group member commit: %w", err)
	}

	return affected > 0, nil
}

// RemoveUserFromGroup removes a user from a group.
// Returns true if a row was deleted, false if not found.
func (d *DB) RemoveUserFromGroup(groupID, userID int64) (bool, error) {
	if d == nil || d.SQL == nil {
		return false, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return false, fmt.Errorf("groupID must be > 0")
	}
	if userID <= 0 {
		return false, fmt.Errorf("userID must be > 0")
	}

	res, err := d.SQL.Exec(`
DELETE FROM group_users
 WHERE group_id = ?
   AND user_id  = ?;
`, groupID, userID)
	if err != nil {
		return false, fmt.Errorf("remove user from group: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("remove user from group rows affected: %w", err)
	}
	return affected > 0, nil
}

// RemoveGroupMember removes a user from group_users and clears their group membership role.
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
	if err != nil && err != sql.ErrNoRows {
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
DELETE FROM group_users
 WHERE group_id = ?
   AND user_id  = ?;
`, groupID, userID)
	if err != nil {
		return false, fmt.Errorf("remove group member: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("remove group member rows affected: %w", err)
	}

	if _, err := tx.Exec(`
DELETE FROM memberships
 WHERE entity_type = ?
   AND entity_id   = ?
   AND user_id     = ?;
`, EntityTypeGroup, groupID, userID); err != nil {
		return false, fmt.Errorf("remove group member role: %w", err)
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
  FROM group_users
 WHERE group_id = ?
   AND user_id  = ?
 LIMIT 1;
`, groupID, userID).Scan(&one)
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
  JOIN group_users gu ON gu.group_id = g.id
 WHERE gu.user_id = ?
 ORDER BY g.id ASC;
`, userID)
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

// ListUsersByGroupID returns all users belonging to the given group.
func (d *DB) ListUsersByGroupID(groupID int64) ([]User, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT u.id, u.firstname, u.lastname, u.email, u.locale, u.status, u.created_at, u.updated_at
  FROM users u
  JOIN group_users gu ON gu.user_id = u.id
 WHERE gu.group_id = ?
 ORDER BY u.id ASC;
`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list users by group: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(
			&u.ID,
			&u.FirstName,
			&u.LastName,
			&u.Email,
			&u.Locale,
			&u.Status,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user by group: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users by group: %w", err)
	}

	return out, nil
}

// ListGroupMembersByGroupID returns all users belonging to the given group
// enriched with their explicit group role, if any.
func (d *DB) ListGroupMembersByGroupID(groupID int64) ([]GroupMember, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT u.id,
       u.firstname,
       u.lastname,
       u.email,
       u.locale,
       u.status,
       COALESCE(m.role, '') AS role,
       u.created_at,
       u.updated_at
  FROM users u
  JOIN group_users gu ON gu.user_id = u.id
  LEFT JOIN memberships m ON m.entity_type = ?
                          AND m.entity_id   = gu.group_id
                          AND m.user_id     = u.id
 WHERE gu.group_id = ?
 ORDER BY u.id ASC;
`, EntityTypeGroup, groupID)
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

// GroupWithRole combines group data with the user's role in that group (from memberships).
// Role is empty if no explicit membership entry exists.
type GroupWithRole struct {
	ID        int64  `json:"ID"`
	Name      string `json:"Name"`
	Role      string `json:"Role,omitempty"`
	CreatedAt int64  `json:"CreatedAt,omitempty"`
	UpdatedAt int64  `json:"UpdatedAt,omitempty"`
}

// ListGroupsWithRoleByUserID returns all groups the user belongs to, enriched with
// the user's role from the memberships table (entity_type='group').
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
	for _, m := range memberships {
		switch {
		case m.EntityType == EntityTypeGroup:
			roleByGroup[m.EntityID] = m.Role
		case m.EntityType == EntityTypeSystem && m.EntityID == SystemGroupID:
			// System-admin membership is stored under entity_type='system',
			// but the system group (id=1) still appears in group_users.
			roleByGroup[SystemGroupID] = m.Role
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
	fmt.Println("Userid: ", userID)
	fmt.Println(roleByGroup)
	fmt.Println(memberships)
	fmt.Println(out)
	return out, nil
}

// ListAllGroupUsers returns all group_user rows. Intended for full-database exports.
func (d *DB) ListAllGroupUsers() ([]GroupUser, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT group_id, user_id
  FROM group_users
 ORDER BY group_id, user_id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list all group users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]GroupUser, 0)
	for rows.Next() {
		var gu GroupUser
		if err := rows.Scan(&gu.GroupID, &gu.UserID); err != nil {
			return nil, fmt.Errorf("scan group user: %w", err)
		}
		out = append(out, gu)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group users: %w", err)
	}

	return out, nil
}
