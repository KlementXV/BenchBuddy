package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/clementlevoux/benchbuddy/internal/cli"
)

func main() {
	err := cli.NewRootCommand().Execute()
	if err == nil {
		return
	}
	if errors.Is(err, cli.ErrHighFindings) {
		// Findings already displayed in the report; exit 1 without error text.
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(2)
}
