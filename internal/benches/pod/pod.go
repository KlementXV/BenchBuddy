package pod

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/clementlevoux/benchbuddy/internal/benches"
	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/discover"
	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/labels"
	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

type LogReader func(ctx context.Context, namespace, podName string) ([]byte, error)

type Bench struct {
	logReader       LogReader
	podReadyTimeout time.Duration
}

func New() *Bench { return &Bench{podReadyTimeout: 90 * time.Second} }

func (b *Bench) WithLogReader(r LogReader) *Bench { b.logReader = r; return b }

func (b *Bench) Name() string                { return "pod" }
func (b *Bench) ConflictsWith(_ string) bool { return false }

type PodTaskKind string

const (
	KindStartup PodTaskKind = "startup"
	KindCPU     PodTaskKind = "cpu"
)

type PodTaskSpec struct {
	Kind PodTaskKind
	Node string
}

func (b *Bench) Plan(_ context.Context, _ config.RunConfig, d discover.Discovery) ([]benches.Task, error) {
	if len(d.Nodes) == 0 {
		return nil, nil
	}
	node := d.Nodes[0]
	return []benches.Task{
		{ID: "pod/startup", Subject: "pod startup latency (" + node + ")",
			Spec: PodTaskSpec{Kind: KindStartup, Node: node}},
		{ID: "pod/cpu", Subject: "single-thread CPU ops/sec (" + node + ")",
			Spec: PodTaskSpec{Kind: KindCPU, Node: node}},
	}, nil
}

func (b *Bench) Run(ctx context.Context, kc *kube.Client, runID string, t benches.Task, cfg config.RunConfig) (runresult.Result, error) {
	spec, ok := t.Spec.(PodTaskSpec)
	if !ok {
		return runresult.Result{}, fmt.Errorf("invalid task spec: %T", t.Spec)
	}
	switch spec.Kind {
	case KindStartup:
		return b.runStartup(ctx, kc, runID, t, cfg, spec)
	case KindCPU:
		return b.runCPU(ctx, kc, runID, t, cfg, spec)
	}
	return runresult.Result{}, fmt.Errorf("unknown task kind: %s", spec.Kind)
}

func (b *Bench) runStartup(ctx context.Context, kc *kube.Client, runID string, t benches.Task, cfg config.RunConfig, spec PodTaskSpec) (runresult.Result, error) {
	start := time.Now()
	cs := kc.Clientset()
	ns := cfg.Namespace

	cleaned := false
	cleanup := func() {
		if cleaned {
			return
		}
		cleaned = true
		ccx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		_ = cs.CoreV1().Pods(ns).DeleteCollection(ccx, metav1.DeleteOptions{},
			metav1.ListOptions{LabelSelector: labels.RunIDKey + "=" + runID + ",benchbuddy.io/task=pod_startup"})
	}
	defer cleanup()

	n := cfg.Benches.Pod.SampleCount
	if n <= 0 {
		n = 5
	}

	samples := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		pod := PausePod(runID, ns, spec.Node, i, cfg)
		created := time.Now()
		if _, err := cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
			return failed(t, fmt.Sprintf("create sample %d", i), err, start), nil
		}
		if err := waitForPodReady(ctx, cs, ns, pod.Name, b.podReadyTimeout); err != nil {
			return failed(t, fmt.Sprintf("wait sample %d ready", i), err, start), nil
		}
		samples = append(samples, time.Since(created))
		// Delete immediately so we don't fill the node with idle pods.
		_ = cs.CoreV1().Pods(ns).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	}

	metrics := startupPercentiles(samples)
	return runresult.Result{
		BenchName: "pod", TaskID: t.ID, Subject: t.Subject,
		Status: runresult.StatusOK, Metrics: metrics, Duration: time.Since(start),
	}, nil
}

func (b *Bench) runCPU(ctx context.Context, kc *kube.Client, runID string, t benches.Task, cfg config.RunConfig, spec PodTaskSpec) (runresult.Result, error) {
	start := time.Now()
	cs := kc.Clientset()
	ns := cfg.Namespace

	cleaned := false
	cleanup := func() {
		if cleaned {
			return
		}
		cleaned = true
		ccx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = cs.CoreV1().Pods(ns).DeleteCollection(ccx, metav1.DeleteOptions{},
			metav1.ListOptions{LabelSelector: labels.RunIDKey + "=" + runID + ",benchbuddy.io/task=pod_cpu"})
	}
	defer cleanup()

	dur := cfg.Benches.Pod.CPUDuration
	if dur <= 0 {
		dur = 10 * time.Second
	}
	pod := CPUPod(runID, ns, spec.Node, dur.String(), cfg)
	if _, err := cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return failed(t, "create cpu pod", err, start), nil
	}
	if err := waitForPodCompletion(ctx, cs, ns, pod.Name, cfg.Timeout); err != nil {
		return failed(t, "wait cpu pod completion", err, start), nil
	}
	logs, err := b.readLogs(ctx, ns, pod.Name)
	if err != nil {
		return failed(t, "read cpu logs", err, start), nil
	}
	metrics, raw, err := runresult.ParseMarker(strings.NewReader(string(logs)))
	if err != nil {
		return runresult.Result{
			BenchName: "pod", TaskID: t.ID, Subject: t.Subject,
			Status: runresult.StatusPartial, RawOutput: raw,
			Errors: []string{err.Error()}, Duration: time.Since(start),
		}, nil
	}
	return runresult.Result{
		BenchName: "pod", TaskID: t.ID, Subject: t.Subject,
		Status: runresult.StatusOK, Metrics: metrics, RawOutput: raw,
		Duration: time.Since(start),
	}, nil
}

func (b *Bench) readLogs(ctx context.Context, ns, podName string) ([]byte, error) {
	if b.logReader != nil {
		return b.logReader(ctx, ns, podName)
	}
	return nil, errors.New("logReader not configured")
}

func (b *Bench) Cleanup(ctx context.Context, kc *kube.Client, runID string) error {
	return kc.Clientset().CoreV1().Pods("").DeleteCollection(ctx, metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: labels.RunIDKey + "=" + runID + ",benchbuddy.io/bench=pod"})
}

func failed(t benches.Task, stage string, err error, start time.Time) runresult.Result {
	return runresult.Result{
		BenchName: "pod", TaskID: t.ID, Subject: t.Subject,
		Status:   runresult.StatusFailed,
		Errors:   []string{stage + ": " + err.Error()},
		Duration: time.Since(start),
	}
}

func startupPercentiles(samples []time.Duration) map[string]runresult.Metric {
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	pick := func(p float64) float64 {
		if len(samples) == 0 {
			return 0
		}
		idx := int(float64(len(samples)-1) * p)
		return float64(samples[idx].Milliseconds())
	}
	return map[string]runresult.Metric{
		"startup_p50_ms": {Value: pick(0.50), Unit: "ms"},
		"startup_p95_ms": {Value: pick(0.95), Unit: "ms"},
		"startup_p99_ms": {Value: pick(0.99), Unit: "ms"},
		"sample_count":   {Value: float64(len(samples)), Unit: "samples"},
	}
}

func waitForPodReady(ctx context.Context, cs kubernetes.Interface, ns, name string, timeout time.Duration) error {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return wait.PollUntilContextCancel(tctx, 250*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		pod, err := cs.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		for _, c := range pod.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
}

func waitForPodCompletion(ctx context.Context, cs kubernetes.Interface, ns, name string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return wait.PollUntilContextCancel(tctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		pod, err := cs.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed:
			return true, nil
		}
		return false, nil
	})
}
