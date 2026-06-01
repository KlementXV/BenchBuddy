package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/clementlevoux/benchbuddy/internal/report/terminal"
	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func newShowCommand() *cobra.Command {
	var noColor bool
	cmd := &cobra.Command{
		Use:   "show <report.json>",
		Short: "Pretty-print a previously saved JSON report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := loadReport(args[0])
			if err != nil {
				return err
			}
			return terminal.Render(cmd.OutOrStdout(), r, terminal.RenderOptions{NoColor: noColor})
		},
	}
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable ANSI colors")
	return cmd
}

func loadReport(path string) (runresult.Report, error) {
	var r runresult.Report
	f, err := os.Open(path)
	if err != nil {
		return r, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&r); err != nil {
		return r, fmt.Errorf("decode %s: %w", path, err)
	}
	return r, nil
}

