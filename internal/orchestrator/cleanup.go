package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/clementlevoux/benchbuddy/internal/labels"
)

// CleanupByLabel deletes pods, PVCs, and configmaps in `namespace` matching
// run-id=`runID`. It uses propagation: foreground to wait for cascading deletes.
func CleanupByLabel(ctx context.Context, cs kubernetes.Interface, namespace, runID string) error {
	selector := labels.SelectorForRun(runID)
	listOpts := metav1.ListOptions{LabelSelector: selector}
	propagation := metav1.DeletePropagationForeground
	delOpts := metav1.DeleteOptions{PropagationPolicy: &propagation}

	pods, err := cs.CoreV1().Pods(namespace).List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	for _, p := range pods.Items {
		if err := cs.CoreV1().Pods(namespace).Delete(ctx, p.Name, delOpts); err != nil {
			return fmt.Errorf("delete pod %s: %w", p.Name, err)
		}
	}

	pvcs, err := cs.CoreV1().PersistentVolumeClaims(namespace).List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("list pvcs: %w", err)
	}
	for _, pvc := range pvcs.Items {
		if err := cs.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvc.Name, delOpts); err != nil {
			return fmt.Errorf("delete pvc %s: %w", pvc.Name, err)
		}
	}

	cms, err := cs.CoreV1().ConfigMaps(namespace).List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("list configmaps: %w", err)
	}
	for _, cm := range cms.Items {
		if err := cs.CoreV1().ConfigMaps(namespace).Delete(ctx, cm.Name, delOpts); err != nil {
			return fmt.Errorf("delete configmap %s: %w", cm.Name, err)
		}
	}

	return nil
}

// TrapSignals returns a context that cancels on SIGINT/SIGTERM, and a "drained"
// channel the caller can use to await final cleanup.
//
// Pattern:
//
//	runCtx, drained := TrapSignals(parentCtx)
//	defer drained()  // run-level cleanup is invoked by the caller, on a fresh ctx
//
// The returned context's cancellation signals "user pressed Ctrl+C, stop work".
// Cleanup itself must be performed by the caller on a NEW context with a
// generous timeout, not on runCtx.
func TrapSignals(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-ctx.Done():
		case <-sigCh:
			cancel()
		}
	}()
	return ctx, func() {
		signal.Stop(sigCh)
		cancel()
	}
}

// CleanupContext returns a fresh context for cleanup, decoupled from any
// upstream cancellation (so SIGINT does not abort cleanup itself).
func CleanupContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
