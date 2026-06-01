package advisor

import (
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func apiResult(taskID string, p99ms float64) runresult.Result {
	return runresult.Result{
		BenchName: "api",
		TaskID:    taskID,
		Status:    runresult.StatusOK,
		Metrics:   map[string]runresult.Metric{"latency_p99_ms": {Value: p99ms, Unit: "ms"}},
	}
}

func TestRuleAPIListPodsSlow(t *testing.T) {
	// between 500 and 2000 → MEDIUM
	findings := ruleAPIListPodsSlow.Evaluate([]runresult.Result{apiResult("api/list-pods", 1000)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityMedium {
		t.Errorf("expected 1 MEDIUM finding, got %v", findings)
	}
	// >= 2000 → handled by api-list-pods-critical
	findings = ruleAPIListPodsSlow.Evaluate([]runresult.Result{apiResult("api/list-pods", 3000)})
	if len(findings) != 0 {
		t.Error("p99 >= 2000 should be handled by api-list-pods-critical")
	}
	// < 500 → fine
	findings = ruleAPIListPodsSlow.Evaluate([]runresult.Result{apiResult("api/list-pods", 200)})
	if len(findings) != 0 {
		t.Error("expected 0 findings below threshold")
	}
}

func TestRuleAPIListPodsCritical(t *testing.T) {
	findings := ruleAPIListPodsCritical.Evaluate([]runresult.Result{apiResult("api/list-pods", 3000)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityHigh {
		t.Errorf("expected 1 HIGH finding, got %v", findings)
	}
	findings = ruleAPIListPodsCritical.Evaluate([]runresult.Result{apiResult("api/list-pods", 1000)})
	if len(findings) != 0 {
		t.Error("expected 0 findings below 2000ms threshold")
	}
}

func TestRuleAPIGetPodSlow(t *testing.T) {
	findings := ruleAPIGetPodSlow.Evaluate([]runresult.Result{apiResult("api/get-pod", 300)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityLow {
		t.Errorf("expected 1 LOW finding, got %v", findings)
	}
	findings = ruleAPIGetPodSlow.Evaluate([]runresult.Result{apiResult("api/get-pod", 100)})
	if len(findings) != 0 {
		t.Error("expected 0 findings below threshold")
	}
}

func TestRuleAPIWatchSlow(t *testing.T) {
	findings := ruleAPIWatchPodsSlow.Evaluate([]runresult.Result{apiResult("api/watch-pods", 700)})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityMedium {
		t.Errorf("expected 1 MEDIUM finding, got %v", findings)
	}
	findings = ruleAPIWatchPodsSlow.Evaluate([]runresult.Result{apiResult("api/watch-pods", 200)})
	if len(findings) != 0 {
		t.Error("expected 0 findings below 500ms threshold")
	}
}
