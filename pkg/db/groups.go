package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SystemGroupID is the reserved ID of the system group (analogous to gid=0 in Unix).
// It is set during migration and must never be deleted or renamed.
const SystemGroupID = int64(1)

type Group struct {
	ID        int64  `json:"ID"`
	Name      string `json:"Name,omitempty"`
	CreatedAt int64  `json:"CreatedAt,omitempty"`
	UpdatedAt int64  `json:"UpdatedAt,omitempty"`
}

func normalizeGroup(g Group) (Group, error) {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return Group{}, fmt.Errorf("name is required")
	}

	now := time.Now().Unix()
	if g.CreatedAt == 0 {
		g.CreatedAt = now
	}
	g.UpdatedAt = now

	return g, nil
}

func scanGroup(row *sql.Row) (Group, error) {
	var g Group
	if err := row.Scan(
		&g.ID,
		&g.Name,
		&g.CreatedAt,
		&g.UpdatedAt,
	); err != nil {
		return Group{}, err
	}
	return g, nil
}

// CreateGroup creates a new record.
func (d *DB) CreateGroup(g *Group) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if g == nil {
		return fmt.Errorf("group is nil")
	}

	normalized, err := normalizeGroup(*g)
	if err != nil {
		return err
	}

	res, err := d.SQL.Exec(`
INSERT INTO groups (name, created_at, updated_at)
VALUES (?, ?, ?);
`, normalized.Name, normalized.CreatedAt, normalized.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create group: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("create group last_insert_id: %w", err)
	}

	normalized.ID = id
	*g = normalized
	return nil
}

// CreateGroupWithDefaultCurrencies creates a group and copies template currencies into it.
func (d *DB) CreateGroupWithDefaultCurrencies(g *Group) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if g == nil {
		return fmt.Errorf("group is nil")
	}

	normalized, err := normalizeGroup(*g)
	if err != nil {
		return err
	}

	tx, err := d.SQL.Begin()
	if err != nil {
		return fmt.Errorf("create group with default currencies: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`
INSERT INTO groups (name, created_at, updated_at)
VALUES (?, ?, ?);
`, normalized.Name, normalized.CreatedAt, normalized.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create group with default currencies: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("create group with default currencies last_insert_id: %w", err)
	}

	now := time.Now().Unix()
	if _, err := tx.Exec(`
INSERT INTO currencies (
  group_id, currency, name, decimal_places, status, created_at, updated_at
)
SELECT ?, currency, name, decimal_places, status, ?, ?
  FROM currencies
 WHERE group_id = 0;
`, id, now, now); err != nil {
		return fmt.Errorf("create group with default currencies: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create group with default currencies commit: %w", err)
	}

	normalized.ID = id
	normalized.UpdatedAt = now
	*g = normalized
	return nil
}

// CreateGroupWithDefaultCurrenciesAndAdmin creates a group, copies template currencies,
// adds the creator to group_users, and grants the creator the group admin membership.
func (d *DB) CreateGroupWithDefaultCurrenciesAndAdmin(g *Group, userID int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if g == nil {
		return fmt.Errorf("group is nil")
	}
	if userID <= 0 {
		return fmt.Errorf("user_id must be > 0")
	}

	normalized, err := normalizeGroup(*g)
	if err != nil {
		return err
	}

	tx, err := d.SQL.Begin()
	if err != nil {
		return fmt.Errorf("create group with default currencies and admin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`
INSERT INTO groups (name, created_at, updated_at)
VALUES (?, ?, ?);
`, normalized.Name, normalized.CreatedAt, normalized.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create group with default currencies and admin: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("create group with default currencies and admin last_insert_id: %w", err)
	}

	now := time.Now().Unix()
	if _, err := tx.Exec(`
INSERT INTO currencies (
  group_id, currency, name, decimal_places, status, created_at, updated_at
)
SELECT ?, currency, name, decimal_places, status, ?, ?
  FROM currencies
 WHERE group_id = 0;
`, id, now, now); err != nil {
		return fmt.Errorf("create group with default currencies and admin: %w", err)
	}

	if _, err := tx.Exec(`
INSERT INTO group_users (group_id, user_id)
VALUES (?, ?)
ON CONFLICT(group_id, user_id) DO NOTHING;
`, id, userID); err != nil {
		return fmt.Errorf("create group with default currencies and admin: %w", err)
	}

	if _, err := tx.Exec(`
INSERT INTO memberships (entity_type, entity_id, user_id, role, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(entity_type, entity_id, user_id) DO UPDATE SET
  role = excluded.role,
  updated_at = excluded.updated_at;
`, EntityTypeGroup, id, userID, RoleGroupAdmin, now, now); err != nil {
		return fmt.Errorf("create group with default currencies and admin: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create group with default currencies and admin commit: %w", err)
	}

	normalized.ID = id
	normalized.UpdatedAt = now
	*g = normalized
	return nil
}

// GetGroupByID returns data for the requested input.
func (d *DB) GetGroupByID(id int64) (Group, bool, error) {
	if d == nil || d.SQL == nil {
		return Group{}, false, fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return Group{}, false, fmt.Errorf("id must be > 0")
	}

	row := d.SQL.QueryRow(`
SELECT id, name, created_at, updated_at
  FROM groups
 WHERE id = ?
 LIMIT 1;
`, id)

	g, err := scanGroup(row)
	if err == sql.ErrNoRows {
		return Group{}, false, nil
	}
	if err != nil {
		return Group{}, false, fmt.Errorf("get group by id: %w", err)
	}

	return g, true, nil
}

// UpdateGroup updates the group record by ID.
// The system group (SystemGroupID) cannot be renamed.
func (d *DB) UpdateGroup(g *Group) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if g == nil {
		return fmt.Errorf("group is nil")
	}
	if g.ID <= 0 {
		return fmt.Errorf("id must be > 0")
	}
	if g.ID == SystemGroupID {
		return fmt.Errorf("system group cannot be modified")
	}

	normalized, err := normalizeGroup(*g)
	if err != nil {
		return err
	}

	_, err = d.SQL.Exec(`
UPDATE groups
   SET name = ?,
       updated_at = ?
 WHERE id = ?;
`, normalized.Name, normalized.UpdatedAt, normalized.ID)
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}

	*g = normalized
	return nil
}

// DeleteGroup deletes the group record by ID.
// The system group (SystemGroupID) cannot be deleted.
func (d *DB) DeleteGroup(id int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}
	if id == SystemGroupID {
		return fmt.Errorf("system group cannot be deleted")
	}

	tx, err := d.SQL.Begin()
	if err != nil {
		return fmt.Errorf("delete group begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM withholding_tax_defaults WHERE group_id = ?;`, id); err != nil {
		return fmt.Errorf("delete group withholding tax defaults: %w", err)
	}

	_, err = tx.Exec(`DELETE FROM groups WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete group commit: %w", err)
	}

	return nil
}

// ListGroups returns all group rows.
func (d *DB) ListGroups() ([]Group, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT id, name, created_at, updated_at
  FROM groups
 ORDER BY id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
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
			return nil, fmt.Errorf("scan group: %w", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate groups: %w", err)
	}

	return out, nil
}
