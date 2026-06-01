package dns

import (
	"context"
	"errors"
	"fmt"
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
func (b *Bench) Name() string                     { return "dns" }
func (b *Bench) ConflictsWith(other string) bool  { return other == "dns" } // CoreDNS is shared

// DNSSpec is the Task.Spec payload.
type DNSSpec struct {
	Node    string
	Queries []string
}

func (b *Bench) Plan(_ context.Context, cfg config.RunConfig, d discover.Discovery) ([]benches.Task, error) {
	if len(d.Nodes) == 0 {
		return nil, nil
	}
	queries := buildQueries(cfg.Benches.DNS.Targets, cfg.Namespace)
	if len(queries) == 0 {
		return nil, nil
	}
	return []benches.Task{{
		ID:      "dns/in-cluster",
		Subject: "in-cluster resolution",
		Spec:    DNSSpec{Node: d.Nodes[0], Queries: queries},
	}}, nil
}

// buildQueries expands target tags ("in-cluster") into concrete FQDNs.
func buildQueries(targets []string, _ string) []string {
	var out []string
	for _, t := range targets {
		switch t {
		case "in-cluster":
			out = append(out,
				"kubernetes.default.svc.cluster.local",
				"kube-dns.kube-system.svc.cluster.local",
			)
		default:
			out = append(out, t) // pass user-supplied FQDN through
		}
	}
	return out
}

func (b *Bench) Run(ctx context.Context, kc *kube.Client, runID string, t benches.Task, cfg config.RunConfig) (runresult.Result, error) {
	start := time.Now()
	spec, ok := t.Spec.(DNSSpec)
	if !ok {
		return runresult.Result{}, fmt.Errorf("invalid task spec: %T", t.Spec)
	}

	cs := kc.Clientset()
	ns := cfg.Namespace

	// Per-task cleanup (panic-safe).
	cleaned := false
	cleanup := func() {
		if cleaned {
			return
		}
		cleaned = true
		ccx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = cs.CoreV1().Pods(ns).DeleteCollection(ccx, metav1.DeleteOptions{},
			metav1.ListOptions{LabelSelector: labels.SelectorForTask(runID, t.ID)})
	}
	defer cleanup()

	pod := DNSPod(runID, ns, spec.Node, spec.Queries, cfg)
	if _, err := cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return failed(t, "create dns pod", err, start), nil
	}
	if err := waitForPodCompletion(ctx, cs, ns, pod.Name, cfg.Timeout); err != nil {
		return failed(t, "wait dns pod", err, start), nil
	}
	logs, err := b.readLogs(ctx, ns, pod.Name)
	if err != nil {
		return failed(t, "read dns logs", err, start), nil
	}
	metrics, raw, err := runresult.ParseMarker(strings.NewReader(string(logs)))
	if err != nil {
		return runresult.Result{
			BenchName: "dns", TaskID: t.ID, Subject: t.Subject,
			Status: runresult.StatusPartial, RawOutput: raw,
			Errors: []string{err.Error()}, Duration: time.Since(start),
		}, nil
	}
	return runresult.Result{
		BenchName: "dns", TaskID: t.ID, Subject: t.Subject,
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
		metav1.ListOptions{LabelSelector: labels.RunIDKey + "=" + runID + ",benchbuddy.io/bench=dns"})
}

func failed(t benches.Task, stage string, err error, start time.Time) runresult.Result {
	return runresult.Result{
		BenchName: "dns", TaskID: t.ID, Subject: t.Subject,
		Status: runresult.StatusFailed,
		Errors: []string{stage + ": " + err.Error()},
		Duration: time.Since(start),
	}
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
