package api

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/clementlevoux/benchbuddy/internal/benches"
	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/discover"
	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func TestPlan_ProducesTaskPerOperation(t *testing.T) {
	b := New()
	cfg := config.RunConfig{
		Benches: config.BenchesConfig{
			API: config.APIBenchConfig{
				Operations: []string{"list-pods", "list-namespaces", "get-pod"},
			},
		},
	}
	tasks, err := b.Plan(context.Background(), cfg, discover.Discovery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestRun_ListPods(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "ns"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p2", Namespace: "ns"}},
	)
	b := New()
	cfg := config.RunConfig{
		Namespace: "ns",
		Benches:   config.BenchesConfig{API: config.APIBenchConfig{Duration: 200 * time.Millisecond}},
	}
	res, err := b.Run(context.Background(), kube.NewFakeClient(cs), "run-1",
		benches.Task{ID: "api/list-pods", Spec: APISpec{Operation: "list-pods"}}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != runresult.StatusOK {
		t.Fatalf("status=%s errors=%v", res.Status, res.Errors)
	}
	if _, ok := res.Metrics["latency_p50_ms"]; !ok {
		t.Errorf("missing p50 metric; got %+v", res.Metrics)
	}
	if v, ok := res.Metrics["ops_per_sec"]; !ok || v.Value <= 0 {
		t.Errorf("missing/zero ops_per_sec: %+v", res.Metrics)
	}
}

func TestRun_UnknownOperation(t *testing.T) {
	cs := fake.NewSimpleClientset()
	b := New()
	res, _ := b.Run(context.Background(), kube.NewFakeClient(cs), "run-1",
		benches.Task{ID: "api/bogus", Spec: APISpec{Operation: "bogus"}}, config.RunConfig{Namespace: "ns"})
	if res.Status != runresult.StatusFailed {
		t.Errorf("expected Failed, got %s", res.Status)
	}
}

func TestConflictsWith_None(t *testing.T) {
	b := New()
	if b.ConflictsWith("network") || b.ConflictsWith("api") {
		t.Error("api bench should not conflict with anything (read-only API calls)")
	}
}
