package cli

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "benchbuddy",
		Short:         "Kubernetes cluster performance benchmark",
		Long:          "BenchBuddy runs one-shot diagnostic benchmarks against a Kubernetes cluster.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newCleanCommand())
	cmd.AddCommand(newListRunsCommand())
	cmd.AddCommand(newImagesCommand())
	return cmd
}
