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
