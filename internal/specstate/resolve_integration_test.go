package specstate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// statuslessSpecPath and statuslessSpecContent are the fixture spec this
// file lands by three different Git integration strategies: no `status:`
// line at all (the design's "The schema permits new active specifications
// to omit the persisted status field"). Ahead of the sibling task that
// formally makes internal/artifact's schema accept an omitted status,
// this package only tolerates a STATUSLESS CANDIDATE'S OWN bytes — its own
// path is always excluded from its own supersession-corpus decode
// (resolve.go's per-candidate self-exclusion). It does NOT yet tolerate a
// statusless spec anywhere ELSE in the default-branch active corpus: that
// unrelated spec still fails internal/artifact.DecodeSpec during the
// corpus scan, and the current, correct, fail-closed behavior is Unproven
// with a decode-witness disclosure — proven by
// TestProjector_UnrelatedStatuslessCorpusSpec_Integration below, not
// end-to-end tolerance. The sibling schema task flips that unrelated case
// to a clean scan (and, transitively, AcceptedPendingBuild here) once it
// lands.
const statuslessSpecPath = ".verdi/specs/active/statusless-feature/spec.md"

const statuslessSpecContent = `---
id: spec/statusless-feature
kind: spec
class: feature
title: Statusless Feature
owners: [platform]
---

Body.
`

// runGitForTest execs git in dir, failing the test on a non-zero exit —
// duplicated locally rather than shared with internal/gitx's own test
// helper of the same name (test-only, package-private, cheap to
// duplicate; CLAUDE.md's shared-code rule is about production code).
func runGitForTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// writeStatuslessSpecFile writes the fixture spec at its real store path
// inside dir.
func writeStatuslessSpecFile(t *testing.T, dir string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(statuslessSpecPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for spec file: %v", err)
	}
	if err := os.WriteFile(full, []byte(statuslessSpecContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}
}

// mergeStrategy names which Git landing strategy buildLandedSpecRepo
// exercises.
type mergeStrategy string

const (
	strategyMerge  mergeStrategy = "merge"
	strategySquash mergeStrategy = "squash"
	strategyRebase mergeStrategy = "rebase"
)

// buildLandedSpecRepo mirrors internal/gitx/blob_test.go's own
// buildMergeTopology (same diverged main/design-branch shape, same three
// landing strategies), but writes the spec at its real store path so this
// package's own Projector — not gitx's raw primitives — is what is under
// test here. Only test-local raw git invocations perform the merge/
// squash/rebase mechanics; gitx and specstate expose no production API
// for any of them.
func buildLandedSpecRepo(t *testing.T, strategy mergeStrategy) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"base.txt": "base\n"}, Message: "root"},
	})
	root := repo.Head

	runGitForTest(t, repo.Dir, "checkout", "--quiet", "-b", "design/statusless-feature", root)
	writeStatuslessSpecFile(t, repo.Dir)
	runGitForTest(t, repo.Dir, "add", "-A")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "propose statusless feature")

	runGitForTest(t, repo.Dir, "checkout", "--quiet", "main")
	if err := os.WriteFile(filepath.Join(repo.Dir, "main-only.txt"), []byte("main-only\n"), 0o644); err != nil {
		t.Fatalf("write main-only.txt: %v", err)
	}
	runGitForTest(t, repo.Dir, "add", "-A")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "advance main")

	switch strategy {
	case strategyMerge:
		runGitForTest(t, repo.Dir, "merge", "--no-ff", "--no-edit", "design/statusless-feature")
	case strategySquash:
		runGitForTest(t, repo.Dir, "merge", "--squash", "design/statusless-feature")
		runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "squash statusless feature")
	case strategyRebase:
		runGitForTest(t, repo.Dir, "checkout", "--quiet", "design/statusless-feature")
		runGitForTest(t, repo.Dir, "rebase", "main")
		runGitForTest(t, repo.Dir, "checkout", "--quiet", "main")
		runGitForTest(t, repo.Dir, "merge", "--ff-only", "design/statusless-feature")
	default:
		t.Fatalf("buildLandedSpecRepo: unknown strategy %q", strategy)
	}

	repo.Head = strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "HEAD"))
	return repo
}

