package kube

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewClient_MissingKubeconfig(t *testing.T) {
	t.Setenv("KUBECONFIG", "/nonexistent/path")
	t.Setenv("HOME", t.TempDir()) // ensure no fallback to ~/.kube/config
	_, err := NewClient(Options{})
	if err == nil {
		t.Fatal("expected error for missing kubeconfig, got nil")
	}
}

func TestNewClient_LoadsFromExplicitPath(t *testing.T) {
	// minimal valid kubeconfig
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")
	if err := os.WriteFile(path, []byte(minimalKubeconfig), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := NewClient(Options{KubeconfigPath: path})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.Clientset() == nil {
		t.Fatal("Clientset() returned nil")
	}
}

const minimalKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://example.test
    insecure-skip-tls-verify: true
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user:
    token: dummy
`
