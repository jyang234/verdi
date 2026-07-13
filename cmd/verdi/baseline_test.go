package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/upstream"
)

func specWithImpacts(impacts []string) *artifact.SpecFrontmatter {
	return &artifact.SpecFrontmatter{
		Base:  artifact.Base{ID: "spec/stale-decline", Kind: artifact.KindSpec, Title: "t", Owners: []string{"unassigned"}},
		Class: artifact.ClassFeature, Status: "draft", Story: "jira:LOAN-1482",
		Impacts:            impacts,
		AcceptanceCriteria: []artifact.AcceptanceCriterion{{ID: "ac-1", Text: "x", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}}},
	}
}

// TestRegenerateBaseline_Happy proves regenerateBaseline scopes to the
// spec's impacted services (loansvc, matched by store.Service.Name against
// spec.Impacts) and writes the same four-file bundle shape sync's own
// regeneration writes, keyed by the SPEC ref/commit (RefSlug(spec.id)) so
// the workbench preview matrix actually reaches it (true-closure).
func TestRegenerateBaseline_Happy(t *testing.T) {
	repo := buildPhase7Repo(t)
	var stderr bytes.Buffer
	deps := syncDeps{Runner: fakeGraphRunner(), GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &stderr}

	regenerateBaseline(context.Background(), repo.Dir, repo.Head, specWithImpacts([]string{"loansvc"}), deps, "design start", &stderr)

	dir := filepath.Join(repo.Dir, ".verdi", "data", "derived", "spec--stale-decline", repo.Head)
	for _, name := range derivedFileNames {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v (stderr=%s)", name, err, stderr.String())
		}
	}
}

// TestRegenerateBaseline_NoToolchain proves a nil Runner (no toolchain:
// block in verdi.yaml) is a disclosed, graceful skip — never an error, and
// never a partial write.
func TestRegenerateBaseline_NoToolchain(t *testing.T) {
	repo := buildPhase7Repo(t)
	var stderr bytes.Buffer
	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &stderr}

	regenerateBaseline(context.Background(), repo.Dir, repo.Head, specWithImpacts([]string{"loansvc"}), deps, "design start", &stderr)

	if !contains(stderr.String(), "no toolchain configured") {
		t.Fatalf("stderr = %q, want a disclosed no-toolchain message", stderr.String())
	}
	assertNoDerivedDir(t, repo.Dir, "spec--stale-decline", repo.Head)
}

// TestRegenerateBaseline_NoImpactedService proves an empty (or
// non-matching) impacts: list is a graceful skip, not an error — the
// honest state at `design start` scaffold time, before impacts: is filled
// in during design.
func TestRegenerateBaseline_NoImpactedService(t *testing.T) {
	repo := buildPhase7Repo(t)
	var stderr bytes.Buffer
	deps := syncDeps{Runner: fakeGraphRunner(), GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &stderr}

	regenerateBaseline(context.Background(), repo.Dir, repo.Head, specWithImpacts(nil), deps, "design start", &stderr)

	if !contains(stderr.String(), "declares no impacted service") {
		t.Fatalf("stderr = %q, want a disclosed no-impacted-service message", stderr.String())
	}
	assertNoDerivedDir(t, repo.Dir, "spec--stale-decline", repo.Head)

	stderr.Reset()
	regenerateBaseline(context.Background(), repo.Dir, repo.Head, specWithImpacts([]string{"no-such-service"}), deps, "design start", &stderr)
	if !contains(stderr.String(), "declares no impacted service") {
		t.Fatalf("stderr = %q, want a disclosed no-impacted-service message for an unmatched impact", stderr.String())
	}
}

// TestRegenerateBaseline_ToolchainUnreachable proves a Runner-level
// failure (network/exec failure standing in for "toolchain unreachable")
// degrades gracefully instead of aborting the calling verb, and never
// leaves a partial bundle on disk.
func TestRegenerateBaseline_ToolchainUnreachable(t *testing.T) {
	repo := buildPhase7Repo(t)
	fr := upstream.NewFakeRunner()
	fr.EnqueueError("flowmap", "graph", errors.New("simulated: module unreachable"))

	var stderr bytes.Buffer
	deps := syncDeps{Runner: fr, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &stderr}

	regenerateBaseline(context.Background(), repo.Dir, repo.Head, specWithImpacts([]string{"loansvc"}), deps, "design start", &stderr)

	if !contains(stderr.String(), "toolchain unreachable") {
		t.Fatalf("stderr = %q, want a disclosed toolchain-unreachable message", stderr.String())
	}
	assertNoDerivedDir(t, repo.Dir, "spec--stale-decline", repo.Head)
}

// store.FilterImpacted itself (hoisted from this file's former private
// filterImpacted so internal/align could share it too, PLAN.md Phase 8) is
// covered directly by internal/store's own tests.

func assertNoDerivedDir(t *testing.T, root, refSlug, commit string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "data", "derived", refSlug, commit)
	if _, err := os.Stat(dir); err == nil {
		t.Fatalf("expected no derived dir at %s, but it exists", dir)
	}
}

func contains(haystack, needle string) bool {
	return bytes.Contains([]byte(haystack), []byte(needle))
}
