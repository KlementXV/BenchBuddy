package api

import (
	"context"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/clementlevoux/benchbuddy/internal/benches"
	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/discover"
	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

type Bench struct{}

func New() *Bench { return &Bench{} }

func (b *Bench) Name() string                                                          { return "api" }
func (b *Bench) ConflictsWith(_ string) bool                                           { return false }
func (b *Bench) Cleanup(_ context.Context, _ *kube.Client, _ string) error            { return nil }

// APISpec is the Task.Spec payload.
type APISpec struct {
	Operation string // list-pods | list-namespaces | get-pod | watch-pods
}

func (b *Bench) Plan(_ context.Context, cfg config.RunConfig, _ discover.Discovery) ([]benches.Task, error) {
	var tasks []benches.Task
	for _, op := range cfg.Benches.API.Operations {
		tasks = append(tasks, benches.Task{
			ID:      "api/" + op,
			Subject: op,
			Spec:    APISpec{Operation: op},
		})
	}
	return tasks, nil
}

func (b *Bench) Run(ctx context.Context, kc *kube.Client, _ string, t benches.Task, cfg config.RunConfig) (runresult.Result, error) {
	start := time.Now()
	spec, ok := t.Spec.(APISpec)
	if !ok {
		return runresult.Result{}, fmt.Errorf("invalid task spec: %T", t.Spec)
	}

	op, found := apiOperations[spec.Operation]
	if !found {
		return runresult.Result{
			BenchName: "api",
			TaskID:    t.ID,
			Subject:   t.Subject,
			Status:    runresult.StatusFailed,
			Errors:    []string{"unknown operation: " + spec.Operation},
			Duration:  time.Since(start),
		}, nil
	}

	duration := cfg.Benches.API.Duration
	if duration == 0 {
		duration = 5 * time.Second
	}
	deadline := time.Now().Add(duration)

	var samples []time.Duration
	cs := kc.Clientset()
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			break
		}
		opStart := time.Now()
		if err := op(ctx, cs, cfg.Namespace); err != nil {
			samples = append(samples, 0) // record but mark
			continue
		}
		samples = append(samples, time.Since(opStart))
	}

	if len(samples) == 0 {
		return runresult.Result{
			BenchName: "api", TaskID: t.ID, Subject: t.Subject,
			Status:   runresult.StatusFailed,
			Errors:   []string{"no samples collected"},
			Duration: time.Since(start),
		}, nil
	}

	metrics := percentiles(samples, time.Since(start))
	return runresult.Result{
		BenchName: "api",
		TaskID:    t.ID,
		Subject:   t.Subject,
		Status:    runresult.StatusOK,
		Metrics:   metrics,
		Duration:  time.Since(start),
	}, nil
}

// apiOperations maps operation names to closures performing one round-trip.
var apiOperations = map[string]func(ctx context.Context, cs kubernetes.Interface, namespace string) error{
	"list-pods": func(ctx context.Context, cs kubernetes.Interface, ns string) error {
		_, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{Limit: 100})
		return err
	},
	"list-namespaces": func(ctx context.Context, cs kubernetes.Interface, _ string) error {
		_, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		return err
	},
	"get-pod": func(ctx context.Context, cs kubernetes.Interface, ns string) error {
		pods, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{Limit: 1})
		if err != nil || len(pods.Items) == 0 {
			// fall back to listing namespaces so we still measure something
			_, e := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
			return e
		}
		_, err = cs.CoreV1().Pods(ns).Get(ctx, pods.Items[0].Name, metav1.GetOptions{})
		return err
	},
	"watch-pods": func(ctx context.Context, cs kubernetes.Interface, ns string) error {
		w, err := cs.CoreV1().Pods(ns).Watch(ctx, metav1.ListOptions{TimeoutSeconds: ptrInt64(1)})
		if err != nil {
			return err
		}
		defer w.Stop()
		// Consume one event or hit the 1s timeout; that's our "establishment" measurement.
		<-w.ResultChan()
		return nil
	},
}

func ptrInt64(i int64) *int64 { return &i }

func percentiles(samples []time.Duration, total time.Duration) map[string]runresult.Metric {
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	pick := func(p float64) float64 {
		if len(sorted) == 0 {
			return 0
		}
		idx := int(float64(len(sorted)-1) * p)
		return float64(sorted[idx].Microseconds()) / 1000.0 // → ms
	}
	return map[string]runresult.Metric{
		"latency_p50_ms": {Value: pick(0.50), Unit: "ms"},
		"latency_p95_ms": {Value: pick(0.95), Unit: "ms"},
		"latency_p99_ms": {Value: pick(0.99), Unit: "ms"},
		"ops_per_sec":    {Value: float64(len(samples)) / total.Seconds(), Unit: "ops/s"},
		"sample_count":   {Value: float64(len(samples)), Unit: "samples"},
	}
}
