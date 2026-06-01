package storage

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

	// React to PVC create: mark as Bound immediately.
	cs.PrependReactor("create", "persistentvolumeclaims", func(a k8stesting.Action) (bool, runtime.Object, error) {
		pvc := a.(k8stesting.CreateAction).GetObject().(*corev1.PersistentVolumeClaim)
		pvc.Status = corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}
		return false, pvc, nil
	})

	// React to pod create: mark as Succeeded immediately.
	cs.PrependReactor("create", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		p := a.(k8stesting.CreateAction).GetObject().(*corev1.Pod)
		p.Status = corev1.PodStatus{Phase: corev1.PodSucceeded}
		return false, p, nil
	})

	b := New().WithLogReader(func(_ context.Context, _, _ string) ([]byte, error) {
		return []byte("fio output...\n" +
			"BENCHBUDDY_RESULT: {\"iops\":{\"value\":12450,\"unit\":\"iops\"}," +
			"\"latency_p99_us\":{\"value\":1820,\"unit\":\"us\"}}\n"), nil
	})
	b.podReadyTimeout = 1 * time.Second

	cfg := config.RunConfig{
		Namespace: "ns",
		Timeout:   1 * time.Second,
		Benches: config.BenchesConfig{
			Storage: config.StorageBenchConfig{
				Duration: 1 * time.Second, Size: "1Gi",
				BlockSizes: []string{"4k"}, Patterns: []string{"randread"},
			},
		},
		Images: config.ImageConfig{
			Registry: "reg",
			Runner:   config.RunnerImage{Repository: "r", Tag: "v1", PullPolicy: "IfNotPresent"},
		},
	}
	task := benches.Task{
		ID: "storage/gp3/randread/4k",
		Spec: StorageTaskSpec{
			StorageClass: "gp3", Pattern: "randread", BlockSize: "4k", Node: "n1",
		},
	}
	res, err := b.Run(context.Background(), kube.NewFakeClient(cs), "run-x", task, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != runresult.StatusOK {
		t.Fatalf("status=%s errors=%v", res.Status, res.Errors)
	}
	if v, ok := res.Metrics["iops"]; !ok || v.Value != 12450 {
		t.Errorf("iops metric: %+v", res.Metrics)
	}
}
