package orchestrator

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/clementlevoux/benchbuddy/internal/benches"
	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/discover"
	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

type mockBench struct {
	name        string
	tasks       []benches.Task
	conflicts   bool
	maxParallel *int32
	current     atomic.Int32
	runDelay    time.Duration
	runErr      error
}

func (m *mockBench) Name() string { return m.name }
func (m *mockBench) Plan(_ context.Context, _ config.RunConfig, _ discover.Discovery) ([]benches.Task, error) {
	return m.tasks, nil
}
func (m *mockBench) ConflictsWith(other string) bool { return m.conflicts && other == m.name }
func (m *mockBench) Cleanup(_ context.Context, _ *kube.Client, _ string) error { return nil }
func (m *mockBench) Run(ctx context.Context, _ *kube.Client, _ string, t benches.Task, _ config.RunConfig) (runresult.Result, error) {
	cur := m.current.Add(1)
	defer m.current.Add(-1)
	if m.maxParallel != nil {
		for {
			max := atomic.LoadInt32(m.maxParallel)
			if cur > max {
				if atomic.CompareAndSwapInt32(m.maxParallel, max, cur) {
					break
				}
				continue
			}
			break
		}
	}
	select {
	case <-ctx.Done():
		return runresult.Result{}, ctx.Err()
	case <-time.After(m.runDelay):
	}
	if m.runErr != nil {
		return runresult.Result{
			BenchName: m.name, TaskID: t.ID, Status: runresult.StatusFailed,
			Errors: []string{m.runErr.Error()},
		}, nil
	}
	return runresult.Result{BenchName: m.name, TaskID: t.ID, Status: runresult.StatusOK}, nil
}

func mkTasks(n int, prefix string) []benches.Task {
	ts := make([]benches.Task, n)
	for i := range ts {
		ts[i] = benches.Task{ID: prefix + "/" + string(rune('a'+i))}
	}
	return ts
}

func TestOrchestrator_PlanCollectsAllTasks(t *testing.T) {
	o := New(nil, config.RunConfig{Parallelism: 3})
	o.Register(&mockBench{name: "a", tasks: mkTasks(2, "a")})
	o.Register(&mockBench{name: "b", tasks: mkTasks(3, "b")})

	plan, err := o.Plan(context.Background(), discover.Discovery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 5 {
		t.Errorf("plan size: %d, want 5", len(plan))
	}
}

func TestOrchestrator_RespectsParallelism(t *testing.T) {
	var observed int32
	m := &mockBench{
		name:        "fast",
		tasks:       mkTasks(10, "fast"),
		maxParallel: &observed,
		runDelay:    20 * time.Millisecond,
	}
	o := New(nil, config.RunConfig{Parallelism: 3})
	o.Register(m)

	plan, err := o.Plan(context.Background(), discover.Discovery{})
	if err != nil {
		t.Fatal(err)
	}
	results, err := o.Run(context.Background(), "run-1", plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 10 {
		t.Errorf("results: %d, want 10", len(results))
	}
	if got := atomic.LoadInt32(&observed); got > 3 {
		t.Errorf("max concurrency exceeded: %d > 3", got)
	}
}

func TestOrchestrator_ConflictsSerialize(t *testing.T) {
	var observed int32
	m := &mockBench{
		name:        "net",
		tasks:       mkTasks(4, "net"),
		conflicts:   true,
		maxParallel: &observed,
		runDelay:    20 * time.Millisecond,
	}
	o := New(nil, config.RunConfig{Parallelism: 4})
	o.Register(m)

	plan, _ := o.Plan(context.Background(), discover.Discovery{})
	if _, err := o.Run(context.Background(), "run-1", plan); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&observed); got > 1 {
		t.Errorf("conflicting bench ran in parallel (max %d)", got)
	}
}

func TestOrchestrator_TaskErrorDoesNotAbortOthers(t *testing.T) {
	m := &mockBench{
		name:     "x",
		tasks:    mkTasks(3, "x"),
		runDelay: 5 * time.Millisecond,
		runErr:   context.DeadlineExceeded,
	}
	o := New(nil, config.RunConfig{Parallelism: 3})
	o.Register(m)
	plan, _ := o.Plan(context.Background(), discover.Discovery{})
	results, err := o.Run(context.Background(), "run-1", plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("results: %d", len(results))
	}
	for _, r := range results {
		if r.Status != runresult.StatusFailed {
			t.Errorf("expected Failed, got %s for %s", r.Status, r.TaskID)
		}
	}
}
