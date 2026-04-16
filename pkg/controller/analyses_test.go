package controller

import (
	"testing"

	"github.com/geschke/schrevind/pkg/db"
)

func TestUniqueDepotBaseCurrency(t *testing.T) {
	currency, ok, currencies := uniqueDepotBaseCurrency([]db.Depot{
		{ID: 1, BaseCurrency: "EUR"},
		{ID: 2, BaseCurrency: "EUR"},
	})
	if !ok {
		t.Fatalf("uniqueDepotBaseCurrency() ok = false, want true")
	}
	if currency != "EUR" {
		t.Fatalf("uniqueDepotBaseCurrency() currency = %q, want EUR", currency)
	}
	if len(currencies) != 1 || currencies[0] != "EUR" {
		t.Fatalf("uniqueDepotBaseCurrency() currencies = %v, want [EUR]", currencies)
	}
}

func TestUniqueDepotBaseCurrencyRejectsMixedCurrencies(t *testing.T) {
	_, ok, currencies := uniqueDepotBaseCurrency([]db.Depot{
		{ID: 1, BaseCurrency: "USD"},
		{ID: 2, BaseCurrency: "EUR"},
	})
	if ok {
		t.Fatalf("uniqueDepotBaseCurrency() ok = true, want false")
	}
	if len(currencies) != 2 || currencies[0] != "EUR" || currencies[1] != "USD" {
		t.Fatalf("uniqueDepotBaseCurrency() currencies = %v, want [EUR USD]", currencies)
	}
}

func TestBuildDividendsByYearRows(t *testing.T) {
	rows, ok := buildDividendsByYearRows([]db.DividendsByYearSourceRow{
		{Year: "2022", Gross: "100.00", AfterWithholding: "80.00", Net: "75.00"},
		{Year: "2021", Gross: "50.00", AfterWithholding: "40.00", Net: "35.00"},
		{Year: "2022", Gross: "25.50", AfterWithholding: "20.25", Net: "19.25"},
	}, 2, "en-US")
	if !ok {
		t.Fatalf("buildDividendsByYearRows() ok = false, want true")
	}
	if len(rows) != 2 {
		t.Fatalf("buildDividendsByYearRows() len = %d, want 2", len(rows))
	}

	assertAnalysisRow(t, rows[0], "2021", "50.00", "40.00", "35.00")
	assertAnalysisRow(t, rows[1], "2022", "125.50", "100.25", "94.25")
}

func TestBuildDividendsByYearRowsRejectsInvalidDecimal(t *testing.T) {
	_, ok := buildDividendsByYearRows([]db.DividendsByYearSourceRow{
		{Year: "2022", Gross: "bad", AfterWithholding: "80.00", Net: "75.00"},
	}, 2, "en-US")
	if ok {
		t.Fatalf("buildDividendsByYearRows() ok = true, want false")
	}
}

func TestBuildDividendsByYearRowsFormatsLocale(t *testing.T) {
	rows, ok := buildDividendsByYearRows([]db.DividendsByYearSourceRow{
		{Year: "2022", Gross: "100.00", AfterWithholding: "80.00", Net: "75.50"},
	}, 2, "de-DE")
	if !ok {
		t.Fatalf("buildDividendsByYearRows() ok = false, want true")
	}
	if len(rows) != 1 {
		t.Fatalf("buildDividendsByYearRows() len = %d, want 1", len(rows))
	}

	assertAnalysisRow(t, rows[0], "2022", "100,00", "80,00", "75,50")
}

func assertAnalysisRow(t *testing.T, row AnalysisRow, year, gross, afterWithholding, net string) {
	t.Helper()
	if row["year"] != year {
		t.Fatalf("row year = %q, want %q", row["year"], year)
	}
	if row["gross"] != gross {
		t.Fatalf("row gross = %q, want %q", row["gross"], gross)
	}
	if row["after_withholding"] != afterWithholding {
		t.Fatalf("row after_withholding = %q, want %q", row["after_withholding"], afterWithholding)
	}
	if row["net"] != net {
		t.Fatalf("row net = %q, want %q", row["net"], net)
	}
}
