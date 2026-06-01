package dns

import (
	"context"
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/discover"
)

func TestPlan_OneTaskPerProfile(t *testing.T) {
	b := New()
	cfg := config.RunConfig{
		Benches: config.BenchesConfig{
			DNS: config.DNSBenchConfig{
				Targets: []string{"in-cluster"},
			},
		},
	}
	tasks, err := b.Plan(context.Background(), cfg, discover.Discovery{Nodes: []string{"n1"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

func TestPlan_NoNodes(t *testing.T) {
	b := New()
	tasks, _ := b.Plan(context.Background(), config.RunConfig{}, discover.Discovery{})
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks when no nodes, got %d", len(tasks))
	}
}

func TestConflictsWithSelf(t *testing.T) {
	if !New().ConflictsWith("dns") {
		t.Error("dns should conflict with itself (CoreDNS shared)")
	}
}
