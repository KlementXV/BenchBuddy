package pod

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/clementlevoux/benchbuddy/internal/benches"
	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func TestRun_StartupSamples(t *testing.T) {
	cs := fake.NewSimpleClientset()
	// Mark pause pods as Ready quickly.
	cs.PrependReactor("create", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		p := a.(k8stesting.CreateAction).GetObject().(*corev1.Pod)
		p.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{
				Type: corev1.PodReady, Status: corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now()),
			}},
		}
		return false, p, nil
	})

	b := New()
	b.podReadyTimeout = 2 * time.Second
	cfg := config.RunConfig{
		Namespace: "ns",
		Benches: config.BenchesConfig{
			Pod: config.PodBenchConfig{SampleCount: 3, PauseImage: "registry.k8s.io/pause:3.9"},
		},
	}
	res, err := b.Run(context.Background(), kube.NewFakeClient(cs), "run-x",
		benches.Task{ID: "pod/startup", Subject: "startup", Spec: PodTaskSpec{Kind: KindStartup, Node: "n1"}}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != runresult.StatusOK {
		t.Fatalf("status=%s errors=%v", res.Status, res.Errors)
	}
	if v, ok := res.Metrics["sample_count"]; !ok || v.Value != 3 {
		t.Errorf("sample_count: %+v", res.Metrics)
	}
	if _, ok := res.Metrics["startup_p50_ms"]; !ok {
		t.Errorf("missing startup_p50_ms")
	}
}

func TestRun_CPU(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("create", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		p := a.(k8stesting.CreateAction).GetObject().(*corev1.Pod)
		p.Status = corev1.PodStatus{Phase: corev1.PodSucceeded}
		return false, p, nil
	})

	b := New().WithLogReader(func(_ context.Context, _, _ string) ([]byte, error) {
		return []byte("BENCHBUDDY_RESULT: {\"ops_per_sec\":{\"value\":2.5e8,\"unit\":\"ops/s\"}}\n"), nil
	})
	b.podReadyTimeout = 2 * time.Second

	cfg := config.RunConfig{
		Namespace: "ns",
		Timeout:   2 * time.Second,
		Benches: config.BenchesConfig{
			Pod: config.PodBenchConfig{CPUDuration: 1 * time.Second, PauseImage: "x"},
		},
		Images: config.ImageConfig{
			Registry: "reg",
			Runner:   config.RunnerImage{Repository: "r", Tag: "v1", PullPolicy: "IfNotPresent"},
		},
	}
	res, err := b.Run(context.Background(), kube.NewFakeClient(cs), "run-x",
		benches.Task{ID: "pod/cpu", Spec: PodTaskSpec{Kind: KindCPU, Node: "n1"}}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != runresult.StatusOK {
		t.Fatalf("status=%s errors=%v", res.Status, res.Errors)
	}
	if v, ok := res.Metrics["ops_per_sec"]; !ok || v.Value != 2.5e8 {
		t.Errorf("ops_per_sec metric: %+v", res.Metrics)
	}
}
