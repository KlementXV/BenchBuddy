package advisor

import (
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func dnsResult(p99ms, errorCount, sampleCount float64) runresult.Result {
	return runresult.Result{
		BenchName: "dns",
		TaskID:    "dns/in-cluster",
		Status:    runresult.StatusOK,
		Metrics: map[string]runresult.Metric{
			"latency_p99_ms": {Value: p99ms, Unit: "ms"},
			"error_count":    {Value: errorCount, Unit: "errors"},
			"sample_count":   {Value: sampleCount, Unit: "samples"},
		},
	}
}

func TestRuleDNSP99High(t *testing.T) {
	// 20 ms: between 10 and 50 → MEDIUM
	findings := ruleDNSP99High.Evaluate([]runresult.Result{dnsResult(20, 0, 100)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityMedium {
		t.Errorf("expected 1 MEDIUM, got %v", findings)
	}
	// 60 ms: handled by dns-p99-critical, not this rule
	findings = ruleDNSP99High.Evaluate([]runresult.Result{dnsResult(60, 0, 100)})
	if len(findings) != 0 {
		t.Error("p99 >= 50ms should be handled by dns-p99-critical, not dns-p99-high")
	}
	// 5 ms: fine
	findings = ruleDNSP99High.Evaluate([]runresult.Result{dnsResult(5, 0, 100)})
	if len(findings) != 0 {
		t.Error("expected 0 findings below threshold")
	}
}

func TestRuleDNSP99Critical(t *testing.T) {
	findings := ruleDNSP99Critical.Evaluate([]runresult.Result{dnsResult(60, 0, 100)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityHigh {
		t.Errorf("expected 1 HIGH finding, got %v", findings)
	}
	findings = ruleDNSP99Critical.Evaluate([]runresult.Result{dnsResult(20, 0, 100)})
	if len(findings) != 0 {
		t.Error("expected 0 findings below 50ms threshold")
	}
}

func TestRuleDNSErrors(t *testing.T) {
	findings := ruleDNSErrors.Evaluate([]runresult.Result{dnsResult(5, 3, 100)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityLow {
		t.Errorf("expected 1 LOW finding, got %v", findings)
	}
	findings = ruleDNSErrors.Evaluate([]runresult.Result{dnsResult(5, 0, 100)})
	if len(findings) != 0 {
		t.Error("expected 0 findings with zero errors")
	}
}
