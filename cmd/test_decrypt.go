package cmd

// NOTE: This command is temporary and intended for testing the encrypted export
// feature only. It will be removed once the feature is verified.

import (
	"fmt"
	"os"

	"github.com/geschke/schrevind/pkg/export"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// init configures package-level command and flag wiring.
func init() {
	rootCmd.AddCommand(testDecryptCmd)
}

var testDecryptCmd = &cobra.Command{
	Use:   "test-decrypt <filename>",
	Short: "Test-decrypt an encrypted export file (temporary, for testing only)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]

		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		// Prompt for password without echoing it to the terminal.
		fmt.Fprint(os.Stderr, "Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}

		plaintext, err := export.Decrypt(data, string(passwordBytes))
		if err != nil {
			return fmt.Errorf("%s", err.Error())
		}

		fmt.Println(string(plaintext))
		return nil
	},
}
