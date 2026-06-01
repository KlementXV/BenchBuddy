package storage

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
	pvcBoundTimeout time.Duration
	podReadyTimeout time.Duration
}

func New() *Bench {
	return &Bench{
		pvcBoundTimeout: 60 * time.Second,
		podReadyTimeout: 120 * time.Second,
	}
}

func (b *Bench) WithLogReader(r LogReader) *Bench { b.logReader = r; return b }

func (b *Bench) Name() string                { return "storage" }
func (b *Bench) ConflictsWith(_ string) bool { return false }

type StorageTaskSpec struct {
	StorageClass string
	Pattern      string
	BlockSize    string
	Node         string
}

func (b *Bench) Plan(_ context.Context, cfg config.RunConfig, d discover.Discovery) ([]benches.Task, error) {
	if len(d.Nodes) == 0 {
		return nil, nil
	}
	excluded := func(name string) bool {
		for _, e := range cfg.Excludes.StorageClasses {
			if e == name {
				return true
			}
		}
		return false
	}
	included := func(name string) bool {
		if len(cfg.Includes.StorageClasses) == 0 {
			return true
		}
		for _, i := range cfg.Includes.StorageClasses {
			if i == name {
				return true
			}
		}
		return false
	}
	var tasks []benches.Task
	for _, sc := range d.StorageClasses {
		if !included(sc) || excluded(sc) {
			continue
		}
		for _, bs := range cfg.Benches.Storage.BlockSizes {
			for _, pat := range cfg.Benches.Storage.Patterns {
				tasks = append(tasks, benches.Task{
					ID:      "storage/" + sc + "/" + pat + "/" + bs,
					Subject: sc + " " + pat + " bs=" + bs,
					Spec:    StorageTaskSpec{StorageClass: sc, Pattern: pat, BlockSize: bs, Node: d.Nodes[0]},
				})
			}
		}
	}
	return tasks, nil
}

func (b *Bench) Run(ctx context.Context, kc *kube.Client, runID string, t benches.Task, cfg config.RunConfig) (runresult.Result, error) {
	start := time.Now()
	spec, ok := t.Spec.(StorageTaskSpec)
	if !ok {
		return runresult.Result{}, fmt.Errorf("invalid task spec: %T", t.Spec)
	}

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
		sel := labels.RunIDKey + "=" + runID + ",benchbuddy.io/bench=storage"
		_ = cs.CoreV1().Pods(ns).DeleteCollection(ccx, metav1.DeleteOptions{},
			metav1.ListOptions{LabelSelector: sel})
		_ = cs.CoreV1().PersistentVolumeClaims(ns).DeleteCollection(ccx, metav1.DeleteOptions{},
			metav1.ListOptions{LabelSelector: sel})
	}
	defer cleanup()

	// 1. Create PVC.
	pvc := PVC(runID, ns, spec.StorageClass, cfg.Benches.Storage.Size)
	if _, err := cs.CoreV1().PersistentVolumeClaims(ns).Create(ctx, pvc, metav1.CreateOptions{}); err != nil {
		return failed(t, "create pvc", err, start), nil
	}

	// 2. Wait for Bound.
	if err := waitForPVCBound(ctx, cs, ns, pvc.Name, b.pvcBoundTimeout); err != nil {
		return failed(t, "wait pvc bound", err, start), nil
	}

	// 3. Create Pod (mounts PVC).
	pod := FioPod(runID, ns, pvc.Name, spec, cfg)
	if _, err := cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return failed(t, "create fio pod", err, start), nil
	}

	// 4. Wait for completion.
	if err := waitForPodCompletion(ctx, cs, ns, pod.Name, cfg.Timeout); err != nil {
		return failed(t, "wait fio pod completion", err, start), nil
	}

	// 5. Read logs + parse marker.
	logs, err := b.readLogs(ctx, ns, pod.Name)
	if err != nil {
		return failed(t, "read fio logs", err, start), nil
	}
	metrics, raw, err := runresult.ParseMarker(strings.NewReader(string(logs)))
	if err != nil {
		return runresult.Result{
			BenchName: "storage", TaskID: t.ID, Subject: t.Subject,
			Status: runresult.StatusPartial, RawOutput: raw,
			Errors: []string{err.Error()}, Duration: time.Since(start),
		}, nil
	}
	return runresult.Result{
		BenchName: "storage", TaskID: t.ID, Subject: t.Subject,
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
	cs := kc.Clientset()
	sel := labels.RunIDKey + "=" + runID + ",benchbuddy.io/bench=storage"
	if err := cs.CoreV1().Pods("").DeleteCollection(ctx, metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: sel}); err != nil {
		return err
	}
	return cs.CoreV1().PersistentVolumeClaims("").DeleteCollection(ctx, metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: sel})
}

func failed(t benches.Task, stage string, err error, start time.Time) runresult.Result {
	return runresult.Result{
		BenchName: "storage", TaskID: t.ID, Subject: t.Subject,
		Status:   runresult.StatusFailed,
		Errors:   []string{stage + ": " + err.Error()},
		Duration: time.Since(start),
	}
}

func waitForPVCBound(ctx context.Context, cs kubernetes.Interface, ns, name string, timeout time.Duration) error {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return wait.PollUntilContextCancel(tctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		pvc, err := cs.CoreV1().PersistentVolumeClaims(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
}

func waitForPodCompletion(ctx context.Context, cs kubernetes.Interface, ns, name string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return wait.PollUntilContextCancel(tctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
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