// TestProjector_MergeStrategies_Integration lands the same statusless
// feature by a regular (non-fast-forward) merge, a squash merge, and a
// rebase-then-fast-forward, over the REAL git plumbing (NewProjector, no
// fakes) — proving all three converge on the same effective state and
// blob identity while reporting the strategy-specific landing commit
// FirstParentBlobLanding actually names.
func TestProjector_MergeStrategies_Integration(t *testing.T) {
	ctx := context.Background()
	p := NewProjector()

	strategies := []mergeStrategy{strategyMerge, strategySquash, strategyRebase}
	blobs := map[mergeStrategy]string{}
	landings := map[mergeStrategy]string{}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			repo := buildLandedSpecRepo(t, strategy)
			t.Setenv("CI_DEFAULT_BRANCH", "main")

			result, err := p.Resolve(ctx, repo.Dir, Candidate{
				Path:    statuslessSpecPath,
				Content: []byte(statuslessSpecContent),
			})
			if err != nil {
				t.Fatalf("Resolve(%s): %v", strategy, err)
			}
			if result.State != AcceptedPendingBuild || result.Relation != RelationExact {
				t.Fatalf("Resolve(%s) = %+v, want AcceptedPendingBuild/exact", strategy, result)
			}
			if result.Baseline == nil || result.Baseline.Blob == "" || result.Baseline.LandingCommit == "" {
				t.Fatalf("Resolve(%s): incomplete baseline %+v", strategy, result.Baseline)
			}
			blobs[strategy] = result.Baseline.Blob
			landings[strategy] = result.Baseline.LandingCommit
		})
	}

	if blobs[strategyMerge] != blobs[strategySquash] || blobs[strategySquash] != blobs[strategyRebase] {
		t.Fatalf("blob oids differ across strategies for identical bytes: %+v", blobs)
	}
	// The real two-parent merge commit, the linear squash commit, and the
	// feature's own rebased commit (a new SHA — its parent changed) are
	// three genuinely different commits even though the landed content is
	// byte-identical.
	if landings[strategyMerge] == landings[strategySquash] ||
		landings[strategySquash] == landings[strategyRebase] ||
		landings[strategyMerge] == landings[strategyRebase] {
		t.Fatalf("expected three distinct strategy-specific landing commits, got %+v", landings)
	}
}

// TestProjector_UnrelatedStatuslessCorpusSpec_Integration proves fix-round-1
// finding 3's documented current behavior over REAL git (no fakes): a
// default branch carrying the (schema-valid) candidate PLUS one wholly
// unrelated statusless active spec resolves the candidate as Unproven,
// naming the unrelated spec's path as the decode witness — never a silent
// AcceptedPendingBuild. This is the correct, fail-closed posture ahead of
// the sibling schema task; that task's own landing is what should flip
// this exact test to AcceptedPendingBuild.
func TestProjector_UnrelatedStatuslessCorpusSpec_Integration(t *testing.T) {
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"base.txt": "base\n"}, Message: "root"},
	})

	candidatePath := ".verdi/specs/active/payments/spec.md"
	candidateContent := []byte(`---
id: spec/payments
kind: spec
class: feature
title: Payments
owners: [platform]
status: accepted-pending-build
frozen: { at: "2024-01-01", commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }
acceptance_criteria:
  - { id: ac-1, text: works, evidence: [static] }
---
body
`)
	unrelatedPath := ".verdi/specs/active/statusless-feature/spec.md"

	runGitForTest(t, repo.Dir, "checkout", "--quiet", "main")
	if err := os.MkdirAll(filepath.Join(repo.Dir, filepath.FromSlash(filepath.Dir(candidatePath))), 0o755); err != nil {
		t.Fatalf("mkdir for candidate spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, filepath.FromSlash(candidatePath)), candidateContent, 0o644); err != nil {
		t.Fatalf("write candidate spec: %v", err)
	}
	writeStatuslessSpecFile(t, repo.Dir) // the unrelated statusless spec, at unrelatedPath
	runGitForTest(t, repo.Dir, "add", "-A")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "add payments plus an unrelated statusless spec")

	t.Setenv("CI_DEFAULT_BRANCH", "main")

	p := NewProjector()
	result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: candidatePath, Content: candidateContent})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.State != Unproven || result.Relation != RelationUnproven {
		t.Fatalf("Resolve = %+v, want Unproven/unproven (an unrelated statusless corpus spec must still block the scan today)", result)
	}
	foundWitness := false
	for _, d := range result.Disclosures {
		if strings.Contains(d, unrelatedPath) && strings.Contains(d, "failed to decode") {
			foundWitness = true
		}
	}
	if !foundWitness {
		t.Fatalf("Resolve disclosures = %v, want one naming %s as the decode witness", result.Disclosures, unrelatedPath)
	}
}

// TestProjector_UnmergedBranch_Integration proves a design branch that
// never lands on the default branch stays Proposed.
func TestProjector_UnmergedBranch_Integration(t *testing.T) {
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"base.txt": "base\n"}, Message: "root"},
	})
	runGitForTest(t, repo.Dir, "checkout", "--quiet", "-b", "design/statusless-feature", repo.Head)
	writeStatuslessSpecFile(t, repo.Dir)
	runGitForTest(t, repo.Dir, "add", "-A")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "propose statusless feature")
	// Left unmerged: main never advances past root, so it never gains the
	// spec at all.

	t.Setenv("CI_DEFAULT_BRANCH", "main")

	p := NewProjector()
	result, err := p.Resolve(ctx, repo.Dir, Candidate{
		Path:    statuslessSpecPath,
		Content: []byte(statuslessSpecContent),
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.State != Proposed || result.Relation != RelationNew {
		t.Fatalf("Resolve(unmerged) = %+v, want Proposed/new", result)
	}
	if result.Baseline != nil {
		t.Fatalf("Resolve(unmerged): want nil baseline, got %+v", result.Baseline)
	}
}
