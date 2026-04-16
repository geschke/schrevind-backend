package format

import (
	"strings"

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
