package config

import (
	"time"
)

// FileConfig is the structure parsed from a user-provided --config YAML file.
// Pointers are used so that "unset" is distinguishable from "zero".
type FileConfig struct {
	Namespace   *string        `yaml:"namespace,omitempty"`
	Profile     *string        `yaml:"profile,omitempty"`
	Parallelism *int           `yaml:"parallelism,omitempty"`
	Timeout     *time.Duration `yaml:"timeout,omitempty"`
	Images      *ImageConfig   `yaml:"images,omitempty"`
}

// FlagOverrides captures the subset of RunConfig that can be set via CLI flags.
// Pointers signal "user explicitly provided this flag".
type FlagOverrides struct {
	Namespace   string
	Profile     string
	Parallelism *int
	Timeout     *time.Duration
	Keep        bool
	Yes         bool

	Excludes ExcludeConfig
	Includes IncludeConfig

	Registry        string
	RunnerImageRepo string
	RunnerImageTag  string
	RunnerDigest    string
	PauseImage      string
	PullSecrets     []string
	PullPolicy      string
}

type MergeInputs struct {
	ProfileName string
	File        *FileConfig
	Flags       FlagOverrides
}

// Merge applies precedence: compiled defaults < profile < file < CLI flags.
func Merge(in MergeInputs) (RunConfig, error) {
	name := in.ProfileName
	if in.File != nil && in.File.Profile != nil {
		name = *in.File.Profile
	}
	if in.Flags.Profile != "" {
		name = in.Flags.Profile
	}
	if name == "" {
		name = "quick"
	}
	profile, err := LoadProfile(name)
	if err != nil {
		return RunConfig{}, err
	}

	cfg := RunConfig{
		Profile:     profile.Name,
		Parallelism: profile.Parallelism,
		Timeout:     profile.Timeout,
		Benches:     profile.Benches,
		Images:      defaultImageConfig(),
	}

	if in.File != nil {
		f := in.File
		if f.Namespace != nil {
			cfg.Namespace = *f.Namespace
		}
		if f.Parallelism != nil {
			cfg.Parallelism = *f.Parallelism
		}
		if f.Timeout != nil {
			cfg.Timeout = *f.Timeout
		}
		if f.Images != nil {
			cfg.Images = mergeImages(cfg.Images, *f.Images)
		}
	}

	fl := in.Flags
	if fl.Namespace != "" {
		cfg.Namespace = fl.Namespace
	}
	if fl.Parallelism != nil {
		cfg.Parallelism = *fl.Parallelism
	}
	if fl.Timeout != nil {
		cfg.Timeout = *fl.Timeout
	}
	cfg.Keep = fl.Keep
	cfg.Yes = fl.Yes
	cfg.Excludes = fl.Excludes
	cfg.Includes = fl.Includes
	if fl.Registry != "" {
		cfg.Images.Registry = fl.Registry
	}
	if fl.RunnerImageRepo != "" {
		cfg.Images.Runner.Repository = fl.RunnerImageRepo
	}
	if fl.RunnerImageTag != "" {
		cfg.Images.Runner.Tag = fl.RunnerImageTag
	}
	if fl.RunnerDigest != "" {
		cfg.Images.Runner.Digest = fl.RunnerDigest
	}
	if len(fl.PullSecrets) > 0 {
		cfg.Images.PullSecrets = fl.PullSecrets
	}
	if fl.PullPolicy != "" {
		cfg.Images.Runner.PullPolicy = fl.PullPolicy
	}
	if fl.PauseImage != "" {
		cfg.Benches.Pod.PauseImage = fl.PauseImage
	}

	return cfg, nil
}

func defaultImageConfig() ImageConfig {
	return ImageConfig{
		Registry: "ghcr.io/klementxv",
		Runner: RunnerImage{
			Repository: "benchbuddy-runner",
			Tag:        "latest",
			PullPolicy: "IfNotPresent",
		},
	}
}

func mergeImages(base, over ImageConfig) ImageConfig {
	if over.Registry != "" {
		base.Registry = over.Registry
	}
	if over.Runner.Repository != "" {
		base.Runner.Repository = over.Runner.Repository
	}
	if over.Runner.Tag != "" {
		base.Runner.Tag = over.Runner.Tag
	}
	if over.Runner.Digest != "" {
		base.Runner.Digest = over.Runner.Digest
	}
	if over.Runner.PullPolicy != "" {
		base.Runner.PullPolicy = over.Runner.PullPolicy
	}
	if len(over.PullSecrets) > 0 {
		base.PullSecrets = over.PullSecrets
	}
	return base
}
