package validate

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	ErrDecimalEmpty           = "ERR_DECIMAL_EMPTY"
	ErrDecimalWhitespace      = "ERR_DECIMAL_WHITESPACE"
	ErrDecimalNegative        = "ERR_DECIMAL_NEGATIVE"
	ErrDecimalInvalidChars    = "ERR_DECIMAL_INVALID_CHARS"
	ErrDecimalMissingDigits   = "ERR_DECIMAL_MISSING_DIGITS"
	ErrDecimalInvalidFormat   = "ERR_DECIMAL_INVALID_FORMAT"
	ErrDecimalInvalidGrouping = "ERR_DECIMAL_INVALID_GROUPING"
)

// NormalizeDecimalString validates and normalizes decimal input without using floating point conversion.
func NormalizeDecimalString(raw string, allowNegative bool) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf(ErrDecimalEmpty)
	}
	if containsWhitespace(value) {
		return "", fmt.Errorf(ErrDecimalWhitespace)
	}

	sign := ""
	if strings.HasPrefix(value, "-") {
		if !allowNegative {
			return "", fmt.Errorf(ErrDecimalNegative)
		}
		sign = "-"
		value = value[1:]
		if value == "" {
			return "", fmt.Errorf(ErrDecimalMissingDigits)
		}
	}
	if strings.Contains(value, "-") {
		return "", fmt.Errorf(ErrDecimalInvalidFormat)
	}

	if !containsOnlyDecimalChars(value) {
		return "", fmt.Errorf(ErrDecimalInvalidChars)
	}
	if !containsDigit(value) {
		return "", fmt.Errorf(ErrDecimalMissingDigits)
	}

	dotCount := strings.Count(value, ".")
	commaCount := strings.Count(value, ",")

	switch {
	case dotCount == 0 && commaCount == 0:
		return sign + value, nil
	case dotCount > 0 && commaCount > 0:
		normalized, err := normalizeMixedDecimal(value)
		if err != nil {
			return "", err
		}
		return sign + normalized, nil
	case dotCount > 0:
		normalized, err := normalizeSingleSeparatorDecimal(value, '.')
		if err != nil {
			return "", err
		}
		return sign + normalized, nil
	default:
		normalized, err := normalizeSingleSeparatorDecimal(value, ',')
		if err != nil {
			return "", err
		}
		return sign + normalized, nil
	}
}

func containsWhitespace(value string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func containsOnlyDecimalChars(value string) bool {
	for _, r := range value {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' {
			continue
		}
		return false
	}
	return true
}

func containsDigit(value string) bool {
	for _, r := range value {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func normalizeSingleSeparatorDecimal(value string, separator byte) (string, error) {
	count := strings.Count(value, string(separator))
	if count == 1 {
		return normalizeDecimalSeparator(value, separator)
	}

	normalized, ok := normalizeGroupedInteger(value, separator)
	if !ok {
		return "", fmt.Errorf(ErrDecimalInvalidGrouping)
	}
	return normalized, nil
}

func normalizeMixedDecimal(value string) (string, error) {
	lastDot := strings.LastIndex(value, ".")
	lastComma := strings.LastIndex(value, ",")

	decimalSeparator := byte('.')
	thousandsSeparator := byte(',')
	if lastComma > lastDot {
		decimalSeparator = ','
		thousandsSeparator = '.'
	}

	if strings.Count(value, string(decimalSeparator)) != 1 {
		return "", fmt.Errorf(ErrDecimalInvalidFormat)
	}

	decimalIndex := strings.LastIndexByte(value, decimalSeparator)
	integerPart := value[:decimalIndex]
	fractionalPart := value[decimalIndex+1:]
	if integerPart == "" || fractionalPart == "" {
		return "", fmt.Errorf(ErrDecimalInvalidFormat)
	}
	if !isAllDigits(fractionalPart) {
		return "", fmt.Errorf(ErrDecimalInvalidFormat)
	}

	normalizedInteger := integerPart
	if strings.Contains(integerPart, string(thousandsSeparator)) {
		var ok bool
		normalizedInteger, ok = normalizeGroupedInteger(integerPart, thousandsSeparator)
		if !ok {
			return "", fmt.Errorf(ErrDecimalInvalidGrouping)
		}
	} else if !isAllDigits(integerPart) {
		return "", fmt.Errorf(ErrDecimalInvalidFormat)
	}

	return normalizedInteger + "." + fractionalPart, nil
}

func normalizeDecimalSeparator(value string, separator byte) (string, error) {
	index := strings.IndexByte(value, separator)
	integerPart := value[:index]
	fractionalPart := value[index+1:]
	if integerPart == "" || fractionalPart == "" {
		return "", fmt.Errorf(ErrDecimalInvalidFormat)
	}
	if !isAllDigits(integerPart) || !isAllDigits(fractionalPart) {
		return "", fmt.Errorf(ErrDecimalInvalidFormat)
	}
	if separator == ',' {
		return integerPart + "." + fractionalPart, nil
	}
	return value, nil
}

func normalizeGroupedInteger(value string, separator byte) (string, bool) {
	parts := strings.Split(value, string(separator))
	if len(parts) < 2 {
		return "", false
	}

	if len(parts[0]) == 0 || len(parts[0]) > 3 || !isAllDigits(parts[0]) {
		return "", false
	}
	for _, part := range parts[1:] {
		if len(part) != 3 || !isAllDigits(part) {
			return "", false
		}
	}

	return strings.Join(parts, ""), true
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
