package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Depot struct {
	ID            int64  `json:"ID"`
	Name          string `json:"Name,omitempty"`
	BrokerName    string `json:"BrokerName,omitempty"`
	AccountNumber string `json:"AccountNumber,omitempty"`
	BaseCurrency  string `json:"BaseCurrency,omitempty"`
	Description   string `json:"Description,omitempty"`
	Status        string `json:"Status,omitempty"`
	CreatedAt     int64  `json:"CreatedAt,omitempty"`
	UpdatedAt     int64  `json:"UpdatedAt,omitempty"`
}

// normalizeDepot performs its package-specific operation.
func normalizeDepot(depot Depot) (Depot, error) {
	depot.Name = strings.TrimSpace(depot.Name)
	depot.BrokerName = strings.TrimSpace(depot.BrokerName)
	depot.AccountNumber = strings.TrimSpace(depot.AccountNumber)
	depot.BaseCurrency = strings.TrimSpace(depot.BaseCurrency)
	depot.Description = strings.TrimSpace(depot.Description)
	depot.Status = strings.TrimSpace(depot.Status)

	now := time.Now().Unix()
	if depot.CreatedAt == 0 {
		depot.CreatedAt = now
	}
	depot.UpdatedAt = now

	return depot, nil
}

// scanDepot performs its package-specific operation.
func scanDepot(row *sql.Row) (Depot, error) {
	var depot Depot
	if err := row.Scan(
		&depot.ID,
		&depot.Name,
		&depot.BrokerName,
		&depot.AccountNumber,
		&depot.BaseCurrency,
		&depot.Description,
		&depot.Status,
		&depot.CreatedAt,
		&depot.UpdatedAt,
	); err != nil {
		return Depot{}, err
	}
	return depot, nil
}

// CreateDepot creates a new record.
func (d *DB) CreateDepot(depot *Depot) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if depot == nil {
		return fmt.Errorf("depot is nil")
	}

	normalized, err := normalizeDepot(*depot)
	if err != nil {
		return err
	}

	res, err := d.SQL.Exec(`
INSERT INTO depots (
  name, broker_name, account_number, base_currency, description, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);
`, normalized.Name, normalized.BrokerName, normalized.AccountNumber, normalized.BaseCurrency, normalized.Description, normalized.Status, normalized.CreatedAt, normalized.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create depot: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("create depot last_insert_id: %w", err)
	}

	normalized.ID = id
	*depot = normalized
	return nil
}

// UpdateDepot updates the depot record by ID.
func (d *DB) UpdateDepot(depot *Depot) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if depot == nil {
		return fmt.Errorf("depot is nil")
	}
	if depot.ID <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	normalized, err := normalizeDepot(*depot)
	if err != nil {
		return err
	}

	_, err = d.SQL.Exec(`
UPDATE depots
   SET name = ?,
       broker_name = ?,
       account_number = ?,
       base_currency = ?,
       description = ?,
       status = ?,
       updated_at = ?
 WHERE id = ?;
`, normalized.Name, normalized.BrokerName, normalized.AccountNumber, normalized.BaseCurrency, normalized.Description, normalized.Status, normalized.UpdatedAt, normalized.ID)
	if err != nil {
		return fmt.Errorf("update depot: %w", err)
	}

	*depot = normalized
	return nil
}

// GetDepotByID returns data for the requested input.
func (d *DB) GetDepotByID(id int64) (Depot, bool, error) {
	if d == nil || d.SQL == nil {
		return Depot{}, false, fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return Depot{}, false, fmt.Errorf("id must be > 0")
	}

	row := d.SQL.QueryRow(`
SELECT id, name, broker_name, account_number, base_currency, description, status, created_at, updated_at
  FROM depots
 WHERE id = ?
 LIMIT 1;
`, id)

	depot, err := scanDepot(row)
	if err == sql.ErrNoRows {
		return Depot{}, false, nil
	}
	if err != nil {
		return Depot{}, false, fmt.Errorf("get depot by id: %w", err)
	}

	return depot, true, nil
}

