package totpcrypto

import (
	"strings"
	"testing"
)

func TestEncryptDecryptTOTPSecret(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	encrypted, err := EncryptTOTPSecret("SECRET123", key)
	if err != nil {
		t.Fatalf("EncryptTOTPSecret() error = %v", err)
	}
	if encrypted == "" || encrypted == "SECRET123" {
		t.Fatalf("encrypted = %q, want non-empty ciphertext", encrypted)
	}

	decrypted, err := DecryptTOTPSecret(encrypted, key)
	if err != nil {
		t.Fatalf("DecryptTOTPSecret() error = %v", err)
	}
	if decrypted != "SECRET123" {
		t.Fatalf("decrypted = %q, want SECRET123", decrypted)
	}
}

func TestGenerateAndValidateBackupCodes(t *testing.T) {
	plain, hashes, err := GenerateBackupCodes(8)
	if err != nil {
		t.Fatalf("GenerateBackupCodes() error = %v", err)
	}
	if len(plain) != 8 || len(hashes) != 8 {
		t.Fatalf("lengths = %d/%d, want 8/8", len(plain), len(hashes))
	}
	for _, code := range plain {
		if len(code) != 10 {
			t.Fatalf("backup code %q length = %d, want 10", code, len(code))
		}
		if code != strings.ToUpper(code) {
			t.Fatalf("backup code %q should be uppercase", code)
		}
	}

	match, index, err := ValidateBackupCode(plain[3], hashes)
	if err != nil {
		t.Fatalf("ValidateBackupCode() error = %v", err)
	}
	if !match || index != 3 {
		t.Fatalf("ValidateBackupCode() = %v, %d; want true, 3", match, index)
	}

	match, index, err = ValidateBackupCode("WRONGCODE1", hashes)
	if err != nil {
		t.Fatalf("ValidateBackupCode(wrong) error = %v", err)
	}
	if match || index != -1 {
		t.Fatalf("ValidateBackupCode(wrong) = %v, %d; want false, -1", match, index)
	}
}
