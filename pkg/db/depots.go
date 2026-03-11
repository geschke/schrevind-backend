package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Depot struct {
	ID            int64  `json:"ID"`
	UserID        int64  `json:"UserID,omitempty"`
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

	if depot.UserID <= 0 {
		return Depot{}, fmt.Errorf("userID must be > 0")
	}

	now := time.Now().Unix()
	if depot.CreatedAt == 0 {
		depot.CreatedAt = now
	}
	depot.UpdatedAt = now

	return depot, nil
}

// scanDepot performs its package-specific operation.
func scanDepot(scanner interface {
	Scan(dest ...any) error
}) (*Depot, error) {
	var depot Depot
	if err := scanner.Scan(
		&depot.ID,
		&depot.UserID,
		&depot.Name,
		&depot.BrokerName,
		&depot.AccountNumber,
		&depot.BaseCurrency,
		&depot.Description,
		&depot.Status,
		&depot.CreatedAt,
		&depot.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &depot, nil
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
  user_id, name, broker_name, account_number, base_currency, description, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
`, normalized.UserID, normalized.Name, normalized.BrokerName, normalized.AccountNumber, normalized.BaseCurrency, normalized.Description, normalized.Status, normalized.CreatedAt, normalized.UpdatedAt)
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
   SET user_id = ?,
       name = ?,
       broker_name = ?,
       account_number = ?,
       base_currency = ?,
       description = ?,
       status = ?,
       updated_at = ?
 WHERE id = ?;
`, normalized.UserID, normalized.Name, normalized.BrokerName, normalized.AccountNumber, normalized.BaseCurrency, normalized.Description, normalized.Status, normalized.UpdatedAt, normalized.ID)
	if err != nil {
		return fmt.Errorf("update depot: %w", err)
	}

	*depot = normalized
	return nil
}

// GetDepotByID returns data for the requested input.
func (d *DB) GetDepotByID(id int64) (*Depot, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be > 0")
	}

	row := d.SQL.QueryRow(`
SELECT id, user_id, name, broker_name, account_number, base_currency, description, status, created_at, updated_at
  FROM depots
 WHERE id = ?
 LIMIT 1;
`, id)

	depot, err := scanDepot(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get depot by id: %w", err)
	}

	return depot, nil
}

// ListDepotsByUserID returns a list for the requested filter.
func (d *DB) ListDepotsByUserID(userID int64) ([]Depot, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("userID must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT id, user_id, name, broker_name, account_number, base_currency, description, status, created_at, updated_at
  FROM depots
 WHERE user_id = ?
 ORDER BY id ASC;
`, userID)
	if err != nil {
		return nil, fmt.Errorf("list depots by user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Depot, 0)
	for rows.Next() {
		depot, err := scanDepot(rows)
		if err != nil {
			return nil, fmt.Errorf("scan depot: %w", err)
		}
		out = append(out, *depot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate depots by user: %w", err)
	}

	return out, nil
}

// ListActiveDepotsByUserID returns a list for the requested filter.
func (d *DB) ListActiveDepotsByUserID(userID int64) ([]Depot, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("userID must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT id, user_id, name, broker_name, account_number, base_currency, description, status, created_at, updated_at
  FROM depots
 WHERE user_id = ?
   AND status = ?
 ORDER BY id ASC;
`, userID, "active")
	if err != nil {
		return nil, fmt.Errorf("list active depots by user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Depot, 0)
	for rows.Next() {
		depot, err := scanDepot(rows)
		if err != nil {
			return nil, fmt.Errorf("scan active depot: %w", err)
		}
		out = append(out, *depot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active depots by user: %w", err)
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
