package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/labels"
)

type listRunsFlags struct {
	kubeconfig  string
	contextName string
	namespace   string
}

func newListRunsCommand() *cobra.Command {
	var f listRunsFlags
	cmd := &cobra.Command{
		Use:   "list-runs",
		Short: "List orphaned BenchBuddy runs in a namespace",
		RunE: func(cmd *cobra.Command, _ []string) error {
			kc, err := kube.NewClient(kube.Options{KubeconfigPath: f.kubeconfig, Context: f.contextName})
			if err != nil {
				return fmt.Errorf("kube: %w", err)
			}
			return listRunsCmdWithClient(cmd.Context(), cmd.OutOrStdout(), kc, f)
		},
	}
	cmd.Flags().StringVar(&f.kubeconfig, "kubeconfig", "", "path to kubeconfig")
	cmd.Flags().StringVar(&f.contextName, "context", "", "kubeconfig context to use")
	cmd.Flags().StringVarP(&f.namespace, "namespace", "n", "", "namespace to inspect (required)")
	_ = cmd.MarkFlagRequired("namespace")
	return cmd
}

type runSummary struct {
	runID     string
	createdAt time.Time
	objects   int
}

func listRunsCmdWithClient(parent context.Context, stdout io.Writer, kc *kube.Client, f listRunsFlags) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	runs := map[string]*runSummary{}
	addEntry := func(id string, ts time.Time) {
		if _, ok := runs[id]; !ok {
			runs[id] = &runSummary{runID: id, createdAt: ts}
		}
		if ts.Before(runs[id].createdAt) {
			runs[id].createdAt = ts
		}
		runs[id].objects++
	}

	pods, err := kc.Clientset().CoreV1().Pods(f.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorForAllRuns(),
	})
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	for _, p := range pods.Items {
		if id, ok := p.Labels[labels.RunIDKey]; ok {
			addEntry(id, p.CreationTimestamp.Time)
		}
	}

	pvcs, err := kc.Clientset().CoreV1().PersistentVolumeClaims(f.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorForAllRuns(),
	})
	if err != nil {
		return fmt.Errorf("list pvcs: %w", err)
	}
	for _, pvc := range pvcs.Items {
		if id, ok := pvc.Labels[labels.RunIDKey]; ok {
			addEntry(id, pvc.CreationTimestamp.Time)
		}
	}

	if len(runs) == 0 {
		fmt.Fprintln(stdout, "no orphaned runs found")
		return nil
	}

	entries := make([]*runSummary, 0, len(runs))
	for _, e := range runs {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].createdAt.Before(entries[j].createdAt)
	})

	fmt.Fprintf(stdout, "%-20s  %-30s  %s\n", "RUN-ID", "CREATED", "OBJECTS")
	for _, e := range entries {
		fmt.Fprintf(stdout, "%-20s  %-30s  %d\n", e.runID, e.createdAt.Format(time.RFC3339), e.objects)
	}
	return nil
}
