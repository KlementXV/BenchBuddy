package terminal

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

var update = flag.Bool("update", false, "update golden files")

func TestRender_Sample(t *testing.T) {
	rep := runresult.Report{
		Meta: runresult.RunMeta{
			RunID:      "01HXY3K",
			Profile:    "quick",
			Namespace:  "benchbuddy",
			K8sVersion: "v1.30.0",
			CNI:        "kindnet",
			NodeCount:  2,
			Duration:   45 * time.Second,
		},
		Results: []runresult.Result{
			{
				BenchName: "network",
				TaskID:    "network/same-node/tcp",
				Subject:   "node-1 (tcp, same-node)",
				Status:    runresult.StatusOK,
				Metrics: map[string]runresult.Metric{
					"bandwidth_sent_gbps": {Value: 9.1, Unit: "Gbps"},
				},
				Duration: 12 * time.Second,
			},
			{
				BenchName: "network",
				TaskID:    "network/cross-node/tcp",
				Subject:   "node-1 → node-2 (tcp)",
				Status:    runresult.StatusOK,
				Metrics: map[string]runresult.Metric{
					"bandwidth_sent_gbps": {Value: 4.2, Unit: "Gbps"},
				},
				Duration: 12 * time.Second,
			},
		},
	}

	var buf bytes.Buffer
	if err := Render(&buf, rep, RenderOptions{NoColor: true}); err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "sample.golden")
	if *update {
		_ = os.WriteFile(golden, buf.Bytes(), 0644)
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to create)", err)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		t.Errorf("output mismatch.\n--- want ---\n%s\n--- got ---\n%s", want, buf.Bytes())
	}
}
