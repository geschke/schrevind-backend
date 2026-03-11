package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type DividendEntry struct {
	ID int64 `json:"ID"`

	UserID     int64 `json:"UserID,omitempty"`
	DepotID    int64 `json:"DepotID,omitempty"`
	SecurityID int64 `json:"SecurityID,omitempty"`

	PayDate string `json:"PayDate,omitempty"`
	ExDate  string `json:"ExDate,omitempty"`

	SecurityName   string `json:"SecurityName,omitempty"`
	SecurityISIN   string `json:"SecurityISIN,omitempty"`
	SecurityWKN    string `json:"SecurityWKN,omitempty"`
	SecuritySymbol string `json:"SecuritySymbol,omitempty"`

	Quantity string `json:"Quantity,omitempty"`

	DividendPerUnitAmount   string `json:"DividendPerUnitAmount,omitempty"`
	DividendPerUnitCurrency string `json:"DividendPerUnitCurrency,omitempty"`

	FXRateLabel string `json:"FXRateLabel,omitempty"`
	FXRate      string `json:"FXRate,omitempty"`

	GrossAmount   string `json:"GrossAmount,omitempty"`
	GrossCurrency string `json:"GrossCurrency,omitempty"`

	PayoutAmount   string `json:"PayoutAmount,omitempty"`
	PayoutCurrency string `json:"PayoutCurrency,omitempty"`

	WithholdingTaxCountryCode string `json:"WithholdingTaxCountryCode,omitempty"`
	WithholdingTaxPercent     string `json:"WithholdingTaxPercent,omitempty"`

	WithholdingTaxAmount   string `json:"WithholdingTaxAmount,omitempty"`
	WithholdingTaxCurrency string `json:"WithholdingTaxCurrency,omitempty"`

	WithholdingTaxAmountCredit         string `json:"WithholdingTaxAmountCredit,omitempty"`
	WithholdingTaxAmountCreditCurrency string `json:"WithholdingTaxAmountCreditCurrency,omitempty"`

	WithholdingTaxAmountRefundable         string `json:"WithholdingTaxAmountRefundable,omitempty"`
	WithholdingTaxAmountRefundableCurrency string `json:"WithholdingTaxAmountRefundableCurrency,omitempty"`

	ForeignFeesAmount   string `json:"ForeignFeesAmount,omitempty"`
	ForeignFeesCurrency string `json:"ForeignFeesCurrency,omitempty"`

	Note string `json:"Note,omitempty"`

	CalcGrossAmountBase            string `json:"CalcGrossAmountBase,omitempty"`
	CalcAfterWithholdingAmountBase string `json:"CalcAfterWithholdingAmountBase,omitempty"`

	CreatedAt int64 `json:"CreatedAt,omitempty"`
	UpdatedAt int64 `json:"UpdatedAt,omitempty"`
}

