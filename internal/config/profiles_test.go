package config

import (
	"testing"
	"time"
)

func TestLoadProfile_Quick(t *testing.T) {
	p, err := LoadProfile("quick")
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if p.Name != "quick" {
		t.Errorf("name: got %q, want quick", p.Name)
	}
	if p.Parallelism != 3 {
		t.Errorf("parallelism: got %d, want 3", p.Parallelism)
	}
	if p.Benches.Network.Duration != 10*time.Second {
		t.Errorf("network duration: got %v, want 10s", p.Benches.Network.Duration)
	}
}

func TestLoadProfile_AllNamed(t *testing.T) {
	for _, name := range []string{"quick", "standard", "deep"} {
		if _, err := LoadProfile(name); err != nil {
			t.Errorf("LoadProfile(%q): %v", name, err)
		}
	}
}

func TestLoadProfile_Unknown(t *testing.T) {
	_, err := LoadProfile("nosuch")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestLoadProfile_Quick_ExtendedBenches(t *testing.T) {
	p, err := LoadProfile("quick")
	if err != nil {
		t.Fatal(err)
	}
	if p.Benches.Storage.Duration != 15*time.Second {
		t.Errorf("storage duration: %v", p.Benches.Storage.Duration)
	}
	if p.Benches.Storage.Size != "1Gi" {
		t.Errorf("storage size: %q", p.Benches.Storage.Size)
	}
	if len(p.Benches.Storage.BlockSizes) != 2 {
		t.Errorf("block sizes: %v", p.Benches.Storage.BlockSizes)
	}
	if p.Benches.DNS.QueriesPerSecond != 50 {
		t.Errorf("dns qps: %d", p.Benches.DNS.QueriesPerSecond)
	}
	if len(p.Benches.API.Operations) != 3 {
		t.Errorf("api operations: %v", p.Benches.API.Operations)
	}
	if p.Benches.Pod.SampleCount != 5 {
		t.Errorf("pod sample count: %d", p.Benches.Pod.SampleCount)
	}
	if p.Benches.Pod.PauseImage == "" {
		t.Error("pod pause image empty")
	}
}
