// The V1-P4 exit criterion: the full round-four lifecycle loop, scripted
// end-to-end against a fresh fixturegit repo (03 §Lifecycle: the
// feature-first cascade; §The amendment ladder rungs 3 and 4).
//
// Script: design start --kind feature (with an optional epic ref) -> edit
// -> accept -> design start --kind story (stub-matched) -> accept (stamps
// stub_matched: true) -> build start succeeds only once accepted -> ONE
// rung-3 event (story supersession: file a conflict, author story-spec v2,
// accept it, re-point the build branch) -> ONE rung-4 event (feature
// supersession: file a conflict, supersede with a supersession: block,
// verify the computed blast-radius quorum disclosure, verify a stale story
// is refused by build start until re-affirmed, then succeeds once it is).
//
// A round-four class: feature spec is birds-eye and implementation-blind
// (03 §The feature fold) and is never itself buildable — this supersedes
// v0's single-level model, where "feature start" built directly against a
// class: feature spec. The pre-round-four ritual test this file used to
// carry (design start a class: feature spec, then feature start it
// directly) is no longer a legal round-four sequence: `verdi build
// start`/the `feature start` alias now REFUSE a class: feature spec
// outright (buildstart.go) — a deliberate behavior change, not a
// regression, and exactly what 03's two-level model requires.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/provider"
	providerfake "github.com/OWNER/verdi/internal/provider/fake"
	"github.com/OWNER/verdi/internal/store"
)

// seedRitualProvider seeds titles for both tracker refs the round-four
// loop below uses: the feature's optional epic ref, and the story's ref
// (title "Stale Decline" — store.RefSlug("Stale Decline") == "stale-decline",
// the stub slug editFeatureStub below plants on the feature, so the
// story's design-start scaffold, whose title comes straight from this
// provider, drives R4-I-12's RefSlug(title)-equals-stub-slug condition
// without any further editing of the story's own title).
func seedRitualProvider(t *testing.T) *providerfake.Provider {
	t.Helper()
	p := providerfake.New()
	p.SeedStory(provider.Story{Ref: "jira:LOAN-1483", Title: "Loan management Q3", Status: "Open", URL: "https://example.atlassian.net/browse/LOAN-1483"})
	p.SeedStory(provider.Story{Ref: "jira:LOAN-1482", Title: "Stale Decline", Status: "In Progress", URL: "https://example.atlassian.net/browse/LOAN-1482"})
	return p
}

// editSpecField does a regex find-and-replace against one spec.md's
// frontmatter and commits the edit — the test's stand-in for the ordinary
// design-branch content editing every scaffold's TODOs expect (design.go's
// scaffolds are deliberately minimal placeholders; ritual_test.go's own
// injectImpacts established this pattern pre-round-four).
func editSpecField(t *testing.T, ctx context.Context, root, name string, re *regexp.Regexp, replacement, commitMsg string) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "specs", "active", name, "spec.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !re.Match(raw) {
		t.Fatalf("spec.md at %s does not match the expected edit anchor %q", path, re.String())
	}
	edited := re.ReplaceAll(raw, []byte(replacement))

	// Self-validate before committing (CLAUDE.md: never fake success).
	fm, _, err := artifact.SplitFrontmatter(edited)
	if err != nil {
		t.Fatalf("edited %s failed to split frontmatter: %v", path, err)
	}
	if _, err := artifact.DecodeSpec(fm); err != nil {
		t.Fatalf("edited %s failed to decode: %v\n--- content ---\n%s", path, err, edited)
	}

	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatalf("writing edited %s: %v", path, err)
	}
	if err := gitx.AddAll(ctx, root); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, root, commitMsg); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
}

// fileConflict writes .verdi/conflicts/<name>.md and commits it — 03
// §Challenging closed decisions step 1, "this is also the rung-3/4
// blocker record": both rungs below open by filing a conflict here, with
// the build discovery as witness.
func fileConflict(t *testing.T, ctx context.Context, root, name, challengesRef, witness string) {
	t.Helper()
	content := fmt.Sprintf(`---
id: conflict/%s
kind: conflict
title: %q
owners: [platform-team]
status: open
links:
  - { type: challenges, ref: %s, note: %q }
---
# %s
`, name, name, challengesRef, witness, name)
	dir := filepath.Join(root, ".verdi", "conflicts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, name+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	if err := gitx.AddAll(ctx, root); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, root, "conflict: file "+name); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
}

// writeDraftSpec cuts a design branch and writes content verbatim as a new
// spec — used for the two rung-3/rung-4 superseding revisions, which carry
// supersession: blocks and multi-entry links: no scaffold function
// produces (design.go's scaffolds are the ordinary, no-supersession
// first-acceptance shape).
func writeDraftSpec(t *testing.T, ctx context.Context, root, name, content string) {
	t.Helper()
	branch := "design/" + name
	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		t.Fatalf("checking out %s: %v", branch, err)
	}
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("%s: split frontmatter: %v", name, err)
	}
	if _, err := artifact.DecodeSpec(fm); err != nil {
		t.Fatalf("%s: decode: %v\n--- content ---\n%s", name, err, content)
	}
	dir := filepath.Join(root, ".verdi", "specs", "active", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing spec.md: %v", err)
	}
	if err := gitx.AddAll(ctx, root); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, root, "design start: scaffold spec/"+name); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
}

