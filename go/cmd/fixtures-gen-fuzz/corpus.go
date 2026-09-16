// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
)

// corpusDirName is the conformance corpus directory inside the
// spec tree, relative to the repository root.
const corpusDirName = "spec/v1.0/conformance"

// corpusRoot resolves the authored conformance corpus for a module
// rooted at module: <module>/../spec/v1.0/conformance in the
// poly-vstar monorepo, where the corpus is authored and the module's
// testdata/ is a generated mirror of it; <module>/testdata on the
// hop-top/vstar mirror, where the module is a subtree of go/ alone
// and there is no sibling spec/ tree.
//
// Generated fixtures always land in the corpus. fixtures-verify then
// mirrors the corpus into testdata/ when the spec tree is present.
func corpusRoot(module string) string {
	candidate := filepath.Join(filepath.Dir(module), corpusDirName)
	if st, err := os.Stat(candidate); err == nil && st.IsDir() {
		return candidate
	}
	return filepath.Join(module, "testdata")
}
