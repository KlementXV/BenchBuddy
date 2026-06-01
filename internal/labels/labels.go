package labels

import (
	"regexp"
	"strings"
	"time"
)

const (
	Prefix    = "benchbuddy.io/"
	RunIDKey  = Prefix + "run-id"
	BenchKey  = Prefix + "bench"
	TaskKey   = Prefix + "task"
	CreatedAt = Prefix + "created-at"
	Version   = Prefix + "cli-version"
)

// ForRun returns the minimal label set tagging an object with a specific run.
func ForRun(runID string) map[string]string {
	return map[string]string{
		RunIDKey:  runID,
		CreatedAt: sanitizeForLabelValue(time.Now().UTC().Format(time.RFC3339)),
	}
}

// ForTask extends ForRun with bench + task labels. Values are sanitized to
// fit the K8s label value charset (DNS-1123).
func ForTask(runID, bench, taskID string) map[string]string {
	l := ForRun(runID)
	l[BenchKey] = sanitizeForLabelValue(bench)
	l[TaskKey] = sanitizeForLabelValue(taskID)
	return l
}

// SelectorForRun returns a label selector matching all objects from one run.
func SelectorForRun(runID string) string {
	return RunIDKey + "=" + runID
}

// SelectorForAllRuns returns a label selector matching all BenchBuddy-created objects
// regardless of run ID (i.e., any object that has the run-id label).
func SelectorForAllRuns() string {
	return RunIDKey
}

var labelInvalid = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func sanitizeForLabelValue(s string) string {
	s = labelInvalid.ReplaceAllString(s, "_")
	s = strings.Trim(s, "-_.")
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}
