package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/labels"
	"github.com/clementlevoux/benchbuddy/internal/orchestrator"
)

type cleanFlags struct {
	kubeconfig  string
	contextName string
	namespace   string
	runID       string
	olderThan   time.Duration
}

func newCleanCommand() *cobra.Command {
	var f cleanFlags
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Delete orphaned BenchBuddy resources from a namespace",
		RunE: func(cmd *cobra.Command, _ []string) error {
			kc, err := kube.NewClient(kube.Options{KubeconfigPath: f.kubeconfig, Context: f.contextName})
			if err != nil {
				return fmt.Errorf("kube: %w", err)
			}
			return cleanCmdWithClient(cmd.Context(), cmd.OutOrStdout(), kc, f)
		},
	}
	cmd.Flags().StringVar(&f.kubeconfig, "kubeconfig", "", "path to kubeconfig")
	cmd.Flags().StringVar(&f.contextName, "context", "", "kubeconfig context to use")
	cmd.Flags().StringVarP(&f.namespace, "namespace", "n", "", "namespace to clean (required)")
	cmd.Flags().StringVar(&f.runID, "run-id", "", "delete only this specific run")
	cmd.Flags().DurationVar(&f.olderThan, "older-than", 0, "delete runs older than this duration (e.g. 1h)")
	_ = cmd.MarkFlagRequired("namespace")
	return cmd
}

func cleanCmdWithClient(parent context.Context, stdout io.Writer, kc *kube.Client, f cleanFlags) error {
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()

	if f.runID != "" {
		if err := orchestrator.CleanupByLabel(ctx, kc.Clientset(), f.namespace, f.runID); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "cleaned run %s\n", f.runID)
		return nil
	}

	runIDs, err := collectRunIDs(ctx, kc, f.namespace, f.olderThan)
	if err != nil {
		return err
	}
	if len(runIDs) == 0 {
		fmt.Fprintln(stdout, "no orphaned runs found")
		return nil
	}
	for id := range runIDs {
		if err := orchestrator.CleanupByLabel(ctx, kc.Clientset(), f.namespace, id); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "cleaned run %s\n", id)
	}
	return nil
}

func collectRunIDs(ctx context.Context, kc *kube.Client, namespace string, olderThan time.Duration) (map[string]struct{}, error) {
	cutoff := time.Now().Add(-olderThan)
	runIDs := map[string]struct{}{}

	addID := func(id string, ts time.Time) {
		if olderThan > 0 && ts.After(cutoff) {
			return
		}
		runIDs[id] = struct{}{}
	}

	pods, err := kc.Clientset().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorForAllRuns(),
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	for _, p := range pods.Items {
		if id, ok := p.Labels[labels.RunIDKey]; ok {
			addID(id, p.CreationTimestamp.Time)
		}
	}

	pvcs, err := kc.Clientset().CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorForAllRuns(),
	})
	if err != nil {
		return nil, fmt.Errorf("list pvcs: %w", err)
	}
	for _, pvc := range pvcs.Items {
		if id, ok := pvc.Labels[labels.RunIDKey]; ok {
			addID(id, pvc.CreationTimestamp.Time)
		}
	}

	return runIDs, nil
}
