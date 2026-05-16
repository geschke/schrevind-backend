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
	GroupID   int64  `json:"GroupID"`
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

	if security.GroupID <= 0 {
		return Security{}, fmt.Errorf("groupID must be > 0")
	}
	if security.Name == "" {
		return Security{}, fmt.Errorf("name is required")
	}
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
		&security.GroupID,
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
	case "ID":
		return "id", nil
	case "", "Name":
		return "name COLLATE NOCASE", nil
	case "ISIN":
		return "isin COLLATE NOCASE", nil
	case "WKN":
		return "wkn COLLATE NOCASE", nil
	case "Symbol":
		return "symbol COLLATE NOCASE", nil
	case "Status":
		return "status COLLATE NOCASE", nil
	case "CreatedAt":
		return "created_at", nil
	case "UpdatedAt":
		return "updated_at", nil
	default:
		return "", fmt.Errorf("invalid sort")
	}
}

func normalizeSecuritySortDirection(direction string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(direction)) {
	case "", "ASC":
		return "ASC", nil
	case "DESC":
		return "DESC", nil
	default:
		return "", fmt.Errorf("invalid direction")
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
  group_id, name, isin, wkn, symbol, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);
`, normalized.GroupID, normalized.Name, normalized.ISIN, normalized.WKN, normalized.Symbol, normalized.Status, normalized.CreatedAt, normalized.UpdatedAt)
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

// UpdateSecurity updates the security record by ID and group ID.
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
 WHERE id = ?
   AND group_id = ?;
`, normalized.Name, normalized.ISIN, normalized.WKN, normalized.Symbol, normalized.Status, normalized.UpdatedAt, normalized.ID, normalized.GroupID)
	if err != nil {
		return fmt.Errorf("update security: %w", err)
	}

	*security = normalized
	return nil
}

// GetSecurityByIDAndGroupID returns the security with the requested ID and group ID, or nil when not found.
func (d *DB) GetSecurityByIDAndGroupID(id, groupID int64) (*Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be > 0")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}

	row := d.SQL.QueryRow(`
SELECT id, group_id, name, isin, wkn, symbol, status, created_at, updated_at
  FROM securities
 WHERE id = ?
   AND group_id = ?
 LIMIT 1;
`, id, groupID)

	security, err := scanSecurity(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get security by id and group id: %w", err)
	}

	return security, nil
}

// GetSecurityGroupIDByID returns the group ID of a security, or false when not found.
func (d *DB) GetSecurityGroupIDByID(id int64) (int64, bool, error) {
	if d == nil || d.SQL == nil {
		return 0, false, fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return 0, false, fmt.Errorf("id must be > 0")
	}

	var groupID int64
	err := d.SQL.QueryRow(`SELECT group_id FROM securities WHERE id = ? LIMIT 1;`, id).Scan(&groupID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("get security group id by id: %w", err)
	}

	return groupID, true, nil
}

// GetSecurityByISINAndGroupID returns the security for the requested ISIN and group ID, or nil when not found.
func (d *DB) GetSecurityByISINAndGroupID(isin string, groupID int64) (*Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}

	isin = strings.ToUpper(strings.TrimSpace(isin))
	if isin == "" {
		return nil, fmt.Errorf("isin is required")
	}

	row := d.SQL.QueryRow(`
SELECT id, group_id, name, isin, wkn, symbol, status, created_at, updated_at
  FROM securities
 WHERE group_id = ?
   AND isin = ?
 LIMIT 1;
`, groupID, isin)

	security, err := scanSecurity(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get security by isin and group id: %w", err)
	}

	return security, nil
}

// GetSecurityByNameAndGroupID returns the security for the requested name and group ID, or nil when not found.
func (d *DB) GetSecurityByNameAndGroupID(name string, groupID int64) (*Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	row := d.SQL.QueryRow(`
SELECT id, group_id, name, isin, wkn, symbol, status, created_at, updated_at
  FROM securities
 WHERE group_id = ?
   AND name = ?
 LIMIT 1;
`, groupID, name)

	security, err := scanSecurity(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get security by name and group id: %w", err)
	}

	return security, nil
}

