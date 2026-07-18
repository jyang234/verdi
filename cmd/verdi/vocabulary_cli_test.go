// spec/vocabulary-surfaces ac-1: CLI verdict and status output resolves
// display names through the resolved model (store.Open -> Config.Model ->
// DisplayState) over a vocab-rename fixture store — driving the BUILT
// verdi binary (buildVerdiBinary + exec, the gc_test.go convention) so
// the proof covers cmd*'s real wiring, never a package-internal stand-in.
//
// The fixture model is internal/model/testdata/vocab-rename.yaml —
// model-schema's own frontier fixture (accept -> "Sign off",
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
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

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
				".verdi/verdi.yaml":                            phase7ManifestYAML,
				".verdi/model.yaml":                            vocabModelYAML(t),
				".verdi/specs/active/some-feature/spec.md":     someFeatureMD,
				".verdi/specs/active/" + predName + "/spec.md": predMD,
				".verdi/specs/active/" + succName + "/spec.md": succMD,
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

// TestVocabularyCLI_RenamedStateLabels drives accept (both its own flip
// line and its refusal), the flipped-predecessor confirmation, and build
// start's status-mismatch refusal plus its success line over the
// vocab-rename store: every state word prints as its renamed label.
func TestVocabularyCLI_RenamedStateLabels(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildVocabRenameRepo(t, "pred-story", predStoryAcceptedMD, "succ-story", succStorySupersedesMD)

	// 1. accept the draft successor: its own verdict line resolves both
	// states ("status: draft -> Ready to build"), and the predecessor's
	// flip confirmation resolves its from/to pair.
	code, stdout, stderr := runVerdi(t, bin, repo.Dir, "accept", "spec/succ-story")
	if code != 0 {
		t.Fatalf("accept spec/succ-story = %d, want 0; stderr=%s\nstdout=%s", code, stderr, stdout)
	}
	if !contains(stdout, "status: draft -> Ready to build") {
		t.Fatalf("accept stdout = %q, want the renamed accept confirmation %q", stdout, "status: draft -> Ready to build")
	}
	if !contains(stdout, "(status: Ready to build -> superseded") {
		t.Fatalf("accept stdout = %q, want the renamed flipped-predecessor confirmation %q", stdout, "(status: Ready to build -> superseded")
	}

	// 2. accept refuses a non-draft spec, printing its CURRENT status
	// through the model.
	code, _, stderr = runVerdi(t, bin, repo.Dir, "accept", "spec/succ-story")
	if code != 1 {
		t.Fatalf("accept(already accepted) = %d, want 1; stderr=%s", code, stderr)
	}
	// The FULLY-resolved refusal sentence: the current status, the wanted
	// state, AND the trailing "only a <draft> spec" all resolve through the
	// model (judged-ac4-draft-prose-leak). draft carries no rename in this
	// fixture, so it renders "draft" with the agreeing article "a" — the
	// focused TestVocabularyCLI_AcceptRefusalResolvesDraftWord below exercises
	// a store that DOES rename draft, witnessing the routing.
	if want := `status is "Ready to build", not draft; only a draft spec can be accepted`; !contains(stderr, want) {
		t.Fatalf("accept refusal stderr = %q, want the fully-resolved refusal sentence %q", stderr, want)
	}

	// 3. build start's status-mismatch refusal names the wanted state
	// through the model (some-feature is still draft).
	code, _, stderr = runVerdi(t, bin, repo.Dir, "build", "start", "spec/some-feature")
	if code != 1 {
		t.Fatalf("build start (draft spec) = %d, want 1; stderr=%s", code, stderr)
	}
	if !contains(stderr, `status is "draft", not Ready to build`) {
		t.Fatalf("build start refusal stderr = %q, want the renamed wanted-state %q", stderr, `status is "draft", not Ready to build`)
	}

	// 4. build start's success line resolves the accepted state.
	code, stdout, stderr = runVerdi(t, bin, repo.Dir, "build", "start", "spec/succ-story")
	if code != 0 {
		t.Fatalf("build start spec/succ-story = %d, want 0; stderr=%s", code, stderr)
	}
	if !contains(stdout, "(status: Ready to build)") {
		t.Fatalf("build start stdout = %q, want the renamed success suffix %q", stdout, "(status: Ready to build)")
	}
}

// TestVocabularyCLI_AcceptRefusalResolvesDraftWord is judged-ac4-draft-prose-
// leak's guard: accept's non-draft refusal routes its TRAILING state word
// ("only a draft spec can be accepted") through the model exactly like the two
// resolved words beside it, its article agreeing via model.Article. The shared
// vocab-rename fixture does not rename `draft`, so the sibling test above cannot
// witness the routing (draft renders "draft" either way); this store DOES rename
// draft — to the vowel-initial "Idea", so the article visibly becomes "an" — and
// so is what actually distinguishes the routed word from a re-hard-coded one.
func TestVocabularyCLI_AcceptRefusalResolvesDraftWord(t *testing.T) {
	bin := buildVerdiBinary(t)

	// Reuse the shared vocab-rename fixture as the base — never edited here, it
	// has a large cross-package + e2e blast radius — injecting one extra states
	// rename in memory so DisplayState(draft) != "draft" for this store alone.
	renamedModel := strings.Replace(vocabModelYAML(t),
		"    accepted-pending-build: \"Ready to build\"\n",
		"    accepted-pending-build: \"Ready to build\"\n    draft: \"Idea\"\n", 1)
	if !contains(renamedModel, `draft: "Idea"`) {
		t.Fatal("test setup: failed to inject the draft rename into the vocab-rename base fixture")
	}

	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                        phase7ManifestYAML,
				".verdi/model.yaml":                        renamedModel,
				".verdi/specs/active/some-feature/spec.md": someFeatureMD,
				".verdi/specs/active/pred-story/spec.md":   predStoryAcceptedMD,
			},
			Message: "init store with an accepted story + a draft-renaming model",
		},
	})

	// pred-story is accepted-pending-build, so accept refuses it (exit 1) at
	// the status check.
	code, _, stderr := runVerdi(t, bin, repo.Dir, "accept", "spec/pred-story")
	if code != 1 {
		t.Fatalf("accept (accepted-pending-build spec) = %d, want 1; stderr=%s", code, stderr)
	}
	// The fully-resolved refusal sentence: current status "Ready to build",
	// wanted-state "Idea", AND the trailing "only an Idea spec" — model.Article
	// turning "a" into "an" before the vowel-initial rename.
	if want := `status is "Ready to build", not Idea; only an Idea spec can be accepted`; !contains(stderr, want) {
		t.Fatalf("accept refusal stderr = %q, want the fully-resolved sentence with the routed draft word %q", stderr, want)
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

// TestVocabularyCLI_UnflippedPredecessorDisclosure covers supersede.go's
// left-unflipped disclosure: a closed predecessor's status line and the
// legal-transition notation both resolve through the model.
func TestVocabularyCLI_UnflippedPredecessorDisclosure(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildVocabRenameRepo(t, "pred-feature", predFeatureClosedMD, "succ-feature", succFeatureWholeSpecSupersedesMD)

	code, stdout, stderr := runVerdi(t, bin, repo.Dir, "accept", "spec/succ-feature")
	if code != 0 {
		t.Fatalf("accept spec/succ-feature = %d, want 0; stderr=%s\nstdout=%s", code, stderr, stdout)
	}
	if !contains(stdout, `not Ready to build; left unflipped (only Ready to build->superseded is a legal ritual transition`) {
		t.Fatalf("accept stdout = %q, want the renamed left-unflipped disclosure", stdout)
	}
}
