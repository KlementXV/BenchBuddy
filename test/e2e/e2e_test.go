//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestE2E_AllBenches_OnKind(t *testing.T) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		t.Fatal("KUBECONFIG must be set (point at the kind cluster)")
	}

	must(t, exec.Command("kubectl", "--kubeconfig", kubeconfig, "create", "ns", "bb-e2e"))

	cmd := exec.Command("./benchbuddy", "run",
		"--namespace=bb-e2e",
		"--profile=quick",
		"--yes",
		"--registry=docker.io/library",
		"--runner-image=benchbuddy-runner:e2e",
		"--image-pull-policy=Never",
	)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	out, err := cmd.CombinedOutput()
	t.Logf("benchbuddy output:\n%s", out)
	if err != nil {
		t.Fatalf("benchbuddy run failed: %v", err)
	}

	// At least one task per bench should appear in the output. Storage is
	// asserted only if kind ships a default StorageClass (it does:
	// rancher.io/local-path).
	for _, want := range []string{
		"network/same-node/tcp",
		"network/cross-node/tcp",
		"dns/in-cluster",
		"api/list-pods",
		"pod/startup",
		"pod/cpu",
		"storage/standard", // kind's default SC is named "standard"
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("expected result for %q in output", want)
		}
	}

	// Namespace should be clean after the run.
	pods, _ := exec.Command("kubectl", "--kubeconfig", kubeconfig, "-n", "bb-e2e", "get", "pods", "-o", "name").Output()
	if strings.TrimSpace(string(pods)) != "" {
		t.Errorf("expected namespace clean post-run, found pods: %s", pods)
	}
	pvcs, _ := exec.Command("kubectl", "--kubeconfig", kubeconfig, "-n", "bb-e2e", "get", "pvc", "-o", "name").Output()
	if strings.TrimSpace(string(pvcs)) != "" {
		t.Errorf("expected no PVCs post-run, found: %s", pvcs)
	}
}

func must(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", strings.Join(cmd.Args, " "), err, out)
	}
}
