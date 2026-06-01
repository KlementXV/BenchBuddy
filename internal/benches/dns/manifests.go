package dns

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/labels"
)

func DNSPod(runID, namespace, nodeName string, queries []string, cfg config.RunConfig) *corev1.Pod {
	name := fmt.Sprintf("bb-dns-%s", shortID(runID))
	lbls := labels.ForTask(runID, "dns", "dns/in-cluster")

	args := []string{
		"--bench=dns",
		"--queries=" + strings.Join(queries, ","),
		fmt.Sprintf("--duration=%s", cfg.Benches.DNS.Duration),
		fmt.Sprintf("--qps=%d", cfg.Benches.DNS.QueriesPerSecond),
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			// HostNetwork=false: we want to measure cluster DNS, not host DNS.
			NodeSelector: map[string]string{"kubernetes.io/hostname": nodeName},
			Containers: []corev1.Container{{
				Name:            "runner",
				Image:           cfg.Images.Runner.FullRef(cfg.Images.Registry),
				ImagePullPolicy: corev1.PullPolicy(cfg.Images.Runner.PullPolicy),
				Args:            args,
			}},
			ImagePullSecrets: pullSecrets(cfg.Images.PullSecrets),
		},
	}
}

func pullSecrets(names []string) []corev1.LocalObjectReference {
	out := make([]corev1.LocalObjectReference, 0, len(names))
	for _, n := range names {
		out = append(out, corev1.LocalObjectReference{Name: n})
	}
	return out
}

func shortID(runID string) string {
	if len(runID) <= 8 {
		return runID
	}
	return runID[:8]
}
