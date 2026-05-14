package controller

import (
	"path/filepath"
	"testing"

	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/validate"
)

const testCurrencyGroupID = int64(1)

func validDividendEntryPayloadForValidation() db.DividendEntry {
	return db.DividendEntry{
		DepotID:                            1,
		SecurityID:                         1,
		PayDate:                            "2026-01-15",
		ExDate:                             "2026-01-10",
		SecurityName:                       "Example AG",
		SecurityISIN:                       "DE0000000001",
		Quantity:                           "1.234,50",
		DividendPerUnitAmount:              "2,35",
		DividendPerUnitCurrency:            "EUR",
		FXRate:                             "   ",
		GrossAmount:                        "2.567,05",
		GrossCurrency:                      "EUR",
		PayoutAmount:                       "2,000.05",
		PayoutCurrency:                     "EUR",
		WithholdingTaxPercent:              "26,375",
		WithholdingTaxAmount:               "",
		WithholdingTaxAmountCredit:         "0.000000123456",
		WithholdingTaxAmountCreditCurrency: "EUR",
		ForeignFeesAmount:                  "1,23",
		ForeignFeesCurrency:                "EUR",
	}
}

func TestNormalizeDividendEntryPayloadNormalizesDecimalFields(t *testing.T) {
	entry, errors := normalizeDividendEntryPayload(validDividendEntryPayloadForValidation())
	if len(errors) > 0 {
		t.Fatalf("normalizeDividendEntryPayload() errors = %v", errors)
	}

	assertString(t, "Quantity", entry.Quantity, "1234.50")
	assertString(t, "DividendPerUnitAmount", entry.DividendPerUnitAmount, "2.35")
	assertString(t, "FXRate", entry.FXRate, "")
	assertString(t, "GrossAmount", entry.GrossAmount, "2567.05")
	assertString(t, "PayoutAmount", entry.PayoutAmount, "2000.05")
	assertString(t, "WithholdingTaxPercent", entry.WithholdingTaxPercent, "26.375")
	assertString(t, "WithholdingTaxAmount", entry.WithholdingTaxAmount, "")
	assertString(t, "WithholdingTaxAmountCredit", entry.WithholdingTaxAmountCredit, "0.000000123456")
	assertString(t, "ForeignFeesAmount", entry.ForeignFeesAmount, "1.23")
}

func TestNormalizeDividendEntryPayloadCollectsFieldErrors(t *testing.T) {
	entry := validDividendEntryPayloadForValidation()
	entry.DepotID = 0
	entry.PayDate = " "
	entry.Quantity = ""
	entry.GrossAmount = "2.35,76"

	_, errors := normalizeDividendEntryPayload(entry)

	assertFieldError(t, errors, "DepotID", "INVALID_DEPOT_ID")
	assertFieldError(t, errors, "PayDate", "MISSING_PAY_DATE")
	assertFieldError(t, errors, "Quantity", validate.ErrDecimalEmpty)
	assertFieldError(t, errors, "GrossAmount", validate.ErrDecimalInvalidGrouping)
}

func TestPrepareInlandTaxFieldsKeepsExplicitAmount(t *testing.T) {
	entry := validDividendEntryPayloadForValidation()
	entry.InlandTaxAmount = "5.00"
	entry.InlandTaxCurrency = "EUR"
	entry.InlandTaxDetails = []db.InlandTaxDetail{
		{Amount: "1.00", Currency: "EUR"},
		{Amount: "2.00", Currency: "EUR"},
	}
	errors := fieldErrors{}

	prepareInlandTaxFields(&entry, errors)

	if len(errors) > 0 {
		t.Fatalf("prepareInlandTaxFields() field errors = %v", errors)
	}
	assertString(t, "InlandTaxAmount", entry.InlandTaxAmount, "5.00")
	assertString(t, "InlandTaxCurrency", entry.InlandTaxCurrency, "EUR")
}

