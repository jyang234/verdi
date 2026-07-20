package residue

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// fixtureFrozenCommit is a fixed, valid-shaped (40 lowercase hex) frozen
// stamp commit for fixture spec.md files (artifact.Frozen.Validate only
// checks shape, never that it resolves to a real commit) — the same
// literal cmd/verdi's own close/closefeature fixtures use.
const fixtureFrozenCommit = "0000000000000000000000000000000000000a"

// frozenRequiredForStatus mirrors artifact.SpecFrontmatter's own
// requireFrozen gate for the feature/story lifecycle (spec.go's
// validateFeature/validateStory): frozen: is REQUIRED at these three
// statuses and FORBIDDEN at every other one (requireFrozen rejects a
// frozen stamp's mere presence when not required) — so a fixture builder
// must emit it conditionally, never unconditionally.
func frozenRequiredForStatus(status string) bool {
	return status == "accepted-pending-build" || status == "closed" || status == "superseded"
}

// frozenLine renders the frozen: line for status, or "" when status
// forbids one (frozenRequiredForStatus).
func frozenLine(status string) string {
	if !frozenRequiredForStatus(status) {
		return ""
	}
	return "frozen: { at: 2024-01-01, commit: " + fixtureFrozenCommit + " }\n"
}

// storySpecMD renders a minimal, artifact.DecodeSpec-valid class: story
// spec.md at status, implementing featureRef's ac-1 (a placeholder ref —
// residue's own scan never resolves this edge, so the target need not
// exist in the fixture at all).
func storySpecMD(name, status, featureRef string) string {
	return `---
id: spec/` + name + `
kind: spec
class: story
title: "` + name + `"
status: ` + status + `
owners: [platform-team]
story: jira:RESIDUE-1
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/` + featureRef + `#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the fixture behavior holds", evidence: [static, behavioral] }
` + frozenLine(status) + `---
# ` + name + `
`
}

// featureSpecMD renders a minimal, artifact.DecodeSpec-valid class:
// feature spec.md at status, declaring one stubs[] entry per slug in
// stubSlugs (each pinned to ac-1 — Stub.Validate checks shape only, never
// cross-references against a real declared AC).
func featureSpecMD(name, status string, stubSlugs ...string) string {
	var stubsBlock strings.Builder
	if len(stubSlugs) > 0 {
		stubsBlock.WriteString("stubs:\n")
		for _, slug := range stubSlugs {
			stubsBlock.WriteString("  - { slug: " + slug + ", acceptance_criteria: [ac-1] }\n")
		}
	}
	return `---
id: spec/` + name + `
kind: spec
class: feature
title: "` + name + `"
status: ` + status + `
owners: [platform-team]
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the fixture outcome holds", evidence: [static, attestation] }
` + stubsBlock.String() + frozenLine(status) + `---
# ` + name + `
`
}

// closedArchiveStorySpecMD renders a minimal, valid, ALREADY-CLOSED story
// spec.md — pattern (b)'s realization target: an on-disk
// specs/archive/<slug>/spec.md carrying status: closed.
func closedArchiveStorySpecMD(name, featureRef string) string {
	return storySpecMD(name, "closed", featureRef)
}

// runGit runs git in dir with a fixed author/committer identity (for any
// invocation that creates a commit) and fails the test on a non-zero exit
// — mirrors internal/wtmanager's and internal/store's own per-package test
// helper of the same name/shape (CLAUDE.md precedent: each package that
// needs raw git beyond gitx's own wrapped primitives defines its own tiny
// copy, never a shared production dependency for test-only plumbing).
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
		"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// cutCloseBranch checks out a new close/<name> branch at root's current
// HEAD, leaving it checked out (mirrors spec/close-verb's own
// gitx.CheckoutNewBranch step).
func cutCloseBranch(t *testing.T, root, name string) {
	t.Helper()
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, root, "close/"+name); err != nil {
		t.Fatalf("CheckoutNewBranch(close/%s): %v", name, err)
	}
}

// runCloseRitualArchiveCommit performs the SAME active->archive move
// spec/close-verb's real closure ritual performs (store.ArchiveMove — the
// identical production mover, never a hand-rolled rename) against root's
// CURRENTLY CHECKED OUT branch, and commits it with msg — the exact shape
// a stranded or completed close/<name> branch carries on its own tip.
// Returns the new commit's sha.
//
// msg is caller-supplied (never a fixed literal) because two archive-move
// commits for the SAME name, from the SAME parent, with the SAME fixed
// fixture author/committer identity, produce an otherwise BYTE-IDENTICAL
// commit (same tree, same parent, same author/committer, and — absent an
// explicit GIT_AUTHOR_DATE/GIT_COMMITTER_DATE override, which this helper
// deliberately does not set — very likely the same wall-clock second too)
// — i.e. the SAME sha. The "superseded-elsewhere" fixture shape needs
// TWO cases (this branch's own tip, and main independently) to be
// genuinely different commits despite reaching the same resulting tree,
// so every caller must pass a distinguishing message.
func runCloseRitualArchiveCommit(t *testing.T, root, name, msg string) string {
	t.Helper()
	if err := store.ArchiveMove(root, name); err != nil {
		t.Fatalf("store.ArchiveMove(%s): %v", name, err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", msg)
	tip, err := gitx.RevParse(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD) after archive commit: %v", err)
	}
	return tip
}

// mustDecodeSpecFM decodes a spec.md's frontmatter block (splitting it
// from the full document text a fixture helper renders) for a unit test
// that needs an *artifact.SpecFrontmatter directly rather than a full
// fixturegit checkout — t.Fatal on any decode error, since every fixture
// string this package's helpers render is meant to be valid.
func mustDecodeSpecFM(t *testing.T, specMD string) *artifact.SpecFrontmatter {
	t.Helper()
	fm, _, err := artifact.SplitFrontmatter([]byte(specMD))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\n%s", err, specMD)
	}
	decoded, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v\n%s", err, specMD)
	}
	return decoded
}

// checkoutMain returns root to its main branch — every fixture builder
// that cuts a close/<name> branch returns here afterward so the NEXT
// close/<name> branch (if any) is cut from main's own tip, matching how
// two sibling close/<name> branches in a real repo both descend from the
// default branch, not from each other.
func checkoutMain(t *testing.T, root string) {
	t.Helper()
	if err := gitx.Checkout(context.Background(), root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
}
