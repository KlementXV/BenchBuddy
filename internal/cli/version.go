package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is injected at build time via -ldflags
var version = "dev"

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print benchbuddy version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "benchbuddy %s\n", version)
			return err
		},
	}
}
