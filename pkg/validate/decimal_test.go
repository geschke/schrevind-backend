package validate

import "testing"

func TestNormalizeDecimalStringValidPositiveValues(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "comma decimal", raw: "2,35", want: "2.35"},
		{name: "dot decimal", raw: "2.35", want: "2.35"},
		{name: "comma decimal high precision", raw: "2,350505", want: "2.350505"},
		{name: "dot decimal high precision", raw: "2.350505", want: "2.350505"},
		{name: "comma decimal without grouping", raw: "2567,05", want: "2567.05"},
		{name: "dot decimal without grouping", raw: "2567.05", want: "2567.05"},
		{name: "dot grouped comma decimal", raw: "2.567,05", want: "2567.05"},
		{name: "comma grouped dot decimal", raw: "2,567.05", want: "2567.05"},
		{name: "dot grouped integer", raw: "1.234.567", want: "1234567"},
		{name: "comma grouped integer", raw: "1,234,567", want: "1234567"},
		{name: "integer", raw: "1000", want: "1000"},
		{name: "high precision decimal", raw: "0.000000123456", want: "0.000000123456"},
		{name: "trimmed value", raw: "  2,35  ", want: "2.35"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeDecimalString(tt.raw, false)
			if err != nil {
				t.Fatalf("NormalizeDecimalString() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeDecimalString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeDecimalStringValidNegativeValues(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "negative comma decimal", raw: "-2,35", want: "-2.35"},
		{name: "negative dot decimal", raw: "-2.35", want: "-2.35"},
		{name: "negative dot grouped comma decimal", raw: "-2.567,05", want: "-2567.05"},
		{name: "negative dot grouped integer", raw: "-1.234.567", want: "-1234567"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeDecimalString(tt.raw, true)
			if err != nil {
				t.Fatalf("NormalizeDecimalString() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeDecimalString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeDecimalStringInvalidValues(t *testing.T) {
	tests := []string{
		"1.23.45",
		"1,23,45",
		"2.35,76",
		"2,35.76",
		"12,34.56,78",
		"123,",
		"123.",
		"",
		"   ",
		"1 234,56",
		"abc",
		"12a",
		"12_34",
		"€12",
		"-2,35",
		"--1",
		"1-2",
		"-",
		"-,",
		"-.",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if got, err := NormalizeDecimalString(raw, false); err == nil {
				t.Fatalf("NormalizeDecimalString() = %q, want error", got)
			}
		})
	}
}

func TestNormalizeDecimalStringErrorCodes(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		allowNegative bool
		want          string
	}{
		{name: "empty", raw: "", want: ErrDecimalEmpty},
		{name: "whitespace", raw: "1 234,56", want: ErrDecimalWhitespace},
		{name: "invalid chars", raw: "12a", want: ErrDecimalInvalidChars},
		{name: "negative not allowed", raw: "-2,35", want: ErrDecimalNegative},
		{name: "missing digits after minus", raw: "-", allowNegative: true, want: ErrDecimalMissingDigits},
		{name: "invalid format", raw: "123,", want: ErrDecimalInvalidFormat},
		{name: "invalid grouping", raw: "1.23.45", want: ErrDecimalInvalidGrouping},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeDecimalString(tt.raw, tt.allowNegative)
			if err == nil {
				t.Fatalf("NormalizeDecimalString() error = nil, want %q", tt.want)
			}
			if err.Error() != tt.want {
				t.Fatalf("NormalizeDecimalString() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestNormalizeMixedDecimalIntegerPart(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "dot grouped integer part", raw: "1.234,56", want: "1234.56"},
		{name: "comma grouped integer part", raw: "1,234.56", want: "1234.56"},
		{name: "ungrouped comma decimal", raw: "1234,56", want: "1234.56"},
		{name: "ungrouped dot decimal", raw: "1234.56", want: "1234.56"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMixedDecimal(tt.raw)
			if err != nil {
				t.Fatalf("normalizeMixedDecimal() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeMixedDecimal() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeDecimalStringInvalidNegativeValues(t *testing.T) {
	tests := []string{
		"--1",
		"1-2",
		"-",
		"-,",
		"-.",
		"-abc",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if got, err := NormalizeDecimalString(raw, true); err == nil {
				t.Fatalf("NormalizeDecimalString() = %q, want error", got)
			}
		})
	}
}
