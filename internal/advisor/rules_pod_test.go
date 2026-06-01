package advisor

import (
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func podStartupResult(p50ms, p99ms float64) runresult.Result {
	return runresult.Result{
		BenchName: "pod",
		TaskID:    "pod/startup",
		Status:    runresult.StatusOK,
		Metrics: map[string]runresult.Metric{
			"startup_p50_ms": {Value: p50ms, Unit: "ms"},
			"startup_p99_ms": {Value: p99ms, Unit: "ms"},
		},
	}
}

func TestRulePodStartupP99Slow(t *testing.T) {
	// 45000 ms = 45 s: between 30s and 60s → MEDIUM
	findings := rulePodStartupP99Slow.Evaluate([]runresult.Result{podStartupResult(5000, 45000)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityMedium {
		t.Errorf("expected 1 MEDIUM finding, got %v", findings)
	}
	// 90000 ms = 90 s: handled by critical rule
	findings = rulePodStartupP99Slow.Evaluate([]runresult.Result{podStartupResult(5000, 90000)})
	if len(findings) != 0 {
		t.Error("p99 >= 60s should be handled by critical rule")
	}
	// 10000 ms = 10 s: fine
	findings = rulePodStartupP99Slow.Evaluate([]runresult.Result{podStartupResult(5000, 10000)})
	if len(findings) != 0 {
		t.Error("expected 0 findings below threshold")
	}
}

func TestRulePodStartupP99Critical(t *testing.T) {
	// 90000 ms = 90 s → HIGH
	findings := rulePodStartupP99Critical.Evaluate([]runresult.Result{podStartupResult(5000, 90000)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityHigh {
		t.Errorf("expected 1 HIGH finding, got %v", findings)
	}
	findings = rulePodStartupP99Critical.Evaluate([]runresult.Result{podStartupResult(5000, 45000)})
	if len(findings) != 0 {
		t.Error("expected 0 findings below 60s threshold")
	}
}

func TestRulePodStartupP50Slow(t *testing.T) {
	// 15000 ms = 15 s → LOW
	findings := rulePodStartupP50Slow.Evaluate([]runresult.Result{podStartupResult(15000, 20000)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityLow {
		t.Errorf("expected 1 LOW finding, got %v", findings)
	}
	findings = rulePodStartupP50Slow.Evaluate([]runresult.Result{podStartupResult(5000, 20000)})
	if len(findings) != 0 {
		t.Error("expected 0 findings for fast median")
	}
}
