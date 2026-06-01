package network

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

// LogReader fetches the full log of a finished pod. Production uses GetLogs;
// tests inject a fake.
type LogReader func(ctx context.Context, namespace, podName string) ([]byte, error)

// Bench implements benches.Bench for network pod-to-pod measurement.
type Bench struct {
	logReader       LogReader
	podReadyTimeout time.Duration
}

func New() *Bench {
	return &Bench{podReadyTimeout: 90 * time.Second}
}

// WithLogReader is a test seam.
func (b *Bench) WithLogReader(r LogReader) *Bench { b.logReader = r; return b }

func (b *Bench) Name() string { return "network" }

func (b *Bench) ConflictsWith(other string) bool {
	// All network measurements need exclusive cluster network use.
	return other == "network"
}

// NodePair is the Spec attached to each Task.
type NodePair struct {
	ServerNode string
	ClientNode string
	Protocol   string // "tcp" | "udp"
	SameNode   bool
}

func (b *Bench) Plan(_ context.Context, cfg config.RunConfig, d discover.Discovery) ([]benches.Task, error) {
	var tasks []benches.Task
	if len(d.Nodes) == 0 {
		return tasks, nil
	}

	for _, combo := range cfg.Benches.Network.Combinations {
		switch combo {
		case "same-node":
			node := d.Nodes[0]
			for _, proto := range cfg.Benches.Network.Protocols {
				tasks = append(tasks, benches.Task{
					ID:      "network/same-node/" + proto,
					Subject: node + " (" + proto + ", same-node)",
					Spec:    NodePair{ServerNode: node, ClientNode: node, Protocol: proto, SameNode: true},
				})
			}
		case "cross-node":
			if len(d.Nodes) < 2 {
				continue
			}
			server, client := d.Nodes[0], d.Nodes[1]
			for _, proto := range cfg.Benches.Network.Protocols {
				tasks = append(tasks, benches.Task{
					ID:      "network/cross-node/" + proto,
					Subject: server + " → " + client + " (" + proto + ")",
					Spec:    NodePair{ServerNode: server, ClientNode: client, Protocol: proto},
				})
			}
		}
	}
	return tasks, nil
}

func (b *Bench) Run(ctx context.Context, kc *kube.Client, runID string, t benches.Task, cfg config.RunConfig) (runresult.Result, error) {
	start := time.Now()
	pair, ok := t.Spec.(NodePair)
	if !ok {
		return runresult.Result{}, fmt.Errorf("invalid task spec: %T", t.Spec)
	}

	cs := kc.Clientset()
	ns := cfg.Namespace
	port := 5201

	// Per-task cleanup ALWAYS runs, even on panic/error.
	taskCleaned := false
	cleanup := func() {
		if taskCleaned {
			return
		}
		taskCleaned = true
		cleanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = cs.CoreV1().Pods(ns).DeleteCollection(cleanCtx, metav1.DeleteOptions{},
			metav1.ListOptions{LabelSelector: labels.SelectorForTask(runID, t.ID)})
	}
	defer cleanup()

	// 1. Create server pod.
	serverPod := ServerPod(runID, ns, pair, port, cfg)
	if _, err := cs.CoreV1().Pods(ns).Create(ctx, serverPod, metav1.CreateOptions{}); err != nil {
		return failed(t, "create server pod", err, start), nil
	}

	// 2. Wait for server to be running and expose a PodIP.
	serverIP, err := waitForPodIP(ctx, cs, ns, serverPod.Name, b.podReadyTimeout)
	if err != nil {
		return failed(t, "wait server ready", err, start), nil
	}

	// 3. Create client pod targeting serverIP.
	duration := cfg.Benches.Network.Duration.String()
	if duration == "0s" {
		duration = "10s"
	}
	clientPod := ClientPod(runID, ns, pair, serverIP, port, duration, cfg)
	if _, err := cs.CoreV1().Pods(ns).Create(ctx, clientPod, metav1.CreateOptions{}); err != nil {
		return failed(t, "create client pod", err, start), nil
	}

	// 4. Wait for client to finish (Succeeded or Failed).
	if err := waitForPodCompletion(ctx, cs, ns, clientPod.Name, cfg.Timeout); err != nil {
		return failed(t, "wait client completion", err, start), nil
	}

	// 5. Fetch client logs.
	logs, err := b.readLogs(ctx, ns, clientPod.Name)
	if err != nil {
		return failed(t, "read client logs", err, start), nil
	}

	// 6. Parse marker.
	metrics, raw, err := runresult.ParseMarker(strings.NewReader(string(logs)))
	if err != nil {
		return runresult.Result{
			BenchName: "network",
			TaskID:    t.ID,
			Subject:   t.Subject,
			Status:    runresult.StatusPartial,
			RawOutput: raw,
			Errors:    []string{err.Error()},
			Duration:  time.Since(start),
		}, nil
	}

	return runresult.Result{
		BenchName: "network",
		TaskID:    t.ID,
		Subject:   t.Subject,
		Status:    runresult.StatusOK,
		Metrics:   metrics,
		RawOutput: raw,
		Duration:  time.Since(start),
	}, nil
}

func (b *Bench) readLogs(ctx context.Context, ns, podName string) ([]byte, error) {
	if b.logReader != nil {
		return b.logReader(ctx, ns, podName)
	}
	return nil, errors.New("logReader not configured (use --kubeconfig in production wiring)")
}

func (b *Bench) Cleanup(ctx context.Context, kc *kube.Client, runID string) error {
	cs := kc.Clientset()
	return cs.CoreV1().Pods("").DeleteCollection(ctx, metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: labels.RunIDKey + "=" + runID + ",benchbuddy.io/bench=network"})
}

func failed(t benches.Task, stage string, err error, start time.Time) runresult.Result {
	return runresult.Result{
		BenchName: "network",
		TaskID:    t.ID,
		Subject:   t.Subject,
		Status:    runresult.StatusFailed,
		Errors:    []string{stage + ": " + err.Error()},
		Duration:  time.Since(start),
	}
}

func waitForPodIP(ctx context.Context, cs kubernetes.Interface, ns, name string, timeout time.Duration) (string, error) {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var ip string
	err := wait.PollUntilContextCancel(tctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		pod, err := cs.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if pod.Status.PodIP != "" && pod.Status.Phase != corev1.PodPending {
			ip = pod.Status.PodIP
			return true, nil
		}
		return false, nil
	})
	return ip, err
}

func waitForPodCompletion(ctx context.Context, cs kubernetes.Interface, ns, name string, timeout time.Duration) error {
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
