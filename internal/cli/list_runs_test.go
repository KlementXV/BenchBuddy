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

func TestListRunsCmd_Empty(t *testing.T) {
	cs := fake.NewSimpleClientset()
	var out bytes.Buffer
	kc := kube.NewFakeClient(cs)
	f := listRunsFlags{namespace: "test"}

	if err := listRunsCmdWithClient(context.Background(), &out, kc, f); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no orphaned") {
		t.Errorf("expected 'no orphaned' message, got: %s", out.String())
	}
}

func TestListRunsCmd_ShowsRuns(t *testing.T) {
	cs := fake.NewSimpleClientset()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "bench-pod",
			Namespace:         "test",
			Labels:            map[string]string{labels.RunIDKey: "run-abc"},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Minute)),
		},
	}
	if _, err := cs.CoreV1().Pods("test").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	kc := kube.NewFakeClient(cs)
	f := listRunsFlags{namespace: "test"}

	if err := listRunsCmdWithClient(context.Background(), &out, kc, f); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "run-abc") {
		t.Errorf("expected 'run-abc' in output, got: %s", out.String())
	}
}
