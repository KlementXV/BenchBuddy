package runner

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

// NetworkArgs are flags accepted by `runner --bench=network`.
type NetworkArgs struct {
	Role     string        // "server" or "client"
	Port     int
	Target   string        // client only
	Protocol string        // "tcp" or "udp"
	Duration time.Duration // client only
}

// ParseNetworkArgs parses --role/--target/--protocol/--duration/--port.
func ParseNetworkArgs(args []string) (NetworkArgs, error) {
	fs := flag.NewFlagSet("network", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	a := NetworkArgs{Port: 5201, Protocol: "tcp", Duration: 10 * time.Second}
	fs.StringVar(&a.Role, "role", "", "")
	fs.IntVar(&a.Port, "port", a.Port, "")
	fs.StringVar(&a.Target, "target", "", "")
	fs.StringVar(&a.Protocol, "protocol", a.Protocol, "")
	fs.DurationVar(&a.Duration, "duration", a.Duration, "")
	if err := fs.Parse(args); err != nil {
		return a, err
	}
	switch a.Role {
	case "server", "client":
	default:
		return a, fmt.Errorf("--role must be 'server' or 'client', got %q", a.Role)
	}
	return a, nil
}

// RunNetwork dispatches to server or client mode and writes the
// BENCHBUDDY_RESULT marker on stdout when applicable.
func RunNetwork(ctx context.Context, a NetworkArgs, stdout io.Writer) error {
	switch a.Role {
	case "server":
		return runIperfServer(ctx, a, stdout)
	case "client":
		return runIperfClient(ctx, a, stdout)
	}
	return fmt.Errorf("unsupported role: %s", a.Role)
}

func runIperfServer(ctx context.Context, a NetworkArgs, stdout io.Writer) error {
	cmd := exec.CommandContext(ctx, "iperf3", "-s", "-1", "-p", fmt.Sprint(a.Port))
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	return cmd.Run()
}

func runIperfClient(ctx context.Context, a NetworkArgs, stdout io.Writer) error {
	args := []string{
		"-c", a.Target,
		"-p", fmt.Sprint(a.Port),
		"-J",
		"-t", fmt.Sprint(int(a.Duration.Seconds())),
	}
	if strings.EqualFold(a.Protocol, "udp") {
		args = append(args, "-u", "-b", "0")
	}
	out, err := exec.CommandContext(ctx, "iperf3", args...).Output()
	if err != nil {
		return fmt.Errorf("iperf3: %w", err)
	}
	// Echo raw iperf3 JSON to stdout for debugging
	if _, werr := stdout.Write(out); werr != nil {
		return werr
	}
	if _, werr := io.WriteString(stdout, "\n"); werr != nil {
		return werr
	}

	metrics, err := parseIperf3JSON(out)
	if err != nil {
		return err
	}
	return runresult.EmitMarker(stdout, metrics)
}

// parseIperf3JSON converts the iperf3 --json output to typed Metrics.
// Only a small subset of fields is extracted (the rest is preserved in the
// raw log).
func parseIperf3JSON(b []byte) (map[string]runresult.Metric, error) {
	var raw struct {
		End struct {
			SumSent struct {
				BitsPerSecond float64 `json:"bits_per_second"`
				LostPercent   float64 `json:"lost_percent"`
				JitterMs      float64 `json:"jitter_ms"`
			} `json:"sum_sent"`
			SumReceived struct {
				BitsPerSecond float64 `json:"bits_per_second"`
			} `json:"sum_received"`
		} `json:"end"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("decode iperf3 json: %w", err)
	}
	m := map[string]runresult.Metric{
		"bandwidth_sent_gbps":     {Value: raw.End.SumSent.BitsPerSecond / 1e9, Unit: "Gbps"},
		"bandwidth_received_gbps": {Value: raw.End.SumReceived.BitsPerSecond / 1e9, Unit: "Gbps"},
	}
	if raw.End.SumSent.JitterMs > 0 {
		m["jitter_ms"] = runresult.Metric{Value: raw.End.SumSent.JitterMs, Unit: "ms"}
	}
	if raw.End.SumSent.LostPercent > 0 {
		m["lost_percent"] = runresult.Metric{Value: raw.End.SumSent.LostPercent, Unit: "%"}
	}
	return m, nil
}
