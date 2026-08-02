package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// scaffoldAndDesign builds a fresh Phase 7 repo and runs design start on
// it (jira:LOAN-1482, --name stale-decline), returning the repo and the
// design branch's commit right after the scaffold.
func scaffoldAndDesign(t *testing.T) (repo *fixturegit.Repo, preFlipHead string) {
	t.Helper()
	repo = buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}

	var stdout, stderr bytes.Buffer
	if got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "stale-decline", manifest, phase7Model(t), deps, &stdout, &stderr); got != 0 {
		t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
	}
	head, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	return repo, head
}

// TestRunAccept_NonMutation is Step 1's own characterization (docs/
// superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md,
// "Command behavior"): `verdi accept` performs NO filesystem, index,
// branch, or commit mutation whatsoever — not the target spec's own
// bytes, not a predecessor's bytes (even one its supersedes edge names —
// the old ritual's flipPredecessorToSuperseded would have flipped
// pred-feature's status in the SAME commit accept used to create), not
// the obligation directory, not git's index or HEAD. Exercised against a
// fixture carrying both a "successor" candidate ref (spec/succ-feature,
// itself carrying a supersedes edge to spec/pred-feature) so the
// predecessor-mutation removal (Step 5) has a witness here too.
func TestRunAccept_NonMutation(t *testing.T) {
	repo := buildPredecessorFlipRepo(t, "pred-feature", predFeatureAcceptedMD, "succ-feature", succFeatureWholeSpecSupersedesMD)
	ctx := context.Background()

	beforeStatus := porcelainStatus(t, repo.Dir)
	beforeHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	_, beforeSpecRaw := readSpec(t, repo.Dir, "succ-feature")
	_, beforePredRaw := readSpec(t, repo.Dir, "pred-feature")
	obligationDir := filepath.Join(repo.Dir, ".verdi", "obligations")
	if _, err := os.Stat(obligationDir); !os.IsNotExist(err) {
		t.Fatalf("test setup: obligation dir unexpectedly exists before accept: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "spec/succ-feature", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAccept = %d, want 0 (a valid proposal invocation is informational, not an acceptance claim); stderr=%s", got, stderr.String())
	}
	const wantNotice = "accept is retired: merge the reviewed specification pull request into the configured default branch to accept this exact revision"
	if !contains(stdout.String(), wantNotice) {
		t.Fatalf("stdout = %q, want it to contain the merge-signaled retirement notice %q", stdout.String(), wantNotice)
	}

	if got := porcelainStatus(t, repo.Dir); got != beforeStatus {
		t.Fatalf("git status --porcelain changed: before=%q after=%q", beforeStatus, got)
	}
	afterHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if afterHead != beforeHead {
		t.Fatalf("HEAD changed: before=%q after=%q — accept must never commit", beforeHead, afterHead)
	}
	_, afterSpecRaw := readSpec(t, repo.Dir, "succ-feature")
	if !bytes.Equal(beforeSpecRaw, afterSpecRaw) {
		t.Fatal("accept mutated the target spec's own bytes")
	}
	_, afterPredRaw := readSpec(t, repo.Dir, "pred-feature")
	if !bytes.Equal(beforePredRaw, afterPredRaw) {
		t.Fatal("accept mutated the predecessor spec's bytes")
	}
	if _, err := os.Stat(obligationDir); !os.IsNotExist(err) {
		t.Fatalf("accept created the obligation directory: %v", err)
	}
}

// TestRunAccept_AnyValidSpecRefExitsZero proves the informational posture
// is unconditional: accept no longer determines eligibility at all (that
// is `verdi spec state`'s job now), so it exits 0 and prints the same
// notice for ANY well-formed spec ref regardless of whether a spec by
// that name even exists on disk, and performs no mutation either way.
func TestRunAccept_AnyValidSpecRefExitsZero(t *testing.T) {
	repo := buildAcceptNegativeRepo(t)
	ctx := context.Background()
	before := porcelainStatus(t, repo.Dir)

	cases := []string{"spec/does-not-exist", "spec/some-component", "spec/already-accepted"}
	for _, ref := range cases {
		t.Run(ref, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := runAccept(ctx, repo.Dir, ref, &stdout, &stderr)
			if got != 0 {
				t.Fatalf("runAccept(%s) = %d, want 0; stderr=%s", ref, got, stderr.String())
			}
			if !contains(stdout.String(), acceptRetiredNotice) {
				t.Fatalf("stdout = %q, want the retirement notice", stdout.String())
			}
		})
	}

	if got := porcelainStatus(t, repo.Dir); got != before {
		t.Fatalf("git status --porcelain changed across every invocation: before=%q after=%q", before, got)
	}
}

const componentSpecMD = `---
id: spec/some-component
kind: spec
title: "Some component"
owners: [platform-team]
class: component
status: active
---
# Some component
`

const alreadyAcceptedSpecMD = `---
id: spec/already-accepted
kind: spec
title: "Already accepted"
owners: [platform-team]
class: feature
status: accepted-pending-build
story: jira:LOAN-2000
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [static] }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Already accepted
`

func buildAcceptNegativeRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                            phase7ManifestYAML,
				".verdi/specs/active/some-component/spec.md":   componentSpecMD,
				".verdi/specs/active/already-accepted/spec.md": alreadyAcceptedSpecMD,
			},
			Message: "init store with pre-existing specs",
		},
	})
}

// TestRunAccept_Negative covers runAccept's own remaining operational-error
// paths: a malformed/non-spec/diagram ref is still exit 2, unchanged from
// before this retirement — the only refusal left, since there is no more
// business precondition to fail.
func TestRunAccept_Negative(t *testing.T) {
	ctx := context.Background()

	t.Run("not a spec or diagram ref", func(t *testing.T) {
		repo := buildAcceptNegativeRepo(t)
		var stdout, stderr bytes.Buffer
		got := runAccept(ctx, repo.Dir, "jira:LOAN-1482", &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runAccept(story ref) = %d, want 2", got)
		}
	})

	t.Run("pinned ref", func(t *testing.T) {
		repo := buildAcceptNegativeRepo(t)
		var stdout, stderr bytes.Buffer
		got := runAccept(ctx, repo.Dir, "spec/some-component@deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runAccept(pinned ref) = %d, want 2", got)
		}
	})
}

// TestCmdAccept_UsageNegative proves cmdAccept's own argument-count check.
func TestCmdAccept_UsageNegative(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := cmdAccept(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdAccept(no args) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := cmdAccept([]string{"spec/a", "spec/b"}, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdAccept(two args) = %d, want 2", got)
	}
}

// TestRun_AcceptDispatchesToRealVerb proves dispatch.go routes "accept" to
// the real implementation.
func TestRun_AcceptDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"accept", "spec/x"}, &stderr)
	if got != 2 {
		t.Fatalf("run([accept spec/x]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}
