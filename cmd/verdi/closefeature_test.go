package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/fixturegit"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// This file proves spec/close-verb's deferred feature half (closefeature.go
// / closuregatefeature.go): `verdi close <feature-spec>` archives a feature
// spec exactly as 03 §Closure ritual and 05 §CLI describe, on a fixture
// deliberately shaped so a FULLY clean `verdi lint` pass over the
// post-close store is achievable and asserted — not just the two rules
// (VL-002/VL-010) close_test.go's own story-level re-lint narrowly checks.
// That requires every frozen: stamp in the fixture to cite REAL git
// history (VL-009), which creates a chicken-and-egg problem an ordinary
// single-layer fixture can't solve: a committed file cannot embed its own
// future commit SHA. featureCloseScaffoldSHA resolves this the way
// fixturegit's own Repo.Heads doc comment anticipates ("callers that pin
// frontmatter refs or frozen stamps at a specific, earlier layer's
// commit... need these"): a throwaway prelude build learns the
// deterministic SHA fixturegit.Build assigns a given layer (proven
// byte-stable by fixturegit_test.go's own TestBuild_Deterministic), and
// that SAME layer is then reused as layer one of the real, two-layer
// build — so its SHA, now known, can be embedded in layer two's spec
// content and still resolve as real history.

// featureCloseScaffoldLayer is layer one of every fixture this file
// builds: verdi.yaml (github forge + a configured jira provider, so both
// the story specs' required story: refs and the feature's optional epic
// ref pass VL-005), .verdi/.gitignore (excluding data/, so the close
// ritual's own `git add -A` — cmd/verdi/closefeature.go's runCloseFeature
// — never stages the evidence records this file writes directly to
// .verdi/data/derived/, keeping VL-013 clean), and .gitattributes (the
// three generated-path lines VL-012 requires for the github forge).
var featureCloseScaffoldLayer = fixturegit.Layer{
	Files: map[string]string{
		".verdi/verdi.yaml": `schema: verdi.layout/v1
forge: github
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
`,
		".verdi/.gitignore": "data/\n",
		".gitattributes": `.verdi/specs/*/*/board.json linguist-generated
.verdi/specs/*/*/rollup.json linguist-generated
.verdi/specs/*/*/deviation-report.md linguist-generated
`,
	},
	Message: "scaffold: verdi.yaml, .gitignore, .gitattributes",
}

// featureCloseScaffoldSHA is the deterministic commit SHA
// featureCloseScaffoldLayer resolves to on its own — computed once via a
// throwaway prelude build and reused as every fixture's frozen.commit
// stamp (see this file's top doc comment).
func featureCloseScaffoldSHA(t *testing.T) string {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{featureCloseScaffoldLayer}).Head
}

// closeFeatureFixtureOpts controls the one axis each subtest deliberately
// varies, holding everything else at the otherwise-closeable happy-path
// shape (buildCloseFeatureRepo's zero value IS the happy path).
type closeFeatureFixtureOpts struct {
	// FeatureStory is the feature's optional story: epic ref ("" — no
	// tracker at all, spec/true-closure's own real shape — is the default
	// and the happy path's own choice, so the happy path doubles as the
	// "skip publish" proof).
	FeatureStory string
	// Story2Status is fixture-story-two's frontmatter status: "closed"
	// (the happy-path default) or "accepted-pending-build". Determines
	// which zone (archive vs active) it is placed in, per VL-002.
	Story2Status string
	// Story2OwnVerdict is the verdict of the one evidence record planted
	// on fixture-story-two's OWN ac-1 (its OWN derived directory, distinct
	// from the feature's) — "pass" (the happy-path default, and Case 1's
	// choice, making story-two self-ELIGIBLE even when not closed) or
	// "fail" (Case 2's mechanism: propagates to story-two's own
	// Violated=true, and so to the feature AC it implements).
	Story2OwnVerdict string
	// FeatureAC2FloorSatisfied controls whether a source: ci PASS record
	// bound directly to the feature's own ac-2 (the outcome floor's
	// automated-record path, 03 §The feature fold) is planted. false is
	// Case 3's mechanism (outcome floor unmet).
	FeatureAC2FloorSatisfied bool
}

// defaultCloseFeatureFixtureOpts is the happy path: both stories closed,
// both self-consistent, both feature ACs' outcome floors satisfied, no
// story: tracker ref on the feature at all.
func defaultCloseFeatureFixtureOpts() closeFeatureFixtureOpts {
	return closeFeatureFixtureOpts{
		FeatureStory:             "",
		Story2Status:             "closed",
		Story2OwnVerdict:         "pass",
		FeatureAC2FloorSatisfied: true,
	}
}

