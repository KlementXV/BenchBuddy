package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/clementlevoux/benchbuddy/internal/runner"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: runner --bench=<name> [bench flags]")
	}

	// Split --bench=<name> from the rest.
	var bench string
	var rest []string
	for _, arg := range os.Args[1:] {
		if v, ok := stripPrefix(arg, "--bench="); ok {
			bench = v
			continue
		}
		rest = append(rest, arg)
	}
	if bench == "" {
		fail("missing --bench=<name>")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch bench {
	case "network":
		args, err := runner.ParseNetworkArgs(rest)
		if err != nil {
			fail(err.Error())
		}
		if err := runner.RunNetwork(ctx, args, os.Stdout); err != nil {
			fail(err.Error())
		}
	case "dns":
		args, err := runner.ParseDNSArgs(rest)
		if err != nil {
			fail(err.Error())
		}
		if err := runner.RunDNS(ctx, args, os.Stdout); err != nil {
			fail(err.Error())
		}
	case "storage":
		args, err := runner.ParseStorageArgs(rest)
		if err != nil {
			fail(err.Error())
		}
		if err := runner.RunStorage(ctx, args, os.Stdout); err != nil {
			fail(err.Error())
		}
	case "pod-cpu":
		args, err := runner.ParsePodCPUArgs(rest)
		if err != nil {
			fail(err.Error())
		}
		if err := runner.RunPodCPU(ctx, args, os.Stdout); err != nil {
			fail(err.Error())
		}
	default:
		fail("unknown bench: " + bench)
	}
}

func stripPrefix(s, prefix string) (string, bool) {
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return "", false
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "runner:", msg)
	os.Exit(2)
}
