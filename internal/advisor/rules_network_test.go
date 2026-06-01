package advisor

import (
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func netResult(taskID string, gbps float64) runresult.Result {
	return runresult.Result{
		BenchName: "network",
		TaskID:    taskID,
		Status:    runresult.StatusOK,
		Metrics:   map[string]runresult.Metric{"bandwidth_sent_gbps": {Value: gbps, Unit: "Gbps"}},
	}
}

func udpResult(taskID string, gbps, lostPct, jitterMs float64) runresult.Result {
	return runresult.Result{
		BenchName: "network",
		TaskID:    taskID,
		Status:    runresult.StatusOK,
		Metrics: map[string]runresult.Metric{
			"bandwidth_sent_gbps": {Value: gbps, Unit: "Gbps"},
			"lost_percent":        {Value: lostPct, Unit: "%"},
			"jitter_ms":           {Value: jitterMs, Unit: "ms"},
		},
	}
}

func TestRuleNetCrossNodeTCPDegradation(t *testing.T) {
	tests := []struct {
		name    string
		same    float64
		cross   float64
		wantN   int
		wantSev runresult.Severity
	}{
		{"no finding when cross >= 70%", 10.0, 7.0, 0, ""},
		{"fires MEDIUM when cross < 70%", 10.0, 4.0, 1, runresult.SeverityMedium},
		{"no finding when same result missing", 0, 4.0, 0, ""},
		{"no finding when cross result missing", 10.0, 0, 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var results []runresult.Result
			if tc.same > 0 {
				results = append(results, netResult("network/same-node/tcp", tc.same))
			}
			if tc.cross > 0 {
				results = append(results, netResult("network/cross-node/tcp", tc.cross))
			}
			findings := ruleNetCrossNodeTCPDegradation.Evaluate(results)
			if len(findings) != tc.wantN {
				t.Errorf("got %d findings, want %d", len(findings), tc.wantN)
			}
			if tc.wantN > 0 && findings[0].Severity != tc.wantSev {
				t.Errorf("severity: got %q, want %q", findings[0].Severity, tc.wantSev)
			}
		})
	}
}

func TestRuleNetCrossNodeTCPLow(t *testing.T) {
	tests := []struct {
		name  string
		gbps  float64
		wantN int
	}{
		{"no finding above 1 Gbps", 1.5, 0},
		{"fires HIGH below 1 Gbps", 0.5, 1},
		{"no finding when result missing", 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var results []runresult.Result
			if tc.gbps > 0 {
				results = append(results, netResult("network/cross-node/tcp", tc.gbps))
			}
			findings := ruleNetCrossNodeTCPLow.Evaluate(results)
			if len(findings) != tc.wantN {
				t.Errorf("got %d findings, want %d", len(findings), tc.wantN)
			}
			if tc.wantN > 0 && findings[0].Severity != runresult.SeverityHigh {
				t.Errorf("expected HIGH, got %q", findings[0].Severity)
			}
		})
	}
}

func TestRuleNetSameNodeTCPLow(t *testing.T) {
	findings := ruleNetSameNodeTCPLow.Evaluate([]runresult.Result{netResult("network/same-node/tcp", 3.0)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityMedium {
		t.Errorf("expected 1 MEDIUM finding, got %v", findings)
	}
	findings = ruleNetSameNodeTCPLow.Evaluate([]runresult.Result{netResult("network/same-node/tcp", 8.0)})
	if len(findings) != 0 {
		t.Errorf("expected 0 findings above threshold, got %d", len(findings))
	}
}

func TestRuleNetUDPPacketLoss(t *testing.T) {
	findings := ruleNetUDPPacketLoss.Evaluate([]runresult.Result{udpResult("network/cross-node/udp", 5.0, 2.5, 1.0)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityMedium {
		t.Errorf("expected 1 MEDIUM finding, got %v", findings)
	}
	if findings[0].SourceIDs[0] != "network/cross-node/udp" {
		t.Errorf("expected source network/cross-node/udp, got %v", findings[0].SourceIDs)
	}
	findings = ruleNetUDPPacketLoss.Evaluate([]runresult.Result{udpResult("network/cross-node/udp", 5.0, 0.5, 1.0)})
	if len(findings) != 0 {
		t.Error("expected 0 findings below threshold")
	}
	// same-node fallback when no cross-node result
	findings = ruleNetUDPPacketLoss.Evaluate([]runresult.Result{udpResult("network/same-node/udp", 5.0, 2.5, 1.0)})
	if len(findings) != 1 {
		t.Error("expected finding for same-node UDP packet loss")
	}
	if findings[0].SourceIDs[0] != "network/same-node/udp" {
		t.Errorf("expected source network/same-node/udp, got %v", findings[0].SourceIDs)
	}
}

func TestRuleNetUDPJitter(t *testing.T) {
	findings := ruleNetUDPJitter.Evaluate([]runresult.Result{udpResult("network/cross-node/udp", 5.0, 0, 8.0)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityLow {
		t.Errorf("expected 1 LOW finding, got %v", findings)
	}
	findings = ruleNetUDPJitter.Evaluate([]runresult.Result{udpResult("network/cross-node/udp", 5.0, 0, 2.0)})
	if len(findings) != 0 {
		t.Error("expected 0 findings below threshold")
	}
}
