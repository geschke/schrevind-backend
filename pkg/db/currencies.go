package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	CurrencyStatusActive   = "active"
	CurrencyStatusInactive = "inactive"
	CurrencyStatusDeleted  = "deleted"
)

type Currency struct {
	ID            int64  `json:"ID"`
	GroupID       int64  `json:"GroupID"`
	Currency      string `json:"Currency,omitempty"`
	Name          string `json:"Name,omitempty"`
	DecimalPlaces int64  `json:"DecimalPlaces"`
	Status        string `json:"Status,omitempty"`
	CreatedAt     int64  `json:"CreatedAt,omitempty"`
	UpdatedAt     int64  `json:"UpdatedAt,omitempty"`
}

// normalizeCurrency performs its package-specific operation.
func normalizeCurrency(currency Currency) (Currency, error) {
	currency.Currency = strings.ToUpper(strings.TrimSpace(currency.Currency))
	currency.Name = strings.TrimSpace(currency.Name)
	currency.Status = strings.TrimSpace(currency.Status)

	if !isValidCurrencyCode(currency.Currency) {
		return Currency{}, fmt.Errorf("currency must be a 3-letter uppercase code")
	}
	if currency.GroupID < 0 {
		return Currency{}, fmt.Errorf("group_id must be >= 0")
	}
	if currency.DecimalPlaces < 0 {
		return Currency{}, fmt.Errorf("decimal_places must be >= 0")
	}

	now := time.Now().Unix()
	if currency.CreatedAt == 0 {
		currency.CreatedAt = now
	}
	currency.UpdatedAt = now

	return currency, nil
}

// isValidCurrencyCode performs its package-specific operation.
func isValidCurrencyCode(v string) bool {
	if len(v) != 3 {
		return false
	}
	for i := 0; i < len(v); i++ {
		if v[i] < 'A' || v[i] > 'Z' {
			return false
		}
	}
	return true
}

// scanCurrency performs its package-specific operation.
func scanCurrency(scanner interface {
	Scan(dest ...any) error
}) (*Currency, error) {
	var currency Currency
	if err := scanner.Scan(
		&currency.ID,
		&currency.GroupID,
		&currency.Currency,
		&currency.Name,
		&currency.DecimalPlaces,
		&currency.Status,
		&currency.CreatedAt,
		&currency.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &currency, nil
}

// mapCurrencySortColumn performs its package-specific operation.
func mapCurrencySortColumn(sortBy string) (string, error) {
	switch strings.TrimSpace(sortBy) {
	case "", "Currency":
		return "currency", nil
	case "Name":
		return "name", nil
	case "DecimalPlaces":
		return "decimal_places", nil
	default:
		return "", fmt.Errorf("invalid sort")
	}
}

// CreateCurrency creates a new record.
func (d *DB) CreateCurrency(currency *Currency) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if currency == nil {
		return fmt.Errorf("currency is nil")
	}

	normalized, err := normalizeCurrency(*currency)
	if err != nil {
		return err
	}

	res, err := d.SQL.Exec(`
INSERT INTO currencies (
  group_id, currency, name, decimal_places, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?);
`, normalized.GroupID, normalized.Currency, normalized.Name, normalized.DecimalPlaces, normalized.Status, normalized.CreatedAt, normalized.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create currency: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("create currency last_insert_id: %w", err)
	}

	normalized.ID = id
	*currency = normalized
	return nil
}

// UpdateCurrency updates the currency record by ID.
func (d *DB) UpdateCurrency(currency *Currency) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if currency == nil {
		return fmt.Errorf("currency is nil")
	}
	if currency.ID <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	normalized, err := normalizeCurrency(*currency)
	if err != nil {
		return err
	}

	_, err = d.SQL.Exec(`
UPDATE currencies
   SET currency = ?,
       name = ?,
       decimal_places = ?,
       status = ?,
       updated_at = ?
 WHERE id = ?
   AND group_id = ?;
`, normalized.Currency, normalized.Name, normalized.DecimalPlaces, normalized.Status, normalized.UpdatedAt, normalized.ID, normalized.GroupID)
	if err != nil {
		return fmt.Errorf("update currency: %w", err)
	}

	*currency = normalized
	return nil
}

// GetCurrencyByIDAndGroupID returns data for the requested input.
func (d *DB) GetCurrencyByIDAndGroupID(id, groupID int64) (*Currency, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be > 0")
	}
	if groupID < 0 {
		return nil, fmt.Errorf("group_id must be >= 0")
	}

	row := d.SQL.QueryRow(`
SELECT id, group_id, currency, name, decimal_places, status, created_at, updated_at
  FROM currencies
 WHERE id = ?
   AND group_id = ?
 LIMIT 1;
`, id, groupID)

	currency, err := scanCurrency(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get currency by id: %w", err)
	}

	return currency, nil
}

