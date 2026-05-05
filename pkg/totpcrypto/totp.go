package totpcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const backupCodeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// EncryptTOTPSecret encrypts a TOTP secret using AES-GCM and returns base64(nonce|ciphertext).
func EncryptTOTPSecret(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	out := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptTOTPSecret decrypts a base64(nonce|ciphertext) AES-GCM payload.
func DecryptTOTPSecret(ciphertext string, key []byte) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ciphertext))
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	if len(raw) <= gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce := raw[:gcm.NonceSize()]
	encrypted := raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}

	return string(plaintext), nil
}

// GenerateBackupCodes creates n plaintext backup codes and bcrypt hashes.
func GenerateBackupCodes(n int) ([]string, []string, error) {
	if n <= 0 {
		return nil, nil, fmt.Errorf("n must be > 0")
	}

	plain := make([]string, 0, n)
	hashes := make([]string, 0, n)
	for i := 0; i < n; i++ {
		code, err := randomBackupCode(10)
		if err != nil {
			return nil, nil, err
		}
		hash, err := HashBackupCode(code)
		if err != nil {
			return nil, nil, err
		}
		plain = append(plain, code)
		hashes = append(hashes, hash)
	}

	return plain, hashes, nil
}

// HashBackupCode hashes one plaintext backup code with bcrypt.
func HashBackupCode(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(plain)), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash backup code: %w", err)
	}
	return string(hash), nil
}

// ValidateBackupCode checks a plaintext backup code against bcrypt hashes and returns the match index.
func ValidateBackupCode(plain string, hashes []string) (bool, int, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return false, -1, nil
	}

	for i, hash := range hashes {
		err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
		if err == nil {
			return true, i, nil
		}
		if err != bcrypt.ErrMismatchedHashAndPassword {
			return false, -1, fmt.Errorf("validate backup code: %w", err)
		}
	}

	return false, -1, nil
}

func randomBackupCode(length int) (string, error) {
	var b strings.Builder
	b.Grow(length)
	max := big.NewInt(int64(len(backupCodeAlphabet)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("read backup code random: %w", err)
		}
		b.WriteByte(backupCodeAlphabet[n.Int64()])
	}
	return b.String(), nil
}
