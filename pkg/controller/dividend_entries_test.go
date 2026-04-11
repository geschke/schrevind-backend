package controller

import (
	"testing"

	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/validate"
)

func validDividendEntryPayloadForValidation() db.DividendEntry {
	return db.DividendEntry{
		DepotID:                    1,
		SecurityID:                 1,
		PayDate:                    "2026-01-15",
		ExDate:                     "2026-01-10",
		SecurityName:               "Example AG",
		SecurityISIN:               "DE0000000001",
		Quantity:                   "1.234,50",
		DividendPerUnitAmount:      "2,35",
		DividendPerUnitCurrency:    "EUR",
		FXRate:                     "   ",
		GrossAmount:                "2.567,05",
		GrossCurrency:              "EUR",
		PayoutAmount:               "2,000.05",
		PayoutCurrency:             "EUR",
		WithholdingTaxPercent:      "26,375",
		WithholdingTaxAmount:       "",
		WithholdingTaxAmountCredit: "0.000000123456",
		ForeignFeesAmount:          "1,23",
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
	entry.FXRate = "1 2"
	entry.GrossAmount = "2.35,76"

	_, errors := normalizeDividendEntryPayload(entry)

	assertFieldError(t, errors, "DepotID", "INVALID_DEPOT_ID")
	assertFieldError(t, errors, "PayDate", "MISSING_PAY_DATE")
	assertFieldError(t, errors, "Quantity", validate.ErrDecimalEmpty)
	assertFieldError(t, errors, "FXRate", validate.ErrDecimalWhitespace)
	assertFieldError(t, errors, "GrossAmount", validate.ErrDecimalInvalidGrouping)
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
