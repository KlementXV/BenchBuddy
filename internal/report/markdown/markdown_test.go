// internal/report/markdown/markdown_test.go
package markdown

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

func TestRender_Golden(t *testing.T) {
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
				BenchName: "network", TaskID: "network/same-node/tcp",
				Subject: "node-1 (tcp, same-node)", Status: runresult.StatusOK,
				Metrics: map[string]runresult.Metric{"bandwidth_sent_gbps": {Value: 9.1, Unit: "Gbps"}},
			},
			{
				BenchName: "network", TaskID: "network/cross-node/tcp",
				Subject: "node-1 → node-2 (tcp)", Status: runresult.StatusOK,
				Metrics: map[string]runresult.Metric{"bandwidth_sent_gbps": {Value: 4.2, Unit: "Gbps"}},
			},
		},
		Findings: []runresult.Finding{
			{
				Severity: runresult.SeverityMedium,
				Category: "network",
				Title:    "Cross-node TCP bandwidth degradation",
				Detail:   "Cross-node 4.20 Gbps is 46% of same-node 9.10 Gbps. CNI overhead suspected.",
				Fix:      []string{"Check CNI encapsulation mode (VXLAN/IPIP adds overhead on Calico, Flannel)"},
				Refs:     []string{"https://docs.cilium.io/en/stable/operations/performance/"},
			},
		},
	}

	var buf bytes.Buffer
	if err := Render(&buf, rep); err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "sample.golden.md")
	if *update {
		_ = os.MkdirAll("testdata", 0755)
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
