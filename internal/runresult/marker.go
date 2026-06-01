package runresult

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// MarkerPrefix is the constant prefix used to delimit machine-readable
// result lines emitted by bench pods on stdout. Both the runner (writer)
// and the CLI (reader) share this constant.
const MarkerPrefix = "BENCHBUDDY_RESULT:"

// ErrNoMarker indicates the input contains no recognizable marker line.
var ErrNoMarker = errors.New("no BENCHBUDDY_RESULT marker found")

// ParseMarker scans rawLog and returns the metrics from the LAST marker
// line found. If no marker is present or the JSON fails to parse, an error
// is returned. The second return value is the raw log (for storage in
// Result.RawOutput).
func ParseMarker(rawLog io.Reader) (map[string]Metric, string, error) {
	scanner := bufio.NewScanner(rawLog)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var raw strings.Builder
	var lastPayload string
	for scanner.Scan() {
		line := scanner.Text()
		raw.WriteString(line)
		raw.WriteByte('\n')

		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, MarkerPrefix); ok {
			lastPayload = strings.TrimSpace(after)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, raw.String(), err
	}
	if lastPayload == "" {
		return nil, raw.String(), ErrNoMarker
	}

	metrics := map[string]Metric{}
	if err := json.Unmarshal([]byte(lastPayload), &metrics); err != nil {
		return nil, raw.String(), fmt.Errorf("parse marker JSON: %w", err)
	}
	return metrics, raw.String(), nil
}

// EmitMarker writes a single marker line containing metrics. Used by the
// runner binary at the end of each bench invocation.
func EmitMarker(w io.Writer, metrics map[string]Metric) error {
	payload, err := json.Marshal(metrics)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(w, MarkerPrefix+" "+string(payload)+"\n"); err != nil {
		return fmt.Errorf("emit marker: %w", err)
	}
	return nil
}
