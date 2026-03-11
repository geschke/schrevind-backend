package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type WithholdingTaxDefault struct {
	ID                                 int64  `json:"ID"`
	DepotID                            int64  `json:"DepotID,omitempty"`
	CountryCode                        string `json:"CountryCode,omitempty"`
	CountryName                        string `json:"CountryName,omitempty"`
	WithholdingTaxPercentDefault       string `json:"WithholdingTaxPercentDefault,omitempty"`
	WithholdingTaxPercentCreditDefault string `json:"WithholdingTaxPercentCreditDefault,omitempty"`
	CreatedAt                          int64  `json:"CreatedAt,omitempty"`
	UpdatedAt                          int64  `json:"UpdatedAt,omitempty"`
}

// normalizeWithholdingTaxDefault performs its package-specific operation.
func normalizeWithholdingTaxDefault(item WithholdingTaxDefault) (WithholdingTaxDefault, error) {
	item.CountryCode = strings.ToUpper(strings.TrimSpace(item.CountryCode))
	item.CountryName = strings.TrimSpace(item.CountryName)
	item.WithholdingTaxPercentDefault = strings.TrimSpace(item.WithholdingTaxPercentDefault)
	item.WithholdingTaxPercentCreditDefault = strings.TrimSpace(item.WithholdingTaxPercentCreditDefault)

	if item.CountryCode == "" {
		return WithholdingTaxDefault{}, fmt.Errorf("countryCode is required")
	}

	now := time.Now().Unix()
	if item.CreatedAt == 0 {
		item.CreatedAt = now
	}
	item.UpdatedAt = now

	return item, nil
}

// nullableDepotID performs its package-specific operation.
func nullableDepotID(depotID int64) sql.NullInt64 {
	if depotID > 0 {
		return sql.NullInt64{Int64: depotID, Valid: true}
	}
	return sql.NullInt64{}
}

// scanWithholdingTaxDefault performs its package-specific operation.
func scanWithholdingTaxDefault(scanner interface {
	Scan(dest ...any) error
}) (*WithholdingTaxDefault, error) {
	var (
		item    WithholdingTaxDefault
		depotID sql.NullInt64
	)
	if err := scanner.Scan(
		&item.ID,
		&depotID,
		&item.CountryCode,
		&item.CountryName,
		&item.WithholdingTaxPercentDefault,
		&item.WithholdingTaxPercentCreditDefault,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if depotID.Valid {
		item.DepotID = depotID.Int64
	}
	return &item, nil
}

// CreateWithholdingTaxDefault creates a new record.
func (d *DB) CreateWithholdingTaxDefault(item *WithholdingTaxDefault) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if item == nil {
		return fmt.Errorf("item is nil")
	}

	normalized, err := normalizeWithholdingTaxDefault(*item)
	if err != nil {
		return err
	}

	res, err := d.SQL.Exec(`
INSERT INTO withholding_tax_defaults (
  depot_id, country_code, country_name, withholding_tax_percent_default, withholding_tax_percent_credit_default, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?);
`, nullableDepotID(normalized.DepotID), normalized.CountryCode, normalized.CountryName, normalized.WithholdingTaxPercentDefault, normalized.WithholdingTaxPercentCreditDefault, normalized.CreatedAt, normalized.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create withholding tax default: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("create withholding tax default last_insert_id: %w", err)
	}

	normalized.ID = id
	*item = normalized
	return nil
}

// UpdateWithholdingTaxDefault updates the record by ID.
func (d *DB) UpdateWithholdingTaxDefault(item *WithholdingTaxDefault) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if item == nil {
		return fmt.Errorf("item is nil")
	}
	if item.ID <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	normalized, err := normalizeWithholdingTaxDefault(*item)
	if err != nil {
		return err
	}

	_, err = d.SQL.Exec(`
UPDATE withholding_tax_defaults
   SET depot_id = ?,
       country_code = ?,
       country_name = ?,
       withholding_tax_percent_default = ?,
       withholding_tax_percent_credit_default = ?,
       updated_at = ?
 WHERE id = ?;
`, nullableDepotID(normalized.DepotID), normalized.CountryCode, normalized.CountryName, normalized.WithholdingTaxPercentDefault, normalized.WithholdingTaxPercentCreditDefault, normalized.UpdatedAt, normalized.ID)
	if err != nil {
		return fmt.Errorf("update withholding tax default: %w", err)
	}

	*item = normalized
	return nil
}

