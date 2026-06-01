package benches

import (
	"context"

	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/discover"
	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

// Task is one atomic unit of work for a bench (one pair of nodes, one
// StorageClass, etc.).
type Task struct {
	ID      string      // e.g. "network/cross-node/tcp/nodeA-nodeB"
	Subject string      // human-readable, e.g. "nodeA → nodeB (TCP)"
	Spec    interface{} // bench-specific (NodePair, PVCSpec, …)
}

// Bench is the contract each bench category (network, storage, …) implements.
type Bench interface {
	// Name returns a stable bench identifier, e.g. "network".
	Name() string

	// Plan derives the list of Tasks to execute given the discovery snapshot
	// and the run config.
	Plan(ctx context.Context, cfg config.RunConfig, d discover.Discovery) ([]Task, error)

	// Run executes one Task and returns a typed Result. Errors are also
	// captured into Result.Errors; only catastrophic failures (e.g. cancellation)
	// should be returned as the error value.
	Run(ctx context.Context, kc *kube.Client, runID string, t Task, cfg config.RunConfig) (runresult.Result, error)

	// ConflictsWith reports whether this bench cannot run in parallel with
	// other instances of itself (e.g. the network bench wants exclusive use
	// of the cluster network).
	ConflictsWith(other string) bool

	// Cleanup deletes any resources created for the given run.
	Cleanup(ctx context.Context, kc *kube.Client, runID string) error
}
