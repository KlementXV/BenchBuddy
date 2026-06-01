package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/labels"
)

func TestCleanCmd_SpecificRunID(t *testing.T) {
	cs := fake.NewSimpleClientset()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bench-pod",
			Namespace: "test",
			Labels:    labels.ForTask("abc", "network", "network/same-node/tcp"),
		},
	}
	if _, err := cs.CoreV1().Pods("test").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	f := cleanFlags{namespace: "test", runID: "abc"}
	kc := kube.NewFakeClient(cs)

	if err := cleanCmdWithClient(context.Background(), &out, kc, f); err != nil {
		t.Fatalf("cleanCmd: %v", err)
	}
	if !strings.Contains(out.String(), "abc") {
		t.Errorf("expected 'abc' in output, got: %s", out.String())
	}

	pods, err := cs.CoreV1().Pods("test").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 0 {
		t.Errorf("expected 0 pods after clean, got %d", len(pods.Items))
	}
}

func TestCleanCmd_OlderThan(t *testing.T) {
	cs := fake.NewSimpleClientset()
	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "old-pod",
			Namespace:         "test",
			Labels:            map[string]string{labels.RunIDKey: "old-run"},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
	}
	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "new-pod",
			Namespace:         "test",
			Labels:            map[string]string{labels.RunIDKey: "new-run"},
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
	}
	for _, p := range []*corev1.Pod{oldPod, newPod} {
		if _, err := cs.CoreV1().Pods("test").Create(context.Background(), p, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	var out bytes.Buffer
	f := cleanFlags{namespace: "test", olderThan: 1 * time.Hour}
	kc := kube.NewFakeClient(cs)

	if err := cleanCmdWithClient(context.Background(), &out, kc, f); err != nil {
		t.Fatalf("cleanCmd: %v", err)
	}
	if !strings.Contains(out.String(), "old-run") {
		t.Errorf("expected 'old-run' cleaned, got: %s", out.String())
	}

	pods, _ := cs.CoreV1().Pods("test").List(context.Background(), metav1.ListOptions{})
	if len(pods.Items) != 1 || pods.Items[0].Name != "new-pod" {
		t.Errorf("expected only new-pod to remain, got %d pods", len(pods.Items))
	}
}

func TestCleanCmd_NoOrphans(t *testing.T) {
	cs := fake.NewSimpleClientset()
	var out bytes.Buffer
	f := cleanFlags{namespace: "test"}
	kc := kube.NewFakeClient(cs)

	if err := cleanCmdWithClient(context.Background(), &out, kc, f); err != nil {
		t.Fatalf("cleanCmd: %v", err)
	}
	if !strings.Contains(out.String(), "no orphaned") {
		t.Errorf("expected 'no orphaned' message, got: %s", out.String())
	}
}
