package mcpserve

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/store"
)

// asdSampleSpec is the one shared ASD-tool fixture spec (mirrors
// internal/designapp's own testSpec/internal/draftmutation's own
// baseSpec fixtures — kept independent per package, never shared across
// a package boundary in Go).
const asdSampleSpec = `---
id: spec/sample
kind: spec
class: feature
title: Sample
owners: [platform-team]
problem: { text: "old problem", anchor: "#problem" }
outcome: { text: "old outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "first", evidence: [static], anchor: "#ac-1" }
---
# Sample

## Problem

Old prose stays.

## Outcome

Old prose stays.

## ac-1

First.
`

// newASDTestBackend builds a hermetic fixturegit repository carrying the
// existing internal/policyauthority ASD policy fixture (design_assistance
// mode draft-write), on a checked-out design/sample branch, with
// asdSampleSpec written at the active spec path, and returns a Backend
// rooted at it plus the resolved checkout root. CI_DEFAULT_BRANCH is
// pinned (t.Setenv) since this fixture carries no "origin" remote for
// specstate's own hermetic fallback to resolve otherwise.
func newASDTestBackend(t *testing.T) (*Backend, string) {
	t.Helper()
	return newASDTestBackendWithMode(t, "draft-write")
}

// offModeASDTestBackend is newASDTestBackend with design_assistance mode
// off, for proving a delegated agent is refused when policy forbids
// writes.
func offModeASDTestBackend(t *testing.T) (*Backend, string) {
	t.Helper()
	return newASDTestBackendWithMode(t, "off")
}

func newASDTestBackendWithMode(t *testing.T, mode string) (*Backend, string) {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	files := map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"}
	source := filepath.Join("..", "policyauthority", "testdata", "store")
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if entry.Name() == "go-toolchain.md" {
			data = bytes.Replace(data, []byte("mode: proposal-only"), []byte("mode: "+mode), 1)
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "adopt draft mutation policy"}})

	checkout := exec.Command("git", "checkout", "-b", "design/sample")
	checkout.Dir = repo.Dir
	if output, err := checkout.CombinedOutput(); err != nil {
		t.Fatalf("git checkout design/sample: %v\n%s", err, output)
	}

	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.ToSlash(resolved)

	specDir := store.SpecDir(root, store.ZoneActive, "sample")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.SpecPath(root, store.ZoneActive, "sample"), []byte(asdSampleSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	return &Backend{Root: root}, root
}
