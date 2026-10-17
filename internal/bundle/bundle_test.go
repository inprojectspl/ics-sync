package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func readTar(t *testing.T, data []byte) map[string]*tar.Header {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gr)
	out := map[string]*tar.Header{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out[h.Name] = h
	}
}

func names(headers map[string]*tar.Header) []string {
	var out []string
	for n := range headers {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func TestWriteIsDeterministic(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"b.md": "b", "a/x.md": "x", "café.md": "utf-8 name"})

	var first, second bytes.Buffer
	if _, err := Write(&first, root, nil); err != nil {
		t.Fatal(err)
	}
	// A different mtime must not change the bundle.
	future := zeroTime.AddDate(50, 0, 0)
	if err := os.Chtimes(filepath.Join(root, "b.md"), future, future); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(&second, root, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("same tree produced different bundles")
	}

	headers := readTar(t, first.Bytes())
	if got, want := names(headers), []string{"a/x.md", "b.md", "café.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	for _, h := range headers {
		if h.Mode != 0o644 || !h.ModTime.Equal(zeroTime) || h.Uid != 0 || h.Gid != 0 {
			t.Fatalf("unstable metadata on %s: %+v", h.Name, h)
		}
	}
}

func TestWriteSkipsGitMetadata(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		".git/config":       "[remote] url = https://gitlab-ci-token:secret@example.com/r.git",
		".git/objects/ab/c": "blob",
		"vendor/lib/.git":   "gitdir: ../../.git/modules/lib",
		".gitignore":        "*.tmp",
		"README.md":         "# hi",
	})

	var buf bytes.Buffer
	count, err := Write(&buf, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(readTar(t, buf.Bytes())), []string{".gitignore", "README.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func TestWriteSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeTree(t, root, map[string]string{"doc.md": "doc"})
	writeTree(t, outside, map[string]string{"secret.pem": "key"})
	if err := os.Symlink(filepath.Join(outside, "secret.pem"), filepath.Join(root, "key.pem")); err != nil {
		t.Skip("symlinks not supported:", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked-dir")); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := Write(&buf, root, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := names(readTar(t, buf.Bytes())), []string{"doc.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
}

func TestIsExcluded(t *testing.T) {
	patterns := []string{"node_modules/**", "*.test.*", "docs/draft.md"}
	cases := map[string]bool{
		"node_modules":        true,
		"node_modules/a/b.js": true,
		"node_modules_old/a":  false,
		"src/app.test.ts":     true,
		"docs/draft.md":       true,
		"docs/final.md":       false,
		"deep/node_modules/x": false,
		"README.md":           false,
	}
	for rel, want := range cases {
		if got := isExcluded(rel, patterns); got != want {
			t.Errorf("isExcluded(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestWriteRejectsFileRoot(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"one.md": "x"})
	if _, err := Write(io.Discard, filepath.Join(root, "one.md"), nil); err == nil {
		t.Fatal("expected an error for a file root")
	}
}
