package json

import (
	"encoding/json"
	"io"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

// Render serializes report as indented JSON to w.
func Render(w io.Writer, r runresult.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
