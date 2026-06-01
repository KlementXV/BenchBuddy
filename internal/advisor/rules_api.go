package advisor

import (
	"fmt"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func init() {
	RegisteredRules = append(RegisteredRules,
		ruleAPIListPodsSlow,
		ruleAPIListPodsCritical,
		ruleAPIGetPodSlow,
		ruleAPIWatchPodsSlow,
	)
}

var ruleAPIListPodsSlow = Rule{
	ID: "api-list-pods-slow",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "api/list-pods")
		if m == nil {
			return nil
		}
		p99 := metricVal(m, "latency_p99_ms")
		if p99 < 500 || p99 >= 2000 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityMedium,
			Category: "api",
			Title:    "API server LIST pods latency elevated",
			Detail:   fmt.Sprintf("list-pods p99 latency is %.0f ms (threshold: 500 ms). Controllers performing frequent LIST will slow down.", p99),
			Fix: []string{
				"Check etcd performance: etcdctl endpoint health --write-out=table",
				"Review etcd data size — large pod counts inflate LIST responses",
				"Use informers/watches instead of polling LIST in controllers",
			},
			SourceIDs: []string{"api/list-pods"},
		}}
	},
}

var ruleAPIListPodsCritical = Rule{
	ID: "api-list-pods-critical",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "api/list-pods")
		if m == nil {
			return nil
		}
		p99 := metricVal(m, "latency_p99_ms")
		if p99 < 2000 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityHigh,
			Category: "api",
			Title:    "API server LIST pods latency critical",
			Detail:   fmt.Sprintf("list-pods p99 latency is %.0f ms (threshold: 2000 ms). Controllers and kubectl will timeout.", p99),
			Fix: []string{
				"Check API server CPU/memory resource usage and limits",
				"Check etcd latency — slow etcd is the most common cause",
				"Enable API Priority and Fairness (APF) to protect critical requests",
			},
			SourceIDs: []string{"api/list-pods"},
		}}
	},
}

var ruleAPIGetPodSlow = Rule{
	ID: "api-get-pod-slow",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "api/get-pod")
		if m == nil {
			return nil
		}
		p99 := metricVal(m, "latency_p99_ms")
		if p99 < 200 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityLow,
			Category: "api",
			Title:    "API server GET pod latency elevated",
			Detail:   fmt.Sprintf("get-pod p99 latency is %.0f ms (threshold: 200 ms). Single-resource reads should be near-instant.", p99),
			Fix: []string{
				"Check API server cache warm-up time after restarts",
				"Review API server --watch-cache-sizes configuration",
			},
			SourceIDs: []string{"api/get-pod"},
		}}
	},
}

var ruleAPIWatchPodsSlow = Rule{
	ID: "api-watch-pods-slow",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "api/watch-pods")
		if m == nil {
			return nil
		}
		p99 := metricVal(m, "latency_p99_ms")
		if p99 < 500 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityMedium,
			Category: "api",
			Title:    "API server WATCH latency elevated",
			Detail:   fmt.Sprintf("watch-pods p99 latency is %.0f ms (threshold: 500 ms). Operators relying on watches will have slow reaction times.", p99),
			Fix: []string{
				"Reduce concurrent watchers — each adds load to the API server event fan-out",
				"Enable informer sharing across controllers to coalesce watch streams",
			},
			SourceIDs: []string{"api/watch-pods"},
		}}
	},
}
