package dns

import (
	"strings"
	"testing"
	"time"

	"github.com/clementlevoux/benchbuddy/internal/config"
)

func TestDNSPodSpec(t *testing.T) {
	cfg := config.RunConfig{
		Images: config.ImageConfig{
			Registry: "reg",
			Runner:   config.RunnerImage{Repository: "runner", Tag: "v1", PullPolicy: "IfNotPresent"},
		},
		Benches: config.BenchesConfig{
			DNS: config.DNSBenchConfig{
				Duration:         5 * time.Second,
				QueriesPerSecond: 25,
			},
		},
	}
	pod := DNSPod("run-1", "ns", "n1",
		[]string{"kubernetes.default.svc.cluster.local"}, cfg)
	if pod.Spec.NodeSelector["kubernetes.io/hostname"] != "n1" {
		t.Errorf("node selector: %v", pod.Spec.NodeSelector)
	}
	if pod.Spec.HostNetwork {
		t.Error("dns pod should NOT use hostNetwork (we want to measure cluster DNS)")
	}
	found := false
	for _, a := range pod.Spec.Containers[0].Args {
		if strings.HasPrefix(a, "--queries=") && strings.Contains(a, "kubernetes.default.svc.cluster.local") {
			found = true
		}
	}
	if !found {
		t.Errorf("missing --queries with target FQDN, args=%v", pod.Spec.Containers[0].Args)
	}
}
