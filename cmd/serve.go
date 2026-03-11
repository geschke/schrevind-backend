/*
Copyright © 2025 Ralf Geschke <ralf@kuerbis.org>
*/
package cmd

import (
	"github.com/geschke/schrevind/pkg/server"
	"github.com/spf13/cobra"
)

// init configures package-level command and flag wiring.
func init() {
	rootCmd.AddCommand(serveCmd)

}

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the schrevind HTTP server.",

	Long: `Starts the schrevind HTTP server using the configuration
provided via config file, environment variables, or CLI flags.

The server uses the configured SQLite database and listens on the
configured HTTP address.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		database, cleanup, err := openDatabase()
		if err != nil {
			return err
		}
		defer cleanup()
		return server.Start(database)
	},
}
