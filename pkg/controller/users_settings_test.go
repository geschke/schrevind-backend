package controller

import "testing"

func TestValidateUserSettingsString(t *testing.T) {
	got, ok := validateUserSettingsString(" dark ")
	if !ok {
		t.Fatalf("validateUserSettingsString() ok = false")
	}
	if got != "dark" {
		t.Fatalf("validateUserSettingsString() = %q, want dark", got)
	}

	if _, ok := validateUserSettingsString("dark\nmode"); ok {
		t.Fatalf("validateUserSettingsString() accepted control character")
	}

	if _, ok := validateUserSettingsString("123456789012345678901234567890123456789012345678901"); ok {
		t.Fatalf("validateUserSettingsString() accepted value longer than 50 characters")
	}
}

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