func TestPrepareInlandTaxFieldsSumsDetailsWhenAmountEmpty(t *testing.T) {
	entry := validDividendEntryPayloadForValidation()
	entry.InlandTaxAmount = ""
	entry.InlandTaxCurrency = ""
	entry.PayoutCurrency = "EUR"
	entry.InlandTaxDetails = []db.InlandTaxDetail{
		{Amount: "1.20", Currency: "EUR"},
		{Amount: "", Currency: "EUR"},
		{Amount: "0.30", Currency: "EUR"},
	}
	errors := fieldErrors{}

	prepareInlandTaxFields(&entry, errors)

	if len(errors) > 0 {
		t.Fatalf("prepareInlandTaxFields() field errors = %v", errors)
	}
	assertString(t, "InlandTaxAmount", entry.InlandTaxAmount, "1.5")
	assertString(t, "InlandTaxCurrency", entry.InlandTaxCurrency, "EUR")
}

func TestPrepareInlandTaxFieldsFallsBackToDifference(t *testing.T) {
	entry := validDividendEntryPayloadForValidation()
	entry.GrossAmount = "100.00"
	entry.WithholdingTaxAmount = "15.00"
	entry.PayoutAmount = "60.00"
	entry.PayoutCurrency = "EUR"
	entry.InlandTaxAmount = ""
	entry.InlandTaxCurrency = ""
	entry.InlandTaxDetails = nil
	errors := fieldErrors{}

	prepareInlandTaxFields(&entry, errors)

	if len(errors) > 0 {
		t.Fatalf("prepareInlandTaxFields() field errors = %v", errors)
	}
	assertString(t, "InlandTaxAmount", entry.InlandTaxAmount, "25")
	assertString(t, "InlandTaxCurrency", entry.InlandTaxCurrency, "EUR")
}

func TestFormatDividendEntryForLocalePadsInlandTaxCurrencyPrecision(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	security := db.Security{
		GroupID: testCurrencyGroupID,
		Name:    "Format Test Security",
		ISIN:    "DE000FORMAT1",
		Status:  db.SecurityStatusActive,
	}
	if err := database.CreateSecurity(&security); err != nil {
		t.Fatalf("CreateSecurity() error = %v", err)
	}

	entry := db.DividendEntry{
		SecurityID:          security.ID,
		Quantity:            "1.5",
		InlandTaxAmount:     "478.7",
		InlandTaxCurrency:   "EUR",
		ForeignFeesAmount:   "1.2",
		ForeignFeesCurrency: "EUR",
		InlandTaxDetails: []db.InlandTaxDetail{
			{Amount: "12.3", Currency: "EUR"},
		},
	}

	formatted, err := formatDividendEntryForLocale(entry, "de-DE", newDividendEntryCurrencyFormatter(database))
	if err != nil {
		t.Fatalf("formatDividendEntryForLocale() error = %v", err)
	}

	assertString(t, "InlandTaxAmount", formatted.InlandTaxAmount, "478,70")
	assertString(t, "InlandTaxDetails[0].Amount", formatted.InlandTaxDetails[0].Amount, "12,30")
	assertString(t, "Quantity", formatted.Quantity, "1,5")
	assertString(t, "ForeignFeesAmount", formatted.ForeignFeesAmount, "1,2")
}

func TestValidateDividendEntryCurrencyPairsAcceptsKnownCurrencies(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	entry := validDividendEntryPayloadForValidation()
	errors := fieldErrors{}

	if err := validateDividendEntryCurrencyPairs(database, testCurrencyGroupID, &entry, errors); err != nil {
		t.Fatalf("validateDividendEntryCurrencyPairs() error = %v", err)
	}
	if len(errors) > 0 {
		t.Fatalf("validateDividendEntryCurrencyPairs() field errors = %v", errors)
	}
}

