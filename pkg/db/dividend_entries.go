package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type DividendEntry struct {
	ID int64 `json:"ID"`

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

	InlandTaxAmount   string            `json:"InlandTaxAmount,omitempty"`
	InlandTaxCurrency string            `json:"InlandTaxCurrency,omitempty"`
	InlandTaxDetails  []InlandTaxDetail `json:"InlandTaxDetails,omitempty"`

	ForeignFeesAmount   string `json:"ForeignFeesAmount,omitempty"`
	ForeignFeesCurrency string `json:"ForeignFeesCurrency,omitempty"`

	Note string `json:"Note,omitempty"`

	CalcGrossAmountBase            string `json:"CalcGrossAmountBase,omitempty"`
	CalcAfterWithholdingAmountBase string `json:"CalcAfterWithholdingAmountBase,omitempty"`

	CreatedAt int64 `json:"CreatedAt,omitempty"`
	UpdatedAt int64 `json:"UpdatedAt,omitempty"`
}

type DividendEntryListFilters struct {
	FromDate string
	ToDate   string
	Search   string
	Year     int
	DepotID  int64
}

type DividendEntryTimeRange struct {
	FirstYear  int
	FirstMonth int
	LastYear   int
	LastMonth  int
}

func EncodeInlandTaxDetails(details []InlandTaxDetail) (string, error) {
	if len(details) == 0 {
		return "[]", nil
	}

	raw, err := json.Marshal(details)
	if err != nil {
		return "", fmt.Errorf("encode inland tax details: %w", err)
	}
	return string(raw), nil
}

