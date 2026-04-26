package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type WithholdingTaxDefault struct {
	ID                                 int64  `json:"ID"`
	GroupID                            int64  `json:"GroupID"`
	DepotID                            int64  `json:"DepotID"`
	CountryCode                        string `json:"CountryCode,omitempty"`
	CountryName                        string `json:"CountryName,omitempty"`
	WithholdingTaxPercentDefault       string `json:"WithholdingTaxPercentDefault,omitempty"`
	WithholdingTaxPercentCreditDefault string `json:"WithholdingTaxPercentCreditDefault,omitempty"`
	CreatedAt                          int64  `json:"CreatedAt,omitempty"`
	UpdatedAt                          int64  `json:"UpdatedAt,omitempty"`
}

// normalizeWithholdingTaxDefault trims user-facing fields and updates timestamps.
func normalizeWithholdingTaxDefault(item WithholdingTaxDefault) (WithholdingTaxDefault, error) {
	item.CountryCode = strings.ToUpper(strings.TrimSpace(item.CountryCode))
	item.CountryName = strings.TrimSpace(item.CountryName)
	item.WithholdingTaxPercentDefault = strings.TrimSpace(item.WithholdingTaxPercentDefault)
	item.WithholdingTaxPercentCreditDefault = strings.TrimSpace(item.WithholdingTaxPercentCreditDefault)

	if item.GroupID <= 0 {
		return WithholdingTaxDefault{}, fmt.Errorf("groupID must be > 0")
	}
	if item.DepotID < 0 {
		return WithholdingTaxDefault{}, fmt.Errorf("depotID must be >= 0")
	}
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

// scanWithholdingTaxDefault reads one withholding-tax-default row from the current scanner position.
func scanWithholdingTaxDefault(scanner interface {
	Scan(dest ...any) error
}) (*WithholdingTaxDefault, error) {
	var item WithholdingTaxDefault
	if err := scanner.Scan(
		&item.ID,
		&item.GroupID,
		&item.DepotID,
		&item.CountryCode,
		&item.CountryName,
		&item.WithholdingTaxPercentDefault,
		&item.WithholdingTaxPercentCreditDefault,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

const withholdingTaxDefaultSelectColumns = `
       id, group_id, depot_id, country_code, country_name,
       withholding_tax_percent_default, withholding_tax_percent_credit_default,
       created_at, updated_at`

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
  group_id, depot_id, country_code, country_name,
  withholding_tax_percent_default, withholding_tax_percent_credit_default,
  created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);
`, normalized.GroupID, normalized.DepotID, normalized.CountryCode, normalized.CountryName, normalized.WithholdingTaxPercentDefault, normalized.WithholdingTaxPercentCreditDefault, normalized.CreatedAt, normalized.UpdatedAt)
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

// UpdateWithholdingTaxDefault updates the record by ID and group ID.
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
 WHERE id = ?
   AND group_id = ?;
`, normalized.DepotID, normalized.CountryCode, normalized.CountryName, normalized.WithholdingTaxPercentDefault, normalized.WithholdingTaxPercentCreditDefault, normalized.UpdatedAt, normalized.ID, normalized.GroupID)
	if err != nil {
		return fmt.Errorf("update withholding tax default: %w", err)
	}

	*item = normalized
	return nil
}

