package controller

import "testing"

func TestValidateUserInlandTaxTemplate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "empty", in: "", want: "", ok: true},
		{name: "known lowercase", in: " de ", want: "DE", ok: true},
		{name: "unknown", in: "XX", want: "XX", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := validateUserInlandTaxTemplate(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("validateUserInlandTaxTemplate(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}
