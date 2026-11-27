package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is overridden at build time with -X forklift/cmd.version, and by main via SetVersion.
var version = "dev"

func SetVersion(v string) {
	if v != "" {
		version = v
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the forklift version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
