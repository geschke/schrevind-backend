package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Security struct {
	ID        int64  `json:"ID"`
	Name      string `json:"Name,omitempty"`
	ISIN      string `json:"ISIN,omitempty"`
	WKN       string `json:"WKN,omitempty"`
	Symbol    string `json:"Symbol,omitempty"`
	Status    string `json:"Status,omitempty"`
	CreatedAt int64  `json:"CreatedAt,omitempty"`
	UpdatedAt int64  `json:"UpdatedAt,omitempty"`
}

// normalizeSecurity performs its package-specific operation.
func normalizeSecurity(security Security) (Security, error) {
	security.Name = strings.TrimSpace(security.Name)
	security.ISIN = strings.ToUpper(strings.TrimSpace(security.ISIN))
	security.WKN = strings.TrimSpace(security.WKN)
	security.Symbol = strings.TrimSpace(security.Symbol)
	security.Status = strings.TrimSpace(security.Status)

	if security.ISIN == "" {
		return Security{}, fmt.Errorf("isin is required")
	}

	now := time.Now().Unix()
	if security.CreatedAt == 0 {
		security.CreatedAt = now
	}
	security.UpdatedAt = now

	return security, nil
}

// scanSecurity performs its package-specific operation.
func scanSecurity(scanner interface {
	Scan(dest ...any) error
}) (*Security, error) {
	var security Security
	if err := scanner.Scan(
		&security.ID,
		&security.Name,
		&security.ISIN,
		&security.WKN,
		&security.Symbol,
		&security.Status,
		&security.CreatedAt,
		&security.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &security, nil
}

// CreateSecurity creates a new record.
func (d *DB) CreateSecurity(security *Security) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if security == nil {
		return fmt.Errorf("security is nil")
	}

	normalized, err := normalizeSecurity(*security)
	if err != nil {
		return err
	}

	res, err := d.SQL.Exec(`
INSERT INTO securities (
  name, isin, wkn, symbol, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?);
`, normalized.Name, normalized.ISIN, normalized.WKN, normalized.Symbol, normalized.Status, normalized.CreatedAt, normalized.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create security: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("create security last_insert_id: %w", err)
	}

	normalized.ID = id
	*security = normalized
	return nil
}

// UpdateSecurity updates the security record by ID.
func (d *DB) UpdateSecurity(security *Security) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if security == nil {
		return fmt.Errorf("security is nil")
	}
	if security.ID <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	normalized, err := normalizeSecurity(*security)
	if err != nil {
		return err
	}

	_, err = d.SQL.Exec(`
UPDATE securities
   SET name = ?,
       isin = ?,
       wkn = ?,
       symbol = ?,
       status = ?,
       updated_at = ?
 WHERE id = ?;
`, normalized.Name, normalized.ISIN, normalized.WKN, normalized.Symbol, normalized.Status, normalized.UpdatedAt, normalized.ID)
	if err != nil {
		return fmt.Errorf("update security: %w", err)
	}

	*security = normalized
	return nil
}

// GetSecurityByID returns data for the requested input.
func (d *DB) GetSecurityByID(id int64) (*Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be > 0")
	}

	row := d.SQL.QueryRow(`
SELECT id, name, isin, wkn, symbol, status, created_at, updated_at
  FROM securities
 WHERE id = ?
 LIMIT 1;
`, id)

	security, err := scanSecurity(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get security by id: %w", err)
	}

	return security, nil
}

// GetSecurityByISIN returns data for the requested input.
func (d *DB) GetSecurityByISIN(isin string) (*Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	isin = strings.ToUpper(strings.TrimSpace(isin))
	if isin == "" {
		return nil, fmt.Errorf("isin is required")
	}

	row := d.SQL.QueryRow(`
SELECT id, name, isin, wkn, symbol, status, created_at, updated_at
  FROM securities
 WHERE isin = ?
 LIMIT 1;
`, isin)

	security, err := scanSecurity(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get security by isin: %w", err)
	}

	return security, nil
}

// ListSecurities returns a list for the requested filter.
func (d *DB) ListSecurities() ([]Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT id, name, isin, wkn, symbol, status, created_at, updated_at
  FROM securities
 ORDER BY id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list securities: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Security, 0)
	for rows.Next() {
		security, err := scanSecurity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan security: %w", err)
		}
		out = append(out, *security)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate securities: %w", err)
	}

	return out, nil
}

// ListActiveSecurities returns a list for the requested filter.
func (d *DB) ListActiveSecurities() ([]Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT id, name, isin, wkn, symbol, status, created_at, updated_at
  FROM securities
 WHERE status = ?
 ORDER BY id ASC;
`, "active")
	if err != nil {
		return nil, fmt.Errorf("list active securities: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Security, 0)
	for rows.Next() {
		security, err := scanSecurity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan active security: %w", err)
		}
		out = append(out, *security)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active securities: %w", err)
	}

	return out, nil
}

// SetSecurityStatus performs its package-specific operation.
func (d *DB) SetSecurityStatus(id int64, status string) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	status = strings.TrimSpace(status)
	_, err := d.SQL.Exec(`
UPDATE securities
   SET status = ?,
       updated_at = ?
 WHERE id = ?;
`, status, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("set security status: %w", err)
	}
	return nil
}

// DeleteSecurity deletes the security record by ID.
func (d *DB) DeleteSecurity(id int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	_, err := d.SQL.Exec(`DELETE FROM securities WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete security: %w", err)
	}
	return nil
}
