package network

import (
	"context"
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/discover"
)

func TestPlan_ProducesCombinationsAndProtocols(t *testing.T) {
	b := New()
	cfg := config.RunConfig{
		Benches: config.BenchesConfig{
			Network: config.NetworkBenchConfig{
				Protocols:    []string{"tcp", "udp"},
				Combinations: []string{"same-node", "cross-node"},
			},
		},
	}
	tasks, err := b.Plan(context.Background(), cfg, discover.Discovery{
		Nodes: []string{"a", "b", "c"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 2 protocols × 2 combinations = 4 tasks
	if len(tasks) != 4 {
		t.Errorf("tasks: %d, want 4 (%v)", len(tasks), tasks)
	}
}

func TestPlan_SkipsCrossNodeWhenOnlyOneNode(t *testing.T) {
	b := New()
	cfg := config.RunConfig{
		Benches: config.BenchesConfig{
			Network: config.NetworkBenchConfig{
				Protocols:    []string{"tcp"},
				Combinations: []string{"same-node", "cross-node"},
			},
		},
	}
	tasks, err := b.Plan(context.Background(), cfg, discover.Discovery{
		Nodes: []string{"only"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ID != "network/same-node/tcp" {
		t.Errorf("expected only same-node/tcp, got %+v", tasks)
	}
}

func TestPlan_NoNodes(t *testing.T) {
	b := New()
	tasks, err := b.Plan(context.Background(), config.RunConfig{}, discover.Discovery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected no tasks, got %d", len(tasks))
	}
}

func TestConflictsWithSelf(t *testing.T) {
	b := New()
	if !b.ConflictsWith("network") {
		t.Error("network bench should conflict with itself")
	}
	if b.ConflictsWith("storage") {
		t.Error("network bench should not conflict with storage")
	}
}
