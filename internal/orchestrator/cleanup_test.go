package orchestrator

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/clementlevoux/benchbuddy/internal/labels"
)

func TestCleanupByLabel_DeletesPodsAndPVCs(t *testing.T) {
	runID := "abc"
	podMine := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mine",
			Namespace: "ns",
			Labels:    map[string]string{labels.RunIDKey: runID},
		},
	}
	podOther := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other",
			Namespace: "ns",
			Labels:    map[string]string{labels.RunIDKey: "other-run"},
		},
	}
	pvcMine := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mine-pvc",
			Namespace: "ns",
			Labels:    map[string]string{labels.RunIDKey: runID},
		},
	}
	cs := fake.NewSimpleClientset(podMine, podOther, pvcMine)

	if err := CleanupByLabel(context.Background(), cs, "ns", runID); err != nil {
		t.Fatal(err)
	}

	pods, _ := cs.CoreV1().Pods("ns").List(context.Background(), metav1.ListOptions{})
	if len(pods.Items) != 1 || pods.Items[0].Name != "other" {
		t.Errorf("expected only 'other' pod left, got %+v", pods.Items)
	}
	pvcs, _ := cs.CoreV1().PersistentVolumeClaims("ns").List(context.Background(), metav1.ListOptions{})
	if len(pvcs.Items) != 0 {
		t.Errorf("expected pvc deleted, got %+v", pvcs.Items)
	}
}
