package network

import (
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/config"
)

func TestServerPodSpec_HasNodeAffinityAndHostNetwork(t *testing.T) {
	cfg := config.RunConfig{
		Images: config.ImageConfig{
			Registry: "reg",
			Runner:   config.RunnerImage{Repository: "runner", Tag: "v1", PullPolicy: "IfNotPresent"},
		},
	}
	pod := ServerPod("run-1", "ns", NodePair{ServerNode: "nodeA", Protocol: "tcp"}, 5201, cfg)
	if pod.Spec.NodeSelector["kubernetes.io/hostname"] != "nodeA" {
		t.Errorf("expected node selector for nodeA, got %v", pod.Spec.NodeSelector)
	}
	if !pod.Spec.HostNetwork {
		t.Error("server pod should use hostNetwork (permissive mode)")
	}
	if pod.Labels["benchbuddy.io/run-id"] != "run-1" {
		t.Errorf("missing run-id label: %v", pod.Labels)
	}
	if len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Image != "reg/runner:v1" {
		t.Errorf("unexpected image: %#v", pod.Spec.Containers[0].Image)
	}
}

func TestClientPodSpec_ReferencesTargetIP(t *testing.T) {
	cfg := config.RunConfig{
		Images: config.ImageConfig{
			Registry: "reg",
			Runner:   config.RunnerImage{Repository: "runner", Tag: "v1", PullPolicy: "IfNotPresent"},
		},
	}
	pod := ClientPod("run-1", "ns",
		NodePair{ServerNode: "nodeA", ClientNode: "nodeB", Protocol: "udp"},
		"10.0.0.5", 5201, "30s", cfg,
	)
	foundTarget := false
	for _, a := range pod.Spec.Containers[0].Args {
		if a == "--target=10.0.0.5" {
			foundTarget = true
		}
	}
	if !foundTarget {
		t.Errorf("client args missing --target=10.0.0.5: %v", pod.Spec.Containers[0].Args)
	}
	if pod.Spec.NodeSelector["kubernetes.io/hostname"] != "nodeB" {
		t.Errorf("client node selector wrong: %v", pod.Spec.NodeSelector)
	}
}
