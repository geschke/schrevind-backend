package format

import (
	"strings"

	"github.com/shopspring/decimal"
	"golang.org/x/text/language"
)

// DecimalForLocale formats a normalized decimal string for display without changing precision.
func DecimalForLocale(value string, locale string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	tag := language.Make(locale)
	base, _ := tag.Base()
	if base.String() == "de" {
		return strings.ReplaceAll(value, ".", ",")
	}

	return value
}

// DecimalForLocaleFixed formats a normalized decimal string with fixed precision for display.
// Invalid values fall back to DecimalForLocale so response formatting does not hide stored data.
func DecimalForLocaleFixed(value string, locale string, decimalPlaces int32) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if decimalPlaces < 0 {
		return DecimalForLocale(value, locale)
	}

	d, err := decimal.NewFromString(value)
	if err != nil {
		return DecimalForLocale(value, locale)
	}

	return DecimalForLocale(d.StringFixed(decimalPlaces), locale)
}
