package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/clementlevoux/benchbuddy/internal/config"
)

type imagesListFlags struct {
	profile        string
	format         string // "text" | "json" | "script"
	sourceRegistry string // upstream source for script format
}

type imageEntry struct {
	Name   string `json:"name"`
	Image  string `json:"image"`
	Digest string `json:"digest,omitempty"`
}

func newImagesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images",
		Short: "Manage BenchBuddy container images",
	}
	cmd.AddCommand(newImagesListCommand())
	return cmd
}

func newImagesListCommand() *cobra.Command {
	var f imagesListFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List images required for a given profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return imagesListCmd(cmd.OutOrStdout(), f)
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "standard", "profile to inspect: quick | standard | deep")
	cmd.Flags().StringVar(&f.format, "format", "text", "output format: text | json | script")
	cmd.Flags().StringVar(&f.sourceRegistry, "source-registry", "ghcr.io/clementlevoux/benchbuddy",
		"upstream source registry for script format (skopeo copy source)")
	return cmd
}

func imagesListCmd(stdout io.Writer, f imagesListFlags) error {
	cfg, err := config.Merge(config.MergeInputs{ProfileName: f.profile})
	if err != nil {
		return err
	}

	entries := []imageEntry{
		{
			Name:   "runner",
			Image:  cfg.Images.Runner.FullRef(cfg.Images.Registry),
			Digest: cfg.Images.Runner.Digest,
		},
	}

	switch f.format {
	case "text":
		for _, e := range entries {
			fmt.Fprintf(stdout, "%s: %s\n", e.Name, e.Image)
		}
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	case "script":
		for _, e := range entries {
			src := f.sourceRegistry + "/" + cfg.Images.Runner.Repository + ":" + cfg.Images.Runner.Tag
			if cfg.Images.Runner.Digest != "" {
				src += "@" + cfg.Images.Runner.Digest
			}
			fmt.Fprintf(stdout, "skopeo copy docker://%s docker://%s\n", src, e.Image)
		}
	default:
		return fmt.Errorf("unsupported format %q: use text, json, or script", f.format)
	}
	return nil
}
