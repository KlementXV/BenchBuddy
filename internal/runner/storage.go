package runner

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

type StorageArgs struct {
	Directory string
	Pattern   string // randread | randwrite | seqread | seqwrite
	BlockSize string // e.g. "4k", "1M"
	Duration  time.Duration
	SizeMB    int // file size; default 256
}

func ParseStorageArgs(args []string) (StorageArgs, error) {
	fs := flag.NewFlagSet("storage", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	a := StorageArgs{Duration: 15 * time.Second, SizeMB: 256}
	fs.StringVar(&a.Directory, "directory", "", "directory to test (mounted PVC)")
	fs.StringVar(&a.Pattern, "pattern", "", "fio pattern: randread|randwrite|seqread|seqwrite")
	fs.StringVar(&a.BlockSize, "block-size", "", "fio block size, e.g. 4k or 1M")
	fs.DurationVar(&a.Duration, "duration", a.Duration, "")
	fs.IntVar(&a.SizeMB, "size-mb", a.SizeMB, "test file size in MB")
	if err := fs.Parse(args); err != nil {
		return a, err
	}
	if a.Directory == "" {
		return a, errors.New("--directory is required")
	}
	switch a.Pattern {
	case "randread", "randwrite", "seqread", "seqwrite":
	default:
		return a, fmt.Errorf("invalid --pattern %q (use randread|randwrite|seqread|seqwrite)", a.Pattern)
	}
	if a.BlockSize == "" {
		return a, errors.New("--block-size is required")
	}
	return a, nil
}

// RunStorage runs fio against StorageArgs and emits the typed marker.
func RunStorage(ctx context.Context, a StorageArgs, stdout io.Writer) error {
	rw := map[string]string{
		"randread":  "randread",
		"randwrite": "randwrite",
		"seqread":   "read",
		"seqwrite":  "write",
	}[a.Pattern]

	args := []string{
		"--name=benchbuddy",
		"--directory=" + a.Directory,
		"--rw=" + rw,
		"--bs=" + a.BlockSize,
		"--size=" + fmt.Sprintf("%dM", a.SizeMB),
		"--time_based=1",
		"--runtime=" + fmt.Sprintf("%d", int(a.Duration.Seconds())),
		"--ioengine=libaio",
		"--direct=1",
		"--iodepth=32",
		"--group_reporting",
		"--output-format=json",
	}
	out, err := exec.CommandContext(ctx, "fio", args...).Output()
	if err != nil {
		return fmt.Errorf("fio: %w", err)
	}
	// Echo raw JSON for debugging.
	if _, werr := stdout.Write(out); werr != nil {
		return werr
	}
	if _, werr := io.WriteString(stdout, "\n"); werr != nil {
		return werr
	}

	metrics, err := parseFioJSON(out, a.Pattern)
	if err != nil {
		return err
	}
	// Cleanup the test file so subsequent runs don't see stale data.
	_ = exec.CommandContext(ctx, "rm", "-f", filepath.Join(a.Directory, "benchbuddy.0.0")).Run()

	return runresult.EmitMarker(stdout, metrics)
}

type fioJobOutput struct {
	Read  fioOp `json:"read"`
	Write fioOp `json:"write"`
}

type fioOp struct {
	IOPS  float64 `json:"iops"`
	Bw    float64 `json:"bw"` // KiB/s
	LatNs struct {
		Mean       float64 `json:"mean"`
		Percentile struct {
			P50 float64 `json:"50.000000"`
			P95 float64 `json:"95.000000"`
			P99 float64 `json:"99.000000"`
		} `json:"percentile"`
	} `json:"lat_ns"`
}

type fioRoot struct {
	Jobs []fioJobOutput `json:"jobs"`
}

func parseFioJSON(b []byte, pattern string) (map[string]runresult.Metric, error) {
	var raw fioRoot
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("decode fio json: %w", err)
	}
	if len(raw.Jobs) == 0 {
		return nil, errors.New("fio returned no jobs")
	}
	op := raw.Jobs[0].Read
	if pattern == "randwrite" || pattern == "seqwrite" {
		op = raw.Jobs[0].Write
	}
	return map[string]runresult.Metric{
		"iops":            {Value: op.IOPS, Unit: "iops"},
		"bandwidth_mb_s":  {Value: op.Bw / 1024.0, Unit: "MB/s"},
		"latency_mean_us": {Value: op.LatNs.Mean / 1000.0, Unit: "us"},
		"latency_p50_us":  {Value: op.LatNs.Percentile.P50 / 1000.0, Unit: "us"},
		"latency_p95_us":  {Value: op.LatNs.Percentile.P95 / 1000.0, Unit: "us"},
		"latency_p99_us":  {Value: op.LatNs.Percentile.P99 / 1000.0, Unit: "us"},
	}, nil
}