// ListDepotsByUserMembership returns all depots the user has direct membership on,
// regardless of status. Used to build the depot selector in the UI.
func (d *DB) ListDepotsByUserMembership(userID int64) ([]Depot, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("userID must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT d.id, d.name, d.broker_name, d.account_number, d.base_currency, d.description, d.status, d.created_at, d.updated_at
  FROM depots d
  JOIN memberships m ON m.entity_type = ? AND m.entity_id = d.id
 WHERE m.user_id = ?
 ORDER BY d.id ASC;
`, EntityTypeDepot, userID)
	if err != nil {
		return nil, fmt.Errorf("list depots by user membership: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Depot, 0)
	for rows.Next() {
		var depot Depot
		if err := rows.Scan(
			&depot.ID,
			&depot.Name,
			&depot.BrokerName,
			&depot.AccountNumber,
			&depot.BaseCurrency,
			&depot.Description,
			&depot.Status,
			&depot.CreatedAt,
			&depot.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan depot by membership: %w", err)
		}
		out = append(out, depot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate depots by user membership: %w", err)
	}

	return out, nil
}

// ListDepotsForActionScope returns depots covered by a many-entity permission scope.
func (d *DB) ListDepotsForActionScope(userID int64, all bool, roles []string) ([]Depot, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("userID must be > 0")
	}

	query := ""
	args := make([]any, 0, 1+len(roles))
	if all {
		query = `
SELECT id, name, broker_name, account_number, base_currency, description, status, created_at, updated_at
  FROM depots
 ORDER BY id ASC;
`
	} else {
		query = `
SELECT DISTINCT d.id, d.name, d.broker_name, d.account_number, d.base_currency, d.description, d.status, d.created_at, d.updated_at
  FROM depots d
  JOIN memberships m ON m.entity_type = ? AND m.entity_id = d.id
 WHERE m.user_id = ?
`
		args = append(args, EntityTypeDepot, userID)
		if len(roles) > 0 {
			query += "   AND m.role IN (" + sqlPlaceholders(len(roles)) + ")\n"
			for _, role := range roles {
				args = append(args, role)
			}
		}
		query += " ORDER BY d.id ASC;"
	}

	rows, err := d.SQL.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list depots for action scope: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Depot, 0)
	for rows.Next() {
		var depot Depot
		if err := rows.Scan(
			&depot.ID,
			&depot.Name,
			&depot.BrokerName,
			&depot.AccountNumber,
			&depot.BaseCurrency,
			&depot.Description,
			&depot.Status,
			&depot.CreatedAt,
			&depot.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan depot for action scope: %w", err)
		}
		out = append(out, depot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate depots for action scope: %w", err)
	}

	return out, nil
}

// ListDepotsByGroupID returns all depots accessible to any user in the given group,
// via their depot memberships. Used for the group admin depot overview.
func (d *DB) ListDepotsByGroupID(groupID int64) ([]Depot, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT DISTINCT d.id, d.name, d.broker_name, d.account_number, d.base_currency, d.description, d.status, d.created_at, d.updated_at
  FROM depots d
  JOIN memberships m ON m.entity_type = ? AND m.entity_id = d.id
  JOIN group_users gu ON gu.user_id = m.user_id
 WHERE gu.group_id = ?
 ORDER BY d.id ASC;
`, EntityTypeDepot, groupID)
	if err != nil {
		return nil, fmt.Errorf("list depots by group: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Depot, 0)
	for rows.Next() {
		var depot Depot
		if err := rows.Scan(
			&depot.ID,
			&depot.Name,
			&depot.BrokerName,
			&depot.AccountNumber,
			&depot.BaseCurrency,
			&depot.Description,
			&depot.Status,
			&depot.CreatedAt,
			&depot.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan depot by group: %w", err)
		}
		out = append(out, depot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate depots by group: %w", err)
	}

	return out, nil
}

// ListAllDepots returns all depot rows without any filter. Intended for full-database exports.
func (d *DB) ListAllDepots() ([]Depot, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT id, name, broker_name, account_number, base_currency, description, status, created_at, updated_at
  FROM depots
 ORDER BY id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list all depots for export: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Depot, 0)
	for rows.Next() {
		var depot Depot
		if err := rows.Scan(
			&depot.ID,
			&depot.Name,
			&depot.BrokerName,
			&depot.AccountNumber,
			&depot.BaseCurrency,
			&depot.Description,
			&depot.Status,
			&depot.CreatedAt,
			&depot.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan depot for export: %w", err)
		}
		out = append(out, depot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate depots for export: %w", err)
	}

	return out, nil
}

// SetDepotStatus performs its package-specific operation.
func (d *DB) SetDepotStatus(id int64, status string) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	status = strings.TrimSpace(status)
	_, err := d.SQL.Exec(`
UPDATE depots
   SET status = ?,
       updated_at = ?
 WHERE id = ?;
`, status, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("set depot status: %w", err)
	}
	return nil
}

// DeleteDepot deletes the depot record by ID.
func (d *DB) DeleteDepot(id int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	_, err := d.SQL.Exec(`DELETE FROM depots WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete depot: %w", err)
	}
	return nil
}