func TestValidateDividendEntryCurrencyPairsCollectsFieldErrors(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	entry := validDividendEntryPayloadForValidation()
	entry.GrossCurrency = "eur"
	entry.PayoutCurrency = "ABC"
	entry.WithholdingTaxAmount = ""
	entry.WithholdingTaxCurrency = "EUR"
	entry.ForeignFeesAmount = "1.23"
	entry.ForeignFeesCurrency = ""
	errors := fieldErrors{}

	if err := validateDividendEntryCurrencyPairs(database, testCurrencyGroupID, &entry, errors); err != nil {
		t.Fatalf("validateDividendEntryCurrencyPairs() error = %v", err)
	}

	assertFieldError(t, errors, "GrossCurrency", errCurrencyInvalidFormat)
	assertFieldError(t, errors, "PayoutCurrency", errCurrencyUnknown)
	assertFieldError(t, errors, "WithholdingTaxAmount", validate.ErrDecimalEmpty)
	assertFieldError(t, errors, "ForeignFeesCurrency", errCurrencyRequired)
}

func TestValidateDividendEntryFXRateDefaultsEmptyLabel(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	tests := []struct {
		name string
		rate string
	}{
		{name: "empty rate", rate: ""},
		{name: "one", rate: "1"},
		{name: "zero", rate: "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := validDividendEntryPayloadForValidation()
			entry.FXRateLabel = " "
			entry.FXRate = tt.rate
			errors := fieldErrors{}

			if err := validateDividendEntryFXRate(database, testCurrencyGroupID, &entry, errors); err != nil {
				t.Fatalf("validateDividendEntryFXRate() error = %v", err)
			}
			if len(errors) > 0 {
				t.Fatalf("validateDividendEntryFXRate() field errors = %v", errors)
			}
			assertString(t, "FXRateLabel", entry.FXRateLabel, "")
			assertString(t, "FXRate", entry.FXRate, "1")
		})
	}
}

func TestValidateDividendEntryFXRateAcceptsValidLabel(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	entry := validDividendEntryPayloadForValidation()
	entry.FXRateLabel = "EUR/USD"
	entry.FXRate = "1,1234"
	errors := fieldErrors{}

	if err := validateDividendEntryFXRate(database, testCurrencyGroupID, &entry, errors); err != nil {
		t.Fatalf("validateDividendEntryFXRate() error = %v", err)
	}
	if len(errors) > 0 {
		t.Fatalf("validateDividendEntryFXRate() field errors = %v", errors)
	}
	assertString(t, "FXRateLabel", entry.FXRateLabel, "EUR/USD")
	assertString(t, "FXRate", entry.FXRate, "1.1234")
}

func TestValidateDividendEntryFXRateCollectsFieldErrors(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	tests := []struct {
		name  string
		label string
		rate  string
		field string
		want  string
	}{
		{name: "label required", label: "", rate: "1,5", field: "FXRateLabel", want: errFXRateLabelRequired},
		{name: "rate required", label: "EUR/USD", rate: "", field: "FXRate", want: validate.ErrDecimalEmpty},
		{name: "invalid label format", label: "eur/USD", rate: "1.1", field: "FXRateLabel", want: errFXRateLabelInvalidFormat},
		{name: "unknown currency", label: "EUR/ABC", rate: "1.1", field: "FXRateLabel", want: errFXRateLabelUnknownCurrency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := validDividendEntryPayloadForValidation()
			entry.FXRateLabel = tt.label
			entry.FXRate = tt.rate
			errors := fieldErrors{}

			if err := validateDividendEntryFXRate(database, testCurrencyGroupID, &entry, errors); err != nil {
				t.Fatalf("validateDividendEntryFXRate() error = %v", err)
			}
			assertFieldError(t, errors, tt.field, tt.want)
		})
	}
}

