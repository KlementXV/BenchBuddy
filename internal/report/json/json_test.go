package json

import (
	"bytes"
	stdjson "encoding/json"
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func TestRender_RoundTrip(t *testing.T) {
	in := runresult.Report{
		Meta: runresult.RunMeta{
			RunID:     "abc123",
			Profile:   "quick",
			Namespace: "test",
			NodeCount: 2,
		},
		Results: []runresult.Result{
			{BenchName: "network", TaskID: "network/same-node/tcp", Status: runresult.StatusOK,
				Metrics: map[string]runresult.Metric{"bandwidth_sent_gbps": {Value: 9.1, Unit: "Gbps"}}},
		},
		Findings: []runresult.Finding{
			{Severity: runresult.SeverityMedium, Category: "network", Title: "Test finding"},
		},
	}

	var buf bytes.Buffer
	if err := Render(&buf, in); err != nil {
		t.Fatal(err)
	}

	var out runresult.Report
	if err := stdjson.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Meta.RunID != in.Meta.RunID {
		t.Errorf("RunID: got %q, want %q", out.Meta.RunID, in.Meta.RunID)
	}
	if len(out.Results) != 1 {
		t.Errorf("Results: got %d, want 1", len(out.Results))
	}
	if len(out.Findings) != 1 {
		t.Errorf("Findings: got %d, want 1", len(out.Findings))
	}
	if out.Results[0].Metrics["bandwidth_sent_gbps"].Value != 9.1 {
		t.Errorf("metric value: got %v, want 9.1", out.Results[0].Metrics["bandwidth_sent_gbps"].Value)
	}
}

func TestRender_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, runresult.Report{}); err != nil {
		t.Fatal(err)
	}
	if !stdjson.Valid(buf.Bytes()) {
		t.Errorf("output is not valid JSON: %s", buf.String())
	}
}