// normalizeDividendEntry performs its package-specific operation.
func normalizeDividendEntry(entry DividendEntry) (DividendEntry, error) {
	entry.PayDate = strings.TrimSpace(entry.PayDate)
	entry.ExDate = strings.TrimSpace(entry.ExDate)
	entry.SecurityName = strings.TrimSpace(entry.SecurityName)
	entry.SecurityISIN = strings.TrimSpace(entry.SecurityISIN)
	entry.SecurityWKN = strings.TrimSpace(entry.SecurityWKN)
	entry.SecuritySymbol = strings.TrimSpace(entry.SecuritySymbol)
	entry.Quantity = strings.TrimSpace(entry.Quantity)
	entry.DividendPerUnitAmount = strings.TrimSpace(entry.DividendPerUnitAmount)
	entry.DividendPerUnitCurrency = strings.TrimSpace(entry.DividendPerUnitCurrency)
	entry.FXRateLabel = strings.TrimSpace(entry.FXRateLabel)
	entry.FXRate = strings.TrimSpace(entry.FXRate)
	entry.GrossAmount = strings.TrimSpace(entry.GrossAmount)
	entry.GrossCurrency = strings.TrimSpace(entry.GrossCurrency)
	entry.PayoutAmount = strings.TrimSpace(entry.PayoutAmount)
	entry.PayoutCurrency = strings.TrimSpace(entry.PayoutCurrency)
	entry.WithholdingTaxCountryCode = strings.TrimSpace(entry.WithholdingTaxCountryCode)
	entry.WithholdingTaxPercent = strings.TrimSpace(entry.WithholdingTaxPercent)
	entry.WithholdingTaxAmount = strings.TrimSpace(entry.WithholdingTaxAmount)
	entry.WithholdingTaxCurrency = strings.TrimSpace(entry.WithholdingTaxCurrency)
	entry.WithholdingTaxAmountCredit = strings.TrimSpace(entry.WithholdingTaxAmountCredit)
	entry.WithholdingTaxAmountCreditCurrency = strings.TrimSpace(entry.WithholdingTaxAmountCreditCurrency)
	entry.WithholdingTaxAmountRefundable = strings.TrimSpace(entry.WithholdingTaxAmountRefundable)
	entry.WithholdingTaxAmountRefundableCurrency = strings.TrimSpace(entry.WithholdingTaxAmountRefundableCurrency)
	entry.ForeignFeesAmount = strings.TrimSpace(entry.ForeignFeesAmount)
	entry.ForeignFeesCurrency = strings.TrimSpace(entry.ForeignFeesCurrency)
	entry.Note = strings.TrimSpace(entry.Note)
	entry.CalcGrossAmountBase = strings.TrimSpace(entry.CalcGrossAmountBase)
	entry.CalcAfterWithholdingAmountBase = strings.TrimSpace(entry.CalcAfterWithholdingAmountBase)

	if entry.UserID <= 0 {
		return DividendEntry{}, fmt.Errorf("userID must be > 0")
	}
	if entry.DepotID <= 0 {
		return DividendEntry{}, fmt.Errorf("depotID must be > 0")
	}
	if entry.SecurityID <= 0 {
		return DividendEntry{}, fmt.Errorf("securityID must be > 0")
	}

	now := time.Now().Unix()
	if entry.CreatedAt == 0 {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	return entry, nil
}

// scanDividendEntry performs its package-specific operation.
func scanDividendEntry(scanner interface {
	Scan(dest ...any) error
}) (*DividendEntry, error) {
	var entry DividendEntry
	if err := scanner.Scan(
		&entry.ID,
		&entry.UserID,
		&entry.DepotID,
		&entry.SecurityID,
		&entry.PayDate,
		&entry.ExDate,
		&entry.SecurityName,
		&entry.SecurityISIN,
		&entry.SecurityWKN,
		&entry.SecuritySymbol,
		&entry.Quantity,
		&entry.DividendPerUnitAmount,
		&entry.DividendPerUnitCurrency,
		&entry.FXRateLabel,
		&entry.FXRate,
		&entry.GrossAmount,
		&entry.GrossCurrency,
		&entry.PayoutAmount,
		&entry.PayoutCurrency,
		&entry.WithholdingTaxCountryCode,
		&entry.WithholdingTaxPercent,
		&entry.WithholdingTaxAmount,
		&entry.WithholdingTaxCurrency,
		&entry.WithholdingTaxAmountCredit,
		&entry.WithholdingTaxAmountCreditCurrency,
		&entry.WithholdingTaxAmountRefundable,
		&entry.WithholdingTaxAmountRefundableCurrency,
		&entry.ForeignFeesAmount,
		&entry.ForeignFeesCurrency,
		&entry.Note,
		&entry.CalcGrossAmountBase,
		&entry.CalcAfterWithholdingAmountBase,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &entry, nil
}

// listDividendEntriesByColumn performs its package-specific operation.
func (d *DB) listDividendEntriesByColumn(column string, value int64) ([]DividendEntry, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if value <= 0 {
		return nil, fmt.Errorf("%s must be > 0", column)
	}

	rows, err := d.SQL.Query(`
SELECT id, user_id, depot_id, security_id, pay_date, ex_date, security_name, security_isin, security_wkn, security_symbol,
       quantity, dividend_per_unit_amount, dividend_per_unit_currency, fx_rate_label, fx_rate, gross_amount, gross_currency,
       payout_amount, payout_currency, withholding_tax_country_code, withholding_tax_percent, withholding_tax_amount,
       withholding_tax_currency, withholding_tax_amount_credit, withholding_tax_amount_credit_currency,
       withholding_tax_amount_refundable, withholding_tax_amount_refundable_currency, foreign_fees_amount, foreign_fees_currency,
       note, calc_gross_amount_base, calc_after_withholding_amount_base, created_at, updated_at
  FROM dividend_entries
 WHERE `+column+` = ?
 ORDER BY pay_date ASC, id ASC;
`, value)
	if err != nil {
		return nil, fmt.Errorf("list dividend entries by %s: %w", column, err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]DividendEntry, 0)
	for rows.Next() {
		entry, err := scanDividendEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan dividend entry: %w", err)
		}
		out = append(out, *entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dividend entries by %s: %w", column, err)
	}

	return out, nil
}

// listDividendEntriesByColumnAndDateRange performs its package-specific operation.
func (d *DB) listDividendEntriesByColumnAndDateRange(column string, value int64, fromDate, toDate string) ([]DividendEntry, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if value <= 0 {
		return nil, fmt.Errorf("%s must be > 0", column)
	}

	fromDate = strings.TrimSpace(fromDate)
	toDate = strings.TrimSpace(toDate)

	rows, err := d.SQL.Query(`
SELECT id, user_id, depot_id, security_id, pay_date, ex_date, security_name, security_isin, security_wkn, security_symbol,
       quantity, dividend_per_unit_amount, dividend_per_unit_currency, fx_rate_label, fx_rate, gross_amount, gross_currency,
       payout_amount, payout_currency, withholding_tax_country_code, withholding_tax_percent, withholding_tax_amount,
       withholding_tax_currency, withholding_tax_amount_credit, withholding_tax_amount_credit_currency,
       withholding_tax_amount_refundable, withholding_tax_amount_refundable_currency, foreign_fees_amount, foreign_fees_currency,
       note, calc_gross_amount_base, calc_after_withholding_amount_base, created_at, updated_at
  FROM dividend_entries
 WHERE `+column+` = ?
   AND pay_date >= ?
   AND pay_date <= ?
 ORDER BY pay_date ASC, id ASC;
`, value, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("list dividend entries by %s and date range: %w", column, err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]DividendEntry, 0)
	for rows.Next() {
		entry, err := scanDividendEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan dividend entry by date range: %w", err)
		}
		out = append(out, *entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dividend entries by %s and date range: %w", column, err)
	}

	return out, nil
}

// CreateDividendEntry creates a new record.
func (d *DB) CreateDividendEntry(entry *DividendEntry) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if entry == nil {
		return fmt.Errorf("entry is nil")
	}

	normalized, err := normalizeDividendEntry(*entry)
	if err != nil {
		return err
	}

	res, err := d.SQL.Exec(`
INSERT INTO dividend_entries (
  user_id, depot_id, security_id, pay_date, ex_date, security_name, security_isin, security_wkn, security_symbol,
  quantity, dividend_per_unit_amount, dividend_per_unit_currency, fx_rate_label, fx_rate, gross_amount, gross_currency,
  payout_amount, payout_currency, withholding_tax_country_code, withholding_tax_percent, withholding_tax_amount,
  withholding_tax_currency, withholding_tax_amount_credit, withholding_tax_amount_credit_currency,
  withholding_tax_amount_refundable, withholding_tax_amount_refundable_currency, foreign_fees_amount, foreign_fees_currency,
  note, calc_gross_amount_base, calc_after_withholding_amount_base, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`, normalized.UserID, normalized.DepotID, normalized.SecurityID, normalized.PayDate, normalized.ExDate, normalized.SecurityName, normalized.SecurityISIN, normalized.SecurityWKN, normalized.SecuritySymbol, normalized.Quantity, normalized.DividendPerUnitAmount, normalized.DividendPerUnitCurrency, normalized.FXRateLabel, normalized.FXRate, normalized.GrossAmount, normalized.GrossCurrency, normalized.PayoutAmount, normalized.PayoutCurrency, normalized.WithholdingTaxCountryCode, normalized.WithholdingTaxPercent, normalized.WithholdingTaxAmount, normalized.WithholdingTaxCurrency, normalized.WithholdingTaxAmountCredit, normalized.WithholdingTaxAmountCreditCurrency, normalized.WithholdingTaxAmountRefundable, normalized.WithholdingTaxAmountRefundableCurrency, normalized.ForeignFeesAmount, normalized.ForeignFeesCurrency, normalized.Note, normalized.CalcGrossAmountBase, normalized.CalcAfterWithholdingAmountBase, normalized.CreatedAt, normalized.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create dividend entry: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("create dividend entry last_insert_id: %w", err)
	}

	normalized.ID = id
	*entry = normalized
	return nil
}

// UpdateDividendEntry updates the record by ID.
func (d *DB) UpdateDividendEntry(entry *DividendEntry) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if entry == nil {
		return fmt.Errorf("entry is nil")
	}
	if entry.ID <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	normalized, err := normalizeDividendEntry(*entry)
	if err != nil {
		return err
	}

	_, err = d.SQL.Exec(`
UPDATE dividend_entries
   SET user_id = ?,
       depot_id = ?,
       security_id = ?,
       pay_date = ?,
       ex_date = ?,
       security_name = ?,
       security_isin = ?,
       security_wkn = ?,
       security_symbol = ?,
       quantity = ?,
       dividend_per_unit_amount = ?,
       dividend_per_unit_currency = ?,
       fx_rate_label = ?,
       fx_rate = ?,
       gross_amount = ?,
       gross_currency = ?,
       payout_amount = ?,
       payout_currency = ?,
       withholding_tax_country_code = ?,
       withholding_tax_percent = ?,
       withholding_tax_amount = ?,
       withholding_tax_currency = ?,
       withholding_tax_amount_credit = ?,
       withholding_tax_amount_credit_currency = ?,
       withholding_tax_amount_refundable = ?,
       withholding_tax_amount_refundable_currency = ?,
       foreign_fees_amount = ?,
       foreign_fees_currency = ?,
       note = ?,
       calc_gross_amount_base = ?,
       calc_after_withholding_amount_base = ?,
       updated_at = ?
 WHERE id = ?;
`, normalized.UserID, normalized.DepotID, normalized.SecurityID, normalized.PayDate, normalized.ExDate, normalized.SecurityName, normalized.SecurityISIN, normalized.SecurityWKN, normalized.SecuritySymbol, normalized.Quantity, normalized.DividendPerUnitAmount, normalized.DividendPerUnitCurrency, normalized.FXRateLabel, normalized.FXRate, normalized.GrossAmount, normalized.GrossCurrency, normalized.PayoutAmount, normalized.PayoutCurrency, normalized.WithholdingTaxCountryCode, normalized.WithholdingTaxPercent, normalized.WithholdingTaxAmount, normalized.WithholdingTaxCurrency, normalized.WithholdingTaxAmountCredit, normalized.WithholdingTaxAmountCreditCurrency, normalized.WithholdingTaxAmountRefundable, normalized.WithholdingTaxAmountRefundableCurrency, normalized.ForeignFeesAmount, normalized.ForeignFeesCurrency, normalized.Note, normalized.CalcGrossAmountBase, normalized.CalcAfterWithholdingAmountBase, normalized.UpdatedAt, normalized.ID)
	if err != nil {
		return fmt.Errorf("update dividend entry: %w", err)
	}

	*entry = normalized
	return nil
}

