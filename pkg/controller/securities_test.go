package controller

import "testing"

func TestNormalizeSecuritySearch(t *testing.T) {
	got, ok := normalizeSecuritySearch("  Cola   Drinks AG  ")
	if !ok {
		t.Fatalf("normalizeSecuritySearch() ok = false")
	}
	if got != "Cola Drinks AG" {
		t.Fatalf("normalizeSecuritySearch() = %q, want normalized spaces", got)
	}

	got, ok = normalizeSecuritySearch("  kön  München  ")
	if !ok {
		t.Fatalf("normalizeSecuritySearch() with umlaut ok = false")
	}
	if got != "kön München" {
		t.Fatalf("normalizeSecuritySearch() with umlaut = %q, want normalized umlaut search", got)
	}

	if _, ok := normalizeSecuritySearch("Cola; DROP"); ok {
		t.Fatalf("normalizeSecuritySearch() accepted invalid characters")
	}

	if _, ok := normalizeSecuritySearch("123456789012345678901234567890123456789012345678901"); ok {
		t.Fatalf("normalizeSecuritySearch() accepted value longer than 50 characters")
	}
}
