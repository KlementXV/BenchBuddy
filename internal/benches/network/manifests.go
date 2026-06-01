package network

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/labels"
)

func ServerPod(runID, namespace string, pair NodePair, port int, cfg config.RunConfig) *corev1.Pod {
	name := fmt.Sprintf("bb-net-srv-%s-%s", labelSafe(pair.Protocol), shortID(runID))
	return basePod(name, namespace, runID, "server", pair, cfg, []string{
		"--bench=network",
		"--role=server",
		fmt.Sprintf("--port=%d", port),
	}, pair.ServerNode)
}

func ClientPod(runID, namespace string, pair NodePair, targetIP string, port int, duration string, cfg config.RunConfig) *corev1.Pod {
	name := fmt.Sprintf("bb-net-cli-%s-%s", labelSafe(pair.Protocol), shortID(runID))
	return basePod(name, namespace, runID, "client", pair, cfg, []string{
		"--bench=network",
		"--role=client",
		fmt.Sprintf("--target=%s", targetIP),
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--protocol=%s", pair.Protocol),
		fmt.Sprintf("--duration=%s", duration),
	}, pair.ClientNode)
}

func basePod(name, ns, runID, role string, pair NodePair, cfg config.RunConfig, args []string, nodeName string) *corev1.Pod {
	lbls := labels.ForTask(runID, "network", "network/"+combinationFor(pair)+"/"+pair.Protocol)
	lbls["benchbuddy.io/role"] = role
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    lbls,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			HostNetwork:   true,
			NodeSelector:  map[string]string{"kubernetes.io/hostname": nodeName},
			Containers: []corev1.Container{{
				Name:            "runner",
				Image:           cfg.Images.Runner.FullRef(cfg.Images.Registry),
				ImagePullPolicy: corev1.PullPolicy(cfg.Images.Runner.PullPolicy),
				Args:            args,
			}},
			ImagePullSecrets: pullSecrets(cfg.Images.PullSecrets),
			Tolerations:      []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
		},
	}
}

func combinationFor(p NodePair) string {
	if p.SameNode {
		return "same-node"
	}
	return "cross-node"
}

func pullSecrets(names []string) []corev1.LocalObjectReference {
	out := make([]corev1.LocalObjectReference, 0, len(names))
	for _, n := range names {
		out = append(out, corev1.LocalObjectReference{Name: n})
	}
	return out
}

func labelSafe(s string) string {
	if s == "" {
		return "x"
	}
	return s
}

func shortID(runID string) string {
	if len(runID) <= 8 {
		return runID
	}
	return runID[:8]
}
