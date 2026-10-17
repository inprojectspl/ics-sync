// Package bundle builds a deterministic gzip-compressed tarball from a
// directory tree, with glob-style excludes and stable metadata so the
// bundle_sha256 in the manifest is reproducible across runs (the same
// source tree always gives the same hash).
package bundle

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// zeroTime is 1970-01-01T00:00:01Z: epoch zero trips some legacy tar readers;
// 1s is fine and keeps the bundle reproducible.
var zeroTime = time.Unix(1, 0).UTC()

// Write streams a deterministic tar.gz of `root` into `w`. Files added in
// sorted order, fixed mode 0644, ModTime = zeroTime, PAX format. Each
// file is io.Copy'd directly into the tar writer, never materialized as
// a separate []byte, so peak RAM stays at the gzip window plus one open
// file regardless of the source tree's total size.
//
// Git metadata is always left out: a CI checkout's .git holds the whole
// history and often the job token in .git/config. Symlinks and other
// non-regular files are skipped, so nothing outside `root` is read. `skip`
// names absolute paths left out as well, such as the file `w` writes to
// when it lies under `root`.
func Write(w io.Writer, root string, exclude []string, skip ...string) (int, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(rootAbs)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("%s is not a directory", rootAbs)
	}

	var paths []string
	walkErr := filepath.WalkDir(rootAbs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" && p != rootAbs {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() || slices.Contains(skip, p) {
			return nil
		}
		rel, err := filepath.Rel(rootAbs, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if isExcluded(rel, exclude) {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	if walkErr != nil {
		return 0, walkErr
	}
	sort.Strings(paths)

	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	for _, rel := range paths {
		abs := filepath.Join(rootAbs, filepath.FromSlash(rel))
		st, err := os.Stat(abs)
		if err != nil {
			return 0, err
		}
		hdr := &tar.Header{
			Name:    rel,
			Mode:    0o644,
			Size:    st.Size(),
			ModTime: zeroTime,
			Uid:     0,
			Gid:     0,
			Uname:   "",
			Gname:   "",
			Format:  tar.FormatPAX,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return 0, err
		}
		f, err := os.Open(abs)
		if err != nil {
			return 0, err
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return 0, err
		}
		if err := f.Close(); err != nil {
			return 0, err
		}
	}
	if err := tw.Close(); err != nil {
		return 0, err
	}
	if err := gw.Close(); err != nil {
		return 0, err
	}
	return len(paths), nil
}

func isExcluded(rel string, patterns []string) bool {
	for _, pat := range patterns {
		// `node_modules/**` style: filepath.Match does not know `**`, so a
		// trailing `/**` is lowered to "starts with prefix/".
		if strings.HasSuffix(pat, "/**") {
			prefix := strings.TrimSuffix(pat, "/**")
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
			continue
		}
		if ok, _ := filepath.Match(pat, rel); ok {
			return true
		}
		base := filepath.Base(rel)
		if ok, _ := filepath.Match(pat, base); ok {
			return true
		}
	}
	return false
}
