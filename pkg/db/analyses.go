package db

import "fmt"

// DividendsByYearSourceRow contains raw database values used by the yearly dividend analysis.
type DividendsByYearSourceRow struct {
	Year             string
	Gross            string
	AfterWithholding string
	Net              string
}

// DividendsByYearMonthSourceRow contains raw values used by the yearly/monthly dividend analysis.
type DividendsByYearMonthSourceRow struct {
	Year             string
	Month            string
	Gross            string
	AfterWithholding string
	Net              string
}

// DividendsBySecurityYearSourceRow contains raw values used by the security/year dividend analysis.
type DividendsBySecurityYearSourceRow struct {
	SecurityID       int64
	SecurityName     string
	SecurityISIN     string
	Year             string
	PayDate          string
	Quantity         string
	Gross            string
	AfterWithholding string
	Net              string
}

// ListDividendAnalysisRowsByDepotIDs returns dividend-entry values for the requested depots.
func (d *DB) ListDividendAnalysisRowsByDepotIDs(depotIDs []int64) ([]DividendsByYearSourceRow, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if len(depotIDs) == 0 {
		return []DividendsByYearSourceRow{}, nil
	}

	query := `
SELECT strftime('%Y', pay_date) AS year,
       calc_gross_amount_base,
       calc_after_withholding_amount_base,
       payout_amount
  FROM dividend_entries
 WHERE depot_id IN (` + sqlPlaceholders(len(depotIDs)) + `)
   AND pay_date != ''
   AND strftime('%Y', pay_date) IS NOT NULL
 ORDER BY year ASC;
`
	args := make([]any, 0, len(depotIDs))
	for _, depotID := range depotIDs {
		args = append(args, depotID)
	}

	rows, err := d.SQL.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list dividend analysis rows by depot ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]DividendsByYearSourceRow, 0)
	for rows.Next() {
		var row DividendsByYearSourceRow
		if err := rows.Scan(
			&row.Year,
			&row.Gross,
			&row.AfterWithholding,
			&row.Net,
		); err != nil {
			return nil, fmt.Errorf("scan dividend analysis row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dividend analysis rows: %w", err)
	}

	return out, nil
}

// ListDividendAnalysisMonthRowsByDepotIDs returns dividend-entry values grouped later by year and month.
func (d *DB) ListDividendAnalysisMonthRowsByDepotIDs(depotIDs []int64) ([]DividendsByYearMonthSourceRow, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if len(depotIDs) == 0 {
		return []DividendsByYearMonthSourceRow{}, nil
	}

	query := `
SELECT strftime('%Y', pay_date) AS year,
       strftime('%m', pay_date) AS month,
       calc_gross_amount_base,
       calc_after_withholding_amount_base,
       payout_amount
  FROM dividend_entries
 WHERE depot_id IN (` + sqlPlaceholders(len(depotIDs)) + `)
   AND pay_date != ''
   AND strftime('%Y', pay_date) IS NOT NULL
   AND strftime('%m', pay_date) IS NOT NULL
 ORDER BY year ASC, month ASC;
`
	args := make([]any, 0, len(depotIDs))
	for _, depotID := range depotIDs {
		args = append(args, depotID)
	}

	rows, err := d.SQL.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list dividend analysis month rows by depot ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]DividendsByYearMonthSourceRow, 0)
	for rows.Next() {
		var row DividendsByYearMonthSourceRow
		if err := rows.Scan(
			&row.Year,
			&row.Month,
			&row.Gross,
			&row.AfterWithholding,
			&row.Net,
		); err != nil {
			return nil, fmt.Errorf("scan dividend analysis month row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dividend analysis month rows: %w", err)
	}

	return out, nil
}

// ListDividendAnalysisSecurityYearRowsByDepotIDs returns dividend-entry values grouped later by security, year, and quantity.
func (d *DB) ListDividendAnalysisSecurityYearRowsByDepotIDs(depotIDs []int64) ([]DividendsBySecurityYearSourceRow, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if len(depotIDs) == 0 {
		return []DividendsBySecurityYearSourceRow{}, nil
	}

	query := `
SELECT security_id,
       security_name,
       security_isin,
       strftime('%Y', pay_date) AS year,
       pay_date,
       quantity,
       calc_gross_amount_base,
       calc_after_withholding_amount_base,
       payout_amount
  FROM dividend_entries
 WHERE depot_id IN (` + sqlPlaceholders(len(depotIDs)) + `)
   AND pay_date != ''
   AND strftime('%Y', pay_date) IS NOT NULL
 ORDER BY security_name COLLATE NOCASE ASC, year ASC, pay_date ASC, id ASC;
`
	args := make([]any, 0, len(depotIDs))
	for _, depotID := range depotIDs {
		args = append(args, depotID)
	}

	rows, err := d.SQL.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list dividend analysis security year rows by depot ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]DividendsBySecurityYearSourceRow, 0)
	for rows.Next() {
		var row DividendsBySecurityYearSourceRow
		if err := rows.Scan(
			&row.SecurityID,
			&row.SecurityName,
			&row.SecurityISIN,
			&row.Year,
			&row.PayDate,
			&row.Quantity,
			&row.Gross,
			&row.AfterWithholding,
			&row.Net,
		); err != nil {
			return nil, fmt.Errorf("scan dividend analysis security year row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dividend analysis security year rows: %w", err)
	}

	return out, nil
}
