package advisor

import (
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func TestEvaluate_CallsAllRules(t *testing.T) {
	saved := RegisteredRules
	defer func() { RegisteredRules = saved }()

	callCount := 0
	RegisteredRules = []Rule{
		{ID: "r1", Evaluate: func(_ []runresult.Result) []runresult.Finding {
			callCount++
			return []runresult.Finding{{Severity: runresult.SeverityLow, Category: "test", Title: "t1"}}
		}},
		{ID: "r2", Evaluate: func(_ []runresult.Result) []runresult.Finding {
			callCount++
			return nil
		}},
	}

	findings := Evaluate(nil)
	if callCount != 2 {
		t.Errorf("expected 2 rule calls, got %d", callCount)
	}
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

func TestMetricsFor(t *testing.T) {
	results := []runresult.Result{
		{TaskID: "a", Status: runresult.StatusFailed, Metrics: map[string]runresult.Metric{"x": {Value: 1}}},
		{TaskID: "b", Status: runresult.StatusOK, Metrics: map[string]runresult.Metric{"x": {Value: 2}}},
		{TaskID: "c", Status: runresult.StatusPartial, Metrics: map[string]runresult.Metric{"x": {Value: 3}}},
	}
	if m := metricsFor(results, "a"); m != nil {
		t.Error("failed result should not be returned")
	}
	if m := metricsFor(results, "b"); m == nil || m["x"].Value != 2 {
		t.Error("expected metrics for task b")
	}
	if m := metricsFor(results, "c"); m == nil || m["x"].Value != 3 {
		t.Error("partial result should be returned")
	}
	if metricsFor(results, "z") != nil {
		t.Error("missing task should return nil")
	}
}

func TestAllByBench(t *testing.T) {
	results := []runresult.Result{
		{BenchName: "network", TaskID: "t1", Status: runresult.StatusOK},
		{BenchName: "storage", TaskID: "t2", Status: runresult.StatusOK},
		{BenchName: "network", TaskID: "t3", Status: runresult.StatusFailed},
	}
	got := allByBench(results, "network")
	if len(got) != 1 {
		t.Errorf("expected 1 OK network result, got %d", len(got))
	}
}

func TestMetricVal(t *testing.T) {
	m := map[string]runresult.Metric{"bw": {Value: 9.5}}
	if metricVal(m, "bw") != 9.5 {
		t.Error("expected 9.5")
	}
	if metricVal(m, "missing") != 0 {
		t.Error("expected 0 for missing key")
	}
	if metricVal(nil, "bw") != 0 {
		t.Error("expected 0 for nil map")
	}
}
