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

// A misspelt key must fail: `soruce` would fall back to "." and publish the
// whole working directory, `exlude` would drop every exclude.
func TestLoadRejectsUnknownKeys(t *testing.T) {
	cases := map[string]string{
		"top level": "version: 1\nbinding_id: x\npublsh:\n  source: docs\n",
		"source":    "version: 1\nbinding_id: x\npublish:\n  soruce: docs\n",
		"exclude":   "version: 1\nbinding_id: x\npublish:\n  source: .\n  exlude:\n    - internal/**\n",
	}
	for name, body := range cases {
		if _, err := load(t, body); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestLoadRejectsBadExcludePattern(t *testing.T) {
	_, err := load(t, "version: 1\nbinding_id: x\npublish:\n  exclude:\n    - \"docs/[draft\"\n")
	if err == nil {
		t.Fatal("expected an error for a malformed glob")
	}
}

func TestLoadAcceptsTheFileICSGenerates(t *testing.T) {
	body := "version: 1\nbinding_id: x\npublic_key: |\n  -----BEGIN PUBLIC KEY-----\n  MCowBQYDK2VwAyEA\n  -----END PUBLIC KEY-----\n" +
		"publish:\n  source: .\n  kind: md_collection\n  # exclude:\n  #   - \"node_modules/**\"\nmetadata:\n  title: \"Team handbook\"\n"
	c, err := load(t, body)
	if err != nil {
		t.Fatal(err)
	}
	if c.Metadata["title"] != "Team handbook" || c.Publish.Source != "." {
		t.Fatalf("config = %+v", c)
	}
}
