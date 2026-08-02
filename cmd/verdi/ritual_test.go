// The V1-P4 exit criterion: the full round-four lifecycle loop, scripted
// end-to-end against a fresh fixturegit repo (03 §Lifecycle: the
// feature-first cascade; §The amendment ladder rungs 3 and 4).
//
// Script: design start --kind feature (with an optional epic ref) -> edit
// -> merge into main (merge-signaled acceptance) -> design start --kind
// story -> merge -> build start succeeds only once merge-accepted -> ONE
// rung-3 event (story supersession: file a conflict, author story-spec v2,
// merge it) -> ONE rung-4 event (feature supersession: file a conflict,
// supersede with a supersession: block, merge it, verify the derived
// Superseded state, verify a stale story is refused by build start until
// re-affirmed, then succeeds once it is).
//
// Task 7 (docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-
// design.md) retires `verdi accept`'s mutation entirely: acceptance is now
// merge-signaled, so every step that used to call runAccept now performs a
// real (test-local, raw-git) merge into the default branch instead —
// mirroring internal/specstate/resolve_integration_test.go's own
// buildLandedSpecRepo convention, since gitx/specstate expose no
// production merge API (a repository-authority action, never something
// verdi automates). Two consequences, both deliberate and disclosed rather
// than worked around:
//
//   - R4-I-12's stub-match computation and the rung-4 blast-radius quorum
//     disclosure were BOTH computed and printed exclusively inside accept's
//     own ritual (accept.go's old runAccept, stubmatch.go, blastradius.go).
//     Neither has any other production caller — deleting accept's
//     mutation makes both currently unreachable in production. Relocating
//     them into the pre-merge gate is explicit FUTURE work (the design's
//     "Rollout sequence" step 8: "Apply the ceremony audit to closure,
//     supersession, build start, ... before their implementation begins"),
//     out of this task's scope — this script no longer asserts on either.
//   - I-40 (invention ledger, open owner question): a story-class spec can
//     never carry a `supersession:` block (internal/artifact's
//     validateStory rejects it outright), so internal/specstate's two-
//     signal successor-corpus proof can NEVER independently confirm
//     story-level (rung-3) supersession, and supersede.go — the only
//     writer of a legacy `status: superseded` story flip — is deleted by
//     this same task. The projector now records the one-signal supersedes
//     link the successor DOES carry (final fix wave I4) and projects the
//     story predecessor DISCLOSED-UNPROVEN — naming the claiming successor
//     and the missing supersession proof — rather than either silently
//     accepting it as still-buildable or inventing a mechanism to call it
//     Superseded (BINDING per the task brief: "do NOT invent a new
//     story-supersession mechanism"). Build start on such a predecessor
//     exits 2 (cannot honestly decide the precondition). Rung-4
//     (feature-class predecessor/successor) has no such gap — a feature
//     CAN carry `supersession:`, so its derived-Superseded proof below is
//     real and positive, exercised through the full CLI/build-branch
//     workflow (supersedepredecessor_test.go proves the same derivation
//     at a more unit-scoped level).
//
// A round-four class: feature spec is birds-eye and implementation-blind
// (03 §The feature fold) and is never itself buildable — `verdi build
// start`/the `feature start` alias REFUSE a class: feature spec outright
// (buildstart.go).
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/provider"
	providerfake "github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// seedRitualProvider seeds titles for both tracker refs the round-four
// loop below uses: the feature's optional epic ref, and the story's ref
// (title "Stale Decline" — store.RefSlug("Stale Decline") == "stale-decline",
// the slug editFeatureStub below plants on the feature's stub).
func seedRitualProvider(t *testing.T) *providerfake.Provider {
	t.Helper()
	p := providerfake.New()
	p.SeedStory(provider.Story{Ref: "jira:LOAN-1483", Title: "Loan management Q3", Status: "Open", URL: "https://example.atlassian.net/browse/LOAN-1483"})
	p.SeedStory(provider.Story{Ref: "jira:LOAN-1482", Title: "Stale Decline", Status: "In Progress", URL: "https://example.atlassian.net/browse/LOAN-1482"})
	return p
}

// editSpecField does a regex find-and-replace against one spec.md's
// frontmatter and commits the edit — the test's stand-in for the ordinary
// design-branch content editing every scaffold's TODOs expect.
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

// fileConflict writes .verdi/conflicts/<name>.md and commits it directly
// on whatever branch is currently checked out — 03 §Challenging closed
// decisions step 1, "this is also the rung-3/4 blocker record". Conflict
// records are never git-ref-resolved by any check this script exercises,
// so which branch carries the commit is immaterial to every assertion
// below; matching the pre-Task-7 script's own low-ceremony convention.
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

