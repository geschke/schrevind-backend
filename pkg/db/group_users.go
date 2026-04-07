package db

import (
	"database/sql"
	"fmt"
)

// GroupUser represents a single row in the group_users join table.
type GroupUser struct {
	GroupID int64 `json:"GroupID"`
	UserID  int64 `json:"UserID"`
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
SELECT u.id, u.firstname, u.lastname, u.email, u.status, u.created_at, u.updated_at
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