func decodeInlandTaxDetails(raw string) ([]InlandTaxDetail, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var details []InlandTaxDetail
	if err := json.Unmarshal([]byte(raw), &details); err == nil {
		return details, nil
	}

	var object map[string]any
	if err := json.Unmarshal([]byte(raw), &object); err == nil && len(object) == 0 {
		return nil, nil
	}

	return nil, fmt.Errorf("decode inland tax details: invalid JSON array")
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
	entry.InlandTaxAmount = strings.TrimSpace(entry.InlandTaxAmount)
	entry.InlandTaxCurrency = strings.TrimSpace(entry.InlandTaxCurrency)
	for i := range entry.InlandTaxDetails {
		entry.InlandTaxDetails[i].Code = strings.TrimSpace(entry.InlandTaxDetails[i].Code)
		entry.InlandTaxDetails[i].Label = strings.TrimSpace(entry.InlandTaxDetails[i].Label)
		entry.InlandTaxDetails[i].Amount = strings.TrimSpace(entry.InlandTaxDetails[i].Amount)
		entry.InlandTaxDetails[i].Currency = strings.TrimSpace(entry.InlandTaxDetails[i].Currency)
	}
	entry.ForeignFeesAmount = strings.TrimSpace(entry.ForeignFeesAmount)
	entry.ForeignFeesCurrency = strings.TrimSpace(entry.ForeignFeesCurrency)
	entry.Note = strings.TrimSpace(entry.Note)
	entry.CalcGrossAmountBase = strings.TrimSpace(entry.CalcGrossAmountBase)
	entry.CalcAfterWithholdingAmountBase = strings.TrimSpace(entry.CalcAfterWithholdingAmountBase)

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

const dividendEntrySelectColumns = `
       id, depot_id, security_id, pay_date, ex_date, security_name, security_isin, security_wkn, security_symbol,
       quantity, dividend_per_unit_amount, dividend_per_unit_currency, fx_rate_label, fx_rate, gross_amount, gross_currency,
       payout_amount, payout_currency, withholding_tax_country_code, withholding_tax_percent, withholding_tax_amount,
       withholding_tax_currency, withholding_tax_amount_credit, withholding_tax_amount_credit_currency,
       withholding_tax_amount_refundable, withholding_tax_amount_refundable_currency, inland_tax_amount, inland_tax_currency,
       inland_tax_details, foreign_fees_amount, foreign_fees_currency,
       note, calc_gross_amount_base, calc_after_withholding_amount_base, created_at, updated_at`

func scanDividendEntry(row *sql.Row) (DividendEntry, error) {
	var e DividendEntry
	var inlandTaxDetails string
	if err := row.Scan(
		&e.ID,
		&e.DepotID,
		&e.SecurityID,
		&e.PayDate,
		&e.ExDate,
		&e.SecurityName,
		&e.SecurityISIN,
		&e.SecurityWKN,
		&e.SecuritySymbol,
		&e.Quantity,
		&e.DividendPerUnitAmount,
		&e.DividendPerUnitCurrency,
		&e.FXRateLabel,
		&e.FXRate,
		&e.GrossAmount,
		&e.GrossCurrency,
		&e.PayoutAmount,
		&e.PayoutCurrency,
		&e.WithholdingTaxCountryCode,
		&e.WithholdingTaxPercent,
		&e.WithholdingTaxAmount,
		&e.WithholdingTaxCurrency,
		&e.WithholdingTaxAmountCredit,
		&e.WithholdingTaxAmountCreditCurrency,
		&e.WithholdingTaxAmountRefundable,
		&e.WithholdingTaxAmountRefundableCurrency,
		&e.InlandTaxAmount,
		&e.InlandTaxCurrency,
		&inlandTaxDetails,
		&e.ForeignFeesAmount,
		&e.ForeignFeesCurrency,
		&e.Note,
		&e.CalcGrossAmountBase,
		&e.CalcAfterWithholdingAmountBase,
		&e.CreatedAt,
		&e.UpdatedAt,
	); err != nil {
		return DividendEntry{}, err
	}
	details, err := decodeInlandTaxDetails(inlandTaxDetails)
	if err != nil {
		return DividendEntry{}, err
	}
	e.InlandTaxDetails = details
	return e, nil
}

func scanDividendEntryRow(rows *sql.Rows) (DividendEntry, error) {
	var e DividendEntry
	var inlandTaxDetails string
	if err := rows.Scan(
		&e.ID,
		&e.DepotID,
		&e.SecurityID,
		&e.PayDate,
		&e.ExDate,
		&e.SecurityName,
		&e.SecurityISIN,
		&e.SecurityWKN,
		&e.SecuritySymbol,
		&e.Quantity,
		&e.DividendPerUnitAmount,
		&e.DividendPerUnitCurrency,
		&e.FXRateLabel,
		&e.FXRate,
		&e.GrossAmount,
		&e.GrossCurrency,
		&e.PayoutAmount,
		&e.PayoutCurrency,
		&e.WithholdingTaxCountryCode,
		&e.WithholdingTaxPercent,
		&e.WithholdingTaxAmount,
		&e.WithholdingTaxCurrency,
		&e.WithholdingTaxAmountCredit,
		&e.WithholdingTaxAmountCreditCurrency,
		&e.WithholdingTaxAmountRefundable,
		&e.WithholdingTaxAmountRefundableCurrency,
		&e.InlandTaxAmount,
		&e.InlandTaxCurrency,
		&inlandTaxDetails,
		&e.ForeignFeesAmount,
		&e.ForeignFeesCurrency,
		&e.Note,
		&e.CalcGrossAmountBase,
		&e.CalcAfterWithholdingAmountBase,
		&e.CreatedAt,
		&e.UpdatedAt,
	); err != nil {
		return DividendEntry{}, err
	}
	details, err := decodeInlandTaxDetails(inlandTaxDetails)
	if err != nil {
		return DividendEntry{}, err
	}
	e.InlandTaxDetails = details
	return e, nil
}

func mapDividendEntrySortColumn(sortBy string) (string, error) {
	switch strings.TrimSpace(sortBy) {
	case "", "PayDate":
		return "de.pay_date", nil
	case "ExDate":
		return "de.ex_date", nil
	case "SecurityName":
		return "de.security_name COLLATE NOCASE", nil
	default:
		return "", fmt.Errorf("invalid sort")
	}
}

func normalizeDividendEntrySortDirection(direction string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(direction)) {
	case "", "ASC":
		return "ASC", nil
	case "DESC":
		return "DESC", nil
	default:
		return "", fmt.Errorf("invalid direction")
	}
}

func escapeDividendEntryLikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

func dividendEntrySearchPatterns(search string) []string {
	search = strings.TrimSpace(search)
	if search == "" {
		return nil
	}

	// SQLite's LOWER/LIKE are ASCII-centric without ICU. Keep multiple variants
	// so searches for "öst" still match stored values like "Österreichische Post".
	variants := []string{
		search,
		strings.ToLower(search),
		strings.ToUpper(search),
	}
	runes := []rune(search)
	if len(runes) > 0 {
		title := append([]rune(nil), runes...)
		title[0] = []rune(strings.ToUpper(string(title[0])))[0]
		if len(title) > 1 {
			tail := strings.ToLower(string(title[1:]))
			variants = append(variants, string(title[:1])+tail)
		} else {
			variants = append(variants, string(title))
		}
	}

	seen := make(map[string]struct{}, len(variants))
	patterns := make([]string, 0, len(variants))
	for _, variant := range variants {
		variant = strings.TrimSpace(variant)
		if variant == "" {
			continue
		}
		if _, ok := seen[variant]; ok {
			continue
		}
		seen[variant] = struct{}{}
		patterns = append(patterns, "%"+escapeDividendEntryLikePattern(variant)+"%")
	}
	return patterns
}

func appendDividendEntryFilters(query string, args []any, filters DividendEntryListFilters) (string, []any) {
	fromDate := strings.TrimSpace(filters.FromDate)
	toDate := strings.TrimSpace(filters.ToDate)
	search := strings.TrimSpace(filters.Search)

	if fromDate != "" {
		query += "   AND de.pay_date >= ?\n"
		args = append(args, fromDate)
	}
	if toDate != "" {
		query += "   AND de.pay_date <= ?\n"
		args = append(args, toDate)
	}
	if filters.Year > 0 {
		query += "   AND de.pay_date >= ?\n"
		query += "   AND de.pay_date <= ?\n"
		args = append(args, fmt.Sprintf("%04d-01-01", filters.Year), fmt.Sprintf("%04d-12-31", filters.Year))
	}
	if filters.DepotID > 0 {
		query += "   AND de.depot_id = ?\n"
		args = append(args, filters.DepotID)
	}
	if search != "" {
		patterns := dividendEntrySearchPatterns(search)
		if len(patterns) > 0 {
			query += "   AND (\n"
			for i, pattern := range patterns {
				if i > 0 {
					query += "       OR\n"
				}
				// Match both raw and LOWER(column): raw handles Unicode case variants
				// that SQLite LOWER() cannot normalize, LOWER(column) keeps ASCII search easy.
				query += `       de.security_name LIKE ? ESCAPE '\'
       OR LOWER(de.security_name) LIKE ? ESCAPE '\'
       OR de.security_isin LIKE ? ESCAPE '\'
       OR LOWER(de.security_isin) LIKE ? ESCAPE '\'
       OR de.security_wkn LIKE ? ESCAPE '\'
       OR LOWER(de.security_wkn) LIKE ? ESCAPE '\'
       OR de.security_symbol LIKE ? ESCAPE '\'
       OR LOWER(de.security_symbol) LIKE ? ESCAPE '\'
`
				args = append(args, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
			}
			query += "   )\n"
		}
	}

	return query, args
}

func (d *DB) listDividendEntriesByColumnPage(column string, value int64, limit, offset int, sortBy, direction string, filters DividendEntryListFilters) ([]DividendEntry, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if value <= 0 {
		return nil, fmt.Errorf("%s must be > 0", column)
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit must be >= 0")
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset must be >= 0")
	}

	sortColumn, err := mapDividendEntrySortColumn(sortBy)
	if err != nil {
		return nil, fmt.Errorf("list dividend entries page by %s: %w", column, err)
	}
	sortDirection, err := normalizeDividendEntrySortDirection(direction)
	if err != nil {
		return nil, fmt.Errorf("list dividend entries page by %s: %w", column, err)
	}

	query := `
SELECT` + dividendEntrySelectColumns + `
  FROM dividend_entries de
 WHERE de.` + column + ` = ?
`
	args := []any{value}
	query, args = appendDividendEntryFilters(query, args, filters)
	query += " ORDER BY " + sortColumn + " " + sortDirection + ", de.id " + sortDirection + "\n"
	query += " LIMIT ? OFFSET ?;"
	args = append(args, limit, offset)

	rows, err := d.SQL.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list dividend entries page by %s: %w", column, err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]DividendEntry, 0)
	for rows.Next() {
		e, err := scanDividendEntryRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan dividend entry page by %s: %w", column, err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dividend entries page by %s: %w", column, err)
	}

	return out, nil
}

