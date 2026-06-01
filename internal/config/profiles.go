package config

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed profiles/*.yaml
var profileFS embed.FS

// LoadProfile loads a named profile from the embedded YAML files.
func LoadProfile(name string) (ProfileConfig, error) {
	data, err := profileFS.ReadFile("profiles/" + name + ".yaml")
	if err != nil {
		return ProfileConfig{}, fmt.Errorf("unknown profile %q: %w", name, err)
	}
	var p ProfileConfig
	if err := yaml.Unmarshal(data, &p); err != nil {
		return ProfileConfig{}, fmt.Errorf("parse profile %q: %w", name, err)
	}
	return p, nil
}

// AvailableProfiles returns the names of all profiles bundled into the binary.
func AvailableProfiles() []string {
	entries, _ := profileFS.ReadDir("profiles")
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		n := e.Name()
		if len(n) > 5 && n[len(n)-5:] == ".yaml" {
			names = append(names, n[:len(n)-5])
		}
	}
	return names
}
