package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Currency struct {
	ID        int64  `json:"ID"`
	Currency  string `json:"Currency,omitempty"`
	Name      string `json:"Name,omitempty"`
	Status    string `json:"Status,omitempty"`
	CreatedAt int64  `json:"CreatedAt,omitempty"`
	UpdatedAt int64  `json:"UpdatedAt,omitempty"`
}

// normalizeCurrency performs its package-specific operation.
func normalizeCurrency(currency Currency) (Currency, error) {
	currency.Currency = strings.ToUpper(strings.TrimSpace(currency.Currency))
	currency.Name = strings.TrimSpace(currency.Name)
	currency.Status = strings.TrimSpace(currency.Status)

	if !isValidCurrencyCode(currency.Currency) {
		return Currency{}, fmt.Errorf("currency must be a 3-letter uppercase code")
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
		&currency.Currency,
		&currency.Name,
		&currency.Status,
		&currency.CreatedAt,
		&currency.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &currency, nil
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
  currency, name, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?);
`, normalized.Currency, normalized.Name, normalized.Status, normalized.CreatedAt, normalized.UpdatedAt)
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
       status = ?,
       updated_at = ?
 WHERE id = ?;
`, normalized.Currency, normalized.Name, normalized.Status, normalized.UpdatedAt, normalized.ID)
	if err != nil {
		return fmt.Errorf("update currency: %w", err)
	}

	*currency = normalized
	return nil
}

// GetCurrencyByID returns data for the requested input.
func (d *DB) GetCurrencyByID(id int64) (*Currency, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be > 0")
	}

	row := d.SQL.QueryRow(`
SELECT id, currency, name, status, created_at, updated_at
  FROM currencies
 WHERE id = ?
 LIMIT 1;
`, id)

	currency, err := scanCurrency(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get currency by id: %w", err)
	}

	return currency, nil
}

// GetCurrencyByCurrency returns data for the requested input.
func (d *DB) GetCurrencyByCurrency(currencyCode string) (*Currency, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	currencyCode = strings.ToUpper(strings.TrimSpace(currencyCode))
	if !isValidCurrencyCode(currencyCode) {
		return nil, fmt.Errorf("currency must be a 3-letter uppercase code")
	}

	row := d.SQL.QueryRow(`
SELECT id, currency, name, status, created_at, updated_at
  FROM currencies
 WHERE currency = ?
 LIMIT 1;
`, currencyCode)

	currency, err := scanCurrency(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get currency by currency: %w", err)
	}

	return currency, nil
}

// ListCurrencies returns a list for the requested filter.
func (d *DB) ListCurrencies() ([]Currency, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT id, currency, name, status, created_at, updated_at
  FROM currencies
 ORDER BY id ASC;
`)
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

// ListActiveCurrencies returns a list for the requested filter.
func (d *DB) ListActiveCurrencies() ([]Currency, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT id, currency, name, status, created_at, updated_at
  FROM currencies
 WHERE status = ?
 ORDER BY id ASC;
`, "active")
	if err != nil {
		return nil, fmt.Errorf("list active currencies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Currency, 0)
	for rows.Next() {
		currency, err := scanCurrency(rows)
		if err != nil {
			return nil, fmt.Errorf("scan active currency: %w", err)
		}
		out = append(out, *currency)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active currencies: %w", err)
	}

	return out, nil
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

// DeleteCurrency deletes the currency record by ID.
func (d *DB) DeleteCurrency(id int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	_, err := d.SQL.Exec(`DELETE FROM currencies WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete currency: %w", err)
	}
	return nil
}
