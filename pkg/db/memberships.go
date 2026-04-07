package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Entity type constants for use in memberships and audit_log.
const (
	EntityTypeSystem        = "system"
	EntityTypeGroup         = "group"
	EntityTypeDepot         = "depot"
	EntityTypeDividendEntry = "dividend_entry"
)

// Role constants per entity type.
const (
	RoleSystemAdmin = "admin"

	RoleGroupAdmin = "admin"

	RoleDepotOwner  = "owner"
	RoleDepotEditor = "editor"
	RoleDepotViewer = "viewer"
)

// ValidRoles lists the allowed roles per entity type.
var ValidRoles = map[string][]string{
	EntityTypeSystem: {RoleSystemAdmin},
	EntityTypeGroup:  {RoleGroupAdmin},
	EntityTypeDepot:  {RoleDepotOwner, RoleDepotEditor, RoleDepotViewer},
}

type Membership struct {
	EntityType string `json:"EntityType"`
	EntityID   int64  `json:"EntityID"`
	UserID     int64  `json:"UserID"`
	Role       string `json:"Role"`
	CreatedAt  int64  `json:"CreatedAt,omitempty"`
	UpdatedAt  int64  `json:"UpdatedAt,omitempty"`
}

// IsValidDepotRole returns true if role is a valid depot role.
func IsValidDepotRole(role string) bool {
	return isValidRole(EntityTypeDepot, role)
}

func isValidRole(entityType, role string) bool {
	roles, ok := ValidRoles[entityType]
	if !ok {
		return false
	}
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

func sqlPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}

	placeholders := make([]string, count)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ", ")
}

func scanMembership(row *sql.Row) (Membership, error) {
	var m Membership
	if err := row.Scan(
		&m.EntityType,
		&m.EntityID,
		&m.UserID,
		&m.Role,
		&m.CreatedAt,
		&m.UpdatedAt,
	); err != nil {
		return Membership{}, err
	}
	return m, nil
}

// GrantMembership inserts or updates a membership (upsert).
// Use this both for initial grants and role changes.
func (d *DB) GrantMembership(m *Membership) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if m == nil {
		return fmt.Errorf("membership is nil")
	}
	if m.EntityID <= 0 {
		return fmt.Errorf("entity_id must be > 0")
	}
	if m.UserID <= 0 {
		return fmt.Errorf("user_id must be > 0")
	}
	if !isValidRole(m.EntityType, m.Role) {
		return fmt.Errorf("invalid role %q for entity type %q", m.Role, m.EntityType)
	}

	now := time.Now().Unix()
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	_, err := d.SQL.Exec(`
INSERT INTO memberships (entity_type, entity_id, user_id, role, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(entity_type, entity_id, user_id) DO UPDATE SET
  role       = excluded.role,
  updated_at = excluded.updated_at;
`, m.EntityType, m.EntityID, m.UserID, m.Role, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return fmt.Errorf("grant membership: %w", err)
	}

	return nil
}