func (d *DB) countDividendEntriesByColumn(column string, value int64, filters DividendEntryListFilters) (int64, error) {
	if d == nil || d.SQL == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be > 0", column)
	}

	query := `
SELECT COUNT(*)
  FROM dividend_entries de
 WHERE de.` + column + ` = ?
`
	args := []any{value}
	query, args = appendDividendEntryFilters(query, args, filters)
	query += ";"

	var count int64
	err := d.SQL.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count dividend entries by %s: %w", column, err)
	}
	return count, nil
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
	inlandTaxDetails, err := EncodeInlandTaxDetails(normalized.InlandTaxDetails)
	if err != nil {
		return err
	}

	res, err := d.SQL.Exec(`
INSERT INTO dividend_entries (
  depot_id, security_id, pay_date, ex_date, security_name, security_isin, security_wkn, security_symbol,
  quantity, dividend_per_unit_amount, dividend_per_unit_currency, fx_rate_label, fx_rate, gross_amount, gross_currency,
  payout_amount, payout_currency, withholding_tax_country_code, withholding_tax_percent, withholding_tax_amount,
  withholding_tax_currency, withholding_tax_amount_credit, withholding_tax_amount_credit_currency,
  withholding_tax_amount_refundable, withholding_tax_amount_refundable_currency, inland_tax_amount, inland_tax_currency,
  inland_tax_details, foreign_fees_amount, foreign_fees_currency,
  note, calc_gross_amount_base, calc_after_withholding_amount_base, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`, normalized.DepotID, normalized.SecurityID, normalized.PayDate, normalized.ExDate, normalized.SecurityName, normalized.SecurityISIN, normalized.SecurityWKN, normalized.SecuritySymbol, normalized.Quantity, normalized.DividendPerUnitAmount, normalized.DividendPerUnitCurrency, normalized.FXRateLabel, normalized.FXRate, normalized.GrossAmount, normalized.GrossCurrency, normalized.PayoutAmount, normalized.PayoutCurrency, normalized.WithholdingTaxCountryCode, normalized.WithholdingTaxPercent, normalized.WithholdingTaxAmount, normalized.WithholdingTaxCurrency, normalized.WithholdingTaxAmountCredit, normalized.WithholdingTaxAmountCreditCurrency, normalized.WithholdingTaxAmountRefundable, normalized.WithholdingTaxAmountRefundableCurrency, normalized.InlandTaxAmount, normalized.InlandTaxCurrency, inlandTaxDetails, normalized.ForeignFeesAmount, normalized.ForeignFeesCurrency, normalized.Note, normalized.CalcGrossAmountBase, normalized.CalcAfterWithholdingAmountBase, normalized.CreatedAt, normalized.UpdatedAt)
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
	inlandTaxDetails, err := EncodeInlandTaxDetails(normalized.InlandTaxDetails)
	if err != nil {
		return err
	}

	_, err = d.SQL.Exec(`
UPDATE dividend_entries
   SET depot_id = ?,
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
       inland_tax_amount = ?,
       inland_tax_currency = ?,
       inland_tax_details = ?,
       foreign_fees_amount = ?,
       foreign_fees_currency = ?,
       note = ?,
       calc_gross_amount_base = ?,
       calc_after_withholding_amount_base = ?,
       updated_at = ?
 WHERE id = ?;
`, normalized.DepotID, normalized.SecurityID, normalized.PayDate, normalized.ExDate, normalized.SecurityName, normalized.SecurityISIN, normalized.SecurityWKN, normalized.SecuritySymbol, normalized.Quantity, normalized.DividendPerUnitAmount, normalized.DividendPerUnitCurrency, normalized.FXRateLabel, normalized.FXRate, normalized.GrossAmount, normalized.GrossCurrency, normalized.PayoutAmount, normalized.PayoutCurrency, normalized.WithholdingTaxCountryCode, normalized.WithholdingTaxPercent, normalized.WithholdingTaxAmount, normalized.WithholdingTaxCurrency, normalized.WithholdingTaxAmountCredit, normalized.WithholdingTaxAmountCreditCurrency, normalized.WithholdingTaxAmountRefundable, normalized.WithholdingTaxAmountRefundableCurrency, normalized.InlandTaxAmount, normalized.InlandTaxCurrency, inlandTaxDetails, normalized.ForeignFeesAmount, normalized.ForeignFeesCurrency, normalized.Note, normalized.CalcGrossAmountBase, normalized.CalcAfterWithholdingAmountBase, normalized.UpdatedAt, normalized.ID)
	if err != nil {
		return fmt.Errorf("update dividend entry: %w", err)
	}

	*entry = normalized
	return nil
}

