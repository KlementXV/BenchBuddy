package dns

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/clementlevoux/benchbuddy/internal/benches"
	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func TestRun_HappyPath(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("create", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		p := a.(k8stesting.CreateAction).GetObject().(*corev1.Pod)
		p.Status = corev1.PodStatus{Phase: corev1.PodSucceeded}
		return false, p, nil
	})

	b := New().WithLogReader(func(_ context.Context, _, _ string) ([]byte, error) {
		return []byte("dnsperf output...\n" +
			"BENCHBUDDY_RESULT: {\"latency_p50_ms\":{\"value\":1.2,\"unit\":\"ms\"}," +
			"\"queries_per_sec\":{\"value\":48.5,\"unit\":\"qps\"}}\n"), nil
	})
	b.podReadyTimeout = 1 * time.Second

	cfg := config.RunConfig{
		Namespace: "ns",
		Timeout:   1 * time.Second,
		Benches: config.BenchesConfig{
			DNS: config.DNSBenchConfig{Duration: 200 * time.Millisecond, QueriesPerSecond: 10},
		},
		Images: config.ImageConfig{
			Registry: "reg",
			Runner:   config.RunnerImage{Repository: "r", Tag: "v1", PullPolicy: "IfNotPresent"},
		},
	}
	task := benches.Task{
		ID: "dns/in-cluster",
		Spec: DNSSpec{
			Node:    "n1",
			Queries: []string{"kubernetes.default.svc.cluster.local"},
		},
	}
	res, err := b.Run(context.Background(), kube.NewFakeClient(cs), "run-x", task, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != runresult.StatusOK {
		t.Fatalf("status=%s errors=%v", res.Status, res.Errors)
	}
	if v, ok := res.Metrics["latency_p50_ms"]; !ok || v.Value != 1.2 {
		t.Errorf("missing/wrong p50 metric: %+v", res.Metrics)
	}
}
