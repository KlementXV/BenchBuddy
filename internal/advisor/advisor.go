package advisor

import "github.com/clementlevoux/benchbuddy/internal/runresult"

// Rule is a single diagnostic rule evaluated against bench results.
type Rule struct {
	ID       string
	Evaluate func(results []runresult.Result) []runresult.Finding
}

// RegisteredRules holds all rules appended via init() in rules_*.go files.
var RegisteredRules []Rule

// Evaluate runs all registered rules against results and returns the combined findings.
func Evaluate(results []runresult.Result) []runresult.Finding {
	var out []runresult.Finding
	for _, rule := range RegisteredRules {
		out = append(out, rule.Evaluate(results)...)
	}
	return out
}

// metricsFor returns the Metrics of the first OK or Partial result with taskID, or nil.
func metricsFor(results []runresult.Result, taskID string) map[string]runresult.Metric {
	for _, r := range results {
		if r.TaskID == taskID && (r.Status == runresult.StatusOK || r.Status == runresult.StatusPartial) {
			return r.Metrics
		}
	}
	return nil
}

// allByBench returns all OK or Partial results for a given bench name.
func allByBench(results []runresult.Result, benchName string) []runresult.Result {
	var out []runresult.Result
	for _, r := range results {
		if r.BenchName == benchName && (r.Status == runresult.StatusOK || r.Status == runresult.StatusPartial) {
			out = append(out, r)
		}
	}
	return out
}

// metricVal returns the float value of a named metric, or 0 if absent.
func metricVal(m map[string]runresult.Metric, key string) float64 {
	if m == nil {
		return 0
	}
	return m[key].Value
}