func TestPrepareCalculatedDividendFieldsUsesGrossAmountForBaseCurrency(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	depotID := createCalculationTestDepot(t, database, "EUR")
	entry := validDividendEntryPayloadForValidation()
	entry.DepotID = depotID
	entry.GrossAmount = "100.50"
	entry.GrossCurrency = "EUR"
	entry.WithholdingTaxAmount = ""
	entry.WithholdingTaxCurrency = ""
	entry.FXRateLabel = ""
	entry.FXRate = "1"
	errors := fieldErrors{}

	if err := prepareCalculatedDividendFields(database, testCurrencyGroupID, &entry, errors); err != nil {
		t.Fatalf("prepareCalculatedDividendFields() error = %v", err)
	}
	if len(errors) > 0 {
		t.Fatalf("prepareCalculatedDividendFields() field errors = %v", errors)
	}

	assertString(t, "CalcGrossAmountBase", entry.CalcGrossAmountBase, "100.50")
	assertString(t, "CalcAfterWithholdingAmountBase", entry.CalcAfterWithholdingAmountBase, "100.50")
}

func TestPrepareCalculatedDividendFieldsConvertsAndSubtractsWithholding(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	depotID := createCalculationTestDepot(t, database, "EUR")
	entry := validDividendEntryPayloadForValidation()
	entry.DepotID = depotID
	entry.GrossAmount = "110"
	entry.GrossCurrency = "USD"
	entry.WithholdingTaxAmount = "11"
	entry.WithholdingTaxCurrency = "USD"
	entry.FXRateLabel = "EUR/USD"
	entry.FXRate = "1.1"
	errors := fieldErrors{}

	if err := prepareCalculatedDividendFields(database, testCurrencyGroupID, &entry, errors); err != nil {
		t.Fatalf("prepareCalculatedDividendFields() error = %v", err)
	}
	if len(errors) > 0 {
		t.Fatalf("prepareCalculatedDividendFields() field errors = %v", errors)
	}

	assertString(t, "CalcGrossAmountBase", entry.CalcGrossAmountBase, "100.00")
	assertString(t, "CalcAfterWithholdingAmountBase", entry.CalcAfterWithholdingAmountBase, "90.00")
}

func TestPrepareCalculatedDividendFieldsConvertsForwardPair(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	depotID := createCalculationTestDepot(t, database, "USD")
	entry := validDividendEntryPayloadForValidation()
	entry.DepotID = depotID
	entry.GrossAmount = "100"
	entry.GrossCurrency = "EUR"
	entry.WithholdingTaxAmount = ""
	entry.WithholdingTaxCurrency = ""
	entry.FXRateLabel = "EUR/USD"
	entry.FXRate = "1.1"
	errors := fieldErrors{}

	if err := prepareCalculatedDividendFields(database, testCurrencyGroupID, &entry, errors); err != nil {
		t.Fatalf("prepareCalculatedDividendFields() error = %v", err)
	}
	if len(errors) > 0 {
		t.Fatalf("prepareCalculatedDividendFields() field errors = %v", errors)
	}

	assertString(t, "CalcGrossAmountBase", entry.CalcGrossAmountBase, "110.00")
	assertString(t, "CalcAfterWithholdingAmountBase", entry.CalcAfterWithholdingAmountBase, "110.00")
}

func TestPrepareCalculatedDividendFieldsRoundsToBaseCurrencyDecimalPlaces(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	depotID := createCalculationTestDepot(t, database, "EUR")
	entry := validDividendEntryPayloadForValidation()
	entry.DepotID = depotID
	entry.GrossAmount = "14.467"
	entry.GrossCurrency = "USD"
	entry.WithholdingTaxAmount = ""
	entry.WithholdingTaxCurrency = ""
	entry.FXRateLabel = "USD/EUR"
	entry.FXRate = "1.18700"
	errors := fieldErrors{}

	if err := prepareCalculatedDividendFields(database, testCurrencyGroupID, &entry, errors); err != nil {
		t.Fatalf("prepareCalculatedDividendFields() error = %v", err)
	}
	if len(errors) > 0 {
		t.Fatalf("prepareCalculatedDividendFields() field errors = %v", errors)
	}

	assertString(t, "CalcGrossAmountBase", entry.CalcGrossAmountBase, "17.17")
	assertString(t, "CalcAfterWithholdingAmountBase", entry.CalcAfterWithholdingAmountBase, "17.17")
}