// GetWithholdingTaxDefaultByIDAndGroupID returns the row for ID and group ID, or nil when not found.
func (d *DB) GetWithholdingTaxDefaultByIDAndGroupID(id, groupID int64) (*WithholdingTaxDefault, error) {
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
SELECT`+withholdingTaxDefaultSelectColumns+`
  FROM withholding_tax_defaults
 WHERE id = ?
   AND group_id = ?
 LIMIT 1;
`, id, groupID)

	item, err := scanWithholdingTaxDefault(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get withholding tax default by id and group: %w", err)
	}

	return item, nil
}

// GetEffectiveWithholdingTaxDefault returns the depot-specific row first, then the group fallback.
func (d *DB) GetEffectiveWithholdingTaxDefault(groupID, depotID int64, countryCode string) (*WithholdingTaxDefault, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}
	if depotID < 0 {
		return nil, fmt.Errorf("depotID must be >= 0")
	}

	countryCode = strings.ToUpper(strings.TrimSpace(countryCode))
	if countryCode == "" {
		return nil, fmt.Errorf("countryCode is required")
	}

	if depotID > 0 {
		row := d.SQL.QueryRow(`
SELECT`+withholdingTaxDefaultSelectColumns+`
  FROM withholding_tax_defaults
 WHERE group_id = ?
   AND depot_id = ?
   AND country_code = ?
 LIMIT 1;
`, groupID, depotID, countryCode)

		item, err := scanWithholdingTaxDefault(row)
		if err == nil {
			return item, nil
		}
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("get effective withholding tax default by depot: %w", err)
		}
	}

	row := d.SQL.QueryRow(`
SELECT`+withholdingTaxDefaultSelectColumns+`
  FROM withholding_tax_defaults
 WHERE group_id = ?
   AND depot_id = 0
   AND country_code = ?
 LIMIT 1;
`, groupID, countryCode)

	item, err := scanWithholdingTaxDefault(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get effective withholding tax default group fallback: %w", err)
	}

	return item, nil
}

// ListWithholdingTaxDefaultsByGroupID returns all defaults for one group.
func (d *DB) ListWithholdingTaxDefaultsByGroupID(groupID int64) ([]WithholdingTaxDefault, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT`+withholdingTaxDefaultSelectColumns+`
  FROM withholding_tax_defaults
 WHERE group_id = ?
 ORDER BY depot_id ASC, country_code ASC, id ASC;
`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list withholding tax defaults by group: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]WithholdingTaxDefault, 0)
	for rows.Next() {
		item, err := scanWithholdingTaxDefault(rows)
		if err != nil {
			return nil, fmt.Errorf("scan withholding tax default by group: %w", err)
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate withholding tax defaults by group: %w", err)
	}

	return out, nil
}

// ListAllWithholdingTaxDefaultsForExport returns all rows without any filter.
func (d *DB) ListAllWithholdingTaxDefaultsForExport() ([]WithholdingTaxDefault, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT` + withholdingTaxDefaultSelectColumns + `
  FROM withholding_tax_defaults
 ORDER BY id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list all withholding tax defaults for export: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]WithholdingTaxDefault, 0)
	for rows.Next() {
		item, err := scanWithholdingTaxDefault(rows)
		if err != nil {
			return nil, fmt.Errorf("scan withholding tax default for export: %w", err)
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate withholding tax defaults for export: %w", err)
	}

	return out, nil
}

// ListWithholdingTaxDefaultsByDepotIDAndGroupID returns depot-specific defaults for one group.
func (d *DB) ListWithholdingTaxDefaultsByDepotIDAndGroupID(depotID, groupID int64) ([]WithholdingTaxDefault, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if depotID <= 0 {
		return nil, fmt.Errorf("depotID must be > 0")
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("groupID must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT`+withholdingTaxDefaultSelectColumns+`
  FROM withholding_tax_defaults
 WHERE group_id = ?
   AND depot_id = ?
 ORDER BY country_code ASC, id ASC;
`, groupID, depotID)
	if err != nil {
		return nil, fmt.Errorf("list withholding tax defaults by depot and group: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]WithholdingTaxDefault, 0)
	for rows.Next() {
		item, err := scanWithholdingTaxDefault(rows)
		if err != nil {
			return nil, fmt.Errorf("scan withholding tax default by depot and group: %w", err)
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate withholding tax defaults by depot and group: %w", err)
	}

	return out, nil
}

// DeleteWithholdingTaxDefaultByIDAndGroupID deletes the record by ID and group ID.
func (d *DB) DeleteWithholdingTaxDefaultByIDAndGroupID(id, groupID int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}
	if groupID <= 0 {
		return fmt.Errorf("groupID must be > 0")
	}

	_, err := d.SQL.Exec(`DELETE FROM withholding_tax_defaults WHERE id = ? AND group_id = ?;`, id, groupID)
	if err != nil {
		return fmt.Errorf("delete withholding tax default by id and group: %w", err)
	}
	return nil
}

// DeleteWithholdingTaxDefaultsByDepotID deletes all rows for the depot.
func (d *DB) DeleteWithholdingTaxDefaultsByDepotID(depotID int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if depotID <= 0 {
		return fmt.Errorf("depotID must be > 0")
	}

	_, err := d.SQL.Exec(`DELETE FROM withholding_tax_defaults WHERE depot_id = ?;`, depotID)
	if err != nil {
		return fmt.Errorf("delete withholding tax defaults by depot: %w", err)
	}
	return nil
}

// DeleteWithholdingTaxDefaultsByGroupID deletes all rows for the group.
func (d *DB) DeleteWithholdingTaxDefaultsByGroupID(groupID int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if groupID <= 0 {
		return fmt.Errorf("groupID must be > 0")
	}

	_, err := d.SQL.Exec(`DELETE FROM withholding_tax_defaults WHERE group_id = ?;`, groupID)
	if err != nil {
		return fmt.Errorf("delete withholding tax defaults by group: %w", err)
	}
	return nil
}