// writeReaffirmation writes a re-affirmation record satisfying
// verdi.reaffirmation/v1 (02 §Record schemas; 03 §The amendment ladder
// rung 4) at reaffirmations/<storySlug>/<objectID>.md and commits it.
func writeReaffirmation(t *testing.T, ctx context.Context, root, storySlug, objectID, pinnedObjectRef string) {
	t.Helper()
	content := fmt.Sprintf(`---
id: reaffirmation/%s--%s
kind: reaffirmation
title: "Re-affirm %s for %s"
owners: [platform-team]
object: %s
hash: { old: %s, new: %s }
frozen: { at: 2024-06-01, commit: 0000000000000000000000000000000000000c }
---
# Re-affirmation
`, storySlug, objectID, objectID, storySlug, pinnedObjectRef,
		"sha256:"+strings.Repeat("0", 64), "sha256:"+strings.Repeat("1", 64))

	dir := filepath.Join(root, ".verdi", "reaffirmations", storySlug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, objectID+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	if err := gitx.AddAll(ctx, root); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, root, "reaffirm: "+storySlug+"/"+objectID); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
}

var stubSlugRe = regexp.MustCompile(`slug: todo-replace-stub-slug`)
var storyImplementsPlaceholderRe = regexp.MustCompile(`links:\n  - \{ type: implements, ref: "spec/todo-replace-feature-name#ac-1" \}\n`)

