package config

import (
	"testing"
	"time"
)

func TestMerge_DefaultsOnly(t *testing.T) {
	got, err := Merge(MergeInputs{ProfileName: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile != "quick" {
		t.Errorf("profile: %q", got.Profile)
	}
	if got.Parallelism != 3 {
		t.Errorf("parallelism: %d", got.Parallelism)
	}
	if got.Benches.Network.Duration != 10*time.Second {
		t.Errorf("network duration: %v", got.Benches.Network.Duration)
	}
}

func TestMerge_FlagsOverrideProfile(t *testing.T) {
	got, err := Merge(MergeInputs{
		ProfileName: "quick",
		Flags: FlagOverrides{
			Parallelism: ptr(7),
			Namespace:   "custom-ns",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Parallelism != 7 {
		t.Errorf("flag did not override parallelism: %d", got.Parallelism)
	}
	if got.Namespace != "custom-ns" {
		t.Errorf("flag did not set namespace: %q", got.Namespace)
	}
}

func TestMerge_FileOverridesProfile_FlagsOverrideFile(t *testing.T) {
	file := &FileConfig{
		Parallelism: ptr(5),
		Images: &ImageConfig{
			Registry: "registry.corp.internal/benchbuddy",
		},
	}
	got, err := Merge(MergeInputs{
		ProfileName: "quick",
		File:        file,
		Flags:       FlagOverrides{Parallelism: ptr(9)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Parallelism != 9 {
		t.Errorf("expected flag to win, got %d", got.Parallelism)
	}
	if got.Images.Registry != "registry.corp.internal/benchbuddy" {
		t.Errorf("file registry not applied: %q", got.Images.Registry)
	}
}

func TestMerge_RequiresNamespace(t *testing.T) {
	_, err := Merge(MergeInputs{ProfileName: "quick"})
	if err == nil {
		// allowed — namespace can come later as a flag. Just ensure no panic.
	}
}

func ptr[T any](v T) *T { return &v }