// ListSecuritiesByGroupID returns a filtered and paginated list of securities for a group.
func (d *DB) ListSecuritiesByGroupID(groupID int64, limit, offset int, sortBy, direction, status string) ([]Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit must be >= 0")
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset must be >= 0")
	}

	sortColumn, err := mapSecuritySortColumn(sortBy)
	if err != nil {
		return nil, fmt.Errorf("list securities page by group: %w", err)
	}
	sortDirection, err := normalizeSecuritySortDirection(direction)
	if err != nil {
		return nil, fmt.Errorf("list securities page by group: %w", err)
	}

	status = strings.TrimSpace(status)

	query := `
SELECT id, group_id, name, isin, wkn, symbol, status, created_at, updated_at
  FROM securities
 WHERE group_id = ?
`
	args := make([]any, 0, 4)
	args = append(args, groupID)

	if status != "" {
		query += "   AND status = ?\n"
		args = append(args, status)
	}

	query += " ORDER BY " + sortColumn + " " + sortDirection + ", id " + sortDirection + "\n"
	query += " LIMIT ? OFFSET ?;"
	args = append(args, limit, offset)

	rows, err := d.SQL.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list securities page by group: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Security, 0)
	for rows.Next() {
		security, err := scanSecurity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan security page by group: %w", err)
		}
		out = append(out, *security)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate securities page by group: %w", err)
	}

	return out, nil
}

// ListAllSecuritiesByGroupID returns all securities for the group with the fields needed for list UIs.
func (d *DB) ListAllSecuritiesByGroupID(groupID int64) ([]Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT id, group_id, name, isin, wkn, symbol, status, created_at, updated_at
  FROM securities
 WHERE group_id = ?
 ORDER BY name COLLATE NOCASE ASC, id ASC;
`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list all securities by group: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Security, 0)
	for rows.Next() {
		security, err := scanSecurity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan security by group: %w", err)
		}
		out = append(out, *security)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate securities by group: %w", err)
	}

	return out, nil
}

// ListAllSecuritiesForExport returns all security rows without any filter.
func (d *DB) ListAllSecuritiesForExport() ([]Security, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT id, group_id, name, isin, wkn, symbol, status, created_at, updated_at
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

// CountSecuritiesByGroupID returns the total number of securities matching the given group and status filter.
// An empty status string counts all securities in the group regardless of status.
func (d *DB) CountSecuritiesByGroupID(groupID int64, status string) (int64, error) {
	if d == nil || d.SQL == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return 0, fmt.Errorf("groupID must be > 0")
	}

	status = strings.TrimSpace(status)
	query := `SELECT COUNT(*) FROM securities WHERE group_id = ?`
	args := make([]any, 0, 2)
	args = append(args, groupID)
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += ";"

	var count int64
	if err := d.SQL.QueryRow(query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count securities by group: %w", err)
	}
	return count, nil
}

// SetSecurityStatus updates only the status and updated_at fields of the security.
func (d *DB) SetSecurityStatus(id, groupID int64, status string) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}
	if groupID <= 0 {
		return fmt.Errorf("groupID must be > 0")
	}

	status = strings.TrimSpace(status)
	_, err := d.SQL.Exec(`
UPDATE securities
   SET status = ?,
       updated_at = ?
 WHERE id = ?
   AND group_id = ?;
`, status, time.Now().Unix(), id, groupID)
	if err != nil {
		return fmt.Errorf("set security status: %w", err)
	}
	return nil
}

// DeleteSecurity deletes the security record by ID and group ID.
func (d *DB) DeleteSecurity(id, groupID int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}
	if groupID <= 0 {
		return fmt.Errorf("groupID must be > 0")
	}

	_, err := d.SQL.Exec(`DELETE FROM securities WHERE id = ? AND group_id = ?;`, id, groupID)
	if err != nil {
		return fmt.Errorf("delete security: %w", err)
	}
	return nil
}

// SecurityHasDividendEntries returns true when the security is referenced by dividend entries.
func (d *DB) SecurityHasDividendEntries(securityID int64) (bool, error) {
	count, err := d.CountDividendEntriesBySecurityID(securityID, DividendEntryListFilters{})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
