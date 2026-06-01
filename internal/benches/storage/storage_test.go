package storage

import (
	"context"
	"testing"
	"time"

	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/discover"
)

func TestPlan_OneTaskPerSCxPatternxBlockSize(t *testing.T) {
	b := New()
	cfg := config.RunConfig{
		Benches: config.BenchesConfig{
			Storage: config.StorageBenchConfig{
				Duration:   10 * time.Second,
				Size:       "1Gi",
				BlockSizes: []string{"4k", "1M"},
				Patterns:   []string{"randread", "randwrite"},
			},
		},
	}
	d := discover.Discovery{
		Nodes:          []string{"n1"},
		StorageClasses: []string{"gp3", "premium"},
	}
	tasks, err := b.Plan(context.Background(), cfg, d)
	if err != nil {
		t.Fatal(err)
	}
	// 2 SCs * 2 block sizes * 2 patterns = 8 tasks
	if len(tasks) != 8 {
		t.Errorf("expected 8 tasks, got %d", len(tasks))
	}
}

func TestPlan_SkipsExcludedStorageClasses(t *testing.T) {
	b := New()
	cfg := config.RunConfig{
		Excludes: config.ExcludeConfig{StorageClasses: []string{"premium"}},
		Benches: config.BenchesConfig{
			Storage: config.StorageBenchConfig{
				Duration: 1 * time.Second, Size: "1Gi",
				BlockSizes: []string{"4k"}, Patterns: []string{"randread"},
			},
		},
	}
	tasks, _ := b.Plan(context.Background(), cfg, discover.Discovery{
		Nodes: []string{"n1"}, StorageClasses: []string{"gp3", "premium"},
	})
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d (%v)", len(tasks), tasks)
	}
}

func TestPlan_IncludeStorageClassWhitelist(t *testing.T) {
	b := New()
	cfg := config.RunConfig{
		Includes: config.IncludeConfig{StorageClasses: []string{"gp3"}},
		Benches: config.BenchesConfig{
			Storage: config.StorageBenchConfig{
				Duration: 1 * time.Second, Size: "1Gi",
				BlockSizes: []string{"4k"}, Patterns: []string{"randread"},
			},
		},
	}
	tasks, _ := b.Plan(context.Background(), cfg, discover.Discovery{
		Nodes: []string{"n1"}, StorageClasses: []string{"gp3", "premium", "standard"},
	})
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (only gp3), got %d", len(tasks))
	}
	if !contains(tasks[0].ID, "gp3") {
		t.Errorf("expected task for gp3, got %s", tasks[0].ID)
	}
}

func TestPlan_IncludeAndExcludeCombined(t *testing.T) {
	b := New()
	cfg := config.RunConfig{
		Includes: config.IncludeConfig{StorageClasses: []string{"gp3", "premium"}},
		Excludes: config.ExcludeConfig{StorageClasses: []string{"premium"}},
		Benches: config.BenchesConfig{
			Storage: config.StorageBenchConfig{
				Duration: 1 * time.Second, Size: "1Gi",
				BlockSizes: []string{"4k"}, Patterns: []string{"randread"},
			},
		},
	}
	tasks, _ := b.Plan(context.Background(), cfg, discover.Discovery{
		Nodes: []string{"n1"}, StorageClasses: []string{"gp3", "premium", "standard"},
	})
	if len(tasks) != 1 || !contains(tasks[0].ID, "gp3") {
		t.Errorf("expected 1 task for gp3 only (premium excluded, standard not included), got %v", tasks)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestConflictsWithSelf(t *testing.T) {
	// Storage benches on DIFFERENT SCs can run in parallel; only intra-SC matters.
	// For v1 simplicity: do not declare a conflict (parallelism controlled by --parallelism).
	if New().ConflictsWith("storage") {
		t.Error("storage bench should not conflict with itself (different SCs OK in parallel)")
	}
}
