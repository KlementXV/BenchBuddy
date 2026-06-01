package labels

import "testing"

func TestForRun(t *testing.T) {
	l := ForRun("run-123")
	if l[RunIDKey] != "run-123" {
		t.Errorf("run-id label missing, got %v", l)
	}
}

func TestForTask(t *testing.T) {
	l := ForTask("run-123", "network", "same-node/tcp")
	if l[BenchKey] != "network" || l[TaskKey] != "same-node_tcp" {
		t.Errorf("unexpected: %v", l)
	}
}

func TestSelectorForRun(t *testing.T) {
	got := SelectorForRun("run-123")
	want := "benchbuddy.io/run-id=run-123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSanitizeForLabelValue(t *testing.T) {
	cases := map[string]string{
		"same-node/tcp":     "same-node_tcp",
		"a b":               "a_b",
		"VALID":             "VALID",
		"trailing-dash-":    "trailing-dash",
		"-leading":          "leading",
		"more.dots.are.ok": "more.dots.are.ok",
	}
	for in, want := range cases {
		if got := sanitizeForLabelValue(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}
