package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	SecurityStatusActive   = "active"
	SecurityStatusInactive = "inactive"
	SecurityStatusDeleted  = "deleted"
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

// normalizeSecurity trims user-facing fields, normalizes the ISIN, and updates timestamps.
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

// scanSecurity reads one security row from the current scanner position.
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

// mapSecuritySortColumn maps allowed API sort values to SQL column names.
func mapSecuritySortColumn(sortBy string) (string, error) {
	switch strings.TrimSpace(sortBy) {
	case "", "Name":
		return "name", nil
	case "ISIN":
		return "isin", nil
	case "WKN":
		return "wkn", nil
	case "Symbol":
		return "symbol", nil
	default:
		return "", fmt.Errorf("invalid sort")
	}
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

// GetSecurityByID returns the security with the requested ID, or nil when not found.
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

// GetSecurityByISIN returns the security for the requested ISIN, or nil when not found.
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

// ListSecurities returns a filtered and paginated list of securities.
func (d *DB) ListSecurities(limit, offset int, sortBy, status string) ([]Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit must be >= 0")
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset must be >= 0")
	}

	sortColumn, err := mapSecuritySortColumn(sortBy)
	if err != nil {
		return nil, fmt.Errorf("list securities page: %w", err)
	}

	status = strings.TrimSpace(status)

	query := `
SELECT id, name, isin, wkn, symbol, status, created_at, updated_at
  FROM securities
`
	args := make([]any, 0, 3)

	if status != "" {
		query += " WHERE status = ?\n"
		args = append(args, status)
	}

	query += " ORDER BY " + sortColumn + " ASC, id ASC\n"
	query += " LIMIT ? OFFSET ?;"
	args = append(args, limit, offset)

	rows, err := d.SQL.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list securities page: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Security, 0)
	for rows.Next() {
		security, err := scanSecurity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan security page: %w", err)
		}
		out = append(out, *security)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate securities page: %w", err)
	}

	return out, nil
}

// ListAllSecurities returns all security rows without any filter. Intended for full-database exports.
func (d *DB) ListAllSecurities() ([]Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT id, name, isin, wkn, symbol, status, created_at, updated_at
  FROM securities
 ORDER BY id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list all securities for export: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Security, 0)
	for rows.Next() {
		security, err := scanSecurity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan security for export: %w", err)
		}
		out = append(out, *security)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate securities for export: %w", err)
	}

	return out, nil
}

// CountSecurities returns the total number of securities matching the given status filter.
// An empty status string counts all securities regardless of status.
func (d *DB) CountSecurities(status string) (int64, error) {
	if d == nil || d.SQL == nil {
		return 0, fmt.Errorf("db not initialized")
	}

	status = strings.TrimSpace(status)
	query := `SELECT COUNT(*) FROM securities`
	args := make([]any, 0, 1)
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += ";"

	var count int64
	if err := d.SQL.QueryRow(query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count securities: %w", err)
	}
	return count, nil
}

// SetSecurityStatus updates only the status and updated_at fields of the security.
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
