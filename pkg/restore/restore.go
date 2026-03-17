package restore

import (
	"encoding/json"
	"fmt"

	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/export"
)

// formatProbe is used to detect the backup format before full parsing.
type formatProbe struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
}

// Load detects the format of the backup data and returns a parsed ExportDoc.
// If the backup is encrypted, passwordFn is called once to obtain the password.
// Returns an error with message "INVALID_BACKUP_FORMAT" for unrecognised data.
func Load(data []byte, passwordFn func() (string, error)) (*export.ExportDoc, error) {
	var probe formatProbe
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
	}

	switch probe.Format {
	case "schrevind-encrypted-backup":
		password, err := passwordFn()
		if err != nil {
			return nil, err
		}

		plaintext, err := export.Decrypt(data, password)
		if err != nil {
			// export.Decrypt already returns "DECRYPTION_FAILED" or "INVALID_BACKUP_FORMAT".
			return nil, err
		}

		// Re-probe the decrypted content.
		if err := json.Unmarshal(plaintext, &probe); err != nil {
			return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
		}
		if probe.Format != "schrevind-export" || probe.Version != 1 {
			return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
		}

		var doc export.ExportDoc
		if err := json.Unmarshal(plaintext, &doc); err != nil {
			return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
		}
		return &doc, nil

	case "schrevind-export":
		if probe.Version != 1 {
			return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
		}
		var doc export.ExportDoc
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
		}
		return &doc, nil

	default:
		return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
	}
}

// Run performs a full database restore from doc inside a single transaction.
// All existing data is deleted first, then backup data is inserted in FK-safe order.
// On any error the transaction is rolled back and no data is changed.
func Run(database *db.DB, doc *export.ExportDoc) error {
	tx, err := database.SQL.Begin()
	if err != nil {
		return fmt.Errorf("IMPORT_FAILED: begin transaction: %w", err)
	}
	// Rollback is a no-op after a successful Commit (returns sql.ErrTxDone, ignored).
	defer func() { _ = tx.Rollback() }()

	// Delete all rows in reverse FK dependency order to satisfy constraints.
	deleteStmts := []string{
		`DELETE FROM dividend_entries;`,
		`DELETE FROM withholding_tax_defaults;`,
		`DELETE FROM depots;`,
		`DELETE FROM users;`,
		`DELETE FROM securities;`,
		`DELETE FROM currencies;`,
	}
	for _, stmt := range deleteStmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("IMPORT_FAILED: clear table: %w", err)
		}
	}

	// 1. Insert users.
	for _, u := range doc.Data.Users {
		if _, err := tx.Exec(`
INSERT INTO users (id, password, firstname, lastname, email, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);
`, u.ID, u.Password, u.FirstName, u.LastName, u.Email, u.Status, u.CreatedAt, u.UpdatedAt); err != nil {
			return fmt.Errorf("IMPORT_FAILED: insert user id=%d: %w", u.ID, err)
		}
	}

	// 2. Insert depots (references users).
	for _, d := range doc.Data.Depots {
		if _, err := tx.Exec(`
INSERT INTO depots (id, user_id, name, broker_name, account_number, base_currency, description, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`, d.ID, d.UserID, d.Name, d.BrokerName, d.AccountNumber, d.BaseCurrency, d.Description, d.Status, d.CreatedAt, d.UpdatedAt); err != nil {
			return fmt.Errorf("IMPORT_FAILED: insert depot id=%d: %w", d.ID, err)
		}
	}

	// 3. Insert securities.
	for _, s := range doc.Data.Securities {
		if _, err := tx.Exec(`
INSERT INTO securities (id, name, isin, wkn, symbol, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);
`, s.ID, s.Name, s.ISIN, s.WKN, s.Symbol, s.Status, s.CreatedAt, s.UpdatedAt); err != nil {
			return fmt.Errorf("IMPORT_FAILED: insert security id=%d: %w", s.ID, err)
		}
	}

	// 4. Insert currencies.
	for _, c := range doc.Data.Currencies {
		if _, err := tx.Exec(`
INSERT INTO currencies (id, currency, name, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?);
`, c.ID, c.Currency, c.Name, c.Status, c.CreatedAt, c.UpdatedAt); err != nil {
			return fmt.Errorf("IMPORT_FAILED: insert currency id=%d: %w", c.ID, err)
		}
	}

	// 5. Insert withholding_tax_defaults (depot_id is nullable: 0 → NULL).
	for _, w := range doc.Data.WithholdingTaxDefaults {
		var depotID interface{}
		if w.DepotID > 0 {
			depotID = w.DepotID
		}
		if _, err := tx.Exec(`
INSERT INTO withholding_tax_defaults (id, depot_id, country_code, country_name, withholding_tax_percent_default, withholding_tax_percent_credit_default, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);
`, w.ID, depotID, w.CountryCode, w.CountryName, w.WithholdingTaxPercentDefault, w.WithholdingTaxPercentCreditDefault, w.CreatedAt, w.UpdatedAt); err != nil {
			return fmt.Errorf("IMPORT_FAILED: insert withholding_tax_default id=%d: %w", w.ID, err)
		}
	}

	// 6. Insert dividend_entries (references users, depots, securities).
	for _, e := range doc.Data.DividendEntries {
		if _, err := tx.Exec(`
INSERT INTO dividend_entries (
  id, user_id, depot_id, security_id, pay_date, ex_date,
  security_name, security_isin, security_wkn, security_symbol,
  quantity, dividend_per_unit_amount, dividend_per_unit_currency,
  fx_rate_label, fx_rate,
  gross_amount, gross_currency,
  payout_amount, payout_currency,
  withholding_tax_country_code, withholding_tax_percent,
  withholding_tax_amount, withholding_tax_currency,
  withholding_tax_amount_credit, withholding_tax_amount_credit_currency,
  withholding_tax_amount_refundable, withholding_tax_amount_refundable_currency,
  foreign_fees_amount, foreign_fees_currency,
  note, calc_gross_amount_base, calc_after_withholding_amount_base,
  created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`, e.ID, e.UserID, e.DepotID, e.SecurityID, e.PayDate, e.ExDate,
			e.SecurityName, e.SecurityISIN, e.SecurityWKN, e.SecuritySymbol,
			e.Quantity, e.DividendPerUnitAmount, e.DividendPerUnitCurrency,
			e.FXRateLabel, e.FXRate,
			e.GrossAmount, e.GrossCurrency,
			e.PayoutAmount, e.PayoutCurrency,
			e.WithholdingTaxCountryCode, e.WithholdingTaxPercent,
			e.WithholdingTaxAmount, e.WithholdingTaxCurrency,
			e.WithholdingTaxAmountCredit, e.WithholdingTaxAmountCreditCurrency,
			e.WithholdingTaxAmountRefundable, e.WithholdingTaxAmountRefundableCurrency,
			e.ForeignFeesAmount, e.ForeignFeesCurrency,
			e.Note, e.CalcGrossAmountBase, e.CalcAfterWithholdingAmountBase,
			e.CreatedAt, e.UpdatedAt); err != nil {
			return fmt.Errorf("IMPORT_FAILED: insert dividend_entry id=%d: %w", e.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("IMPORT_FAILED: commit: %w", err)
	}

	return nil
}
