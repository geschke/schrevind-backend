package cmd

import (
	"fmt"
	"os"

	"github.com/geschke/schrevind/pkg/restore"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// init configures package-level command and flag wiring.
func init() {
	rootCmd.AddCommand(importCmd)
}

var importCmd = &cobra.Command{
	Use:   "import <filename>",
	Short: "Restore database from a backup file (plain or encrypted)",
	Long: `Performs a full database restore from a backup file.

All existing data is replaced. The operation runs in a single transaction:
if any error occurs, the database is rolled back to its previous state.

Both plain and encrypted backups are supported. For encrypted backups
the password is prompted interactively (without echo).

Examples:
  schrevind import backup.json
  schrevind import backup.enc.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]

		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		database, cleanup, err := openDatabase()
		if err != nil {
			return err
		}
		defer cleanup()

		// passwordFn is called only when the backup is encrypted.
		passwordFn := func() (string, error) {
			fmt.Fprint(os.Stderr, "Password: ")
			passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return "", fmt.Errorf("read password: %w", err)
			}
			return string(passwordBytes), nil
		}

		doc, err := restore.Load(data, passwordFn)
		if err != nil {
			return fmt.Errorf("%s", err.Error())
		}

		if err := restore.Run(database, doc); err != nil {
			return fmt.Errorf("%s", err.Error())
		}

		fmt.Println("Import completed successfully.")
		return nil
	},
}
