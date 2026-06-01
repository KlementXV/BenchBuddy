package runner

import (
	"context"
	"flag"
	"io"
	"runtime"
	"time"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

type PodCPUArgs struct {
	Duration time.Duration
}

func ParsePodCPUArgs(args []string) (PodCPUArgs, error) {
	fs := flag.NewFlagSet("pod-cpu", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	a := PodCPUArgs{Duration: 10 * time.Second}
	fs.DurationVar(&a.Duration, "duration", a.Duration, "")
	if err := fs.Parse(args); err != nil {
		return a, err
	}
	return a, nil
}

// RunPodCPU runs a single-threaded busy loop for the configured duration and
// reports ops/sec. Single-threaded keeps the metric comparable across pods
// regardless of available CPU count.
func RunPodCPU(ctx context.Context, a PodCPUArgs, stdout io.Writer) error {
	runtime.GC()
	deadline := time.Now().Add(a.Duration)
	var ops uint64
	// Tight integer arithmetic loop, modest enough to allow ctx checks.
	var acc uint64 = 1
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			break
		}
		// Inner unrolled loop to amortize the time.Now()/ctx.Err() cost.
		for k := 0; k < 100000; k++ {
			acc = (acc*1103515245 + 12345) & 0x7fffffff
		}
		ops += 100000
	}
	_ = acc // avoid optimizer elimination

	elapsed := a.Duration
	metrics := map[string]runresult.Metric{
		"ops_per_sec":  {Value: float64(ops) / elapsed.Seconds(), Unit: "ops/s"},
		"total_ops":    {Value: float64(ops), Unit: "ops"},
		"cpu_count":    {Value: float64(runtime.NumCPU()), Unit: "cores"},
		"duration_sec": {Value: elapsed.Seconds(), Unit: "s"},
	}
	return runresult.EmitMarker(stdout, metrics)
}