func TestPrepareCalculatedDividendFieldsReportsPairMismatch(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	createCalculationTestCurrency(t, database, "GBP")
	depotID := createCalculationTestDepot(t, database, "EUR")
	entry := validDividendEntryPayloadForValidation()
	entry.DepotID = depotID
	entry.GrossAmount = "100"
	entry.GrossCurrency = "GBP"
	entry.WithholdingTaxAmount = ""
	entry.WithholdingTaxCurrency = ""
	entry.FXRateLabel = "EUR/USD"
	entry.FXRate = "1.1"
	errors := fieldErrors{}

	if err := prepareCalculatedDividendFields(database, testCurrencyGroupID, &entry, errors); err != nil {
		t.Fatalf("prepareCalculatedDividendFields() error = %v", err)
	}

	assertFieldError(t, errors, "FXRateLabel", errFXRatePairMismatch)
}

func TestCalculateWithholdingTaxRefundPrefersDepotDefault(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	depotID := createCalculationTestDepot(t, database, "EUR")
	createWithholdingTaxDefaultForCalculation(t, database, 0, "US", "30", "15")
	createWithholdingTaxDefaultForCalculation(t, database, depotID, "US", "25", "15")
	entry := db.DividendEntry{
		DepotID:                   depotID,
		GrossAmount:               "100.00",
		GrossCurrency:             "EUR",
		WithholdingTaxCountryCode: "US",
		WithholdingTaxAmount:      "20.00",
		WithholdingTaxCurrency:    "EUR",
	}
	errors := fieldErrors{}

	result, err := calculateWithholdingTaxRefund(database, testCurrencyGroupID, &entry, errors)
	if err != nil {
		t.Fatalf("calculateWithholdingTaxRefund() error = %v", err)
	}
	if len(errors) > 0 {
		t.Fatalf("calculateWithholdingTaxRefund() field errors = %v", errors)
	}

	assertString(t, "Amount", result.Amount, "10.00")
	assertString(t, "Currency", result.Currency, "EUR")
	assertString(t, "RefundPercent", result.RefundPercent, "10")
	assertString(t, "Source", result.Source, "depot")
}

func TestCalculateWithholdingTaxRefundFallsBackToGroupAndCapsAtWithholding(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	depotID := createCalculationTestDepot(t, database, "EUR")
	createWithholdingTaxDefaultForCalculation(t, database, 0, "CH", "35", "15")
	entry := db.DividendEntry{
		DepotID:                   depotID,
		GrossAmount:               "100.00",
		GrossCurrency:             "EUR",
		WithholdingTaxCountryCode: "CH",
		WithholdingTaxAmount:      "12.00",
		WithholdingTaxCurrency:    "EUR",
	}
	errors := fieldErrors{}

	result, err := calculateWithholdingTaxRefund(database, testCurrencyGroupID, &entry, errors)
	if err != nil {
		t.Fatalf("calculateWithholdingTaxRefund() error = %v", err)
	}
	if len(errors) > 0 {
		t.Fatalf("calculateWithholdingTaxRefund() field errors = %v", errors)
	}

	assertString(t, "Amount", result.Amount, "12.00")
	assertString(t, "Currency", result.Currency, "EUR")
	assertString(t, "RefundPercent", result.RefundPercent, "20")
	assertString(t, "Source", result.Source, "group")
	if !result.Capped {
		t.Fatalf("Capped = false, want true")
	}
}

