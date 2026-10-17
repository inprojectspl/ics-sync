// Package config parses .inprojects.ics, the per-repo binding manifest
// committed to the GitLab project root.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	// Entry is accepted from older binding files and not used: ICS picks
	// the entry file itself.
	Entry string `yaml:"entry"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	// Unknown keys are errors: a misspelt `source` or `exclude` would
	// otherwise publish the whole working directory.
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil && !errors.Is(err, io.EOF) {
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
	for _, pat := range c.Publish.Exclude {
		if _, err := filepath.Match(strings.TrimSuffix(pat, "/**"), ""); err != nil {
			return nil, fmt.Errorf("exclude %q: %w", pat, err)
		}
	}
	return &c, nil
}
