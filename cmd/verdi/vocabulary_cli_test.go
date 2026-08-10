// spec/vocabulary-surfaces ac-1: CLI verdict and status output resolves
// display names through the resolved model (store.Open -> Config.Model ->
// DisplayState) over a vocab-rename fixture store — driving the BUILT
// verdi binary (buildVerdiBinary + exec, the gc_test.go convention) so
// the proof covers cmd*'s real wiring, never a package-internal stand-in.
//
// The fixture model is internal/model/testdata/vocab-rename.yaml —
// model-schema's own frontier fixture (merge -> "Sign off",
// accepted-pending-build -> "Ready to build", feature -> "Initiative") —
// read at test runtime and planted as the store's .verdi/model.yaml:
// reused, never duplicated.
//
// The parity floor (the AC's other half) is deliberately NOT a new
// assertion here: it is the entire pre-existing golden/substring suite
// across this package continuing to pass unmodified over stores carrying
// no model.yaml.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// diverge rewrites path's on-disk copy so its bytes no longer match
// whatever was committed at that same path — the Git-derived
// (internal/specstate) shape of "still under review, never landed", used
// where a fixture's ORIGINAL literal `status: draft` field is no longer,
// on its own, sufficient to keep a spec whose exact bytes happen to be the
// committed default-branch content from reading as accepted-pending-build
// (Task 4's compatibility reading: exact landed bytes are accepted
// regardless of a stale legacy status word).
func diverge(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("diverge: reading %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(data, []byte("<!-- still under review, diverged from the committed default-branch copy -->\n")...), 0o644); err != nil {
		t.Fatalf("diverge: writing %s: %v", path, err)
	}
}

// vocabModelYAML reads the real vocab-rename fixture out of
// internal/model/testdata — the single source of the rename set.
func vocabModelYAML(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "model", "testdata", "vocab-rename.yaml"))
	if err != nil {
		t.Fatalf("reading internal/model/testdata/vocab-rename.yaml: %v", err)
	}
	return string(data)
}

// buildVocabRenameRepo mirrors buildPredecessorFlipRepo (the same spec
// constants) plus the vocab-rename model.yaml in the committed store.
func buildVocabRenameRepo(t *testing.T, predName, predMD, succName, succMD string) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                                   phase7ManifestYAML,
				".verdi/model.yaml":                                   vocabModelYAML(t),
				".verdi/specs/active/some-feature/spec.md":            someFeatureMD,
				".verdi/specs/active/" + predName + "/spec.md":        predMD,
				".verdi/specs/active/" + succName + "/spec.md":        succMD,
				".verdi/obligations/" + succName + "/ac-1--static.md": fixtureElaboratedObligationMD(succName, "ac-1", artifact.EvidenceStatic, "fixture-static", "1", gateFakeFrozenCommit),
			},
			Message: "init store with predecessor + draft successor + vocab-rename model",
		},
	})
}

// runVerdi execs the built binary with args in dir, returning combined
// exit code, stdout, and stderr.
func runVerdi(t *testing.T, bin, dir string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running %s %v: %v", bin, args, err)
		}
	}
	return code, stdout.String(), stderr.String()
}

