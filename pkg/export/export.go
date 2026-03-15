package export

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/geschke/schrevind/pkg/db"
)

// ExportDoc is the top-level structure written to the JSON export file.
type ExportDoc struct {
	Format     string     `json:"format"`
	Version    int        `json:"version"`
	ExportedAt time.Time  `json:"exported_at"`
	Data       ExportData `json:"data"`
}

// ExportData holds one slice per exported table.
type ExportData struct {
	Users                  []db.User                  `json:"users"`
	Depots                 []db.Depot                 `json:"depots"`
	Securities             []db.Security              `json:"securities"`
	Currencies             []db.Currency              `json:"currencies"`
	WithholdingTaxDefaults []db.WithholdingTaxDefault `json:"withholding_tax_defaults"`
	DividendEntries        []db.DividendEntry         `json:"dividend_entries"`
}

// Run loads all data from database, builds the export document and writes it
// as indented JSON to filePath. The export directory is created if it does not exist.
func Run(database *db.DB, filePath string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("create export directory: %w", err)
	}

	ctx := context.Background()

	users, err := database.ListAllUsersForExport(ctx)
	if err != nil {
		return fmt.Errorf("export users: %w", err)
	}

	depots, err := database.ListAllDepots()
	if err != nil {
		return fmt.Errorf("export depots: %w", err)
	}

	securities, err := database.ListAllSecurities()
	if err != nil {
		return fmt.Errorf("export securities: %w", err)
	}

	currencies, err := database.ListAllCurrencies()
	if err != nil {
		return fmt.Errorf("export currencies: %w", err)
	}

	withholdingTaxDefaults, err := database.ListWithholdingTaxDefaults()
	if err != nil {
		return fmt.Errorf("export withholding_tax_defaults: %w", err)
	}

	dividendEntries, err := database.ListAllDividendEntries()
	if err != nil {
		return fmt.Errorf("export dividend_entries: %w", err)
	}

	doc := ExportDoc{
		Format:     "schrevind-export",
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Data: ExportData{
			Users:                  users,
			Depots:                 depots,
			Securities:             securities,
			Currencies:             currencies,
			WithholdingTaxDefaults: withholdingTaxDefaults,
			DividendEntries:        dividendEntries,
		},
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal export JSON: %w", err)
	}

	if err := os.WriteFile(filePath, out, 0644); err != nil {
		return fmt.Errorf("write export file: %w", err)
	}

	return nil
}