// GetCurrencyByCurrencyAndGroupID returns data for the requested input.
func (d *DB) GetCurrencyByCurrencyAndGroupID(currencyCode string, groupID int64) (*Currency, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID < 0 {
		return nil, fmt.Errorf("group_id must be >= 0")
	}

	currencyCode = strings.ToUpper(strings.TrimSpace(currencyCode))
	if !isValidCurrencyCode(currencyCode) {
		return nil, fmt.Errorf("currency must be a 3-letter uppercase code")
	}

	row := d.SQL.QueryRow(`
SELECT id, group_id, currency, name, decimal_places, status, created_at, updated_at
  FROM currencies
 WHERE currency = ?
   AND group_id = ?
 LIMIT 1;
`, currencyCode, groupID)

	currency, err := scanCurrency(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get currency by currency: %w", err)
	}

	return currency, nil
}

// ListCurrenciesByGroupID returns a list for the requested filter.
func (d *DB) ListCurrenciesByGroupID(groupID int64, limit, offset int, sortBy, status string) ([]Currency, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID < 0 {
		return nil, fmt.Errorf("group_id must be >= 0")
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit must be >= 0")
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset must be >= 0")
	}

	sortColumn, err := mapCurrencySortColumn(sortBy)
	if err != nil {
		return nil, fmt.Errorf("list currencies: %w", err)
	}

	status = strings.TrimSpace(status)

	query := `
SELECT id, group_id, currency, name, decimal_places, status, created_at, updated_at
  FROM currencies
 WHERE group_id = ?
`
	args := make([]any, 0, 4)
	args = append(args, groupID)

	if status != "" {
		query += "   AND status = ?\n"
		args = append(args, status)
	}

	query += " ORDER BY " + sortColumn + " ASC, id ASC\n"
	query += " LIMIT ? OFFSET ?;"
	args = append(args, limit, offset)

	rows, err := d.SQL.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list currencies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Currency, 0)
	for rows.Next() {
		currency, err := scanCurrency(rows)
		if err != nil {
			return nil, fmt.Errorf("scan currency: %w", err)
		}
		out = append(out, *currency)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate currencies: %w", err)
	}

	return out, nil
}

// ListAllCurrencies returns all currency rows without any filter. Intended for full-database exports.
func (d *DB) ListAllCurrencies() ([]Currency, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT id, group_id, currency, name, decimal_places, status, created_at, updated_at
  FROM currencies
 ORDER BY id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list all currencies for export: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Currency, 0)
	for rows.Next() {
		currency, err := scanCurrency(rows)
		if err != nil {
			return nil, fmt.Errorf("scan currency for export: %w", err)
		}
		out = append(out, *currency)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate currencies for export: %w", err)
	}

	return out, nil
}

// CountCurrenciesByGroupID returns the total number of currencies matching the given status filter.
// An empty status string counts all currencies regardless of status.
func (d *DB) CountCurrenciesByGroupID(groupID int64, status string) (int64, error) {
	if d == nil || d.SQL == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	if groupID < 0 {
		return 0, fmt.Errorf("group_id must be >= 0")
	}

	status = strings.TrimSpace(status)
	query := `SELECT COUNT(*) FROM currencies WHERE group_id = ?`
	args := make([]any, 0, 2)
	args = append(args, groupID)
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += ";"

	var count int64
	if err := d.SQL.QueryRow(query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count currencies: %w", err)
	}
	return count, nil
}

// SetCurrencyStatus performs its package-specific operation.
func (d *DB) SetCurrencyStatus(id int64, status string) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	status = strings.TrimSpace(status)
	_, err := d.SQL.Exec(`
UPDATE currencies
   SET status = ?,
       updated_at = ?
 WHERE id = ?;
`, status, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("set currency status: %w", err)
	}
	return nil
}

// DeleteCurrencyByIDAndGroupID deletes the currency record by ID.
func (d *DB) DeleteCurrencyByIDAndGroupID(id, groupID int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}
	if groupID < 0 {
		return fmt.Errorf("group_id must be >= 0")
	}

	_, err := d.SQL.Exec(`DELETE FROM currencies WHERE id = ? AND group_id = ?;`, id, groupID)
	if err != nil {
		return fmt.Errorf("delete currency: %w", err)
	}
	return nil
}

// CopyDefaultCurrenciesToGroup copies all template currencies (group_id = 0) into the target group.
func (d *DB) CopyDefaultCurrenciesToGroup(groupID int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return fmt.Errorf("group_id must be > 0")
	}

	now := time.Now().Unix()
	_, err := d.SQL.Exec(`
INSERT INTO currencies (
  group_id, currency, name, decimal_places, status, created_at, updated_at
)
SELECT ?, currency, name, decimal_places, status, ?, ?
  FROM currencies
 WHERE group_id = 0;
`, groupID, now, now)
	if err != nil {
		return fmt.Errorf("copy default currencies to group: %w", err)
	}
	return nil
}
