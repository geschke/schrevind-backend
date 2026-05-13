package format

import "testing"

func TestDecimalForLocale(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		locale string
		want   string
	}{
		{name: "german comma", value: "123.45", locale: "de-DE", want: "123,45"},
		{name: "german keeps precision", value: "1.18700", locale: "de-DE", want: "1,18700"},
		{name: "english dot", value: "17.17", locale: "en-US", want: "17.17"},
		{name: "integer unchanged", value: "1000", locale: "de-DE", want: "1000"},
		{name: "empty unchanged", value: "", locale: "de-DE", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DecimalForLocale(tt.value, tt.locale); got != tt.want {
				t.Fatalf("DecimalForLocale() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecimalForLocaleFixed(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		locale        string
		decimalPlaces int32
		want          string
	}{
		{name: "german pads zero", value: "478.7", locale: "de-DE", decimalPlaces: 2, want: "478,70"},
		{name: "english pads zero", value: "478.7", locale: "en-US", decimalPlaces: 2, want: "478.70"},
		{name: "zero decimals", value: "478.7", locale: "de-DE", decimalPlaces: 0, want: "479"},
		{name: "keeps empty", value: "", locale: "de-DE", decimalPlaces: 2, want: ""},
		{name: "invalid falls back", value: "abc.7", locale: "de-DE", decimalPlaces: 2, want: "abc,7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DecimalForLocaleFixed(tt.value, tt.locale, tt.decimalPlaces); got != tt.want {
				t.Fatalf("DecimalForLocaleFixed() = %q, want %q", got, tt.want)
			}
		})
	}
}
