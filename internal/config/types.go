package config

import (
	"fmt"
	"time"
)

// RunConfig is the final, merged configuration for a single CLI invocation.
type RunConfig struct {
	Namespace   string
	Profile     string // "quick" | "standard" | "deep"
	Parallelism int
	Timeout     time.Duration
	Keep        bool
	Yes         bool

	Excludes ExcludeConfig
	Includes IncludeConfig
	Images   ImageConfig
	Benches  BenchesConfig
}

type ExcludeConfig struct {
	StorageClasses []string
	Nodes          []string
	Benches        []string
}

// IncludeConfig is a whitelist. When a slice is non-empty, only listed items
// are considered (Excludes are still applied on top).
type IncludeConfig struct {
	StorageClasses []string
}

type ImageConfig struct {
	Registry    string
	Runner      RunnerImage
	PullSecrets []string
}

type RunnerImage struct {
	Repository string
	Tag        string
	Digest     string
	PullPolicy string // "IfNotPresent" (default) | "Always" | "Never"
}

// FullRef returns "<registry>/<repository>:<tag>" with optional "@<digest>".
func (r RunnerImage) FullRef(registry string) string {
	ref := registry + "/" + r.Repository + ":" + r.Tag
	if r.Digest != "" {
		ref += "@" + r.Digest
	}
	return ref
}

type BenchesConfig struct {
	Network NetworkBenchConfig `yaml:"network"`
	Storage StorageBenchConfig `yaml:"storage"`
	DNS     DNSBenchConfig     `yaml:"dns"`
	API     APIBenchConfig     `yaml:"api"`
	Pod     PodBenchConfig     `yaml:"pod"`
}

type NetworkBenchConfig struct {
	Duration     time.Duration `yaml:"-"`
	Protocols    []string      `yaml:"protocols"`    // "tcp", "udp"
	Combinations []string      `yaml:"combinations"` // "same-node", "cross-node"
}

// networkBenchConfigRaw is used internally for YAML unmarshaling.
type networkBenchConfigRaw struct {
	Duration     string   `yaml:"duration"`
	Protocols    []string `yaml:"protocols"`
	Combinations []string `yaml:"combinations"`
}

func (n *NetworkBenchConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw networkBenchConfigRaw
	if err := unmarshal(&raw); err != nil {
		return err
	}
	d, err := time.ParseDuration(raw.Duration)
	if err != nil {
		return fmt.Errorf("invalid network duration %q: %w", raw.Duration, err)
	}
	n.Duration = d
	n.Protocols = raw.Protocols
	n.Combinations = raw.Combinations
	return nil
}

// ProfileConfig is the structure of YAML profile files (quick, standard, deep).
type ProfileConfig struct {
	Name        string        `yaml:"-"`
	Parallelism int           `yaml:"parallelism"`
	Timeout     time.Duration `yaml:"-"`
	Benches     BenchesConfig `yaml:"benches"`
}

// profileConfigRaw is used internally for YAML unmarshaling.
type profileConfigRaw struct {
	Name        string `yaml:"name"`
	Parallelism int    `yaml:"parallelism"`
	Timeout     string `yaml:"timeout"`
}

func (p *ProfileConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw profileConfigRaw
	if err := unmarshal(&raw); err != nil {
		return err
	}
	d, err := time.ParseDuration(raw.Timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout %q: %w", raw.Timeout, err)
	}
	p.Name = raw.Name
	p.Parallelism = raw.Parallelism
	p.Timeout = d

	// Unmarshal benches separately
	var benches struct {
		Benches BenchesConfig `yaml:"benches"`
	}
	if err := unmarshal(&benches); err != nil {
		return err
	}
	p.Benches = benches.Benches
	return nil
}

// --- Storage bench ---

type StorageBenchConfig struct {
	Duration   time.Duration `yaml:"-"`
	Size       string        `yaml:"size"`        // PVC size, e.g. "1Gi"
	BlockSizes []string      `yaml:"block_sizes"` // fio bs, e.g. "4k", "1M"
	Patterns   []string      `yaml:"patterns"`    // randread, randwrite, seqread, seqwrite
}

type storageBenchConfigRaw struct {
	Duration   string   `yaml:"duration"`
	Size       string   `yaml:"size"`
	BlockSizes []string `yaml:"block_sizes"`
	Patterns   []string `yaml:"patterns"`
}

func (s *StorageBenchConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw storageBenchConfigRaw
	if err := unmarshal(&raw); err != nil {
		return err
	}
	d, err := time.ParseDuration(raw.Duration)
	if err != nil {
		return fmt.Errorf("invalid storage duration %q: %w", raw.Duration, err)
	}
	s.Duration = d
	s.Size = raw.Size
	s.BlockSizes = raw.BlockSizes
	s.Patterns = raw.Patterns
	return nil
}

// --- DNS bench ---

type DNSBenchConfig struct {
	Duration         time.Duration `yaml:"-"`
	QueriesPerSecond int           `yaml:"queries_per_second"`
	Targets          []string      `yaml:"targets"` // "in-cluster", or fqdn list
}

type dnsBenchConfigRaw struct {
	Duration         string   `yaml:"duration"`
	QueriesPerSecond int      `yaml:"queries_per_second"`
	Targets          []string `yaml:"targets"`
}

func (d *DNSBenchConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw dnsBenchConfigRaw
	if err := unmarshal(&raw); err != nil {
		return err
	}
	dur, err := time.ParseDuration(raw.Duration)
	if err != nil {
		return fmt.Errorf("invalid dns duration %q: %w", raw.Duration, err)
	}
	d.Duration = dur
	d.QueriesPerSecond = raw.QueriesPerSecond
	d.Targets = raw.Targets
	return nil
}

// --- API bench ---

type APIBenchConfig struct {
	Duration   time.Duration `yaml:"-"`
	Operations []string      `yaml:"operations"` // list-pods, list-namespaces, get-pod, watch-pods
}

type apiBenchConfigRaw struct {
	Duration   string   `yaml:"duration"`
	Operations []string `yaml:"operations"`
}

func (a *APIBenchConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw apiBenchConfigRaw
	if err := unmarshal(&raw); err != nil {
		return err
	}
	d, err := time.ParseDuration(raw.Duration)
	if err != nil {
		return fmt.Errorf("invalid api duration %q: %w", raw.Duration, err)
	}
	a.Duration = d
	a.Operations = raw.Operations
	return nil
}

// --- Pod bench ---

type PodBenchConfig struct {
	SampleCount int           `yaml:"sample_count"`
	PauseImage  string        `yaml:"pause_image"`
	CPUDuration time.Duration `yaml:"-"`
}

type podBenchConfigRaw struct {
	SampleCount int    `yaml:"sample_count"`
	PauseImage  string `yaml:"pause_image"`
	CPUDuration string `yaml:"cpu_duration"`
}

func (p *PodBenchConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw podBenchConfigRaw
	if err := unmarshal(&raw); err != nil {
		return err
	}
	d, err := time.ParseDuration(raw.CPUDuration)
	if err != nil {
		return fmt.Errorf("invalid pod cpu_duration %q: %w", raw.CPUDuration, err)
	}
	p.SampleCount = raw.SampleCount
	p.PauseImage = raw.PauseImage
	p.CPUDuration = d
	return nil
}
