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
