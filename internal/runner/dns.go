package runner

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

type DNSArgs struct {
	Queries  []string
	Duration time.Duration
	QPS      int
}

func ParseDNSArgs(args []string) (DNSArgs, error) {
	fs := flag.NewFlagSet("dns", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var queries string
	a := DNSArgs{Duration: 10 * time.Second, QPS: 50}
	fs.StringVar(&queries, "queries", "", "comma-separated FQDNs")
	fs.DurationVar(&a.Duration, "duration", a.Duration, "")
	fs.IntVar(&a.QPS, "qps", a.QPS, "")
	if err := fs.Parse(args); err != nil {
		return a, err
	}
	if strings.TrimSpace(queries) == "" {
		return a, errors.New("--queries is required")
	}
	for _, q := range strings.Split(queries, ",") {
		q = strings.TrimSpace(q)
		if q != "" {
			a.Queries = append(a.Queries, q)
		}
	}
	if len(a.Queries) == 0 {
		return a, errors.New("--queries is required")
	}
	return a, nil
}

// RunDNS issues lookups against a.Queries (round-robin) at ~a.QPS for a.Duration.
// Each successful lookup contributes a latency sample. Errors are counted but
// not used as samples.
func RunDNS(ctx context.Context, a DNSArgs, stdout io.Writer) error {
	resolver := net.DefaultResolver

	interval := time.Second / time.Duration(a.QPS)
	if interval < time.Millisecond {
		interval = time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	deadline := time.Now().Add(a.Duration)
	var samples []time.Duration
	var errCount int
	i := 0

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			break
		case <-ticker.C:
		}
		target := a.Queries[i%len(a.Queries)]
		i++
		start := time.Now()
		lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, err := resolver.LookupHost(lookupCtx, target)
		cancel()
		if err != nil {
			errCount++
			continue
		}
		samples = append(samples, time.Since(start))
	}

	if len(samples) == 0 {
		return fmt.Errorf("no successful lookups (errors=%d)", errCount)
	}

	metrics := dnsPercentiles(samples, errCount, a.Duration)
	return runresult.EmitMarker(stdout, metrics)
}

func dnsPercentiles(samples []time.Duration, errCount int, dur time.Duration) map[string]runresult.Metric {
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	pick := func(p float64) float64 {
		if len(samples) == 0 {
			return 0
		}
		idx := int(float64(len(samples)-1) * p)
		return float64(samples[idx].Microseconds()) / 1000.0
	}
	return map[string]runresult.Metric{
		"latency_p50_ms":  {Value: pick(0.50), Unit: "ms"},
		"latency_p95_ms":  {Value: pick(0.95), Unit: "ms"},
		"latency_p99_ms":  {Value: pick(0.99), Unit: "ms"},
		"queries_per_sec": {Value: float64(len(samples)) / dur.Seconds(), Unit: "qps"},
		"sample_count":    {Value: float64(len(samples)), Unit: "samples"},
		"error_count":     {Value: float64(errCount), Unit: "errors"},
	}
}
