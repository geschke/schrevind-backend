package export

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/geschke/schrevind/pkg/db"
	"golang.org/x/crypto/scrypt"
)

// scrypt parameters — must be identical in both encryption and decryption.
const (
	scryptN = 32768 // CPU/memory cost (2^15)
	scryptR = 8
	scryptP = 1
	keyLen  = 32 // AES-256
	saltLen = 32
)

// ExportDoc is the top-level structure written to the plain JSON export file.
type ExportDoc struct {
	Format     string     `json:"format"`
	Version    int        `json:"version"`
	ExportedAt time.Time  `json:"exported_at"`
	Data       ExportData `json:"data"`
}

// ExportData holds one slice per exported table.
type ExportData struct {
	Users                  []db.User                  `json:"users"`
	Groups                 []db.Group                 `json:"groups"`
	GroupUsers             []db.GroupUser             `json:"group_users"`
	Memberships            []db.Membership            `json:"memberships"`
	Depots                 []db.Depot                 `json:"depots"`
	Securities             []db.Security              `json:"securities"`
	Currencies             []db.Currency              `json:"currencies"`
	WithholdingTaxDefaults []db.WithholdingTaxDefault `json:"withholding_tax_defaults"`
	DividendEntries        []db.DividendEntry         `json:"dividend_entries"`
}

// EncryptedExportDoc is the top-level structure written to an encrypted export file.
type EncryptedExportDoc struct {
	Format     string `json:"format"`
	Version    int    `json:"version"`
	KDF        string `json:"kdf"`
	Salt       string `json:"salt"`       // base64-encoded random salt for scrypt
	Nonce      string `json:"nonce"`      // base64-encoded AES-GCM nonce
	Ciphertext string `json:"ciphertext"` // base64-encoded AES-256-GCM ciphertext
}

// buildExportJSON loads all data from database and returns the indented JSON bytes.
func buildExportJSON(database *db.DB) ([]byte, error) {
	ctx := context.Background()

	users, err := database.ListAllUsersForExport(ctx)
	if err != nil {
		return nil, fmt.Errorf("export users: %w", err)
	}

	groups, err := database.ListGroups()
	if err != nil {
		return nil, fmt.Errorf("export groups: %w", err)
	}

	groupUsers, err := database.ListAllGroupUsers()
	if err != nil {
		return nil, fmt.Errorf("export group_users: %w", err)
	}

	memberships, err := database.ListAllMemberships()
	if err != nil {
		return nil, fmt.Errorf("export memberships: %w", err)
	}

	depots, err := database.ListAllDepots()
	if err != nil {
		return nil, fmt.Errorf("export depots: %w", err)
	}

	securities, err := database.ListAllSecuritiesForExport()
	if err != nil {
		return nil, fmt.Errorf("export securities: %w", err)
	}

	currencies, err := database.ListAllCurrencies()
	if err != nil {
		return nil, fmt.Errorf("export currencies: %w", err)
	}

	withholdingTaxDefaults, err := database.ListWithholdingTaxDefaults()
	if err != nil {
		return nil, fmt.Errorf("export withholding_tax_defaults: %w", err)
	}

	dividendEntries, err := database.ListAllDividendEntries()
	if err != nil {
		return nil, fmt.Errorf("export dividend_entries: %w", err)
	}

	doc := ExportDoc{
		Format:     "schrevind-export",
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Data: ExportData{
			Users:                  users,
			Groups:                 groups,
			GroupUsers:             groupUsers,
			Memberships:            memberships,
			Depots:                 depots,
			Securities:             securities,
			Currencies:             currencies,
			WithholdingTaxDefaults: withholdingTaxDefaults,
			DividendEntries:        dividendEntries,
		},
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal export JSON: %w", err)
	}

	return out, nil
}

// Run loads all data from database, builds the export document and writes it
// as indented JSON to filePath. The export directory is created if it does not exist.
func Run(database *db.DB, filePath string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("create export directory: %w", err)
	}

	data, err := buildExportJSON(database)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write export file: %w", err)
	}

	return nil
}

// RunEncrypted loads all data from database, encrypts the export JSON with
// AES-256-GCM (key derived via scrypt) and writes the result to filePath.
// The export directory is created if it does not exist.
func RunEncrypted(database *db.DB, filePath string, password string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("create export directory: %w", err)
	}

	plaintext, err := buildExportJSON(database)
	if err != nil {
		return err
	}

	// Generate a random salt for key derivation.
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}

	// Derive a 256-bit key from the password using scrypt.
	key, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return fmt.Errorf("derive key: %w", err)
	}

	// Set up AES-256-GCM.
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	// Generate a random nonce.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt and authenticate the plaintext.
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	doc := EncryptedExportDoc{
		Format:     "schrevind-encrypted-backup",
		Version:    1,
		KDF:        "scrypt",
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal encrypted export: %w", err)
	}

	if err := os.WriteFile(filePath, out, 0644); err != nil {
		return fmt.Errorf("write encrypted export file: %w", err)
	}

	return nil
}

// Decrypt reads an encrypted export document and returns the plaintext JSON.
// Returns an error with message "DECRYPTION_FAILED" if the password is wrong
// or the ciphertext is tampered, and "INVALID_BACKUP_FORMAT" if the file
// cannot be parsed.
func Decrypt(data []byte, password string) ([]byte, error) {
	var doc EncryptedExportDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
	}
	if doc.Format != "schrevind-encrypted-backup" || doc.Version != 1 {
		return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
	}

	salt, err := base64.StdEncoding.DecodeString(doc.Salt)
	if err != nil {
		return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
	}

	nonce, err := base64.StdEncoding.DecodeString(doc.Nonce)
	if err != nil {
		return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(doc.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("INVALID_BACKUP_FORMAT")
	}

	// Derive the key using the same scrypt parameters as during encryption.
	key, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return nil, fmt.Errorf("DECRYPTION_FAILED")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("DECRYPTION_FAILED")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("DECRYPTION_FAILED")
	}

	// Open authenticates and decrypts; returns an error if the tag does not match.
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("DECRYPTION_FAILED")
	}

	return plaintext, nil
}