// RevokeMembership removes a membership record.
// Returns true if a row was deleted, false if not found.
func (d *DB) RevokeMembership(entityType string, entityID, userID int64) (bool, error) {
	if d == nil || d.SQL == nil {
		return false, fmt.Errorf("db not initialized")
	}
	if entityID <= 0 {
		return false, fmt.Errorf("entity_id must be > 0")
	}
	if userID <= 0 {
		return false, fmt.Errorf("user_id must be > 0")
	}

	res, err := d.SQL.Exec(`
DELETE FROM memberships
 WHERE entity_type = ?
   AND entity_id   = ?
   AND user_id     = ?;
`, entityType, entityID, userID)
	if err != nil {
		return false, fmt.Errorf("revoke membership: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("revoke membership rows affected: %w", err)
	}
	return affected > 0, nil
}

// GetMembership returns the membership for the given entity and user.
func (d *DB) GetMembership(entityType string, entityID, userID int64) (Membership, bool, error) {
	if d == nil || d.SQL == nil {
		return Membership{}, false, fmt.Errorf("db not initialized")
	}
	if entityID <= 0 {
		return Membership{}, false, fmt.Errorf("entity_id must be > 0")
	}
	if userID <= 0 {
		return Membership{}, false, fmt.Errorf("user_id must be > 0")
	}

	row := d.SQL.QueryRow(`
SELECT entity_type, entity_id, user_id, role, created_at, updated_at
  FROM memberships
 WHERE entity_type = ?
   AND entity_id   = ?
   AND user_id     = ?
 LIMIT 1;
`, entityType, entityID, userID)

	m, err := scanMembership(row)
	if err == sql.ErrNoRows {
		return Membership{}, false, nil
	}
	if err != nil {
		return Membership{}, false, fmt.Errorf("get membership: %w", err)
	}

	return m, true, nil
}

// ListMembershipsByEntity returns all memberships for a given entity.
func (d *DB) ListMembershipsByEntity(entityType string, entityID int64) ([]Membership, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if entityID <= 0 {
		return nil, fmt.Errorf("entity_id must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT entity_type, entity_id, user_id, role, created_at, updated_at
  FROM memberships
 WHERE entity_type = ?
   AND entity_id   = ?
 ORDER BY user_id ASC;
`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list memberships by entity: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Membership, 0)
	for rows.Next() {
		var m Membership
		if err := rows.Scan(
			&m.EntityType,
			&m.EntityID,
			&m.UserID,
			&m.Role,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memberships by entity: %w", err)
	}

	return out, nil
}

// ListAllMemberships returns all membership rows. Intended for full-database exports.
func (d *DB) ListAllMemberships() ([]Membership, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT entity_type, entity_id, user_id, role, created_at, updated_at
  FROM memberships
 ORDER BY entity_type, entity_id, user_id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list all memberships: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Membership, 0)
	for rows.Next() {
		var m Membership
		if err := rows.Scan(
			&m.EntityType,
			&m.EntityID,
			&m.UserID,
			&m.Role,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan membership for export: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memberships for export: %w", err)
	}

	return out, nil
}

// CountDepotOwnerMemberships returns the number of owner memberships for the given depot.
func (d *DB) CountDepotOwnerMemberships(depotID int64) (int, error) {
	if d == nil || d.SQL == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	if depotID <= 0 {
		return 0, fmt.Errorf("depotID must be > 0")
	}

	var count int
	err := d.SQL.QueryRow(`
SELECT COUNT(*)
  FROM memberships
 WHERE entity_type = ?
   AND entity_id   = ?
   AND role        = ?;
`, EntityTypeDepot, depotID, RoleDepotOwner).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count depot owners: %w", err)
	}
	return count, nil
}

// ListMembershipsByUser returns all memberships for a given user.
func (d *DB) ListMembershipsByUser(userID int64) ([]Membership, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("user_id must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT entity_type, entity_id, user_id, role, created_at, updated_at
  FROM memberships
 WHERE user_id = ?
 ORDER BY entity_type, entity_id ASC;
`, userID)
	if err != nil {
		return nil, fmt.Errorf("list memberships by user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Membership, 0)
	for rows.Next() {
		var m Membership
		if err := rows.Scan(
			&m.EntityType,
			&m.EntityID,
			&m.UserID,
			&m.Role,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memberships by user: %w", err)
	}

	return out, nil
}

// UserHasAnyMembershipWithRoles returns true if the user has at least one membership
// for the given entity type with any of the provided roles.
func (d *DB) UserHasAnyMembershipWithRoles(userID int64, entityType string, roles []string) (bool, error) {
	if d == nil || d.SQL == nil {
		return false, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return false, fmt.Errorf("user_id must be > 0")
	}
	if len(roles) == 0 {
		return false, nil
	}

	args := make([]any, 0, 2+len(roles))
	args = append(args, userID, entityType)
	for _, role := range roles {
		args = append(args, role)
	}

	query := `
SELECT 1
  FROM memberships
 WHERE user_id     = ?
   AND entity_type = ?
   AND role IN (` + sqlPlaceholders(len(roles)) + `)
 LIMIT 1;
`

	var one int
	err := d.SQL.QueryRow(query, args...).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("user has any membership with roles: %w", err)
	}
	return true, nil
}