// GetDividendEntryByID returns data for the requested input.
func (d *DB) GetDividendEntryByID(id int64) (DividendEntry, bool, error) {
	if d == nil || d.SQL == nil {
		return DividendEntry{}, false, fmt.Errorf("db not initialized")
	}
	if id <= 0 {
		return DividendEntry{}, false, fmt.Errorf("id must be > 0")
	}

	row := d.SQL.QueryRow(`
SELECT`+dividendEntrySelectColumns+`
  FROM dividend_entries
 WHERE id = ?
 LIMIT 1;
`, id)

	entry, err := scanDividendEntry(row)
	if err == sql.ErrNoRows {
		return DividendEntry{}, false, nil
	}
	if err != nil {
		return DividendEntry{}, false, fmt.Errorf("get dividend entry by id: %w", err)
	}

	return entry, true, nil
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

// ListAllDividendEntries returns all dividend entry rows without any filter. Intended for full-database exports.
func (d *DB) ListAllDividendEntries() ([]DividendEntry, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := d.SQL.Query(`
SELECT` + dividendEntrySelectColumns + `
  FROM dividend_entries
 ORDER BY pay_date ASC, id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list all dividend entries for export: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]DividendEntry, 0)
	for rows.Next() {
		e, err := scanDividendEntryRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan dividend entry for export: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dividend entries for export: %w", err)
	}

	return out, nil
}

// ListDividendEntriesByUserMembership returns a filtered and paginated list of dividend
// entries for depots the given user has direct depot membership on.
func appendDividendEntryRolesFilter(query string, args []any, roles []string) (string, []any) {
	if len(roles) == 0 {
		return query, args
	}

	query += "   AND m.role IN (" + sqlPlaceholders(len(roles)) + ")\n"
	for _, role := range roles {
		args = append(args, role)
	}

	return query, args
}

func dividendEntryYearMonthFromPayDate(payDate string) (int, int, error) {
	payDate = strings.TrimSpace(payDate)
	if len(payDate) < 7 {
		return 0, 0, fmt.Errorf("invalid pay_date")
	}
	year, err := strconv.Atoi(payDate[:4])
	if err != nil || year <= 0 {
		return 0, 0, fmt.Errorf("invalid pay_date")
	}
	month, err := strconv.Atoi(payDate[5:7])
	if err != nil || month < 1 || month > 12 {
		return 0, 0, fmt.Errorf("invalid pay_date")
	}
	return year, month, nil
}

func dividendEntryTimeRangeFromPayDates(firstPayDate, lastPayDate string) (DividendEntryTimeRange, error) {
	firstYear, firstMonth, err := dividendEntryYearMonthFromPayDate(firstPayDate)
	if err != nil {
		return DividendEntryTimeRange{}, err
	}
	lastYear, lastMonth, err := dividendEntryYearMonthFromPayDate(lastPayDate)
	if err != nil {
		return DividendEntryTimeRange{}, err
	}
	return DividendEntryTimeRange{
		FirstYear:  firstYear,
		FirstMonth: firstMonth,
		LastYear:   lastYear,
		LastMonth:  lastMonth,
	}, nil
}

func scanDividendEntryTimeRange(row *sql.Row) (DividendEntryTimeRange, bool, error) {
	var firstPayDate sql.NullString
	var lastPayDate sql.NullString
	if err := row.Scan(&firstPayDate, &lastPayDate); err != nil {
		return DividendEntryTimeRange{}, false, err
	}
	if !firstPayDate.Valid || !lastPayDate.Valid {
		return DividendEntryTimeRange{}, false, nil
	}

	timeRange, err := dividendEntryTimeRangeFromPayDates(firstPayDate.String, lastPayDate.String)
	if err != nil {
		return DividendEntryTimeRange{}, false, err
	}
	return timeRange, true, nil
}

// GetDividendEntryTimeRangeByDepotID returns the first and last pay-date range for one depot.
func (d *DB) GetDividendEntryTimeRangeByDepotID(depotID int64) (DividendEntryTimeRange, bool, error) {
	if d == nil || d.SQL == nil {
		return DividendEntryTimeRange{}, false, fmt.Errorf("db not initialized")
	}
	if depotID <= 0 {
		return DividendEntryTimeRange{}, false, fmt.Errorf("depotID must be > 0")
	}

	timeRange, found, err := scanDividendEntryTimeRange(d.SQL.QueryRow(`
SELECT MIN(de.pay_date), MAX(de.pay_date)
  FROM dividend_entries de
 WHERE de.depot_id = ?
   AND TRIM(de.pay_date) <> '';
`, depotID))
	if err != nil {
		return DividendEntryTimeRange{}, false, fmt.Errorf("get dividend entry time range by depot: %w", err)
	}
	return timeRange, found, nil
}

// GetAccessibleDividendEntryTimeRangeByUser returns the first and last pay-date range
// for entries accessible to the user for the requested action scope.
func (d *DB) GetAccessibleDividendEntryTimeRangeByUser(userID int64, all bool, roles []string) (DividendEntryTimeRange, bool, error) {
	if d == nil || d.SQL == nil {
		return DividendEntryTimeRange{}, false, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return DividendEntryTimeRange{}, false, fmt.Errorf("userID must be > 0")
	}

	query := ""
	args := make([]any, 0, 2+len(roles))
	if all {
		query = `
SELECT MIN(de.pay_date), MAX(de.pay_date)
  FROM dividend_entries de
 WHERE TRIM(de.pay_date) <> ''
`
	} else {
		query = `
SELECT MIN(de.pay_date), MAX(de.pay_date)
  FROM dividend_entries de
  JOIN memberships m ON m.entity_type = ? AND m.entity_id = de.depot_id
 WHERE m.user_id = ?
   AND TRIM(de.pay_date) <> ''
`
		args = append(args, EntityTypeDepot, userID)
		query, args = appendDividendEntryRolesFilter(query, args, roles)
	}
	query += ";"

	timeRange, found, err := scanDividendEntryTimeRange(d.SQL.QueryRow(query, args...))
	if err != nil {
		return DividendEntryTimeRange{}, false, fmt.Errorf("get accessible dividend entry time range by user: %w", err)
	}
	return timeRange, found, nil
}

// ListAccessibleDividendEntriesByUser returns a filtered and paginated list of dividend
// entries accessible to the user for the requested action scope.
func (d *DB) ListAccessibleDividendEntriesByUser(userID int64, all bool, roles []string, limit, offset int, sortBy, direction string, filters DividendEntryListFilters) ([]DividendEntry, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("userID must be > 0")
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit must be >= 0")
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset must be >= 0")
	}

	sortColumn, err := mapDividendEntrySortColumn(sortBy)
	if err != nil {
		return nil, fmt.Errorf("list accessible dividend entries by user: %w", err)
	}
	sortDirection, err := normalizeDividendEntrySortDirection(direction)
	if err != nil {
		return nil, fmt.Errorf("list accessible dividend entries by user: %w", err)
	}

	query := ""
	args := make([]any, 0, 2+len(roles))
	if all {
		query = `
SELECT` + dividendEntrySelectColumns + `
  FROM dividend_entries de
 WHERE 1 = 1
`
	} else {
		query = `
SELECT` + dividendEntrySelectColumns + `
  FROM dividend_entries de
  JOIN memberships m ON m.entity_type = ? AND m.entity_id = de.depot_id
 WHERE m.user_id = ?
`
		args = append(args, EntityTypeDepot, userID)
		query, args = appendDividendEntryRolesFilter(query, args, roles)
	}

	query, args = appendDividendEntryFilters(query, args, filters)
	query += " ORDER BY " + sortColumn + " " + sortDirection + ", de.id " + sortDirection + "\n"
	query += " LIMIT ? OFFSET ?;"
	args = append(args, limit, offset)

	rows, err := d.SQL.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list accessible dividend entries by user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]DividendEntry, 0)
	for rows.Next() {
		e, err := scanDividendEntryRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan accessible dividend entry by user: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accessible dividend entries by user: %w", err)
	}

	return out, nil
}

// CountAccessibleDividendEntriesByUser returns the total number of filtered dividend
// entries accessible to the user for the requested action scope.
func (d *DB) CountAccessibleDividendEntriesByUser(userID int64, all bool, roles []string, filters DividendEntryListFilters) (int64, error) {
	if d == nil || d.SQL == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return 0, fmt.Errorf("userID must be > 0")
	}

	query := ""
	args := make([]any, 0, 2+len(roles))
	if all {
		query = `
SELECT COUNT(*)
  FROM dividend_entries de
 WHERE 1 = 1
`
	} else {
		query = `
SELECT COUNT(*)
  FROM dividend_entries de
  JOIN memberships m ON m.entity_type = ? AND m.entity_id = de.depot_id
 WHERE m.user_id = ?
`
		args = append(args, EntityTypeDepot, userID)
		query, args = appendDividendEntryRolesFilter(query, args, roles)
	}

	query, args = appendDividendEntryFilters(query, args, filters)
	query += ";"

	var count int64
	err := d.SQL.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count accessible dividend entries by user: %w", err)
	}
	return count, nil
}

// ListAccessibleDividendEntriesBySecurityID returns accessible dividend entries for a security.
func (d *DB) ListAccessibleDividendEntriesBySecurityID(userID int64, all bool, roles []string, securityID int64, limit, offset int, sortBy, direction string, filters DividendEntryListFilters) ([]DividendEntry, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("userID must be > 0")
	}
	if securityID <= 0 {
		return nil, fmt.Errorf("securityID must be > 0")
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit must be >= 0")
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset must be >= 0")
	}

	sortColumn, err := mapDividendEntrySortColumn(sortBy)
	if err != nil {
		return nil, fmt.Errorf("list accessible dividend entries by security: %w", err)
	}
	sortDirection, err := normalizeDividendEntrySortDirection(direction)
	if err != nil {
		return nil, fmt.Errorf("list accessible dividend entries by security: %w", err)
	}

	query := ""
	args := make([]any, 0, 3+len(roles))
	if all {
		query = `
SELECT` + dividendEntrySelectColumns + `
  FROM dividend_entries de
 WHERE de.security_id = ?
`
		args = append(args, securityID)
	} else {
		query = `
SELECT` + dividendEntrySelectColumns + `
  FROM dividend_entries de
  JOIN memberships m ON m.entity_type = ? AND m.entity_id = de.depot_id
 WHERE m.user_id = ?
   AND de.security_id = ?
`
		args = append(args, EntityTypeDepot, userID, securityID)
		query, args = appendDividendEntryRolesFilter(query, args, roles)
	}

	query, args = appendDividendEntryFilters(query, args, filters)
	query += " ORDER BY " + sortColumn + " " + sortDirection + ", de.id " + sortDirection + "\n"
	query += " LIMIT ? OFFSET ?;"
	args = append(args, limit, offset)

	rows, err := d.SQL.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list accessible dividend entries by security: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]DividendEntry, 0)
	for rows.Next() {
		e, err := scanDividendEntryRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan accessible dividend entry by security: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accessible dividend entries by security: %w", err)
	}

	return out, nil
}

// CountAccessibleDividendEntriesBySecurityID returns the total accessible entry count for a security.
func (d *DB) CountAccessibleDividendEntriesBySecurityID(userID int64, all bool, roles []string, securityID int64, filters DividendEntryListFilters) (int64, error) {
	if d == nil || d.SQL == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return 0, fmt.Errorf("userID must be > 0")
	}
	if securityID <= 0 {
		return 0, fmt.Errorf("securityID must be > 0")
	}

	query := ""
	args := make([]any, 0, 3+len(roles))
	if all {
		query = `
SELECT COUNT(*)
  FROM dividend_entries de
 WHERE de.security_id = ?
`
		args = append(args, securityID)
	} else {
		query = `
SELECT COUNT(*)
  FROM dividend_entries de
  JOIN memberships m ON m.entity_type = ? AND m.entity_id = de.depot_id
 WHERE m.user_id = ?
   AND de.security_id = ?
`
		args = append(args, EntityTypeDepot, userID, securityID)
		query, args = appendDividendEntryRolesFilter(query, args, roles)
	}

	query, args = appendDividendEntryFilters(query, args, filters)
	query += ";"

	var count int64
	err := d.SQL.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count accessible dividend entries by security: %w", err)
	}
	return count, nil
}

// ListDividendEntriesByDepotID returns a filtered and paginated list for the requested filter.
func (d *DB) ListDividendEntriesByDepotID(depotID int64, limit, offset int, sortBy, direction string, filters DividendEntryListFilters) ([]DividendEntry, error) {
	return d.listDividendEntriesByColumnPage("depot_id", depotID, limit, offset, sortBy, direction, filters)
}

// CountDividendEntriesByDepotID returns the total number of filtered records for the requested filter.
func (d *DB) CountDividendEntriesByDepotID(depotID int64, filters DividendEntryListFilters) (int64, error) {
	return d.countDividendEntriesByColumn("depot_id", depotID, filters)
}

// ListDividendEntriesBySecurityID returns a filtered and paginated list for the requested filter.
func (d *DB) ListDividendEntriesBySecurityID(securityID int64, limit, offset int, sortBy, direction string, filters DividendEntryListFilters) ([]DividendEntry, error) {
	return d.listDividendEntriesByColumnPage("security_id", securityID, limit, offset, sortBy, direction, filters)
}

// CountDividendEntriesBySecurityID returns the total number of filtered records for the requested filter.
func (d *DB) CountDividendEntriesBySecurityID(securityID int64, filters DividendEntryListFilters) (int64, error) {
	return d.countDividendEntriesByColumn("security_id", securityID, filters)
}