// TestRoundFourRitual_FullLoop drives 03 §Lifecycle's whole feature-first
// cascade end to end: feature design/accept, stub-matched story
// design/accept/build, one rung-3 story supersession, and one rung-4
// feature supersession with its computed blast-radius quorum disclosure
// and re-affirmation enforcement.
func TestRoundFourRitual_FullLoop(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	prov := seedRitualProvider(t)
	designDepsV := designDeps{Provider: prov, Runner: nil, GoTest: fakeGoTest{}}
	buildDeps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	// --- 1. verdi design start jira:LOAN-1483 --kind feature --name loan-mgmt ---
	// (an epic/objective ref stands in for 02 §Kind registry's own
	// okr:LOAN-Q3 example: no provider models an "okr" scheme, matching
	// R4-I-23(b)'s own precedent of removing that exact literal from the
	// v2 fixture rather than inventing an OKR provider type.)
	var stdout, stderr bytes.Buffer
	if got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1483", "loan-mgmt", manifest, designDepsV, &stdout, &stderr); got != 0 {
		t.Fatalf("design start (feature) = %d, want 0; stderr=%s", got, stderr.String())
	}
	feature, _ := readSpec(t, repo.Dir, "loan-mgmt")
	if feature.Class != artifact.ClassFeature || len(feature.Stubs) == 0 || feature.Problem == nil {
		t.Fatalf("scaffolded feature spec missing expected shape: %+v", feature)
	}

	// --- 2. edit: point the scaffolded stub at the story we're about to design ---
	editSpecField(t, ctx, repo.Dir, "loan-mgmt", stubSlugRe, "slug: stale-decline", "edit: point stub at stale-decline")

	// --- 3. verdi accept spec/loan-mgmt ---
	stdout.Reset()
	stderr.Reset()
	if got := runAccept(ctx, repo.Dir, "spec/loan-mgmt", &stdout, &stderr); got != 0 {
		t.Fatalf("accept (feature) = %d, want 0; stderr=%s", got, stderr.String())
	}
	feature, _ = readSpec(t, repo.Dir, "loan-mgmt")
	if feature.Status != "accepted-pending-build" {
		t.Fatalf("feature.Status = %q, want accepted-pending-build", feature.Status)
	}

	// --- 4. verdi design start jira:LOAN-1482 --kind story --name stale-decline-story ---
	stdout.Reset()
	stderr.Reset()
	if got := runDesignStart(ctx, repo.Dir, artifact.ClassStory, "jira:LOAN-1482", "stale-decline-story", manifest, designDepsV, &stdout, &stderr); got != 0 {
		t.Fatalf("design start (story) = %d, want 0; stderr=%s", got, stderr.String())
	}
	story, _ := readSpec(t, repo.Dir, "stale-decline-story")
	if story.Title != "Stale Decline" {
		t.Fatalf("story.Title = %q, want the provider-resolved title Stale Decline", story.Title)
	}

	// --- 5. edit: point the story's implements edge at the real feature AC ---
	editSpecField(t, ctx, repo.Dir, "stale-decline-story", storyImplementsPlaceholderRe,
		"links:\n  - { type: implements, ref: \"spec/loan-mgmt#ac-1\" }\n", "edit: implement loan-mgmt#ac-1")

	// --- 6. verdi accept spec/stale-decline-story: stub-matched fast path ---
	stdout.Reset()
	stderr.Reset()
	if got := runAccept(ctx, repo.Dir, "spec/stale-decline-story", &stdout, &stderr); got != 0 {
		t.Fatalf("accept (story) = %d, want 0; stderr=%s", got, stderr.String())
	}
	if !contains(stdout.String(), "stub-matched") {
		t.Fatalf("accept stdout = %q, want a stub-matched disclosure", stdout.String())
	}
	story, _ = readSpec(t, repo.Dir, "stale-decline-story")
	if story.Status != "accepted-pending-build" {
		t.Fatalf("story.Status = %q, want accepted-pending-build", story.Status)
	}
	if story.Frozen == nil || !story.Frozen.StubMatched {
		t.Fatalf("story.Frozen.StubMatched = %v, want true (R4-I-12)", story.Frozen)
	}

	// --- 7. verdi build start spec/stale-decline-story: succeeds only against accepted-pending-build ---
	stdout.Reset()
	stderr.Reset()
	if got := runBuildStart(ctx, repo.Dir, "spec/stale-decline-story", buildDeps, &stdout, &stderr); got != 0 {
		t.Fatalf("build start (story) = %d, want 0; stderr=%s", got, stderr.String())
	}
	branch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/stale-decline-story" {
		t.Fatalf("branch after build start = %q, want feature/stale-decline-story", branch)
	}

	// verdi feature start (the deprecation alias) prints the new form and
	// proceeds rather than erroring — proven directly against
	// runFeatureStart/runBuildStart's shared precondition set in
	// feature_test.go; not re-proven here to keep this script's single
	// linear branch history unambiguous (a second alias-driven build-start
	// call here would collide with the branch feature/stale-decline-story
	// this step already cut).

	// ================= Rung 3: story supersession =================

	// --- 8. file a conflict: the story's own approach was wrong ---
	fileConflict(t, ctx, repo.Dir, "story-approach-wrong", "spec/stale-decline-story",
		"discovered during build: the API contract in stale-decline-story is wrong; feature ACs unaffected")

	// --- 9. author story-spec v2 (supersedes v1) on a design branch ---
	storyV2 := `---
id: spec/stale-decline-story-v2
kind: spec
title: "Stale Decline"
owners: [platform-team]
class: story
status: draft
story: jira:LOAN-1482
problem: { text: "borrowers see stale decline data", anchor: problem }
outcome: { text: "borrowers see current decline data", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
links:
  - { type: implements, ref: "spec/loan-mgmt#ac-1" }
  - { type: supersedes, ref: "spec/stale-decline-story" }
---
# Stale Decline (v2)

## Problem
x
## Outcome
x
`
	writeDraftSpec(t, ctx, repo.Dir, "stale-decline-story-v2", storyV2)

	// --- 10. accept it: R4-I-12's stub-match is re-computed, disqualified
	// only by the introduced supersedes edge — the implements-set/title-slug
	// halves of the test still resolve to the SAME stub (the feature
	// mapping is unchanged, 03's own words), proving the matching machinery
	// itself is unaffected by the supersession; the supersedes edge alone
	// is what routes v2 to full review, exactly as R4-I-12 specifies
	// ("the story introduces no supersedes/exempts edges" is one of its
	// four conjuncts, not waived for rung-3 revisions).
	stdout.Reset()
	stderr.Reset()
	if got := runAccept(ctx, repo.Dir, "spec/stale-decline-story-v2", &stdout, &stderr); got != 0 {
		t.Fatalf("accept (story v2) = %d, want 0; stderr=%s", got, stderr.String())
	}
	if !contains(stdout.String(), "not stub-matched") || !contains(stdout.String(), "supersedes") {
		t.Fatalf("accept (story v2) stdout = %q, want a not-stub-matched disclosure naming the supersedes edge", stdout.String())
	}
	storyV2Spec, _ := readSpec(t, repo.Dir, "stale-decline-story-v2")
	if storyV2Spec.Frozen == nil || storyV2Spec.Frozen.StubMatched {
		t.Fatalf("story v2 Frozen = %+v, want stub_matched false (supersedes edge disqualifies it)", storyV2Spec.Frozen)
	}

	// ================= Rung 4: feature supersession =================

	// --- 11. file a conflict: a feature AC itself was wrong for everyone ---
	fileConflict(t, ctx, repo.Dir, "feature-ac-wrong", "spec/loan-mgmt",
		"discovered during build: ac-1's declared text under-specifies the outcome; every implementing story is affected")

	// --- 12. supersede the feature with a supersession: block, amending ac-1 ---
	featureV2 := `---
id: spec/loan-mgmt-v2
kind: spec
title: "Loan management Q3"
owners: [platform-team]
class: feature
status: draft
story: jira:LOAN-1483
problem: { text: "borrowers cannot see their loan status accurately", anchor: problem }
outcome: { text: "borrowers see accurate, current loan status", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds, corrected", evidence: [static, attestation] }
links:
  - { type: supersedes, ref: "spec/loan-mgmt" }
supersession:
  amended:
    - { id: ac-1, note: "AC text corrected mid-build (rung-4 discovery)" }
---
# Loan management Q3 (v2)

## Problem
x
## Outcome
x
`
	writeDraftSpec(t, ctx, repo.Dir, "loan-mgmt-v2", featureV2)

	// --- 13. accept it: verdi computes and discloses the blast-radius quorum ---
	stdout.Reset()
	stderr.Reset()
	if got := runAccept(ctx, repo.Dir, "spec/loan-mgmt-v2", &stdout, &stderr); got != 0 {
		t.Fatalf("accept (feature v2) = %d, want 0; stderr=%s", got, stderr.String())
	}
	if !contains(stdout.String(), "computed quorum: two-code-owner") {
		t.Fatalf("accept (feature v2) stdout = %q, want a computed two-code-owner quorum disclosure (both stale-decline-story and its v2 are still in-flight, implementing the amended ac-1)", stdout.String())
	}
	if contains(stdout.String(), "verdi behavior") == false {
		t.Fatalf("accept (feature v2) stdout = %q, want the disclosed 'never verdi behavior' approval-count non-enforcement note", stdout.String())
	}
	featureV2Spec, _ := readSpec(t, repo.Dir, "loan-mgmt-v2")
	v2Commit := featureV2Spec.Frozen.Commit

	// --- 14. verify a stale story is refused by verdi build start until a
	// re-affirmation record exists (story v2 has not been built yet — its
	// build branch was never cut, so this is build start's own precondition
	// check, not a re-check of an existing branch). ---
	stdout.Reset()
	stderr.Reset()
	got := runBuildStart(ctx, repo.Dir, "spec/stale-decline-story-v2", buildDeps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("build start (story v2, pre-reaffirmation) = %d, want 1 (refused); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !contains(stderr.String(), "stale") || !contains(stderr.String(), "re-affirmation") {
		t.Fatalf("stderr = %q, want it to name the stale/re-affirmation refusal", stderr.String())
	}
	if branch, berr := gitx.CurrentBranch(ctx, repo.Dir); berr != nil || branch == "feature/stale-decline-story-v2" {
		t.Fatalf("a refused build start must not cut the build branch: branch=%q err=%v", branch, berr)
	}

	// --- 15. add the re-affirmation record and verify build start now succeeds ---
	writeReaffirmation(t, ctx, repo.Dir, store.RefSlug("jira:LOAN-1482"), "ac-1",
		fmt.Sprintf("spec/loan-mgmt-v2@%s#ac-1", v2Commit))

	stdout.Reset()
	stderr.Reset()
	if got := runBuildStart(ctx, repo.Dir, "spec/stale-decline-story-v2", buildDeps, &stdout, &stderr); got != 0 {
		t.Fatalf("build start (story v2, post-reaffirmation) = %d, want 0; stderr=%s", got, stderr.String())
	}
	branch, err = gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/stale-decline-story-v2" {
		t.Fatalf("branch after build start (v2) = %q, want feature/stale-decline-story-v2 (re-pointed build branch)", branch)
	}
}
