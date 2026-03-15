package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/export"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// init configures package-level command and flag wiring.
func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().BoolVar(&exportEncrypt, "encrypt", false, "Encrypt the export with AES-256-GCM (prompts for password)")
}

var exportEncrypt bool

var exportCmd = &cobra.Command{
	Use:   "export [filename]",
	Short: "Export all data to a JSON backup file",
	Long: `Exports the complete database contents to a JSON file.

If a filename is provided it is used as-is (relative to the current working
directory). If no filename is given, a timestamped file is written to the
configured export directory (export.dir in config, default: ./data/exports).

Use --encrypt to protect the export with a password (AES-256-GCM).
Encrypted exports use the file extension .enc.json.

Example filenames generated automatically:
  schrevind-backup-2026-03-15T21-30-00.json
  schrevind-backup-2026-03-15T21-30-00.enc.json`,
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
			ts := time.Now().UTC().Format("2006-01-02T15-04-05")
			var filename string
			if exportEncrypt {
				filename = fmt.Sprintf("schrevind-backup-%s.enc.json", ts)
			} else {
				filename = fmt.Sprintf("schrevind-backup-%s.json", ts)
			}
			filePath = filepath.Join(config.Cfg.Export.Dir, filename)
		}

		if !exportEncrypt {
			if err := export.Run(database, filePath); err != nil {
				return err
			}
			fmt.Printf("Export written to %s\n", filePath)
			return nil
		}

		// Prompt for password without echoing it to the terminal.
		fmt.Fprint(os.Stderr, "Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}

		fmt.Fprint(os.Stderr, "Confirm password: ")
		confirmBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("read password confirmation: %w", err)
		}

		if string(passwordBytes) != string(confirmBytes) {
			return fmt.Errorf("passwords do not match")
		}
		if len(passwordBytes) == 0 {
			return fmt.Errorf("password must not be empty")
		}

		if err := export.RunEncrypted(database, filePath, string(passwordBytes)); err != nil {
			return err
		}

		fmt.Printf("Encrypted export written to %s\n", filePath)
		return nil
	},
}