// GetDividendEntryByID returns data for the requested input.
func (d *DB) GetDividendEntryByID(id int64) (*DividendEntry, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be > 0")
	}

	row := d.SQL.QueryRow(`
SELECT id, user_id, depot_id, security_id, pay_date, ex_date, security_name, security_isin, security_wkn, security_symbol,
       quantity, dividend_per_unit_amount, dividend_per_unit_currency, fx_rate_label, fx_rate, gross_amount, gross_currency,
       payout_amount, payout_currency, withholding_tax_country_code, withholding_tax_percent, withholding_tax_amount,
       withholding_tax_currency, withholding_tax_amount_credit, withholding_tax_amount_credit_currency,
       withholding_tax_amount_refundable, withholding_tax_amount_refundable_currency, foreign_fees_amount, foreign_fees_currency,
       note, calc_gross_amount_base, calc_after_withholding_amount_base, created_at, updated_at
  FROM dividend_entries
 WHERE id = ?
 LIMIT 1;
`, id)

	entry, err := scanDividendEntry(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get dividend entry by id: %w", err)
	}

	return entry, nil
}

// DeleteDividendEntry deletes the record by ID.
func (d *DB) DeleteDividendEntry(id int64) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("id must be > 0")
	}

	_, err := d.SQL.Exec(`DELETE FROM dividend_entries WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete dividend entry: %w", err)
	}
	return nil
}

// ListDividendEntriesByUserID returns a list for the requested filter.
func (d *DB) ListDividendEntriesByUserID(userID int64) ([]DividendEntry, error) {
	return d.listDividendEntriesByColumn("user_id", userID)
}

// ListDividendEntriesByDepotID returns a list for the requested filter.
func (d *DB) ListDividendEntriesByDepotID(depotID int64) ([]DividendEntry, error) {
	return d.listDividendEntriesByColumn("depot_id", depotID)
}

// ListDividendEntriesBySecurityID returns a list for the requested filter.
func (d *DB) ListDividendEntriesBySecurityID(securityID int64) ([]DividendEntry, error) {
	return d.listDividendEntriesByColumn("security_id", securityID)
}

// ListDividendEntriesByUserIDAndDateRange returns a list for the requested filter.
func (d *DB) ListDividendEntriesByUserIDAndDateRange(userID int64, fromDate, toDate string) ([]DividendEntry, error) {
	return d.listDividendEntriesByColumnAndDateRange("user_id", userID, fromDate, toDate)
}

// ListDividendEntriesByDepotIDAndDateRange returns a list for the requested filter.
func (d *DB) ListDividendEntriesByDepotIDAndDateRange(depotID int64, fromDate, toDate string) ([]DividendEntry, error) {
	return d.listDividendEntriesByColumnAndDateRange("depot_id", depotID, fromDate, toDate)
}

// ListDividendEntriesBySecurityIDAndDateRange returns a list for the requested filter.
func (d *DB) ListDividendEntriesBySecurityIDAndDateRange(securityID int64, fromDate, toDate string) ([]DividendEntry, error) {
	return d.listDividendEntriesByColumnAndDateRange("security_id", securityID, fromDate, toDate)
}
