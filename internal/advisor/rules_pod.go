package advisor

import (
	"fmt"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func init() {
	RegisteredRules = append(RegisteredRules,
		rulePodStartupP99Slow,
		rulePodStartupP99Critical,
		rulePodStartupP50Slow,
	)
}

var rulePodStartupP99Slow = Rule{
	ID: "pod-startup-p99-slow",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "pod/startup")
		if m == nil {
			return nil
		}
		p99ms := metricVal(m, "startup_p99_ms")
		if p99ms < 30000 || p99ms >= 60000 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityMedium,
			Category: "pod",
			Title:    "Pod startup p99 slow",
			Detail:   fmt.Sprintf("Pod startup p99 is %.0f s (threshold: 30 s). Autoscaling events will be slow to respond.", p99ms/1000),
			Fix: []string{
				"Enable image pre-pulling on nodes via a DaemonSet pulling critical images in advance",
				"Reduce image sizes: use distroless or scratch base images",
				"Check node CPU and memory pressure at scheduling time",
			},
			SourceIDs: []string{"pod/startup"},
		}}
	},
}

var rulePodStartupP99Critical = Rule{
	ID: "pod-startup-p99-critical",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "pod/startup")
		if m == nil {
			return nil
		}
		p99ms := metricVal(m, "startup_p99_ms")
		if p99ms < 60000 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityHigh,
			Category: "pod",
			Title:    "Pod startup p99 critically slow",
			Detail:   fmt.Sprintf("Pod startup p99 is %.0f s (threshold: 60 s). HPA scale-out and rolling updates will cause prolonged outages.", p99ms/1000),
			Fix: []string{
				"Check for image pull failures or registry throttling: kubectl describe pod -n <ns>",
				"Verify node resource availability: kubectl describe node | grep -A5 Allocated",
				"Check if the container runtime is healthy: crictl info",
			},
			SourceIDs: []string{"pod/startup"},
		}}
	},
}

var rulePodStartupP50Slow = Rule{
	ID: "pod-startup-p50-slow",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "pod/startup")
		if m == nil {
			return nil
		}
		p50ms := metricVal(m, "startup_p50_ms")
		if p50ms < 10000 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityLow,
			Category: "pod",
			Title:    "Pod startup median slow",
			Detail:   fmt.Sprintf("Pod startup median (p50) is %.0f s (threshold: 10 s). Consistent slowness suggests systemic scheduling or runtime overhead.", p50ms/1000),
			Fix: []string{
				"Review kubelet --image-pull-progress-deadline and --serialize-image-pulls settings",
				"Check scheduler throughput: kubectl get events --field-selector reason=Scheduled",
				"Profile node resource usage at startup time for CPU/memory contention",
			},
			SourceIDs: []string{"pod/startup"},
		}}
	},
}