// closeFeatureSpecMD renders the fixture feature's spec.md: two outcome-
// level ACs (evidence: [behavioral, attestation], per 03 §Declarations'
// outcome-floor requirement — satisfied here via direct behavioral records
// rather than attestation files, so the fixture needs no attestation
// artifacts at all), two stubs (one per AC, one per implementing story).
func closeFeatureSpecMD(scaffoldSHA, story string) string {
	storyLine := ""
	if story != "" {
		storyLine = "story: " + story + "\n"
	}
	return `---
id: spec/close-feature-fixture
kind: spec
class: feature
title: "Close feature fixture"
status: accepted-pending-build
owners: [platform-team]
` + storyLine + `problem: { text: "borrowers need x", anchor: "#problem" }
outcome: { text: "borrowers get y", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the first fixture outcome holds", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "the second fixture outcome holds", evidence: [behavioral, attestation], anchor: "#ac-2" }
stubs:
  - { slug: fixture-story-one, acceptance_criteria: [ac-1] }
  - { slug: fixture-story-two, acceptance_criteria: [ac-2] }
frozen: { at: 2024-01-01, commit: ` + scaffoldSHA + ` }
---
# Close feature fixture

## Problem
x

## Outcome
y

## AC-1
z

## AC-2
z
`
}

// closeFeatureStorySpecMD renders one implementing story's spec.md: name,
// status, tracker ref, and the feature AC fragment it implements are all
// parameterized so the same template builds both fixture stories.
func closeFeatureStorySpecMD(name, scaffoldSHA, status, storyRef, implementsAC string) string {
	return `---
id: spec/` + name + `
kind: spec
class: story
title: "Fixture ` + name + `"
status: ` + status + `
owners: [platform-team]
story: ` + storyRef + `
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/close-feature-fixture#` + implementsAC + `" }
acceptance_criteria:
  - { id: ac-1, text: "the story's own obligation holds", evidence: [static], anchor: "#ac-1" }
frozen: { at: 2024-01-01, commit: ` + scaffoldSHA + ` }
---
# Fixture ` + name + `

## Problem
x

## Outcome
y

## AC-1
z
`
}

// closeFeatureStoryObligationMD renders the companion evidence-obligation
// for a closeFeatureStorySpecMD fixture's ac-1 (evidence: [static]) — added
// so this file's own "fully clean verdi lint pass over the post-close
// store" promise (top doc comment) still holds now that VL-020
// (evidence-obligations wave 2, added after this fixture was first written)
// requires one for every non-draft story AC's declared kind.
func closeFeatureStoryObligationMD(name, scaffoldSHA string) string {
	return `---
id: obligation/` + name + `--ac-1--static
kind: obligation
title: "Fixture ` + name + ` ac-1 static obligation"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/` + name + `" }
frozen: { at: 2024-01-01, commit: ` + scaffoldSHA + ` }
---
# Fixture ` + name + ` ac-1 static obligation

What the static evidence must specifically show.
`
}

// storyDirFor returns the specs/{active,archive} zone a story spec at
// status must live under (02 §Kind registry / VL-002: status-in-path for
// the story class).
func storyDirFor(status string) string {
	if status == "closed" {
		return "archive"
	}
	return "active"
}

// buildCloseFeatureRepo builds the fixturegit repo for opts: the scaffold
// layer, then a second layer carrying the feature spec and its two
// implementing stories (fixture-story-one always closed; fixture-story-two
// per opts.Story2Status) — every frozen: stamp citing the real, prebuilt
// scaffold commit (this file's top doc comment).
func buildCloseFeatureRepo(t *testing.T, opts closeFeatureFixtureOpts) *fixturegit.Repo {
	t.Helper()
	scaffoldSHA := featureCloseScaffoldSHA(t)

	story2Dir := storyDirFor(opts.Story2Status)
	return fixturegit.Build(t, []fixturegit.Layer{
		featureCloseScaffoldLayer,
		{
			Files: map[string]string{
				".verdi/specs/active/close-feature-fixture/spec.md":        closeFeatureSpecMD(scaffoldSHA, opts.FeatureStory),
				".verdi/specs/archive/fixture-story-one/spec.md":           closeFeatureStorySpecMD("fixture-story-one", scaffoldSHA, "closed", "jira:FIXTURE-STORY-1", "ac-1"),
				".verdi/specs/" + story2Dir + "/fixture-story-two/spec.md": closeFeatureStorySpecMD("fixture-story-two", scaffoldSHA, opts.Story2Status, "jira:FIXTURE-STORY-2", "ac-2"),
				".verdi/obligations/fixture-story-one/ac-1--static.md":     closeFeatureStoryObligationMD("fixture-story-one", scaffoldSHA),
				".verdi/obligations/fixture-story-two/ac-1--static.md":     closeFeatureStoryObligationMD("fixture-story-two", scaffoldSHA),
			},
			Message: "add close-feature-fixture + its two implementing stories",
		},
	})
}