// TestVocabularyCLI_RenamedStateLabels drives build start's status-mismatch
// refusal plus its success line over the vocab-rename store: every state
// word prints as its renamed label. Task 7 retires accept's own mutation
// (and therefore its vocabulary-routed flip/refusal lines, and supersede.go's
// flipped-predecessor confirmation) — acceptance is now merge-signaled, so
// this test drives the store's ALREADY-landed bytes directly rather than
// accepting anything first.
func TestVocabularyCLI_RenamedStateLabels(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildVocabRenameRepo(t, "pred-story", predStoryAcceptedMD, "succ-story", succStorySupersedesMD)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	// succ-story's committed bytes are landed on main by construction
	// (buildVocabRenameRepo's own single commit); under Git-derived state
	// (internal/specstate) that alone reads as accepted-pending-build
	// regardless of its persisted `status: draft` word (Task 4's
	// compatibility reading applies to ANY exact-bytes-landed spec — no
	// separate acceptance ceremony is needed or exists anymore). Step 2
	// below specifically wants build start to see some-feature as still
	// under review, so its working-tree copy is diverged from what
	// actually landed — never re-committed.
	diverge(t, filepath.Join(repo.Dir, ".verdi", "specs", "active", "some-feature", "spec.md"))

	// 1. accept prints the fixed retirement notice for ANY spec ref — it no
	// longer resolves or routes any vocabulary at all (Task 7: informational
	// only, never an acceptance claim).
	code, stdout, _ := runVerdi(t, bin, repo.Dir, "accept", "spec/succ-story")
	if code != 0 {
		t.Fatalf("accept spec/succ-story = %d, want 0", code)
	}
	if !contains(stdout, acceptRetiredNotice) {
		t.Fatalf("accept stdout = %q, want the retirement notice", stdout)
	}

	// 2. build start's not-yet-landed refusal names the wanted state
	// through the model (some-feature has not landed on the default
	// branch — the Git-derived reading of "still draft").
	code, _, stderr := runVerdi(t, bin, repo.Dir, "build", "start", "spec/some-feature")
	if code != 1 {
		t.Fatalf("build start (unlanded spec) = %d, want 1; stderr=%s", code, stderr)
	}
	if !contains(stderr, "proposal has not landed") || !contains(stderr, "not yet Ready to build") {
		t.Fatalf("build start refusal stderr = %q, want the renamed wanted-state and the not-landed refusal", stderr)
	}

	// 3. build start's success line resolves the accepted state — succ-story
	// is accepted purely because its bytes are already landed on main, no
	// `verdi accept` call involved at all.
	code, stdout, stderr = runVerdi(t, bin, repo.Dir, "build", "start", "spec/succ-story")
	if code != 0 {
		t.Fatalf("build start spec/succ-story = %d, want 0; stderr=%s", code, stderr)
	}
	if !contains(stdout, "(status: Ready to build)") {
		t.Fatalf("build start stdout = %q, want the renamed success suffix %q", stdout, "(status: Ready to build)")
	}
}

// vocabBirdsFeatureMD is a round-four birds-eye feature (class: feature +
// problem/outcome — matrix.go's two-conjunct discriminator), the shape
// that trips build start's feature-refusal before any status check.
const vocabBirdsFeatureMD = `---
id: spec/birds-feature
kind: spec
title: "Birds feature"
owners: [platform-team]
class: feature
status: draft
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "outcome ac", evidence: [static], anchor: "#ac-1" }
---
# Birds feature
`

// TestVocabularyCLI_RenamedClassWordRefusals drives the class-word refusal
// prose over the vocab-rename store
// (judged-cli-refusal-prose-class-state-words-still-bare): build start's
// feature-refusal speaks the renamed class words with agreeing articles —
// "an Initiative" (model.Article over the vowel-initial rename,
// judged-article-agreement-approximation-undisclosed), never the
// formerly-bare "a feature spec … a story spec".
func TestVocabularyCLI_RenamedClassWordRefusals(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                         phase7ManifestYAML,
				".verdi/model.yaml":                         vocabModelYAML(t),
				".verdi/specs/active/birds-feature/spec.md": vocabBirdsFeatureMD,
			},
			Message: "init store with a birds-eye feature + vocab-rename model",
		},
	})

	code, _, stderr := runVerdi(t, bin, repo.Dir, "build", "start", "spec/birds-feature")
	if code != 2 {
		t.Fatalf("build start (birds-eye feature) = %d, want 2; stderr=%s", code, stderr)
	}
	want := "is an Initiative spec (birds-eye, outcome-level); build start operates on a Workstream spec that implements it, not the Initiative itself"
	if !contains(stderr, want) {
		t.Fatalf("build start refusal stderr = %q, want the renamed class words with agreeing articles %q", stderr, want)
	}
}
