package config

import (
	"os"
	"path/filepath"
	"testing"
)

func load(t *testing.T, body string) (*Config, error) {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".inprojects.ics")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(p)
}

func TestLoadDefaultsSource(t *testing.T) {
	c, err := load(t, "version: 1\nbinding_id: 00000000-0000-0000-0000-000000000000\npublish:\n  kind: md_collection\n")
	if err != nil {
		t.Fatal(err)
	}
	if c.Publish.Source != "." {
		t.Fatalf("source = %q, want %q", c.Publish.Source, ".")
	}
}

// Older binding files may carry `publish.entry`; ICS picks the entry file
// itself, so the key is ignored rather than rejected.
func TestLoadIgnoresLegacyEntry(t *testing.T) {
	c, err := load(t, "version: 1\nbinding_id: 00000000-0000-0000-0000-000000000000\npublish:\n  source: docs\n  kind: md_collection\n  entry: README.md\n")
	if err != nil {
		t.Fatal(err)
	}
	if c.Publish.Source != "docs" || c.Publish.Kind != "md_collection" {
		t.Fatalf("publish = %+v", c.Publish)
	}
}

func TestLoadRejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"unknown version":    "version: 2\nbinding_id: x\n",
		"missing version":    "binding_id: x\n",
		"missing binding id": "version: 1\n",
		"not yaml":           "version: [1\n",
	}
	for name, body := range cases {
		if _, err := load(t, body); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
