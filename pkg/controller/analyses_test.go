package controller

import (
	"encoding/json"
	"net/http"
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

func TestFilterDepotsByRequestedIDsKeepsAllWithoutFilter(t *testing.T) {
	depots := []db.Depot{
		{ID: 1, BaseCurrency: "EUR"},
		{ID: 2, BaseCurrency: "EUR"},
	}

	filtered, ok, statusCode, message := filterDepotsByRequestedIDs(nil, depots)
	if !ok {
		t.Fatalf("filterDepotsByRequestedIDs() ok = false, want true: %d %s", statusCode, message)
	}
	if len(filtered) != 2 || filtered[0].ID != 1 || filtered[1].ID != 2 {
		t.Fatalf("filterDepotsByRequestedIDs() depots = %+v, want IDs [1 2]", filtered)
	}
}

func TestFilterDepotsByRequestedIDsFiltersSingleDepot(t *testing.T) {
	filtered, ok, statusCode, message := filterDepotsByRequestedIDs([]string{"2"}, []db.Depot{
		{ID: 1, BaseCurrency: "EUR"},
		{ID: 2, BaseCurrency: "USD"},
	})
	if !ok {
		t.Fatalf("filterDepotsByRequestedIDs() ok = false, want true: %d %s", statusCode, message)
	}
	if len(filtered) != 1 || filtered[0].ID != 2 {
		t.Fatalf("filterDepotsByRequestedIDs() depots = %+v, want ID [2]", filtered)
	}

	currency, unique, currencies := uniqueDepotBaseCurrency(filtered)
	if !unique || currency != "USD" || len(currencies) != 1 || currencies[0] != "USD" {
		t.Fatalf("uniqueDepotBaseCurrency() = %q %v %v, want USD true [USD]", currency, unique, currencies)
	}
}

func TestFilterDepotsByRequestedIDsFiltersMultipleDepots(t *testing.T) {
	filtered, ok, statusCode, message := filterDepotsByRequestedIDs([]string{"3", "1"}, []db.Depot{
		{ID: 1, BaseCurrency: "EUR"},
		{ID: 2, BaseCurrency: "EUR"},
		{ID: 3, BaseCurrency: "EUR"},
	})
	if !ok {
		t.Fatalf("filterDepotsByRequestedIDs() ok = false, want true: %d %s", statusCode, message)
	}
	if len(filtered) != 2 || filtered[0].ID != 1 || filtered[1].ID != 3 {
		t.Fatalf("filterDepotsByRequestedIDs() depots = %+v, want IDs [1 3]", filtered)
	}
}

func TestFilterDepotsByRequestedIDsRejectsInvalidID(t *testing.T) {
	_, ok, statusCode, message := filterDepotsByRequestedIDs([]string{"abc"}, []db.Depot{{ID: 1}})
	if ok {
		t.Fatalf("filterDepotsByRequestedIDs() ok = true, want false")
	}
	if statusCode != http.StatusBadRequest || message != "INVALID_DEPOT_ID" {
		t.Fatalf("filterDepotsByRequestedIDs() = %d %q, want %d INVALID_DEPOT_ID", statusCode, message, http.StatusBadRequest)
	}
}

func TestFilterDepotsByRequestedIDsRejectsInaccessibleDepot(t *testing.T) {
	_, ok, statusCode, message := filterDepotsByRequestedIDs([]string{"2"}, []db.Depot{{ID: 1}})
	if ok {
		t.Fatalf("filterDepotsByRequestedIDs() ok = true, want false")
	}
	if statusCode != http.StatusForbidden || message != "FORBIDDEN" {
		t.Fatalf("filterDepotsByRequestedIDs() = %d %q, want %d FORBIDDEN", statusCode, message, http.StatusForbidden)
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

func TestBuildDividendsByYearData(t *testing.T) {
	data, ok := buildDividendsByYearData([]db.DividendsByYearSourceRow{
		{Year: "2022", Gross: "100.00", AfterWithholding: "80.00", Net: "75.00"},
		{Year: "2021", Gross: "50.00", AfterWithholding: "40.00", Net: "35.00"},
		{Year: "2022", Gross: "25.50", AfterWithholding: "20.25", Net: "19.25"},
	}, 2, "EUR", "en-US")
	if !ok {
		t.Fatalf("buildDividendsByYearData() ok = false, want true")
	}
	if data.Currency != "EUR" {
		t.Fatalf("buildDividendsByYearData() currency = %q, want EUR", data.Currency)
	}
	if len(data.Rows) != 2 {
		t.Fatalf("buildDividendsByYearData() len = %d, want 2", len(data.Rows))
	}

	assertYearDataRow(t, data.Rows[0], "2021", "50.00", "40.00", "35.00")
	assertYearDataRow(t, data.Rows[1], "2022", "125.50", "100.25", "94.25")
}

func TestBuildDividendsByYearDataRejectsInvalidDecimal(t *testing.T) {
	_, ok := buildDividendsByYearData([]db.DividendsByYearSourceRow{
		{Year: "2022", Gross: "bad", AfterWithholding: "80.00", Net: "75.00"},
	}, 2, "EUR", "en-US")
	if ok {
		t.Fatalf("buildDividendsByYearData() ok = true, want false")
	}
}

func TestBuildDividendsByYearDataFormatsLocale(t *testing.T) {
	data, ok := buildDividendsByYearData([]db.DividendsByYearSourceRow{
		{Year: "2022", Gross: "100.00", AfterWithholding: "80.00", Net: "75.50"},
	}, 2, "EUR", "de-DE")
	if !ok {
		t.Fatalf("buildDividendsByYearData() ok = false, want true")
	}
	if len(data.Rows) != 1 {
		t.Fatalf("buildDividendsByYearData() len = %d, want 1", len(data.Rows))
	}

	assertYearDataRow(t, data.Rows[0], "2022", "100,00", "80,00", "75,50")
}

func assertYearDataRow(t *testing.T, row YearDataRow, year, gross, afterWithholding, net string) {
	t.Helper()
	if row.Year != year {
		t.Fatalf("row Year = %q, want %q", row.Year, year)
	}
	if row.Gross != gross {
		t.Fatalf("row Gross = %q, want %q", row.Gross, gross)
	}
	if row.AfterWithholding != afterWithholding {
		t.Fatalf("row AfterWithholding = %q, want %q", row.AfterWithholding, afterWithholding)
	}
	if row.Net != net {
		t.Fatalf("row Net = %q, want %q", row.Net, net)
	}
}

func TestBuildDividendsByYearChartData(t *testing.T) {
	data, ok := buildDividendsByYearChartData([]db.DividendsByYearSourceRow{
		{Year: "2022", Gross: "100.00", AfterWithholding: "80.00", Net: "75.00"},
		{Year: "2021", Gross: "50.00", AfterWithholding: "40.00", Net: "35.00"},
		{Year: "2022", Gross: "25.50", AfterWithholding: "20.25", Net: "19.25"},
	}, 2, "EUR")
	if !ok {
		t.Fatalf("buildDividendsByYearChartData() ok = false, want true")
	}

	assertStringSlice(t, data.Categories, []string{"2021", "2022"})
	if len(data.Series) != 3 {
		t.Fatalf("buildDividendsByYearChartData() series len = %d, want 3", len(data.Series))
	}

	assertYearChartSeries(t, data.Series[0], "gross", "EUR", []YearChartNumber{"50.00", "125.50"})
	assertYearChartSeries(t, data.Series[1], "after_withholding", "EUR", []YearChartNumber{"40.00", "100.25"})
	assertYearChartSeries(t, data.Series[2], "net", "EUR", []YearChartNumber{"35.00", "94.25"})
}

func TestBuildDividendsByYearChartDataRejectsInvalidDecimal(t *testing.T) {
	_, ok := buildDividendsByYearChartData([]db.DividendsByYearSourceRow{
		{Year: "2022", Gross: "bad", AfterWithholding: "80.00", Net: "75.00"},
	}, 2, "EUR")
	if ok {
		t.Fatalf("buildDividendsByYearChartData() ok = true, want false")
	}
}

func TestBuildDividendsByYearMonthChartData(t *testing.T) {
	data, ok := buildDividendsByYearMonthChartData([]db.DividendsByYearMonthSourceRow{
		{Year: "2022", Month: "01", Gross: "100.00", AfterWithholding: "80.00", Net: "75.00"},
		{Year: "2021", Month: "12", Gross: "50.00", AfterWithholding: "40.00", Net: "35.00"},
		{Year: "2022", Month: "01", Gross: "25.50", AfterWithholding: "20.25", Net: "19.25"},
		{Year: "2022", Month: "03", Gross: "10.00", AfterWithholding: "9.00", Net: "8.00"},
	}, 2, "EUR")
	if !ok {
		t.Fatalf("buildDividendsByYearMonthChartData() ok = false, want true")
	}

	if len(data.Rows) != 24 {
		t.Fatalf("buildDividendsByYearMonthChartData() rows len = %d, want 24", len(data.Rows))
	}

	assertYearMonthChartRow(t, data.Rows[0], "2021", "01", "0.00", "0.00", "0.00", "EUR")
	assertYearMonthChartRow(t, data.Rows[11], "2021", "12", "50.00", "40.00", "35.00", "EUR")
	assertYearMonthChartRow(t, data.Rows[12], "2022", "01", "125.50", "100.25", "94.25", "EUR")
	assertYearMonthChartRow(t, data.Rows[14], "2022", "03", "10.00", "9.00", "8.00", "EUR")
	assertYearMonthChartRow(t, data.Rows[23], "2022", "12", "0.00", "0.00", "0.00", "EUR")
}

func TestBuildDividendsByYearMonthChartDataRejectsInvalidDecimal(t *testing.T) {
	_, ok := buildDividendsByYearMonthChartData([]db.DividendsByYearMonthSourceRow{
		{Year: "2022", Month: "01", Gross: "bad", AfterWithholding: "80.00", Net: "75.00"},
	}, 2, "EUR")
	if ok {
		t.Fatalf("buildDividendsByYearMonthChartData() ok = true, want false")
	}
}

func TestYearChartResponseMarshalsValuesAsJSONNumbers(t *testing.T) {
	response := YearChartResponse{
		Success: true,
		Message: "ANALYSIS_OK",
		Data: YearChartResponseData{
			Categories: []string{"2021"},
			Series: []YearChartSeries{
				{Key: "gross", Currency: "EUR", Values: []YearChartNumber{"1234.50"}},
			},
		},
	}

	got, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	want := `{"success":true,"message":"ANALYSIS_OK","data":{"categories":["2021"],"series":[{"key":"gross","currency":"EUR","values":[1234.50]}]}}`
	if string(got) != want {
		t.Fatalf("json.Marshal() = %s, want %s", got, want)
	}
}

func TestYearMonthChartResponseMarshalsValuesAsJSONNumbers(t *testing.T) {
	response := YearMonthChartResponse{
		Success: true,
		Message: "ANALYSIS_OK",
		Data: YearMonthChartResponseData{
			Rows: []YearMonthChartRow{
				{
					Year:             "2021",
					Month:            "01",
					Gross:            "1234.50",
					AfterWithholding: "1200.00",
					Net:              "1100.75",
					Currency:         "EUR",
				},
			},
		},
	}

	got, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	want := `{"success":true,"message":"ANALYSIS_OK","data":{"rows":[{"year":"2021","month":"01","gross":1234.50,"after_withholding":1200.00,"net":1100.75,"currency":"EUR"}]}}`
	if string(got) != want {
		t.Fatalf("json.Marshal() = %s, want %s", got, want)
	}
}

func TestBuildDividendsBySecurityYearData(t *testing.T) {
	data, ok := buildDividendsBySecurityYearData([]db.DividendsBySecurityYearDataSourceRow{
		{SecurityID: 2, SecurityName: "Example Beta AG", SecurityISIN: "DE000BETA01", Year: "2023", PayDate: "2023-03-01", Quantity: "10", Gross: "5.00", AfterWithholding: "4.00", Net: "3.50"},
		{SecurityID: 1, SecurityName: "Example Alpha AG", SecurityISIN: "DE000ALPHA1", Year: "2024", PayDate: "2024-01-10", Quantity: "75", Gross: "10.00", AfterWithholding: "8.00", Net: "7.50"},
		{SecurityID: 1, SecurityName: "Example Alpha AG", SecurityISIN: "DE000ALPHA1", Year: "2024", PayDate: "2024-04-10", Quantity: "75", Gross: "20.00", AfterWithholding: "17.00", Net: "16.50"},
		{SecurityID: 1, SecurityName: "Example Alpha AG", SecurityISIN: "DE000ALPHA1", Year: "2024", PayDate: "2024-07-10", Quantity: "90", Gross: "15.00", AfterWithholding: "13.00", Net: "12.50"},
	}, 2, "EUR", "en-US")
	if !ok {
		t.Fatalf("buildDividendsBySecurityYearData() ok = false, want true")
	}
	if data.Currency != "EUR" {
		t.Fatalf("buildDividendsBySecurityYearData() currency = %q, want EUR", data.Currency)
	}
	if len(data.Securities) != 2 {
		t.Fatalf("buildDividendsBySecurityYearData() securities len = %d, want 2", len(data.Securities))
	}
	if data.Securities[0].SecurityID != 1 || data.Securities[0].SecurityName != "Example Alpha AG" {
		t.Fatalf("first security = %+v, want Example Alpha AG", data.Securities[0])
	}
	if len(data.Securities[0].Rows) != 3 {
		t.Fatalf("first security rows len = %d, want 3", len(data.Securities[0].Rows))
	}
	assertSecurityYearDataRow(t, data.Securities[0].Rows[0], "2024", "75", "30.00", "25.00", "24.00", "detail")
	assertSecurityYearDataRow(t, data.Securities[0].Rows[1], "2024", "90", "15.00", "13.00", "12.50", "detail")
	assertSecurityYearDataRow(t, data.Securities[0].Rows[2], "", "", "45.00", "38.00", "36.50", "summary")
	assertSecurityYearDataRow(t, data.Securities[1].Rows[0], "2023", "10", "5.00", "4.00", "3.50", "detail")
	assertSecurityYearDataRow(t, data.Securities[1].Rows[1], "", "", "5.00", "4.00", "3.50", "summary")
}

func TestBuildDividendsByYearMonthSecurityData(t *testing.T) {
	data, ok := buildDividendsByYearMonthSecurityData([]db.DividendsByYearMonthSecuritySourceRow{
		{Year: "2024", Month: "01", SecurityID: 2, SecurityName: "Example Beta AG", SecurityISIN: "DE000BETA01", Gross: "5.00", AfterWithholding: "4.00", Net: "3.50"},
		{Year: "2024", Month: "01", SecurityID: 1, SecurityName: "Example Alpha AG", SecurityISIN: "DE000ALPHA1", Gross: "10.00", AfterWithholding: "8.00", Net: "7.50"},
		{Year: "2024", Month: "01", SecurityID: 1, SecurityName: "Example Alpha AG", SecurityISIN: "DE000ALPHA1", Gross: "20.00", AfterWithholding: "17.00", Net: "16.50"},
		{Year: "2024", Month: "02", SecurityID: 1, SecurityName: "Example Alpha AG", SecurityISIN: "DE000ALPHA1", Gross: "7.00", AfterWithholding: "6.00", Net: "5.50"},
	}, 2, "EUR", "en-US")
	if !ok {
		t.Fatalf("buildDividendsByYearMonthSecurityData() ok = false, want true")
	}
	if data.Currency != "EUR" {
		t.Fatalf("buildDividendsByYearMonthSecurityData() currency = %q, want EUR", data.Currency)
	}
	if len(data.Periods) != 2 {
		t.Fatalf("buildDividendsByYearMonthSecurityData() periods len = %d, want 2", len(data.Periods))
	}
	if data.Periods[0].Year != "2024" || data.Periods[0].Month != "01" {
		t.Fatalf("first period = %+v, want 2024/01", data.Periods[0])
	}
	if len(data.Periods[0].Rows) != 3 {
		t.Fatalf("first period rows len = %d, want 3", len(data.Periods[0].Rows))
	}
	assertYearMonthSecurityDataRow(t, data.Periods[0].Rows[0], 1, "Example Alpha AG", "DE000ALPHA1", "30.00", "25.00", "24.00", "detail")
	assertYearMonthSecurityDataRow(t, data.Periods[0].Rows[1], 2, "Example Beta AG", "DE000BETA01", "5.00", "4.00", "3.50", "detail")
	assertYearMonthSecurityDataRow(t, data.Periods[0].Rows[2], 0, "Monat Ergebnis", "", "35.00", "29.00", "27.50", "summary")
	assertYearMonthSecurityDataRow(t, data.Periods[1].Rows[0], 1, "Example Alpha AG", "DE000ALPHA1", "7.00", "6.00", "5.50", "detail")
	assertYearMonthSecurityDataRow(t, data.Periods[1].Rows[1], 0, "Monat Ergebnis", "", "7.00", "6.00", "5.50", "summary")
}

func TestParseAnalysisYearMonthPeriods(t *testing.T) {
	periods, ok := parseAnalysisYearMonthPeriods([]yearMonthSecurityPeriod{
		{Year: "2024", Month: "01"},
		{Year: "2024", Month: "01"},
		{Year: "2025", Month: "12"},
	})
	if !ok {
		t.Fatalf("parseAnalysisYearMonthPeriods() ok = false, want true")
	}
	if len(periods) != 2 {
		t.Fatalf("parseAnalysisYearMonthPeriods() len = %d, want 2", len(periods))
	}
	if periods[0].Year != "2024" || periods[0].Month != "01" || periods[1].Year != "2025" || periods[1].Month != "12" {
		t.Fatalf("parseAnalysisYearMonthPeriods() = %+v, want 2024/01 and 2025/12", periods)
	}

	for _, invalid := range [][]yearMonthSecurityPeriod{
		{{Year: "24", Month: "01"}},
		{{Year: "2024", Month: "1"}},
		{{Year: "2024", Month: "13"}},
	} {
		if _, ok := parseAnalysisYearMonthPeriods(invalid); ok {
			t.Fatalf("parseAnalysisYearMonthPeriods(%+v) ok = true, want false", invalid)
		}
	}
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

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func assertYearChartSeries(t *testing.T, got YearChartSeries, key, currency string, values []YearChartNumber) {
	t.Helper()
	if got.Key != key {
		t.Fatalf("series key = %q, want %q", got.Key, key)
	}
	if got.Currency != currency {
		t.Fatalf("series currency = %q, want %q", got.Currency, currency)
	}
	if len(got.Values) != len(values) {
		t.Fatalf("series values len = %d, want %d", len(got.Values), len(values))
	}
	for i := range got.Values {
		if got.Values[i] != values[i] {
			t.Fatalf("series values[%d] = %q, want %q", i, got.Values[i], values[i])
		}
	}
}

func assertYearMonthChartRow(t *testing.T, got YearMonthChartRow, year, month, gross, afterWithholding, net, currency string) {
	t.Helper()
	if got.Year != year {
		t.Fatalf("row year = %q, want %q", got.Year, year)
	}
	if got.Month != month {
		t.Fatalf("row month = %q, want %q", got.Month, month)
	}
	if got.Gross != YearChartNumber(gross) {
		t.Fatalf("row gross = %q, want %q", got.Gross, gross)
	}
	if got.AfterWithholding != YearChartNumber(afterWithholding) {
		t.Fatalf("row after_withholding = %q, want %q", got.AfterWithholding, afterWithholding)
	}
	if got.Net != YearChartNumber(net) {
		t.Fatalf("row net = %q, want %q", got.Net, net)
	}
	if got.Currency != currency {
		t.Fatalf("row currency = %q, want %q", got.Currency, currency)
	}
}

func assertSecurityYearDataRow(t *testing.T, row SecurityYearDataRow, year, quantity, gross, afterWithholding, net, rowType string) {
	t.Helper()
	if row.Year != year {
		t.Fatalf("row Year = %q, want %q", row.Year, year)
	}
	if row.Quantity != quantity {
		t.Fatalf("row Quantity = %q, want %q", row.Quantity, quantity)
	}
	if row.Gross != gross {
		t.Fatalf("row Gross = %q, want %q", row.Gross, gross)
	}
	if row.AfterWithholding != afterWithholding {
		t.Fatalf("row AfterWithholding = %q, want %q", row.AfterWithholding, afterWithholding)
	}
	if row.Net != net {
		t.Fatalf("row Net = %q, want %q", row.Net, net)
	}
	if row.Type != rowType {
		t.Fatalf("row Type = %q, want %q", row.Type, rowType)
	}
}

func assertYearMonthSecurityDataRow(t *testing.T, row YearMonthSecurityDataRow, securityID int64, securityName, securityISIN, gross, afterWithholding, net, rowType string) {
	t.Helper()
	if row.SecurityID != securityID {
		t.Fatalf("row SecurityID = %d, want %d", row.SecurityID, securityID)
	}
	if row.SecurityName != securityName {
		t.Fatalf("row SecurityName = %q, want %q", row.SecurityName, securityName)
	}
	if row.SecurityISIN != securityISIN {
		t.Fatalf("row SecurityISIN = %q, want %q", row.SecurityISIN, securityISIN)
	}
	if row.Gross != gross {
		t.Fatalf("row Gross = %q, want %q", row.Gross, gross)
	}
	if row.AfterWithholding != afterWithholding {
		t.Fatalf("row AfterWithholding = %q, want %q", row.AfterWithholding, afterWithholding)
	}
	if row.Net != net {
		t.Fatalf("row Net = %q, want %q", row.Net, net)
	}
	if row.Type != rowType {
		t.Fatalf("row Type = %q, want %q", row.Type, rowType)
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
