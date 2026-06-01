package pod

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/labels"
)

// PausePod creates one sample pod for the startup-latency measurement.
// The pod just uses the pause image — its only job is to be "Ready" quickly.
func PausePod(runID, namespace, node string, sampleIndex int, cfg config.RunConfig) *corev1.Pod {
	name := fmt.Sprintf("bb-pod-start-%s-%d", shortID(runID), sampleIndex)
	lbls := labels.ForTask(runID, "pod", "pod/startup")
	lbls["benchbuddy.io/sample-index"] = strconv.Itoa(sampleIndex)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			NodeSelector:                  map[string]string{"kubernetes.io/hostname": node},
			TerminationGracePeriodSeconds: ptrInt64(0),
			Containers: []corev1.Container{{
				Name:            "pause",
				Image:           cfg.Benches.Pod.PauseImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			}},
		},
	}
}

// CPUPod creates the runner pod that executes the single-threaded CPU benchmark.
func CPUPod(runID, namespace, node, duration string, cfg config.RunConfig) *corev1.Pod {
	name := fmt.Sprintf("bb-pod-cpu-%s", shortID(runID))
	lbls := labels.ForTask(runID, "pod", "pod/cpu")
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			NodeSelector:  map[string]string{"kubernetes.io/hostname": node},
			Containers: []corev1.Container{{
				Name:            "runner",
				Image:           cfg.Images.Runner.FullRef(cfg.Images.Registry),
				ImagePullPolicy: corev1.PullPolicy(cfg.Images.Runner.PullPolicy),
				Args:            []string{"--bench=pod-cpu", "--duration=" + duration},
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

func ptrInt64(i int64) *int64 { return &i }
