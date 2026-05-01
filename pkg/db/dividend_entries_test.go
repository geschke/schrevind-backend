package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func newDividendEntriesTestDB(t *testing.T) *DB {
	t.Helper()

	database, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return database
}

func createDividendEntryTestDependencies(t *testing.T, database *DB) (int64, int64) {
	t.Helper()

	group := Group{Name: "Dividend Entry Test Group"}
	if err := database.CreateGroup(&group); err != nil {
		t.Fatalf("create group: %v", err)
	}

	depot := Depot{Name: "Dividend Entry Depot", BaseCurrency: "EUR", Status: "active"}
	if err := database.CreateDepot(&depot); err != nil {
		t.Fatalf("create depot: %v", err)
	}

	security := Security{
		GroupID: group.ID,
		Name:    "Dividend Entry Security",
		ISIN:    "DE0000000008",
		Status:  SecurityStatusActive,
	}
	if err := database.CreateSecurity(&security); err != nil {
		t.Fatalf("create security: %v", err)
	}

	return depot.ID, security.ID
}

func validDividendEntryForDBTest(t *testing.T, database *DB) DividendEntry {
	t.Helper()

	depotID, securityID := createDividendEntryTestDependencies(t, database)
	return DividendEntry{
		DepotID:                        depotID,
		SecurityID:                     securityID,
		PayDate:                        "2026-01-15",
		ExDate:                         "2026-01-10",
		SecurityName:                   "Dividend Entry Security",
		SecurityISIN:                   "DE0000000008",
		Quantity:                       "10",
		DividendPerUnitAmount:          "1.23",
		DividendPerUnitCurrency:        "EUR",
		FXRate:                         "1",
		GrossAmount:                    "12.30",
		GrossCurrency:                  "EUR",
		PayoutAmount:                   "9.00",
		PayoutCurrency:                 "EUR",
		WithholdingTaxAmount:           "1.00",
		WithholdingTaxCurrency:         "EUR",
		InlandTaxAmount:                "2.30",
		InlandTaxCurrency:              "EUR",
		CalcGrossAmountBase:            "12.30",
		CalcAfterWithholdingAmountBase: "11.30",
		InlandTaxDetails: []InlandTaxDetail{
			{Code: " capital_gains_tax ", Label: " Kapitalertragsteuer ", Amount: "2.00", Currency: "EUR"},
			{Code: "solidarity_surcharge", Label: "Solidaritätszuschlag", Amount: "0.30", Currency: "EUR"},
		},
	}
}

func TestCreateDividendEntryStoresInlandTaxDetails(t *testing.T) {
	database := newDividendEntriesTestDB(t)
	entry := validDividendEntryForDBTest(t, database)

	if err := database.CreateDividendEntry(&entry); err != nil {
		t.Fatalf("CreateDividendEntry() error = %v", err)
	}

	got, found, err := database.GetDividendEntryByID(entry.ID)
	if err != nil {
		t.Fatalf("GetDividendEntryByID() error = %v", err)
	}
	if !found {
		t.Fatalf("GetDividendEntryByID() found = false")
	}
	if got.InlandTaxAmount != "2.30" || got.InlandTaxCurrency != "EUR" {
		t.Fatalf("inland tax total = %q/%q, want 2.30/EUR", got.InlandTaxAmount, got.InlandTaxCurrency)
	}
	if len(got.InlandTaxDetails) != 2 {
		t.Fatalf("InlandTaxDetails len = %d, want 2", len(got.InlandTaxDetails))
	}
	if got.InlandTaxDetails[0].Code != "capital_gains_tax" || got.InlandTaxDetails[0].Label != "Kapitalertragsteuer" {
		t.Fatalf("first InlandTaxDetail = %+v, want trimmed values", got.InlandTaxDetails[0])
	}
}

func TestGetDividendEntryAcceptsEmptyInlandTaxDetailsJSON(t *testing.T) {
	database := newDividendEntriesTestDB(t)
	entry := validDividendEntryForDBTest(t, database)
	entry.InlandTaxDetails = nil

	if err := database.CreateDividendEntry(&entry); err != nil {
		t.Fatalf("CreateDividendEntry() error = %v", err)
	}
	var stored string
	if err := database.SQL.QueryRow(`SELECT inland_tax_details FROM dividend_entries WHERE id = ?;`, entry.ID).Scan(&stored); err != nil {
		t.Fatalf("query stored inland_tax_details: %v", err)
	}
	if stored != "[]" {
		t.Fatalf("stored inland_tax_details = %q, want []", stored)
	}
	for _, raw := range []string{"", "[]", "{}"} {
		if _, err := database.SQL.Exec(`UPDATE dividend_entries SET inland_tax_details = ? WHERE id = ?;`, raw, entry.ID); err != nil {
			t.Fatalf("update inland_tax_details to %q: %v", raw, err)
		}
		got, found, err := database.GetDividendEntryByID(entry.ID)
		if err != nil {
			t.Fatalf("GetDividendEntryByID(%q) error = %v", raw, err)
		}
		if !found {
			t.Fatalf("GetDividendEntryByID(%q) found = false", raw)
		}
		if len(got.InlandTaxDetails) != 0 {
			t.Fatalf("InlandTaxDetails for %q = %+v, want empty", raw, got.InlandTaxDetails)
		}
	}
}

func TestGetDividendEntryRejectsBrokenInlandTaxDetailsJSON(t *testing.T) {
	database := newDividendEntriesTestDB(t)
	entry := validDividendEntryForDBTest(t, database)

	if err := database.CreateDividendEntry(&entry); err != nil {
		t.Fatalf("CreateDividendEntry() error = %v", err)
	}
	if _, err := database.SQL.Exec(`UPDATE dividend_entries SET inland_tax_details = ? WHERE id = ?;`, "{broken", entry.ID); err != nil {
		t.Fatalf("update invalid inland_tax_details: %v", err)
	}

	_, _, err := database.GetDividendEntryByID(entry.ID)
	if err == nil || !strings.Contains(err.Error(), "invalid JSON array") {
		t.Fatalf("GetDividendEntryByID() error = %v, want invalid JSON array", err)
	}
}