// writeDraftSpec cuts a design branch (from whatever is currently checked
// out — callers checkout main first) and writes content verbatim as a new
// spec — used for the two rung-3/rung-4 superseding revisions, which carry
// supersession: blocks and multi-entry links: no scaffold function
// produces (design.go's scaffolds are the ordinary, no-supersession
// first-proposal shape).
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

// checkoutMain returns to the default branch — every merge-signaled
// "acceptance" step below lands its design branch here, and every design/
// build branch this script cuts must start from main's current tip so the
// resulting history is a realistic, linear sequence of landed proposals.
func checkoutMain(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	if err := gitx.CheckoutExisting(ctx, root, "main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
}

// mergeIntoMain simulates a real specification (or code) pull request
// landing on the default branch — the merge-signaled design's own
// acceptance ceremony (docs/superpowers/specs/2026-08-01-merge-signals-
// spec-acceptance-design.md): checks out main and merges branch with
// --no-ff, mirroring internal/specstate/resolve_integration_test.go's own
// buildLandedSpecRepo. gitx/specstate expose no production merge API — a
// repository-authority action verdi never automates — so only this
// test-local raw git invocation performs it, never accept.go or any other
// production code path.
func mergeIntoMain(t *testing.T, ctx context.Context, root, branch string) {
	t.Helper()
	checkoutMain(t, ctx, root)
	cmd := exec.Command("git", "merge", "--no-ff", "--no-edit", branch)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git merge --no-ff %s: %v\n%s", branch, err, out)
	}
}

var stubSlugRe = regexp.MustCompile(`slug: todo-replace-stub-slug`)
var storyImplementsPlaceholderRe = regexp.MustCompile(`links:\n  - \{ type: implements, ref: "spec/todo-replace-feature-name#ac-1" \}\n`)

