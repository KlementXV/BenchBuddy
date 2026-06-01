package network

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
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

	// React to pod create: server gets Running+IP so waitForPodIP succeeds;
	// client gets Succeeded so waitForPodCompletion returns immediately.
	cs.PrependReactor("create", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		p := a.(k8stesting.CreateAction).GetObject().(*corev1.Pod)
		if p.Labels["benchbuddy.io/role"] == "client" {
			p.Status = corev1.PodStatus{Phase: corev1.PodSucceeded}
		} else {
			p.Status = corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "10.0.0.5",
			}
		}
		return false, p, nil // let the default tracker store the patched pod
	})

	// The fake tracker has no built-in handler for "delete-collection",
	// so emulate it via the Tracker directly (calling cs.CoreV1() from inside
	// a reactor would deadlock on the Fake's mutex).
	cs.PrependReactor("delete-collection", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		dca := a.(k8stesting.DeleteCollectionAction)
		ns := dca.GetNamespace()
		sel := dca.GetListRestrictions().Labels
		gvr := corev1.SchemeGroupVersion.WithResource("pods")
		gvk := corev1.SchemeGroupVersion.WithKind("Pod")
		obj, err := cs.Tracker().List(gvr, gvk, ns)
		if err != nil {
			return true, nil, err
		}
		list := obj.(*corev1.PodList)
		for _, p := range list.Items {
			if sel == nil || sel.Matches(klabels.Set(p.Labels)) {
				_ = cs.Tracker().Delete(gvr, p.Namespace, p.Name)
			}
		}
		return true, nil, nil
	})

	b := New().WithLogReader(func(_ context.Context, _, _ string) ([]byte, error) {
		// Simulate a runner that emitted the marker.
		return []byte(`{"end":{"sum_sent":{"bits_per_second":9100000000}}}` + "\n" +
			"BENCHBUDDY_RESULT: {\"bandwidth_sent_gbps\":{\"value\":9.1,\"unit\":\"Gbps\"}}\n"), nil
	})
	b.podReadyTimeout = 1 * time.Second

	kc := kube.NewFakeClient(cs)
	cfg := config.RunConfig{
		Namespace: "ns",
		Benches:   config.BenchesConfig{Network: config.NetworkBenchConfig{Duration: 5 * time.Second}},
		Images:    config.ImageConfig{Registry: "reg", Runner: config.RunnerImage{Repository: "r", Tag: "v1", PullPolicy: "IfNotPresent"}},
	}
	pair := NodePair{ServerNode: "nodeA", ClientNode: "nodeB", Protocol: "tcp"}
	task := benches.Task{ID: "network/cross-node/tcp", Spec: pair}

	res, err := b.Run(context.Background(), kc, "run-x", task, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != runresult.StatusOK {
		t.Errorf("status: %s, errors: %v", res.Status, res.Errors)
	}
	if v, ok := res.Metrics["bandwidth_sent_gbps"]; !ok || v.Value != 9.1 {
		t.Errorf("metric missing or wrong: %+v", res.Metrics)
	}

	// Namespace should have no pods left after Run().
	pods, _ := cs.CoreV1().Pods("ns").List(context.Background(), metav1.ListOptions{})
	if len(pods.Items) != 0 {
		t.Errorf("expected per-task cleanup, found %d pods", len(pods.Items))
	}
}
