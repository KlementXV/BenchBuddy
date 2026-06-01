package runner

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestParsePodCPUArgs_Defaults(t *testing.T) {
	a, err := ParsePodCPUArgs([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Duration != 10*time.Second {
		t.Errorf("duration: %v", a.Duration)
	}
}

func TestRunPodCPU_EmitsMarker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var buf bytes.Buffer
	if err := RunPodCPU(ctx, PodCPUArgs{Duration: 300 * time.Millisecond}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "BENCHBUDDY_RESULT:") {
		t.Errorf("marker missing in output: %q", out)
	}
	if !strings.Contains(out, "ops_per_sec") {
		t.Errorf("ops_per_sec metric missing")
	}
}
