package orchestrator

import (
	"context"
	"sync"
	"time"

	"github.com/clementlevoux/benchbuddy/internal/benches"
	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/discover"
	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

// PlannedTask pairs a Task with the Bench that owns it (so the orchestrator
// can route Run() correctly without a name lookup at execution time).
type PlannedTask struct {
	Bench benches.Bench
	Task  benches.Task
}

// Orchestrator coordinates planning, execution, and cleanup across benches.
type Orchestrator struct {
	kc      *kube.Client
	cfg     config.RunConfig
	benches []benches.Bench
}

func New(kc *kube.Client, cfg config.RunConfig) *Orchestrator {
	return &Orchestrator{kc: kc, cfg: cfg}
}

func (o *Orchestrator) Register(b benches.Bench) { o.benches = append(o.benches, b) }

// Plan asks each bench for its tasks and produces a flat, executable plan.
func (o *Orchestrator) Plan(ctx context.Context, d discover.Discovery) ([]PlannedTask, error) {
	var plan []PlannedTask
	for _, b := range o.benches {
		// Skip excluded benches.
		if contains(o.cfg.Excludes.Benches, b.Name()) {
			continue
		}
		tasks, err := b.Plan(ctx, o.cfg, d)
		if err != nil {
			return nil, err
		}
		for _, t := range tasks {
			plan = append(plan, PlannedTask{Bench: b, Task: t})
		}
	}
	return plan, nil
}

// Run executes the plan with bounded parallelism and ConflictsWith honoring.
// The semantics:
//   - At most cfg.Parallelism tasks run concurrently overall.
//   - Two tasks where bench A.ConflictsWith(B.Name()) is true never run together.
func (o *Orchestrator) Run(ctx context.Context, runID string, plan []PlannedTask) ([]runresult.Result, error) {
	if len(plan) == 0 {
		return nil, nil
	}
	parallelism := o.cfg.Parallelism
	if parallelism < 1 {
		parallelism = 1
	}

	results := make([]runresult.Result, len(plan))
	sem := make(chan struct{}, parallelism)

	// running tracks the names of benches currently executing, for ConflictsWith.
	var mu sync.Mutex
	running := map[string]int{}

	acquire := func(name string) {
		for {
			mu.Lock()
			conflict := false
			for other, n := range running {
				if n == 0 {
					continue
				}
				if hasConflict(o.benches, name, other) {
					conflict = true
					break
				}
			}
			if !conflict {
				running[name]++
				mu.Unlock()
				return
			}
			mu.Unlock()
			// Spin with a tiny wait to avoid busy loop.
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
	release := func(name string) {
		mu.Lock()
		running[name]--
		mu.Unlock()
	}

	var wg sync.WaitGroup
	for i, pt := range plan {
		i, pt := i, pt
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			acquire(pt.Bench.Name())
			defer release(pt.Bench.Name())

			res, err := pt.Bench.Run(ctx, o.kc, runID, pt.Task, o.cfg)
			if err != nil {
				res = runresult.Result{
					BenchName: pt.Bench.Name(),
					TaskID:    pt.Task.ID,
					Status:    runresult.StatusFailed,
					Errors:    []string{err.Error()},
				}
			}
			results[i] = res
		}()
	}
	wg.Wait()
	return results, nil
}

func hasConflict(all []benches.Bench, name, otherName string) bool {
	for _, b := range all {
		if b.Name() == name {
			if b.ConflictsWith(otherName) {
				return true
			}
		}
		if b.Name() == otherName {
			if b.ConflictsWith(name) {
				return true
			}
		}
	}
	return false
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
