package advisor

import (
	"fmt"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func init() {
	RegisteredRules = append(RegisteredRules,
		ruleDNSP99High,
		ruleDNSP99Critical,
		ruleDNSErrors,
	)
}

var ruleDNSP99High = Rule{
	ID: "dns-p99-high",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "dns/in-cluster")
		if m == nil {
			return nil
		}
		p99 := metricVal(m, "latency_p99_ms")
		if p99 < 10 || p99 >= 50 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityMedium,
			Category: "dns",
			Title:    "CoreDNS p99 latency elevated",
			Detail:   fmt.Sprintf("DNS p99 resolution latency is %.1f ms (threshold: 10 ms). Service-discovery-heavy workloads will suffer.", p99),
			Fix: []string{
				"Scale CoreDNS replicas: kubectl scale deployment coredns -n kube-system --replicas=3",
				"Enable NodeLocal DNSCache to reduce round-trips to CoreDNS pods",
				"Check CoreDNS memory limits — cache eviction causes latency spikes",
			},
			Refs:      []string{"https://kubernetes.io/docs/tasks/administer-cluster/nodelocaldns/"},
			SourceIDs: []string{"dns/in-cluster"},
		}}
	},
}

var ruleDNSP99Critical = Rule{
	ID: "dns-p99-critical",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "dns/in-cluster")
		if m == nil {
			return nil
		}
		p99 := metricVal(m, "latency_p99_ms")
		if p99 < 50 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityHigh,
			Category: "dns",
			Title:    "CoreDNS p99 latency critical",
			Detail:   fmt.Sprintf("DNS p99 resolution latency is %.1f ms (threshold: 50 ms). Most microservice calls depend on DNS; widespread timeouts are expected.", p99),
			Fix: []string{
				"Immediately scale CoreDNS: kubectl scale deployment coredns -n kube-system --replicas=4",
				"Check CoreDNS pod OOM events: kubectl describe pod -n kube-system -l k8s-app=kube-dns",
				"Deploy NodeLocal DNSCache to reduce centralized CoreDNS pressure urgently",
			},
			Refs:      []string{"https://kubernetes.io/docs/tasks/administer-cluster/nodelocaldns/"},
			SourceIDs: []string{"dns/in-cluster"},
		}}
	},
}

var ruleDNSErrors = Rule{
	ID: "dns-errors",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "dns/in-cluster")
		if m == nil {
			return nil
		}
		errCount := metricVal(m, "error_count")
		if errCount <= 0 {
			return nil
		}
		samples := metricVal(m, "sample_count")
		pct := 0.0
		if samples > 0 {
			pct = errCount / samples * 100
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityLow,
			Category: "dns",
			Title:    "DNS resolution errors detected",
			Detail:   fmt.Sprintf("%.0f DNS errors in %.0f queries (%.1f%%). Even low error rates cause connection failures in microservice environments.", errCount, samples, pct),
			Fix: []string{
				"Check CoreDNS logs: kubectl logs -n kube-system -l k8s-app=kube-dns",
				"Verify upstream DNS servers reachable from CoreDNS pods",
				"Review ndots configuration — excessive search-domain queries overload CoreDNS",
			},
			SourceIDs: []string{"dns/in-cluster"},
		}}
	},
}
