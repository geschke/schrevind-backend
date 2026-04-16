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

func TestBuildDividendsByYearMonthRows(t *testing.T) {
	rows, ok := buildDividendsByYearMonthRows([]db.DividendsByYearMonthSourceRow{
		{Year: "2022", Month: "01", Gross: "100.00", AfterWithholding: "80.00", Net: "75.00"},
		{Year: "2021", Month: "12", Gross: "50.00", AfterWithholding: "40.00", Net: "35.00"},
		{Year: "2022", Month: "01", Gross: "25.50", AfterWithholding: "20.25", Net: "19.25"},
		{Year: "2022", Month: "03", Gross: "10.00", AfterWithholding: "9.00", Net: "8.00"},
	}, 2, "en-US")
	if !ok {
		t.Fatalf("buildDividendsByYearMonthRows() ok = false, want true")
	}
	if len(rows) != 26 {
		t.Fatalf("buildDividendsByYearMonthRows() len = %d, want 26", len(rows))
	}

	assertAnalysisMonthRow(t, rows[0], "year", "2021", "", "2021", "50.00", "40.00", "35.00")
	assertAnalysisMonthRow(t, rows[1], "month", "2021", "01", "01", "0.00", "0.00", "0.00")
	assertAnalysisMonthRow(t, rows[12], "month", "2021", "12", "12", "50.00", "40.00", "35.00")
	assertAnalysisMonthRow(t, rows[13], "year", "2022", "", "2022", "135.50", "109.25", "102.25")
	assertAnalysisMonthRow(t, rows[14], "month", "2022", "01", "01", "125.50", "100.25", "94.25")
	assertAnalysisMonthRow(t, rows[16], "month", "2022", "03", "03", "10.00", "9.00", "8.00")
}

func TestBuildDividendsByYearMonthRowsRejectsInvalidDecimal(t *testing.T) {
	_, ok := buildDividendsByYearMonthRows([]db.DividendsByYearMonthSourceRow{
		{Year: "2022", Month: "01", Gross: "bad", AfterWithholding: "80.00", Net: "75.00"},
	}, 2, "en-US")
	if ok {
		t.Fatalf("buildDividendsByYearMonthRows() ok = true, want false")
	}
}

func TestBuildDividendsByYearMonthRowsFormatsLocale(t *testing.T) {
	rows, ok := buildDividendsByYearMonthRows([]db.DividendsByYearMonthSourceRow{
		{Year: "2022", Month: "02", Gross: "100.00", AfterWithholding: "80.00", Net: "75.50"},
	}, 2, "de-DE")
	if !ok {
		t.Fatalf("buildDividendsByYearMonthRows() ok = false, want true")
	}
	if len(rows) != 13 {
		t.Fatalf("buildDividendsByYearMonthRows() len = %d, want 13", len(rows))
	}

	assertAnalysisMonthRow(t, rows[0], "year", "2022", "", "2022", "100,00", "80,00", "75,50")
	assertAnalysisMonthRow(t, rows[1], "month", "2022", "01", "01", "0,00", "0,00", "0,00")
	assertAnalysisMonthRow(t, rows[2], "month", "2022", "02", "02", "100,00", "80,00", "75,50")
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

func assertAnalysisMonthRow(t *testing.T, row AnalysisRow, level, year, month, period, gross, afterWithholding, net string) {
	t.Helper()
	if row["level"] != level {
		t.Fatalf("row level = %q, want %q", row["level"], level)
	}
	if row["year"] != year {
		t.Fatalf("row year = %q, want %q", row["year"], year)
	}
	if row["month"] != month {
		t.Fatalf("row month = %q, want %q", row["month"], month)
	}
	if row["period"] != period {
		t.Fatalf("row period = %q, want %q", row["period"], period)
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
