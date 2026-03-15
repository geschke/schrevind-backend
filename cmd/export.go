package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/export"
	"github.com/spf13/cobra"
)

// init configures package-level command and flag wiring.
func init() {
	rootCmd.AddCommand(exportCmd)
}

var exportCmd = &cobra.Command{
	Use:   "export [filename]",
	Short: "Export all data to a JSON backup file",
	Long: `Exports the complete database contents to a JSON file.

If a filename is provided it is used as-is (relative to the current working
directory). If no filename is given, a timestamped file is written to the
configured export directory (export.dir in config, default: ./data/exports).

Example filenames generated automatically:
  schrevind-backup-2026-03-15T21-30-00.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, cleanup, err := openDatabase()
		if err != nil {
			return err
		}
		defer cleanup()

		var filePath string
		if len(args) == 1 {
			filePath = args[0]
		} else {
			// Generate a sortable, colon-free timestamp filename.
			ts := time.Now().UTC().Format("2006-01-02T15-04-05")
			filename := fmt.Sprintf("schrevind-backup-%s.json", ts)
			filePath = filepath.Join(config.Cfg.Export.Dir, filename)
		}

		if err := export.Run(database, filePath); err != nil {
			return err
		}

		fmt.Printf("Export written to %s\n", filePath)
		return nil
	},
}
