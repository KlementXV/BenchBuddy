package pod

import (
	"context"
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/discover"
)

func TestPlan_ProducesStartupAndCPUTasks(t *testing.T) {
	b := New()
	cfg := config.RunConfig{
		Benches: config.BenchesConfig{
			Pod: config.PodBenchConfig{
				SampleCount: 5, PauseImage: "registry.k8s.io/pause:3.9",
			},
		},
	}
	tasks, err := b.Plan(context.Background(), cfg, discover.Discovery{Nodes: []string{"n1"}})
	if err != nil {
		t.Fatal(err)
	}
	// expect 2 tasks: pod/startup and pod/cpu
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d: %+v", len(tasks), tasks)
	}
}

func TestPlan_NoNodes(t *testing.T) {
	tasks, _ := New().Plan(context.Background(), config.RunConfig{}, discover.Discovery{})
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestConflictsWith_None(t *testing.T) {
	if New().ConflictsWith("pod") {
		t.Error("pod bench should not conflict with itself (samples are independent)")
	}
}
