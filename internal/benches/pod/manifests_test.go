package pod

import (
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/config"
)

func TestPausePodSpec(t *testing.T) {
	cfg := config.RunConfig{
		Benches: config.BenchesConfig{
			Pod: config.PodBenchConfig{PauseImage: "registry.k8s.io/pause:3.9"},
		},
	}
	pod := PausePod("run-1", "ns", "node-1", 3, cfg)
	if pod.Spec.Containers[0].Image != "registry.k8s.io/pause:3.9" {
		t.Errorf("pause image: %q", pod.Spec.Containers[0].Image)
	}
	if pod.Labels["benchbuddy.io/sample-index"] != "3" {
		t.Errorf("sample-index label missing: %v", pod.Labels)
	}
}

func TestCPUPodSpec(t *testing.T) {
	cfg := config.RunConfig{
		Images: config.ImageConfig{
			Registry: "reg",
			Runner:   config.RunnerImage{Repository: "runner", Tag: "v1", PullPolicy: "IfNotPresent"},
		},
	}
	pod := CPUPod("run-1", "ns", "node-1", "5s", cfg)
	if pod.Spec.Containers[0].Image != "reg/runner:v1" {
		t.Errorf("runner image: %q", pod.Spec.Containers[0].Image)
	}
	found := false
	for _, a := range pod.Spec.Containers[0].Args {
		if a == "--duration=5s" {
			found = true
		}
	}
	if !found {
		t.Errorf("--duration=5s missing in args: %v", pod.Spec.Containers[0].Args)
	}
}