// writeCloseFeatureGateReport writes deviation-report.md directly into the
// close-feature-fixture spec's own directory (X-13/X-16/X-17's
// closure-gate condition 6 needs a living, fully-dispositioned,
// head-covering report before close will freeze rather than refuse) —
// mirrors close_test.go's own writeCloseGateReport for this file's
// differently-named fixture family.
func writeCloseFeatureGateReport(t *testing.T, root, covers, findingsYAML string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", "close-feature-fixture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
%s
digest: sha256:%s
---
# Alignment report
`, covers, findingsYAML, strings.Repeat("0", 64))
	if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing deviation-report.md: %v", err)
	}
}

// featureFixtureEvidenceJSON renders one verdi.evidence/v1 record bound to
// ac, with the given kind/verdict, authoritative (source: ci) at commit —
// mirrors close_test.go's poisonRecord in shape. Named distinctly from
// rollup_test.go's own evidenceRecordJSON (a different, ac-1-only,
// full-array-returning helper) to avoid colliding in this package.
func featureFixtureEvidenceJSON(ac, kind, verdict, commit string) string {
	return `{"schema":"verdi.evidence/v1","evidence_for":["` + ac + `"],"kind":"` + kind + `","verdict":"` + verdict + `","witness":"fixture witness","provenance":{"source":"ci","pipeline":"1","job":"1","commit":"` + commit + `"},"digest":"sha256:` + strings.Repeat("a", 64) + `"}`
}

// writeFixtureVerdicts writes records (already-rendered JSON objects) as a
// verdicts.json array under specRef's derived directory at commit —
// mirrors close_test.go's writePoisonLocalRecord, generalized to any
// spec/commit/record set.
func writeFixtureVerdicts(t *testing.T, root, specRef, commit string, records ...string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(specRef), commit)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "[" + strings.Join(records, ",") + "]\n"
	if err := os.WriteFile(filepath.Join(dir, "verdicts.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedCloseFeatureEvidence plants every evidence record opts implies:
// the feature's own outcome-floor records (ac-1 always satisfied; ac-2 per
// opts.FeatureAC2FloorSatisfied), and fixture-story-two's own ac-1 record
// (per opts.Story2OwnVerdict — feeds ITS OWN fold, propagating to
// Violated when "fail").
func seedCloseFeatureEvidence(t *testing.T, root, commit string, opts closeFeatureFixtureOpts) {
	t.Helper()
	featureRecords := []string{featureFixtureEvidenceJSON("ac-1", "behavioral", "pass", commit)}
	if opts.FeatureAC2FloorSatisfied {
		featureRecords = append(featureRecords, featureFixtureEvidenceJSON("ac-2", "behavioral", "pass", commit))
	}
	writeFixtureVerdicts(t, root, "spec/close-feature-fixture", commit, featureRecords...)
	writeFixtureVerdicts(t, root, "spec/fixture-story-two", commit, featureFixtureEvidenceJSON("ac-1", "static", opts.Story2OwnVerdict, commit))
}

// closeFeatureDeps builds the closeDeps every test in this file uses: a
// reachable, empty fake forge (no open MRs — condition 5 passes outright)
// and an empty FakeRunner (the feature carries no impacts:, so
// align.Compute's regeneration loop never actually iterates, but Compute
// still requires a non-nil Runner unconditionally — close_test.go's own
// documented precondition).
func closeFeatureDeps(registry *fake.Provider) closeDeps {
	return closeDeps{Forge: forgefake.New(), Registry: registry, Runner: upstream.NewFakeRunner()}
}

// nonDisclosureFindings filters lint findings down to SeverityViolation
// only — the same notion of "clean" verdi lint's own CLI exit code uses
// (cmd/verdi/lint.go: only a non-disclosure finding flips the exit code).
// VL-017's disclosed-unproven notice fires unconditionally on every
// new-class spec whenever no data/mutable/ checkout is present (an
// ordinary CI clone, and every fixture in this file) — expected, and NOT
// a lint defect, so it must not fail this assertion.
func nonDisclosureFindings(findings []lint.Finding) []lint.Finding {
	var out []lint.Finding
	for _, f := range findings {
		if f.Severity != lint.SeverityDisclosure {
			out = append(out, f)
		}
	}
	return out
}

// TestRunCloseFeature_EndToEnd is the load-bearing happy-path proof: a
// feature whose two ACs are both evidenced (outcome floor satisfied by
// direct behavioral records; both implementing stories closed) and whose
// two stubs are both realized-by their closed implementing stories closes
// cleanly — quartet archived, status flipped, rollup written with an EMPTY
// story (R4-I-2: this feature carries no story: tracker ref at all,
// spec/true-closure's own real shape) and its tracker publish skipped
// (never fabricated) — and, the D6-20 discipline this task is built
// around, the POST-CLOSE STORE RE-LINTS CLEAN: not merely "VL-002/VL-010
// didn't fire" (close_test.go's own narrower check) but a genuinely empty
// non-disclosure finding list from a full 18-rule engine run, the same
// notion of "clean" `verdi lint`'s own exit code uses. A prior close bug
// (D6-20) produced a lint-INVALID archive that a files-exist-only test
// passed; this is exactly the assertion that would have caught it.
func TestRunCloseFeature_EndToEnd(t *testing.T) {
	opts := defaultCloseFeatureFixtureOpts()
	repo := buildCloseFeatureRepo(t, opts)
	seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
	// The corrected closure ritual (X-16): align (a living report covering
	// head) -> disposition (working-tree edit) -> close (X-13/X-16/X-17's
	// closure-gate condition 6).
	writeCloseFeatureGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
	ctx := context.Background()

	fp := fake.New()
	deps := closeFeatureDeps(fp)
	manifest := &store.Manifest{}

	var stdout, stderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/close-feature-fixture", manifest, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runClose(feature) = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}

	// Every condition PASSED (feature-gate rendering, distinct from the
	// story gate's "closure:" label) — including 6. (X-13/X-16/X-17's
	// disposition-completeness condition).
	for _, cond := range []string{"1.", "2.", "3.", "4.", "5.", "6."} {
		if !strings.Contains(stdout.String(), "[PASS] closure(feature): "+cond) {
			t.Fatalf("stdout missing PASS for condition %s:\n%s", cond, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "[FAIL]") {
		t.Fatalf("stdout should show no FAIL condition:\n%s", stdout.String())
	}

	// The quartet: spec.md moved active->archive, status flipped, and
	// nothing else about it changed (board.json is a SEPARATE test below —
	// this fixture never had one, matching spec/true-closure's real
	// 3-member quartet).
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "close-feature-fixture")); !os.IsNotExist(err) {
		t.Fatal("specs/active/close-feature-fixture should not exist after a successful close")
	}
	archiveDir := filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-feature-fixture")

	archivedSpec, err := os.ReadFile(filepath.Join(archiveDir, "spec.md"))
	if err != nil {
		t.Fatalf("reading archived spec.md: %v", err)
	}
	wantArchivedSpec := strings.Replace(closeFeatureSpecMD(scaffoldSHAFromRepo(t, repo), opts.FeatureStory), "status: accepted-pending-build", "status: closed", 1)
	if string(archivedSpec) != wantArchivedSpec {
		t.Fatalf("archived spec.md is not the pre-close content with a sole status: closed flip:\n--- got ---\n%s\n--- want ---\n%s", archivedSpec, wantArchivedSpec)
	}

	if _, err := os.Stat(filepath.Join(archiveDir, "deviation-report.md")); err != nil {
		t.Fatalf("deviation-report.md missing from archive: %v", err)
	}

	rollRaw, err := os.ReadFile(filepath.Join(archiveDir, "rollup.json"))
	if err != nil {
		t.Fatalf("reading archived rollup.json: %v", err)
	}
	roll, err := artifact.DecodeRollup(rollRaw)
	if err != nil {
		t.Fatalf("DecodeRollup: %v", err)
	}
	if roll.Story != "" {
		t.Fatalf("rollup.Story = %q, want empty (feature carries no story: tracker ref, R4-I-2)", roll.Story)
	}
	if !roll.Eligible {
		t.Fatalf("rollup.Eligible = false, want true: %+v", roll)
	}
	if len(roll.Criteria) != 2 {
		t.Fatalf("rollup.Criteria = %+v, want 2 entries", roll.Criteria)
	}
	for _, c := range roll.Criteria {
		if c.Status != artifact.CriterionEvidenced {
			t.Fatalf("rollup.Criteria[%s].Status = %q, want evidenced: %+v", c.ID, c.Status, roll.Criteria)
		}
	}
	wantDigest, err := rollupDigest(*roll)
	if err != nil {
		t.Fatal(err)
	}
	if roll.Digest != wantDigest {
		t.Fatalf("rollup.Digest = %q, recomputed = %q (not recomputable from pinned inputs)", roll.Digest, wantDigest)
	}

	// Publish was skipped, disclosed, never fabricated — no tracker ref on
	// this feature at all.
	if strings.Contains(stdout.String(), "rollup published to") {
		t.Fatalf("stdout should not claim a publish happened: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "no story: tracker ref") {
		t.Fatalf("stdout should disclose why publish was skipped: %s", stdout.String())
	}
	if _, ok := fp.PublishedField(""); ok {
		t.Fatal("fake provider has a published rollup under an empty story ref — publish must never fabricate one")
	}

	if !strings.Contains(stdout.String(), "git push -u origin close/close-feature-fixture") {
		t.Fatalf("stdout should print the push instruction: %s", stdout.String())
	}

	// THE central proof (D6-20 discipline): re-lint the post-close store
	// and assert it is genuinely clean — no finding at any severity beyond
	// VL-017's expected, unconditional disclosure (see
	// nonDisclosureFindings' doc comment).
	lintFindings, err := lint.NewEngine().Run(ctx, repo.Dir, lint.Context{DiffBase: repo.Head}, lint.Options{})
	if err != nil {
		t.Fatalf("re-lint of post-close store: %v", err)
	}
	if bad := nonDisclosureFindings(lintFindings); len(bad) != 0 {
		var msgs []string
		for _, f := range bad {
			msgs = append(msgs, f.String())
		}
		t.Fatalf("re-lint of post-close store is NOT clean (%d non-disclosure finding(s)):\n%s", len(bad), strings.Join(msgs, "\n"))
	}
}

// scaffoldSHAFromRepo re-derives the scaffold layer's SHA from an already-
// built repo's own history (Heads[0] — the first layer every fixture in
// this file builds) rather than recomputing it via a second throwaway
// build, so assertions that need to reconstruct expected file content
// don't silently drift from what the repo actually contains.
func scaffoldSHAFromRepo(t *testing.T, repo *fixturegit.Repo) string {
	t.Helper()
	if len(repo.Heads) < 1 {
		t.Fatal("scaffoldSHAFromRepo: repo has no layers")
	}
	return repo.Heads[0]
}

// TestRunCloseFeature_WithStoryRef_PublishesRollup is the disclosed
// decision's OTHER side: when the feature DOES carry a story: tracker ref
// (unlike the happy path above, which deliberately mirrors
// spec/true-closure's story-less shape), `verdi close` publishes exactly
// as the story path does — the empty-Story skip in
// cmd/verdi/closefeature.go's runCloseFeature is conditional, not a
// blanket "features never publish" rule.
func TestRunCloseFeature_WithStoryRef_PublishesRollup(t *testing.T) {
	opts := defaultCloseFeatureFixtureOpts()
	opts.FeatureStory = "jira:FIXTURE-EPIC-1"
	repo := buildCloseFeatureRepo(t, opts)
	seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
	writeCloseFeatureGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
	ctx := context.Background()

	fp := fake.New()
	deps := closeFeatureDeps(fp)

	var stdout, stderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runClose(feature, with story ref) = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}

	published, ok := fp.PublishedField("jira:FIXTURE-EPIC-1")
	if !ok {
		t.Fatal("fake provider has no published rollup for jira:FIXTURE-EPIC-1")
	}
	if !published.Eligible {
		t.Fatalf("published rollup = %+v, want eligible=true", published)
	}
	if len(published.Criteria) != 2 {
		t.Fatalf("published rollup criteria = %+v, want 2 entries", published.Criteria)
	}

	archiveDir := filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-feature-fixture")
	rollRaw, err := os.ReadFile(filepath.Join(archiveDir, "rollup.json"))
	if err != nil {
		t.Fatalf("reading archived rollup.json: %v", err)
	}
	roll, err := artifact.DecodeRollup(rollRaw)
	if err != nil {
		t.Fatalf("DecodeRollup: %v", err)
	}
	if roll.Story != "jira:FIXTURE-EPIC-1" {
		t.Fatalf("rollup.Story = %q, want jira:FIXTURE-EPIC-1", roll.Story)
	}
}

// TestRunCloseFeature_BoardJSONGrandfathered_SurvivesIfPresent confirms the
// board.json quartet member's grandfathered handling (03 §Alignment
// report: "new specs archive layout.json ... instead of a frozen
// board.json" — board.json is "the grandfathered form"). This needed NO
// feature-specific code at all: store.ArchiveMove (internal/store,
// consumed unchanged) renames the WHOLE spec directory verbatim, so
// whatever board.json a pre-existing active spec directory happens to
// carry — or its ordinary absence, proven by every other test in this
// file, matching spec/true-closure's real 3-member quartet — simply moves
// (or doesn't exist to move) with everything else. This test plants one
// and confirms it survives byte-identical; it is not asserting new
// behavior close.go had to implement.
func TestRunCloseFeature_BoardJSONGrandfathered_SurvivesIfPresent(t *testing.T) {
	opts := defaultCloseFeatureFixtureOpts()
	repo := buildCloseFeatureRepo(t, opts)
	seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
	writeCloseFeatureGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
	ctx := context.Background()

	const boardJSON = `{"schema":"verdi.board/v1","pins":[],"stickies":[],"yarn":[]}`
	boardPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "close-feature-fixture", "board.json")
	if err := os.WriteFile(boardPath, []byte(boardJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := closeFeatureDeps(fake.New())
	var stdout, stderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runClose(feature, with a pre-existing board.json) = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}

	archivedBoard, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-feature-fixture", "board.json"))
	if err != nil {
		t.Fatalf("board.json did not survive the archive move: %v", err)
	}
	if string(archivedBoard) != boardJSON {
		t.Fatalf("archived board.json = %q, want byte-identical to the pre-close content %q", archivedBoard, boardJSON)
	}
}

// TestRunCloseFeature_RefusesUndispositionedFindings is X-17's own literal
// red-first proof at the feature scope: X-17's exact chronicled failure
// mode is "close's internal freeze-align had CREATED the feature's first
// deviation report (4 findings) and frozen it undispositioned, because no
// report existed pre-close" — this fixture (fully eligible, otherwise a
// clean happy path) has NO deviation-report.md at all before close runs,
// which is precisely what let #162 merge undispositioned before this fix.
// After the fix, the SAME fixture refuses (exit 1), names the ritual, and
// archives nothing.
func TestRunCloseFeature_RefusesUndispositionedFindings(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, root, head string) // "" setup = no report at all (X-17)
		wantSubstr []string
	}{
		{
			name:       "no report at all (X-17's own literal scenario: close's own align call created and froze the first report)",
			setup:      func(t *testing.T, root, head string) {},
			wantSubstr: []string{"no deviation-report.md found at", "the closure ritual is align"},
		},
		{
			name: "a living report covering head with an undispositioned finding",
			setup: func(t *testing.T, root, head string) {
				writeCloseFeatureGateReport(t, root, head, undispositionedFindingYAML)
			},
			wantSubstr: []string{"undispositioned finding(s) [f-1]", "the closure ritual is align"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := defaultCloseFeatureFixtureOpts()
			repo := buildCloseFeatureRepo(t, opts)
			seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
			tc.setup(t, repo.Dir, repo.Head)

			fp := fake.New()
			deps := closeFeatureDeps(fp)
			var stdout, stderr bytes.Buffer
			got := runClose(context.Background(), repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, deps, &stdout, &stderr)
			if got != 1 {
				t.Fatalf("runClose(feature, undispositioned) = %d, want 1 (verdict, not archived); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
			}
			for _, want := range tc.wantSubstr {
				if !contains(stdout.String(), want) {
					t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), want)
				}
			}

			// The X-17 proof itself: nothing archived.
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "close-feature-fixture", "spec.md")); err != nil {
				t.Fatalf("spec.md should remain in specs/active/ after a refused close: %v", err)
			}
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-feature-fixture")); !os.IsNotExist(err) {
				t.Fatal("specs/archive/close-feature-fixture should NOT exist — the undispositioned/missing report must never be silently frozen and archived (X-17)")
			}
			if branch := gitCurrentBranch(t, repo.Dir); branch != "main" {
				t.Fatalf("current branch = %q after a refused close, want main (no closure branch cut)", branch)
			}
		})
	}
}

// TestRunCloseFeature_Negative is the table-driven proof that each of the
// feature-closure gate's distinguishing conditions genuinely blocks
// closure on its own, with no side effects: nothing archived, nothing
// published, the active spec directory untouched, no closure branch cut.
func TestRunCloseFeature_Negative(t *testing.T) {
	cases := []struct {
		name           string
		mutate         func(opts *closeFeatureFixtureOpts)
		wantFailSubstr string // a distinguishing substring from the FAILing condition's own printed reason
	}{
		{
			name: "unreconciled stub: an implementing story eligible but not yet closed",
			mutate: func(opts *closeFeatureFixtureOpts) {
				// fixture-story-two stays accepted-pending-build (self-
				// eligible via its own passing evidence) rather than closed:
				// the feature AC it implements can still fold evidenced
				// (every OTHER condition stays green), but
				// evidence.ReconcileStubs only ever counts CLOSED stories
				// when computing a stub's realized-by coverage (03 §Stub
				// reconciliation) — so fixture-story-two's OWN stub is left
				// unreconciled purely because it hasn't actually closed yet,
				// exactly the gap 03's three-way AND ("... + all
				// implementing stories closed") exists to catch.
				opts.Story2Status = "accepted-pending-build"
			},
			wantFailSubstr: "unreconciled stub(s): [fixture-story-two]",
		},
		{
			name: "un-evidenced AC: a violated implementing story",
			mutate: func(opts *closeFeatureFixtureOpts) {
				// fixture-story-two stays CLOSED (conditions 2 and 3 both
				// stay green — it covers its own stub and is closed) but its
				// own evidence fails, so ITS OWN fold is Violated=true,
				// which propagates straight to the feature AC it implements
				// (03 §The feature fold: "violated propagates up from any
				// implementing story's violated status").
				opts.Story2OwnVerdict = "fail"
			},
			wantFailSubstr: "ac-2=violated",
		},
		{
			name: "outcome floor unmet: no direct record or attestation bound to the feature AC",
			mutate: func(opts *closeFeatureFixtureOpts) {
				// Both stories stay closed and clean (conditions 2 and 3
				// stay green) but no evidence — record or attestation — is
				// ever bound directly to the feature's own ac-2, so the
				// mandatory outcome floor (03 §The feature fold: "evidenced
				// requires at least one such record or attestation bound
				// directly to the feature AC ... story-level bookkeeping
				// alone never satisfies a feature AC") is never cleared.
				opts.FeatureAC2FloorSatisfied = false
			},
			wantFailSubstr: "ac-2=pending",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := defaultCloseFeatureFixtureOpts()
			tc.mutate(&opts)
			repo := buildCloseFeatureRepo(t, opts)
			seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
			ctx := context.Background()

			fp := fake.New()
			deps := closeFeatureDeps(fp)

			var stdout, stderr bytes.Buffer
			got := runClose(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, deps, &stdout, &stderr)
			if got != 1 {
				t.Fatalf("runClose(feature) = %d, want 1 (gate refused); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.wantFailSubstr) {
				t.Fatalf("stdout = %q, want it to contain %q (the specific condition this case exercises)", stdout.String(), tc.wantFailSubstr)
			}
			if !strings.Contains(stdout.String(), "FAIL (feature closure gate not satisfied") {
				t.Fatalf("stdout = %q, want the gate-refused summary line", stdout.String())
			}

			// No side effects: nothing archived, nothing published, no
			// closure branch cut, active directory untouched.
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "close-feature-fixture", "spec.md")); err != nil {
				t.Fatalf("spec.md should remain in specs/active/ after a refused close: %v", err)
			}
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-feature-fixture")); !os.IsNotExist(err) {
				t.Fatal("specs/archive/close-feature-fixture should not exist after a refused close")
			}
			if branch := gitCurrentBranch(t, repo.Dir); branch != "main" {
				t.Fatalf("current branch = %q after a refused close, want main (no closure branch cut)", branch)
			}
			if _, ok := fp.PublishedField(""); ok {
				t.Fatal("fake provider has a published rollup despite the closure gate failing")
			}
		})
	}
}

// TestRunCloseFeature_ClosedStoryDiscovered_NoOperationalError is a small,
// direct proof at the close level (complementing
// TestDiscoverImplementingStories_ClosedStoryInArchive,
// featurematrix_test.go) that runCloseFeature's own discovery call — which
// reuses discoverImplementingStories exactly as `verdi matrix` does —
// resolves already-closed implementing stories without an operational
// error. Every other test in this file already depends on this (both
// fixture stories are placed directly in their final, already-closed
// shape rather than being closed via a nested close run), so this test
// exists only to name the property explicitly rather than leave it
// implicit.
func TestRunCloseFeature_ClosedStoryDiscovered_NoOperationalError(t *testing.T) {
	opts := defaultCloseFeatureFixtureOpts()
	repo := buildCloseFeatureRepo(t, opts)
	seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)

	var stdout, stderr bytes.Buffer
	got := runClose(context.Background(), repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, closeFeatureDeps(fake.New()), &stdout, &stderr)
	if got == 2 {
		t.Fatalf("runClose(feature with closed implementing stories) = 2 (operational error), want 0; stderr=%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), fmt.Sprintf("[PASS] closure(feature): %s", "3. every implementing story closed")) {
		t.Fatalf("stdout should show condition 3 (every implementing story closed) passing: %s", stdout.String())
	}
}

// TestRunCloseFeature_UnreadableAttestation_OperationalFailure pins ADJ-67 /
// D6-38 on the FEATURE closure path. closeFeatureSpecMD declares
// evidence: [behavioral, attestation] on every AC, so evidence.FoldFeature
// (via cmd/verdi's foldFeature) calls LoadAttestationState at the feature
// slug's own attestation path. An attestation file that exists but cannot be
// read (mode 000) must fail closed — never the old stat-only swallow that
// counted an unreadable file as a satisfied outcome attestation.
//
// Two seams are pinned:
//   - fold seam (TEETH): foldFeature() called directly propagates the
//     os.ReadFile error out of evidence.FoldFeature (featurefold.go's
//     LoadAttestationState call) through its "folding feature evidence:"
//     wrap. This assertion FAILS if the swallow is restored.
//   - cmd taxonomy: `verdi close <feature>` exits 2 (operational). NOTE: in
//     the full ritual, runCloseFeature's index.Build walk of .verdi/ is the
//     FIRST reader to fail closed on the same unreadable file (walkDocuments
//     opens every artifact), so the cmd-level exit 2 is real either way — but
//     the fold-seam teeth live in the direct foldFeature call above, not in
//     the exit code (which index.Build would produce even under a restored
//     swallow). Recorded rather than left implicit.
func TestRunCloseFeature_UnreadableAttestation_OperationalFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("DISCLOSURE: running as root — os.Chmod(0o000) does not restrict root's own reads, so this permission-based negative test cannot exercise the unreadable-attestation path under this user")
	}
	opts := defaultCloseFeatureFixtureOpts()
	repo := buildCloseFeatureRepo(t, opts)
	seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
	ctx := context.Background()

	// Plant an outcome attestation for the feature's own ac-1 (FoldFeature
	// folds ac-1 first), then make it unreadable. Its content never matters —
	// the fold fails on the os.ReadFile EACCES before ever parsing it.
	attDir := filepath.Join(repo.Dir, ".verdi", "attestations", "close-feature-fixture")
	if err := os.MkdirAll(attDir, 0o755); err != nil {
		t.Fatal(err)
	}
	attPath := filepath.Join(attDir, "ac-1.md")
	content := "---\nid: attestation/close-feature-fixture--ac-1\nkind: attestation\ntitle: \"ac-1 (deliberately unreadable)\"\nowners: [platform-team]\n---\nPlanted then chmod 000 — the fold must fail closed on it, never read it.\n"
	if err := os.WriteFile(attPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(attPath, 0o000); err != nil {
		t.Fatalf("os.Chmod(%s, 0o000): %v", attPath, err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(attPath, 0o644) // restore so t.TempDir()'s own cleanup can remove it
	})

	// Fold seam (TEETH): foldFeature bypasses index.Build and reaches
	// evidence.FoldFeature → LoadAttestationState directly. This is the
	// assertion that catches a restored stat-only swallow.
	spec, _ := readSpec(t, repo.Dir, "close-feature-fixture")
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		t.Fatalf("ParseRef(%q): %v", spec.ID, err)
	}
	_, ferr := foldFeature(ctx, repo.Dir, spec, specRef, repo.Head, map[string][]evidence.ImplementingStory{}, nil)
	if ferr == nil {
		t.Fatal("foldFeature(unreadable attestation) err = nil — the fold must fail closed on an unreadable outcome attestation, never swallow it to satisfied (ADJ-67/D6-38)")
	}
	if !errors.Is(ferr, os.ErrPermission) {
		t.Fatalf("foldFeature err = %v, want it to wrap os.ErrPermission (the propagated os.ReadFile EACCES)", ferr)
	}
	if !strings.Contains(ferr.Error(), "loading attestation state") {
		t.Fatalf("foldFeature err = %q, want it to name the propagated attestation read (folding feature evidence → loading attestation state)", ferr.Error())
	}

	// Cmd taxonomy: `verdi close <feature>` exits 2 (operational) on the same
	// input (see the doc comment re index.Build shadowing the fold).
	deps := closeFeatureDeps(fake.New())
	var stdout, stderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runClose(feature, unreadable attestation) = %d, want 2 (operational failure, not a swallowed attested=true); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "permission denied") {
		t.Fatalf("stderr = %q, want it to name the propagated permission error", stderr.String())
	}
	// No side effects on an operational failure: nothing archived.
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-feature-fixture")); !os.IsNotExist(err) {
		t.Fatal("specs/archive/close-feature-fixture should not exist after an operational-failure close")
	}
}
