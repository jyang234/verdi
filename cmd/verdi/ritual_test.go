// The Phase 7 exit criterion: "the full ritual design start -> edit ->
// accept -> feature start succeeds", scripted end-to-end against a fresh
// fixturegit repo (PLAN.md Phase 7's own test strategy).
package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/OWNER/verdi/internal/gitx"
)

// injectImpacts is the test's stand-in for a human's design-branch edit:
// it adds an `impacts: [...]` line to a scaffolded draft spec's
// frontmatter (design.go's own scaffold never sets Impacts — a spec truly
// has none until a human declares them during design) and commits the
// edit, exactly the kind of ordinary content change legal on a design
// branch before `accept`.
func injectImpacts(t *testing.T, ctx context.Context, root, name string, impacts []string) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "specs", "active", name, "spec.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	line := "impacts: ["
	for i, s := range impacts {
		if i > 0 {
			line += ", "
		}
		line += s
	}
	line += "]"

	statusRe := regexp.MustCompile(`(?m)^status: draft$`)
	if !statusRe.Match(raw) {
		t.Fatalf("spec.md at %s does not contain the expected 'status: draft' line to anchor the edit", path)
	}
	edited := statusRe.ReplaceAll(raw, []byte("status: draft\n"+line))

	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatalf("writing edited %s: %v", path, err)
	}
	if err := gitx.AddAll(ctx, root); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, root, "edit: declare loansvc as impacted"); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
}

// TestRitual_DesignStartEditAcceptFeatureStart drives the whole Phase 7
// lifecycle end to end: design start cuts the design branch and scaffolds
// a draft spec; a design-branch edit declares an impacted service; accept
// flips the spec to accepted-pending-build; feature start refuses to run
// against the pre-accept state, then succeeds once accepted, cutting the
// build branch and regenerating a real (FakeRunner-backed) baseline
// scoped to the declared impact.
func TestRitual_DesignStartEditAcceptFeatureStart(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)

	// 1. design start
	designDepsV := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}}
	var stdout, stderr bytes.Buffer
	if got := runDesignStart(ctx, repo.Dir, "jira:LOAN-1482", "stale-decline", manifest, designDepsV, &stdout, &stderr); got != 0 {
		t.Fatalf("design start = %d, want 0; stderr=%s", got, stderr.String())
	}
	branch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "design/stale-decline" {
		t.Fatalf("branch after design start = %q, want design/stale-decline", branch)
	}

	// 1b. feature start against the still-draft spec must refuse.
	refuseDeps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	stdout.Reset()
	stderr.Reset()
	if got := runFeatureStart(ctx, repo.Dir, "jira:LOAN-1482", refuseDeps, &stdout, &stderr); got != 1 {
		t.Fatalf("feature start against a draft spec = %d, want 1; stderr=%s", got, stderr.String())
	}

	// 2. edit: declare loansvc as impacted (the ordinary design-branch
	// content editing accept expects to have already happened).
	injectImpacts(t, ctx, repo.Dir, "stale-decline", []string{"loansvc"})

	// 3. accept
	stdout.Reset()
	stderr.Reset()
	if got := runAccept(ctx, repo.Dir, "spec/stale-decline", &stdout, &stderr); got != 0 {
		t.Fatalf("accept = %d, want 0; stderr=%s", got, stderr.String())
	}
	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	if spec.Status != "accepted-pending-build" {
		t.Fatalf("spec.Status after accept = %q, want accepted-pending-build", spec.Status)
	}
	if len(spec.Impacts) != 1 || spec.Impacts[0] != "loansvc" {
		t.Fatalf("spec.Impacts after accept = %v, want [loansvc] (the edit step's contribution survived acceptance)", spec.Impacts)
	}
	acceptHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// 4. feature start, now that the spec is accepted-pending-build —
	// with a real (fake-backed) baseline regeneration scoped to loansvc.
	featureDeps := syncDeps{Runner: fakeGraphRunner(), GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &stderr}
	stdout.Reset()
	stderr.Reset()
	if got := runFeatureStart(ctx, repo.Dir, "jira:LOAN-1482", featureDeps, &stdout, &stderr); got != 0 {
		t.Fatalf("feature start = %d, want 0; stderr=%s", got, stderr.String())
	}

	branch, err = gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/stale-decline" {
		t.Fatalf("branch after feature start = %q, want feature/stale-decline", branch)
	}

	derivedDir := filepath.Join(repo.Dir, ".verdi", "data", "derived", "feature--stale-decline", acceptHead)
	for _, name := range derivedFileNames {
		if _, err := os.Stat(filepath.Join(derivedDir, name)); err != nil {
			t.Fatalf("expected baseline file %s at %s: %v (stderr=%s)", name, derivedDir, err, stderr.String())
		}
	}
}
