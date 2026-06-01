package runresult

import "time"

type Status string

const (
	StatusOK      Status = "OK"
	StatusSkipped Status = "Skipped"
	StatusPartial Status = "Partial"
	StatusFailed  Status = "Failed"
)

type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

type Metric struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type Result struct {
	BenchName string            `json:"benchName"`
	TaskID    string            `json:"taskId"`
	Subject   string            `json:"subject,omitempty"`
	Status    Status            `json:"status"`
	Metrics   map[string]Metric `json:"metrics,omitempty"`
	RawOutput string            `json:"rawOutput,omitempty"`
	Duration  time.Duration     `json:"durationNs"`
	Errors    []string          `json:"errors,omitempty"`
}

type RunMeta struct {
	RunID      string        `json:"runId"`
	Profile    string        `json:"profile"`
	Namespace  string        `json:"namespace"`
	K8sVersion string        `json:"k8sVersion,omitempty"`
	CNI        string        `json:"cni,omitempty"`
	NodeCount  int           `json:"nodeCount"`
	StartedAt  time.Time     `json:"startedAt"`
	Duration   time.Duration `json:"durationNs"`
}

type Finding struct {
	Severity  Severity `json:"severity"`
	Category  string   `json:"category"`
	Title     string   `json:"title"`
	Detail    string   `json:"detail"`
	Fix       []string `json:"fix,omitempty"`
	Refs      []string `json:"refs,omitempty"`
	SourceIDs []string `json:"sourceIds,omitempty"`
}

type Report struct {
	Meta     RunMeta   `json:"meta"`
	Results  []Result  `json:"results"`
	Findings []Finding `json:"findings"`
}
