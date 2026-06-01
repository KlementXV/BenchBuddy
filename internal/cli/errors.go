package cli

import "errors"

// ErrHighFindings is returned by runCmd when the report contains one or more
// HIGH or CRITICAL findings. Callers should exit with code 1 without printing
// an error message (the findings are already displayed in the report).
var ErrHighFindings = errors.New("benchbuddy: HIGH or CRITICAL findings detected")