// TestRoundFourRitual_FullLoop drives 03 §Lifecycle's whole feature-first
// cascade end to end under the merge-signaled design: feature design/
// merge, story design/merge/build, one rung-3 story supersession (I-40's
// disclosed gap: the predecessor's Git-derived state cannot follow, proven
// honestly rather than invented around), and one rung-4 feature
// supersession with its derived Superseded state and re-affirmation
// enforcement.
func TestRoundFourRitual_FullLoop(t *testing.T) {
	repo := buildPhase7Repo(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	ctx := context.Background()
	manifest := phase7Manifest(t)
	prov := seedRitualProvider(t)
	designDepsV := designDeps{Provider: prov, Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}
	buildDeps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	projector := specstate.NewProjector()

	// --- 1. verdi design start jira:LOAN-1483 --kind feature --name loan-mgmt ---
	var stdout, stderr bytes.Buffer
	if got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1483", "loan-mgmt", manifest, phase7Model(t), designDepsV, &stdout, &stderr); got != 0 {
		t.Fatalf("design start (feature) = %d, want 0; stderr=%s", got, stderr.String())
	}
	feature, _ := readSpec(t, repo.Dir, "loan-mgmt")
	if feature.Class != artifact.ClassFeature || len(feature.Stubs) == 0 || feature.Problem == nil {
		t.Fatalf("scaffolded feature spec missing expected shape: %+v", feature)
	}

	// --- 2. edit: point the scaffolded stub at the story we're about to design ---
	editSpecField(t, ctx, repo.Dir, "loan-mgmt", stubSlugRe, "slug: stale-decline", "edit: point stub at stale-decline")

	// --- 3. merge spec/loan-mgmt's design branch into main: THIS is acceptance now ---
	mergeIntoMain(t, ctx, repo.Dir, "design/loan-mgmt")
	if result := resolveCandidate(t, ctx, repo.Dir, "loan-mgmt"); result.State != specstate.AcceptedPendingBuild {
		t.Fatalf("Resolve(loan-mgmt) after merge = %+v, want AcceptedPendingBuild", result)
	}

	// --- 4. verdi design start jira:LOAN-1482 --kind story --name stale-decline-story ---
	stdout.Reset()
	stderr.Reset()
	if got := runDesignStart(ctx, repo.Dir, artifact.ClassStory, "jira:LOAN-1482", "stale-decline-story", manifest, phase7Model(t), designDepsV, &stdout, &stderr); got != 0 {
		t.Fatalf("design start (story) = %d, want 0; stderr=%s", got, stderr.String())
	}
	story, _ := readSpec(t, repo.Dir, "stale-decline-story")
	if story.Title != "Stale Decline" {
		t.Fatalf("story.Title = %q, want the provider-resolved title Stale Decline", story.Title)
	}

	// --- 5. edit: point the story's implements edge at the real feature AC ---
	editSpecField(t, ctx, repo.Dir, "stale-decline-story", storyImplementsPlaceholderRe,
		"links:\n  - { type: implements, ref: \"spec/loan-mgmt#ac-1\" }\n", "edit: implement loan-mgmt#ac-1")

	// --- 6. merge spec/stale-decline-story's design branch into main ---
	mergeIntoMain(t, ctx, repo.Dir, "design/stale-decline-story")
	if result := resolveCandidate(t, ctx, repo.Dir, "stale-decline-story"); result.State != specstate.AcceptedPendingBuild {
		t.Fatalf("Resolve(stale-decline-story) after merge = %+v, want AcceptedPendingBuild", result)
	}
	_, predV1RawBeforeRung3 := readSpec(t, repo.Dir, "stale-decline-story")

	// --- 7. verdi build start spec/stale-decline-story: succeeds only against a landed, accepted spec ---
	checkoutMain(t, ctx, repo.Dir)
	stdout.Reset()
	stderr.Reset()
	if got := runBuildStart(ctx, repo.Dir, "spec/stale-decline-story", projector, buildDeps, &stdout, &stderr); got != 0 {
		t.Fatalf("build start (story) = %d, want 0; stderr=%s", got, stderr.String())
	}
	branch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/stale-decline-story" {
		t.Fatalf("branch after build start = %q, want feature/stale-decline-story", branch)
	}

	// ================= Rung 3: story supersession =================

	checkoutMain(t, ctx, repo.Dir)

	// --- 8. file a conflict: the story's own approach was wrong ---
	fileConflict(t, ctx, repo.Dir, "story-approach-wrong", "spec/stale-decline-story",
		"discovered during build: the API contract in stale-decline-story is wrong; feature ACs unaffected")

	// --- 9. author story-spec v2 (supersedes v1) on a design branch cut from main ---
	storyV2 := `---
id: spec/stale-decline-story-v2
kind: spec
title: "Stale Decline"
owners: [platform-team]
class: story
story: jira:LOAN-1482
problem: { text: "borrowers see stale decline data", anchor: problem }
outcome: { text: "borrowers see current decline data", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static], anchor: ac-1 }
links:
  - { type: implements, ref: "spec/loan-mgmt#ac-1" }
  - { type: supersedes, ref: "spec/stale-decline-story" }
---
# Stale Decline (v2)

## Problem
x
## Outcome
x
## AC-1
x
`
	writeDraftSpec(t, ctx, repo.Dir, "stale-decline-story-v2", storyV2)

	// --- 10. merge spec/stale-decline-story-v2's design branch into main ---
	mergeIntoMain(t, ctx, repo.Dir, "design/stale-decline-story-v2")

	predV1Result := resolveCandidate(t, ctx, repo.Dir, "stale-decline-story")
	// I-40's gap, now DISCLOSED at the projection (final fix wave I4): a
	// story-class spec can never carry a `supersession:` block, so
	// internal/specstate's two-signal successor-corpus proof can never
	// independently confirm story-level supersession, and there is no
	// more writer of a legacy `status: superseded` flip (supersede.go is
	// deleted). The predecessor's Git-derived state is therefore UNPROVEN
	// with a disclosure naming the one-signal successor and the missing
	// proof — never silently AcceptedPendingBuild (a reviewed, merged
	// successor's claim is not nothing), and never an invented Superseded
	// (one signal is not proof). Three-valued honesty, proven here.
	if predV1Result.State != specstate.Unproven {
		t.Fatalf("Resolve(stale-decline-story) after its successor merged = %+v, want Unproven (I-40's gap is disclosed, never silent)", predV1Result)
	}
	wantDisclosed := false
	for _, d := range predV1Result.Disclosures {
		if strings.Contains(d, "stale-decline-story-v2") && strings.Contains(d, "supersession") {
			wantDisclosed = true
		}
	}
	if !wantDisclosed {
		t.Fatalf("Resolve(stale-decline-story) disclosures = %v, want one naming the one-signal successor stale-decline-story-v2 and the missing supersession proof", predV1Result.Disclosures)
	}
	// Build start on the disclosed-unproven predecessor exits 2: it cannot
	// honestly decide the acceptance precondition either way.
	stdout.Reset()
	stderr.Reset()
	if got := runBuildStart(ctx, repo.Dir, "spec/stale-decline-story", projector, buildDeps, &stdout, &stderr); got != 2 {
		t.Fatalf("build start (disclosed-unproven predecessor) = %d, want 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !contains(stderr.String(), "cannot be proven accepted") || !contains(stderr.String(), "stale-decline-story-v2") {
		t.Fatalf("stderr = %q, want the unproven refusal carrying the projector's own successor-naming disclosure", stderr.String())
	}
	predV1, predV1Raw := readSpec(t, repo.Dir, "stale-decline-story")
	if predV1.Status != "" && predV1.Status != "draft" {
		t.Fatalf("predecessor v1 status = %q — must never be mutated by a merge (no writer exists anymore)", predV1.Status)
	}
	if !bytes.Equal(predV1RawBeforeRung3, predV1Raw) {
		t.Fatalf("predecessor v1's bytes changed across its successor's merge — supersession must be derived, never written:\n--- before ---\n%s\n--- after ---\n%s", predV1RawBeforeRung3, predV1Raw)
	}

	if result := resolveCandidate(t, ctx, repo.Dir, "stale-decline-story-v2"); result.State != specstate.AcceptedPendingBuild {
		t.Fatalf("Resolve(stale-decline-story-v2) = %+v, want AcceptedPendingBuild", result)
	}

	// ================= Rung 4: feature supersession =================

	checkoutMain(t, ctx, repo.Dir)

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
story: jira:LOAN-1483
problem: { text: "borrowers cannot see their loan status accurately", anchor: problem }
outcome: { text: "borrowers see accurate, current loan status", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds, corrected", evidence: [static, attestation], anchor: ac-1 }
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
## AC-1
x
`
	writeDraftSpec(t, ctx, repo.Dir, "loan-mgmt-v2", featureV2)

	// --- 13. merge spec/loan-mgmt-v2's design branch into main: derived supersession fires ---
	mergeIntoMain(t, ctx, repo.Dir, "design/loan-mgmt-v2")

	predFeatureResult := resolveCandidate(t, ctx, repo.Dir, "loan-mgmt")
	if predFeatureResult.State != specstate.Superseded {
		t.Fatalf("Resolve(loan-mgmt) after loan-mgmt-v2 merged = %+v, want Superseded (feature-class supersession IS Git-derivable, unlike story-class — I-40)", predFeatureResult)
	}
	_, predFeatureRawAfter := readSpec(t, repo.Dir, "loan-mgmt")
	if predFeatureRawAfter == nil {
		t.Fatal("predecessor feature spec unreadable after merge")
	}

	loanMgmtV2Spec, _ := readSpec(t, repo.Dir, "loan-mgmt-v2")
	v2Commit := loanMgmtV2Spec.Frozen
	if v2Commit != nil {
		t.Fatalf("loan-mgmt-v2 carries a frozen stamp %+v — new merge-accepted artifacts need none (design's Artifact compatibility section)", v2Commit)
	}
	v2Landing := resolveCandidate(t, ctx, repo.Dir, "loan-mgmt-v2")
	if v2Landing.Baseline == nil || v2Landing.Baseline.LandingCommit == "" {
		t.Fatalf("Resolve(loan-mgmt-v2) = %+v, want a resolved Git-derived landing commit", v2Landing)
	}

	// --- 14. verify a stale story is refused by verdi build start until a
	// re-affirmation record exists (its build branch was already cut in
	// step 7's PREDECESSOR proof, not for THIS spec — feature/
	// stale-decline-story-v2 has never been cut, so this is build start's
	// own precondition check, not a re-check of an existing branch). ---
	checkoutMain(t, ctx, repo.Dir)
	stdout.Reset()
	stderr.Reset()
	got := runBuildStart(ctx, repo.Dir, "spec/stale-decline-story-v2", projector, buildDeps, &stdout, &stderr)
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
		fmt.Sprintf("spec/loan-mgmt-v2@%s#ac-1", v2Landing.Baseline.LandingCommit))

	stdout.Reset()
	stderr.Reset()
	if got := runBuildStart(ctx, repo.Dir, "spec/stale-decline-story-v2", projector, buildDeps, &stdout, &stderr); got != 0 {
		t.Fatalf("build start (story v2, post-reaffirmation) = %d, want 0; stderr=%s", got, stderr.String())
	}
	branch, err = gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/stale-decline-story-v2" {
		t.Fatalf("branch after build start (v2) = %q, want feature/stale-decline-story-v2", branch)
	}
}