func TestCalculateWithholdingTaxRefundConvertsToDepotCurrency(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	depotID := createCalculationTestDepot(t, database, "EUR")
	createWithholdingTaxDefaultForCalculation(t, database, 0, "US", "30", "15")
	entry := db.DividendEntry{
		DepotID:                   depotID,
		GrossAmount:               "100.00",
		GrossCurrency:             "USD",
		WithholdingTaxCountryCode: "US",
		WithholdingTaxAmount:      "30.00",
		WithholdingTaxCurrency:    "USD",
		FXRateLabel:               "USD/EUR",
		FXRate:                    "0.9",
	}
	errors := fieldErrors{}

	result, err := calculateWithholdingTaxRefund(database, testCurrencyGroupID, &entry, errors)
	if err != nil {
		t.Fatalf("calculateWithholdingTaxRefund() error = %v", err)
	}
	if len(errors) > 0 {
		t.Fatalf("calculateWithholdingTaxRefund() field errors = %v", errors)
	}

	assertString(t, "Amount", result.Amount, "13.50")
	assertString(t, "Currency", result.Currency, "EUR")
}

func TestCalculateWithholdingTaxRefundReportsMissingCreditPercent(t *testing.T) {
	database := newCurrencyValidationTestDB(t)
	depotID := createCalculationTestDepot(t, database, "EUR")
	createWithholdingTaxDefaultForCalculation(t, database, 0, "US", "30", "")
	entry := db.DividendEntry{
		DepotID:                   depotID,
		GrossAmount:               "100.00",
		GrossCurrency:             "EUR",
		WithholdingTaxCountryCode: "US",
		WithholdingTaxAmount:      "30.00",
		WithholdingTaxCurrency:    "EUR",
	}
	errors := fieldErrors{}

	_, err := calculateWithholdingTaxRefund(database, testCurrencyGroupID, &entry, errors)
	if err != nil {
		t.Fatalf("calculateWithholdingTaxRefund() error = %v", err)
	}

	assertFieldError(t, errors, "WithholdingTaxPercentCreditDefault", "WITHHOLDING_TAX_CREDIT_PERCENT_MISSING")
}

func newCurrencyValidationTestDB(t *testing.T) *db.DB {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	if err := database.CopyDefaultCurrenciesToGroup(testCurrencyGroupID); err != nil {
		t.Fatalf("copy default currencies to test group: %v", err)
	}

	return database
}

func createCalculationTestDepot(t *testing.T, database *db.DB, baseCurrency string) int64 {
	t.Helper()

	depot := db.Depot{
		Name:         "Test Depot",
		BaseCurrency: baseCurrency,
		Status:       "active",
	}
	if err := database.CreateDepot(&depot); err != nil {
		t.Fatalf("create depot: %v", err)
	}
	return depot.ID
}

func createCalculationTestCurrency(t *testing.T, database *db.DB, currency string) {
	t.Helper()

	item := db.Currency{
		GroupID:  testCurrencyGroupID,
		Currency: currency,
		Name:     currency,
		Status:   db.CurrencyStatusActive,
	}
	if err := database.CreateCurrency(&item); err != nil {
		t.Fatalf("create currency: %v", err)
	}
}

func createWithholdingTaxDefaultForCalculation(t *testing.T, database *db.DB, depotID int64, countryCode, withholdingPercent, creditPercent string) {
	t.Helper()

	item := db.WithholdingTaxDefault{
		GroupID:                            testCurrencyGroupID,
		DepotID:                            depotID,
		CountryCode:                        countryCode,
		CountryName:                        countryCode,
		WithholdingTaxPercentDefault:       withholdingPercent,
		WithholdingTaxPercentCreditDefault: creditPercent,
	}
	if err := database.CreateWithholdingTaxDefault(&item); err != nil {
		t.Fatalf("CreateWithholdingTaxDefault() error = %v", err)
	}
}

func assertString(t *testing.T, fieldName, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", fieldName, got, want)
	}
}

func assertFieldError(t *testing.T, errors fieldErrors, fieldName, want string) {
	t.Helper()
	got, ok := errors[fieldName]
	if !ok {
		t.Fatalf("missing field error for %s in %v", fieldName, errors)
	}
	if got != want {
		t.Fatalf("field error for %s = %q, want %q", fieldName, got, want)
	}
}
