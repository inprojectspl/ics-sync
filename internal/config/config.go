// Package config parses .inprojects.ics, the per-repo binding manifest
// committed to the GitLab project root.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version   int               `yaml:"version"`
	BindingID string            `yaml:"binding_id"`
	PublicKey string            `yaml:"public_key"`
	Publish   Publish           `yaml:"publish"`
	Metadata  map[string]string `yaml:"metadata"`
}

type Publish struct {
	Source  string   `yaml:"source"`
	Kind    string   `yaml:"kind"`
	Exclude []string `yaml:"exclude"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.Version != 1 {
		return nil, fmt.Errorf("unsupported version: %d (want 1)", c.Version)
	}
	if c.BindingID == "" {
		return nil, fmt.Errorf("binding_id missing")
	}
	if c.Publish.Source == "" {
		c.Publish.Source = "."
	}
	return &c, nil
}
