// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// corpusDirName is the conformance corpus directory inside the
// spec tree, relative to the repository root.
const corpusDirName = "spec/v1.0/conformance"

// mirrorExcludes are the top-level entries of testdata/ that are
// Go-native and therefore not part of the spec corpus. The root
// package's go-fuzz corpus (read by fuzz_test.go) lives under
// testdata/fuzz/ and must survive the mirror.
var mirrorExcludes = map[string]bool{"fuzz": true}

// corpusRoot resolves the authored conformance corpus for a module
// rooted at module.
//
// In the poly-vstar monorepo the module is go/ and the spec tree is
// its sibling, so the corpus is <module>/../spec/v1.0/conformance
// and testdata/ is a generated mirror of it. On the hop-top/vstar
// mirror the module is a subtree of go/ alone: there is no ../spec,
// so testdata/ is the corpus itself and nothing is mirrored.
//
// Returns (path, true) when the spec tree is present, and
// (<module>/testdata, false) when it is absent. The same guard
// pattern as validate/codes_doc_test.go: a checkout without the
// monorepo's sibling directories skips the step instead of failing
// it.
func corpusRoot(module string) (string, bool) {
	candidate := filepath.Join(filepath.Dir(module), corpusDirName)
	if st, err := os.Stat(candidate); err == nil && st.IsDir() {
		return candidate, true
	}
	return filepath.Join(module, "testdata"), false
}

// mirrorCorpus replaces dst with a copy of src, preserving every
// top-level entry of dst named in mirrorExcludes. Delete-then-copy
// rather than overlay so a fixture removed upstream disappears from
// the mirror too.
//
// It returns the mirror-relative paths it had to add, change or
// remove. An empty result means the mirror already matched the
// corpus byte-for-byte.
//
// The caller needs that list because rewriting the mirror destroys
// the evidence of a hand-edit: restoring a tampered file leaves the
// working tree clean, so `git diff` alone cannot see that someone
// edited the generated mirror instead of the corpus. Reporting the
// changed paths keeps that edit visible.
func mirrorCorpus(src, dst string) ([]string, error) {
	changed, err := diffTrees(src, dst)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return nil, nil
	}
	if err := clearMirror(dst); err != nil {
		return nil, err
	}
	if err := copyTree(src, dst); err != nil {
		return nil, err
	}
	return changed, nil
}

// diffTrees lists the paths, relative to dst, where the mirror
// deviates from the corpus: files present in one tree only, and
// files whose bytes differ. Entries named in mirrorExcludes are not
// part of the mirror and are never reported.
func diffTrees(src, dst string) ([]string, error) {
	srcFiles, err := treeFiles(src, false)
	if err != nil {
		return nil, err
	}
	dstFiles, err := treeFiles(dst, true)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var changed []string
	for rel := range srcFiles {
		seen[rel] = true
		if !dstFiles[rel] {
			changed = append(changed, rel)
			continue
		}
		same, err := sameFile(filepath.Join(src, rel), filepath.Join(dst, rel))
		if err != nil {
			return nil, err
		}
		if !same {
			changed = append(changed, rel)
		}
	}
	for rel := range dstFiles {
		if !seen[rel] {
			changed = append(changed, rel)
		}
	}
	sort.Strings(changed)
	return changed, nil
}

// treeFiles returns the set of regular-file paths under root,
// relative to it. A missing root is an empty set. With excludeTop
// set, top-level entries named in mirrorExcludes are skipped: they
// belong to the module, not to the corpus.
func treeFiles(root string, excludeTop bool) (map[string]bool, error) {
	out := map[string]bool{}
	if _, err := os.Stat(root); errIsNotExist(err) {
		return out, nil
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if excludeTop && mirrorExcludes[strings.Split(rel, string(filepath.Separator))[0]] {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		out[rel] = true
		return nil
	})
	return out, err
}

// sameFile reports whether two files hold identical bytes.
func sameFile(a, b string) (bool, error) {
	ab, err := os.ReadFile(a)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", a, err)
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", b, err)
	}
	return bytes.Equal(ab, bb), nil
}

// clearMirror removes every top-level entry of dst that the mirror
// owns, leaving mirrorExcludes in place. A missing dst is created.
func clearMirror(dst string) error {
	entries, err := os.ReadDir(dst)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dst, 0o755)
		}
		return fmt.Errorf("read %s: %w", dst, err)
	}
	for _, e := range entries {
		if mirrorExcludes[e.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dst, e.Name())); err != nil {
			return fmt.Errorf("remove %s: %w", filepath.Join(dst, e.Name()), err)
		}
	}
	return nil
}

// copyTree copies every regular file under src into the matching
// path under dst, creating directories as needed. Symlinks and
// other irregular entries are an error: the corpus is plain files.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("mirror %s: not a regular file", path)
		}
		return copyFile(path, target)
	})
}

// copyFile copies one regular file, preserving the 0o644 on-disk
// convention the generators write with.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %s: %w", dst, err)
	}
	return out.Close()
}