// GetWithholdingTaxDefaultByID returns data for the requested input.
func (d *DB) GetWithholdingTaxDefaultByID(id int64) (*WithholdingTaxDefault, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be > 0")
	}

	row := d.SQL.QueryRow(`
SELECT id, depot_id, country_code, country_name, withholding_tax_percent_default, withholding_tax_percent_credit_default, created_at, updated_at
  FROM withholding_tax_defaults
 WHERE id = ?
 LIMIT 1;
`, id)

	item, err := scanWithholdingTaxDefault(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get withholding tax default by id: %w", err)
	}

	return item, nil
}

// GetWithholdingTaxDefault returns data for the requested input.
func (d *DB) GetWithholdingTaxDefault(depotID int64, countryCode string) (*WithholdingTaxDefault, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	countryCode = strings.ToUpper(strings.TrimSpace(countryCode))
	if countryCode == "" {
		return nil, fmt.Errorf("countryCode is required")
	}

	if depotID > 0 {
		row := d.SQL.QueryRow(`
SELECT id, depot_id, country_code, country_name, withholding_tax_percent_default, withholding_tax_percent_credit_default, created_at, updated_at
  FROM withholding_tax_defaults
 WHERE depot_id = ?
   AND country_code = ?
 LIMIT 1;
`, depotID, countryCode)

		item, err := scanWithholdingTaxDefault(row)
		if err == nil {
			return item, nil
		}
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("get withholding tax default by depot and country: %w", err)
		}
	}

	row := d.SQL.QueryRow(`
SELECT id, depot_id, country_code, country_name, withholding_tax_percent_default, withholding_tax_percent_credit_default, created_at, updated_at
  FROM withholding_tax_defaults
 WHERE depot_id IS NULL
   AND country_code = ?
 LIMIT 1;
`, countryCode)

	item, err := scanWithholdingTaxDefault(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get global withholding tax default by country: %w", err)
	}

	return item, nil
}

// ListWithholdingTaxDefaults returns a list for the requested filter.
func (d *DB) ListWithholdingTaxDefaults() ([]WithholdingTaxDefault, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT id, depot_id, country_code, country_name, withholding_tax_percent_default, withholding_tax_percent_credit_default, created_at, updated_at
  FROM withholding_tax_defaults
 ORDER BY id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list withholding tax defaults: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]WithholdingTaxDefault, 0)
	for rows.Next() {
		item, err := scanWithholdingTaxDefault(rows)
		if err != nil {
			return nil, fmt.Errorf("scan withholding tax default: %w", err)
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate withholding tax defaults: %w", err)
	}

	return out, nil
}

// ListWithholdingTaxDefaultsByDepotID returns a list for the requested filter.
func (d *DB) ListWithholdingTaxDefaultsByDepotID(depotID int64) ([]WithholdingTaxDefault, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if depotID <= 0 {
		return nil, fmt.Errorf("depotID must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT id, depot_id, country_code, country_name, withholding_tax_percent_default, withholding_tax_percent_credit_default, created_at, updated_at
  FROM withholding_tax_defaults
 WHERE depot_id = ?
 ORDER BY id ASC;
`, depotID)
	if err != nil {
		return nil, fmt.Errorf("list withholding tax defaults by depot: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]WithholdingTaxDefault, 0)
	for rows.Next() {
		item, err := scanWithholdingTaxDefault(rows)
		if err != nil {
			return nil, fmt.Errorf("scan withholding tax default by depot: %w", err)
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate withholding tax defaults by depot: %w", err)
	}

	return out, nil
}

// DeleteWithholdingTaxDefault deletes the record by ID.
func (d *DB) DeleteWithholdingTaxDefault(id int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	_, err := d.SQL.Exec(`DELETE FROM withholding_tax_defaults WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete withholding tax default: %w", err)
	}
	return nil
}
