package runresult

import (
	"testing"
	"time"
)

func TestTypesConstructable(t *testing.T) {
	r := Report{
		Meta: RunMeta{
			RunID:     "01HXY3K",
			Profile:   "quick",
			Namespace: "benchbuddy",
			StartedAt: time.Now(),
		},
		Results: []Result{
			{
				BenchName: "network",
				TaskID:    "network/same-node/tcp",
				Status:    StatusOK,
				Metrics: map[string]Metric{
					"bandwidth_gbps": {Value: 9.1, Unit: "Gbps"},
				},
				Duration: 30 * time.Second,
			},
		},
		Findings: []Finding{
			{
				Severity: SeverityMedium,
				Category: "network",
				Title:    "test",
				Detail:   "test detail",
			},
		},
	}
	if r.Meta.RunID == "" {
		t.Fatal("RunID should be set")
	}
}
