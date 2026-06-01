package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestParseOutputs(t *testing.T) {
	tests := []struct {
		flags    []string
		wantJSON string
		wantMD   string
		wantErr  bool
	}{
		{nil, "", "", false},
		{[]string{"json=out.json"}, "out.json", "", false},
		{[]string{"md=report.md"}, "", "report.md", false},
		{[]string{"json=a.json", "md=b.md"}, "a.json", "b.md", false},
		{[]string{"xml=out.xml"}, "", "", true},
		{[]string{"notaformat"}, "", "", true},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%v", tc.flags), func(t *testing.T) {
			j, m, err := parseOutputs(tc.flags)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if j != tc.wantJSON {
				t.Errorf("jsonPath: got %q, want %q", j, tc.wantJSON)
			}
			if m != tc.wantMD {
				t.Errorf("mdPath: got %q, want %q", m, tc.wantMD)
			}
		})
	}
}

func TestRunCommand_Help(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"run", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	_ = cmd.Execute()

	got := out.String()
	for _, want := range []string{
		"--namespace",
		"--profile",
		"--parallelism",
		"--timeout",
		"--keep",
		"--yes",
		"--exclude-bench",
		"--registry",
		"--runner-image",
		"--runner-digest",
		"--image-pull-secret",
		"--image-pull-policy",
		"--config",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("--help missing flag %q\n%s", want, got)
		}
	}
}
