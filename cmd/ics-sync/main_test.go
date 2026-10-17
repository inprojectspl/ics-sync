package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testKeyPEM(t *testing.T) (ed25519.PublicKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return pub, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestResolvePrivateKey(t *testing.T) {
	_, keyPEM := testKeyPEM(t)
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, []byte(keyPEM), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := resolvePrivateKey("@" + path)
	if err != nil || got != keyPEM {
		t.Fatalf("@file: got %q, %v", got, err)
	}

	t.Setenv("ICS_PRIVATE_KEY", keyPEM)
	if got, err := resolvePrivateKey(""); err != nil || got != keyPEM {
		t.Fatalf("env fallback: got %q, %v", got, err)
	}

	t.Setenv("ICS_PRIVATE_KEY", "")
	if _, err := resolvePrivateKey(""); err == nil {
		t.Fatal("expected an error without a key")
	}
}

// A text CI variable passed with @ holds the key itself; the error must not
// repeat it, or the private key ends up in the job log.
func TestResolvePrivateKeyNeverEchoesTheValue(t *testing.T) {
	_, keyPEM := testKeyPEM(t)
	for _, arg := range []string{"@" + keyPEM, "@/no/such/secret-path.pem"} {
		_, err := resolvePrivateKey(arg)
		if err == nil {
			t.Fatalf("%q: expected an error", arg[:12])
		}
		if strings.Contains(err.Error(), strings.TrimPrefix(arg, "@")) {
			t.Fatalf("error repeats the value: %v", err)
		}
	}
}

func TestParseEd25519PEM(t *testing.T) {
	pub, keyPEM := testKeyPEM(t)
	sk, err := parseEd25519PEM(keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Equal(sk.Public()) {
		t.Fatal("parsed key does not match")
	}

	if _, err := parseEd25519PEM("not a key"); err == nil {
		t.Fatal("expected an error for garbage")
	}
	pubDER, _ := x509.MarshalPKIXPublicKey(pub)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	if _, err := parseEd25519PEM(pubPEM); err == nil {
		t.Fatal("expected an error for a public key")
	}
}

// TestRunPostsSignedManifest drives the whole CLI against a fake ICS and
// checks the wire contract the backend verifies: form fields, unpadded
// URL-safe base64 signature over the exact manifest bytes, and no .git.
func TestRunPostsSignedManifest(t *testing.T) {
	pub, keyPEM := testKeyPEM(t)
	const bindingID = "11111111-2222-3333-4444-555555555555"

	var gotManifest, gotSig []byte
	var gotBundleLen int64
	var gotPath, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotUA = r.URL.Path, r.Header.Get("User-Agent")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		read := func(field string) []byte {
			f, _, err := r.FormFile(field)
			if err != nil {
				t.Errorf("field %s: %v", field, err)
				return nil
			}
			defer f.Close()
			b, _ := io.ReadAll(f)
			return b
		}
		gotManifest, gotSig = read("manifest"), read("signature")
		gotBundleLen = int64(len(read("bundle")))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"version":2}`))
	}))
	defer srv.Close()

	repo := t.TempDir()
	cfg := "version: 1\nbinding_id: " + bindingID + "\npublish:\n  source: .\n  kind: md_collection\n"
	for name, body := range map[string]string{".inprojects.ics": cfg, "README.md": "# hi", ".git/config": "token"} {
		p := filepath.Join(repo, name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(repo)
	t.Setenv("ICS_PRIVATE_KEY", keyPEM)
	t.Setenv("ICS_ENDPOINT", srv.URL+"/")
	t.Setenv("CI_COMMIT_SHA", "abc123")
	t.Setenv("CI_COMMIT_TIMESTAMP", "2026-09-30T10:00:00+02:00")
	os.Args = []string{"ics-sync"}

	if err := run(); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/bindings/"+bindingID+"/sync" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotUA != "ics-sync/dev" {
		t.Fatalf("user agent = %q", gotUA)
	}
	want := `{"binding_id":"` + bindingID + `","commit_sha":"abc123","commit_timestamp":"2026-09-30T10:00:00+02:00","bundle_sha256":"`
	if string(gotManifest[:len(want)]) != want {
		t.Fatalf("manifest = %s", gotManifest)
	}
	sig, err := base64.RawURLEncoding.DecodeString(string(gotSig))
	if err != nil {
		t.Fatalf("signature is not unpadded url-safe base64: %v", err)
	}
	if !ed25519.Verify(pub, gotManifest, sig) {
		t.Fatal("signature does not verify")
	}
	if gotBundleLen == 0 {
		t.Fatal("empty bundle")
	}
}
