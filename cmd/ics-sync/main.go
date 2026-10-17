// ics-sync pushes a GitLab repo's content into ICS as a new asset version.
//
// Reads .inprojects.ics (YAML) for the binding ID + publish source path,
// streams a deterministic tar.gz of the source into a temp file while
// computing its sha256 in one pass (no second read), signs the manifest
// with the Ed25519 private key from --private-key or $ICS_PRIVATE_KEY
// (literal PEM or `@path`), POSTs multipart-streamed to
// $ICS_ENDPOINT/api/bindings/{id}/sync.
//
// Manifest schema: the ICS backend's `Manifest` model in
// backend/app/routes/bindings/_models.py is the source of truth. Keep field
// names and types in sync.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/inprojectspl/ics-sync/internal/bundle"
	"github.com/inprojectspl/ics-sync/internal/config"
)

const httpTimeout = 5 * time.Minute

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ics-sync:", err)
		os.Exit(1)
	}
}

func run() error {
	// A flag set per call, so tests can run the CLI more than once.
	flags := flag.NewFlagSet("ics-sync", flag.ExitOnError)
	cfgPath := flags.String("config", ".inprojects.ics", "path to .inprojects.ics")
	privKeyArg := flags.String("private-key", "", "PEM private key: literal PEM or @path (default $ICS_PRIVATE_KEY)")
	endpoint := flags.String("endpoint", "", "ICS base URL (default $ICS_ENDPOINT)")
	commitSha := flags.String("commit-sha", "", "git commit SHA (default $CI_COMMIT_SHA, then 'manual')")
	commitTS := flags.String("commit-ts", "", "RFC3339 commit timestamp (default $CI_COMMIT_TIMESTAMP, then now)")
	verbose := flags.Bool("v", false, "verbose output")
	showVersion := flags.Bool("version", false, "print the version and exit")
	_ = flags.Parse(os.Args[1:])

	if *showVersion {
		fmt.Println("ics-sync", version)
		return nil
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if *verbose {
		fmt.Fprintf(os.Stderr, "binding %s, source=%s kind=%s\n",
			cfg.BindingID, cfg.Publish.Source, cfg.Publish.Kind)
	}

	pkPEM, err := resolvePrivateKey(*privKeyArg)
	if err != nil {
		return fmt.Errorf("private key: %w", err)
	}
	sk, err := parseEd25519PEM(pkPEM)
	if err != nil {
		return fmt.Errorf("private key parse: %w", err)
	}

	url := strings.TrimRight(envOrFlag(*endpoint, "ICS_ENDPOINT"), "/")
	if url == "" {
		return errors.New("missing endpoint (--endpoint or $ICS_ENDPOINT)")
	}

	sha := envOrFlag(*commitSha, "CI_COMMIT_SHA")
	if sha == "" {
		sha = "manual"
	}
	tsRaw := envOrFlag(*commitTS, "CI_COMMIT_TIMESTAMP")
	var ts time.Time
	if tsRaw == "" {
		ts = time.Now().UTC()
	} else {
		ts, err = time.Parse(time.RFC3339, tsRaw)
		if err != nil {
			return fmt.Errorf("commit-ts: %w", err)
		}
	}

	srcAbs, err := filepath.Abs(cfg.Publish.Source)
	if err != nil {
		return fmt.Errorf("source path: %w", err)
	}

	// Single-pass build: bundle bytes hit MultiWriter(tmpFile, hasher) once.
	// Peak RAM is about the gzip window plus one file buffer, whatever the tree size.
	tmp, err := os.CreateTemp("", "ics-sync-*.tar.gz")
	if err != nil {
		return fmt.Errorf("temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	hasher := sha256.New()
	// TMPDIR may lie under the source (a `.tmp` in the checkout): the
	// archive being written must not pack itself.
	tmpAbs, err := filepath.Abs(tmp.Name())
	if err != nil {
		tmp.Close()
		return fmt.Errorf("temp: %w", err)
	}
	count, err := bundle.Write(io.MultiWriter(tmp, hasher), srcAbs, cfg.Publish.Exclude, tmpAbs)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("bundle: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	bundleSha := fmt.Sprintf("%x", hasher.Sum(nil))
	if *verbose {
		st, _ := os.Stat(tmp.Name())
		fmt.Fprintf(os.Stderr, "bundle: %d files, %d bytes, sha256=%s...\n",
			count, st.Size(), bundleSha[:16])
	}

	manifest := struct {
		BindingID       string `json:"binding_id"`
		CommitSha       string `json:"commit_sha"`
		CommitTimestamp string `json:"commit_timestamp"`
		BundleSha256    string `json:"bundle_sha256"`
	}{
		BindingID:       cfg.BindingID,
		CommitSha:       sha,
		CommitTimestamp: ts.Format(time.RFC3339),
		BundleSha256:    bundleSha,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	sig := ed25519.Sign(sk, manifestBytes)
	sigB64 := strings.TrimRight(base64.URLEncoding.EncodeToString(sig), "=")

	syncURL := fmt.Sprintf("%s/api/bindings/%s/sync", url, cfg.BindingID)
	resp, err := postMultipartStream(syncURL, manifestBytes, []byte(sigB64), tmp.Name())
	if err != nil {
		return fmt.Errorf("POST %s: %w", syncURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: reading the response: %w", syncURL, err)
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("%s returned %d: %s", syncURL, resp.StatusCode, string(body))
	}
	if err := checkSynced(body); err != nil {
		return fmt.Errorf("%s: %w: %s", syncURL, err, string(body))
	}
	fmt.Println(string(body))
	return nil
}

// checkSynced accepts only the answer ICS gives for a stored version, so a
// proxy page or a cut-off body is not taken for a successful sync.
func checkSynced(body []byte) error {
	var out struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.Version < 1 {
		return errors.New("unexpected response, not a new version from ICS")
	}
	return nil
}

func envOrFlag(flagVal, envName string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv(envName)
}

func resolvePrivateKey(arg string) (string, error) {
	val := envOrFlag(arg, "ICS_PRIVATE_KEY")
	if val == "" {
		return "", errors.New("missing key (--private-key or $ICS_PRIVATE_KEY)")
	}
	if strings.HasPrefix(val, "@") {
		path := strings.TrimPrefix(val, "@")
		data, err := os.ReadFile(path)
		if err != nil {
			// Never echo the value: with a text CI variable it is the key itself.
			if strings.Contains(path, "-----BEGIN") {
				return "", errors.New("the value after @ is a PEM key, not a file path; pass it without @")
			}
			var pathErr *fs.PathError
			if errors.As(err, &pathErr) {
				err = pathErr.Err
			}
			return "", fmt.Errorf("cannot read the key file: %w", err)
		}
		return string(data), nil
	}
	return val, nil
}

func parseEd25519PEM(pemStr string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("not a PEM block")
	}
	if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("unexpected PEM type: %s", block.Type)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	sk, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("expected Ed25519, got %T", key)
	}
	return sk, nil
}

// postMultipartStream feeds the request body via io.Pipe so the bundle is
// streamed from disk through the multipart writer to the HTTP socket
// without ever sitting fully in RAM.
func postMultipartStream(url string, manifest, sig []byte, bundlePath string) (*http.Response, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		var err error
		defer func() {
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			pw.Close()
		}()
		defer mw.Close()
		if err = writePart(mw, "manifest", "manifest.json", manifest); err != nil {
			return
		}
		if err = writePart(mw, "signature", "signature.txt", sig); err != nil {
			return
		}
		var f *os.File
		f, err = os.Open(bundlePath)
		if err != nil {
			return
		}
		defer f.Close()
		var w io.Writer
		w, err = mw.CreateFormFile("bundle", "bundle.tar.gz")
		if err != nil {
			return
		}
		_, err = io.Copy(w, f)
	}()

	req, err := http.NewRequest("POST", url, pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("User-Agent", "ics-sync/"+version)
	// A redirect would turn the POST into a GET of another page (a login
	// page behind a proxy, say), so the first answer is the one that counts.
	client := &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return client.Do(req)
}

func writePart(mw *multipart.Writer, field, filename string, data []byte) error {
	w, err := mw.CreateFormFile(field, filename)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
