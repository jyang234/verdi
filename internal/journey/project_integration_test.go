package journey

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/store"
)

// buildFactsRepoNoDefaultBranch mirrors buildFactsRepo but deliberately
// leaves the default branch unresolvable: CI_DEFAULT_BRANCH is cleared,
// and a bare fixturegit repo carries no "origin" remote (so neither
// origin/HEAD nor a remote-tracking main/master fallback exists either) —
// specstate.ResolveDefaultBranch's own three-step chain exhausts with no
// answer, hermetically and deterministically.
func buildFactsRepoNoDefaultBranch(t *testing.T, files map[string]string) *fixturegit.Repo {
	t.Helper()
	base := map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n"}
	for k, v := range files {
		base[k] = v
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: base, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "")
	t.Chdir(repo.Dir)
	return repo
}

func openConfig(t *testing.T, root string) *store.Config {
	t.Helper()
	cfg, err := store.Open(root)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return cfg
}

func installedPolicyFixtureFiles(t *testing.T) map[string]string {
	t.Helper()
	rels := []string{
		".verdi/policy/constitution.md",
		".verdi/policy/exemptions/legacy-service-go.md",
		".verdi/policy/overlays/frontend-go-version.md",
		".verdi/policy/policies/go-toolchain.md",
		".verdi/policy/profiles/solo-default.md",
	}
	files := make(map[string]string, len(rels))
	root := filepath.Join("..", "policyauthority", "testdata", "store")
	for _, rel := range rels {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("reading policy authority fixture %s: %v", rel, err)
		}
		files[rel] = string(data)
	}
	return files
}

func TestProject_Integration_InstalledProfile(t *testing.T) {
	files := installedPolicyFixtureFiles(t)
	files[".verdi/specs/active/payments/spec.md"] = testFeatureSpecMD
	repo := buildFactsRepo(t, files)
	cfg := openConfig(t, repo.Dir)

	rec, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if !rec.Principals.ProfileAdopted {
		t.Fatal("Principals.ProfileAdopted = false, want true for installed authority")
	}
	if rec.Principals.SelectedProfileID != "solo-default" {
		t.Fatalf("Principals.SelectedProfileID = %q, want solo-default", rec.Principals.SelectedProfileID)
	}
	if rec.Principals.SelectedProfileDigest != fixtureSelectedProfileDigest {
		t.Fatalf("Principals.SelectedProfileDigest = %q, want %q", rec.Principals.SelectedProfileDigest, fixtureSelectedProfileDigest)
	}
	const absent = "no governance profile is adopted at the evaluated revision; role and approver requirements beyond the operating-model obligations are unknown"
	for _, disclosure := range rec.Principals.Disclosures {
		if disclosure == absent {
			t.Fatalf("Principals.Disclosures = %v, must not disclose profile absence after adoption", rec.Principals.Disclosures)
		}
	}
	const unproven = "authenticated principal resolution and profile-contributed requirements remain unproven"
	if !containsString(rec.Principals.Disclosures, unproven) {
		t.Fatalf("Principals.Disclosures = %v, want %q", rec.Principals.Disclosures, unproven)
	}
	data, err := Canonical(rec)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	if strings.Contains(string(data), "no governance profile is adopted") {
		t.Fatalf("canonical adopted record emits a false profile-absence disclosure: %s", data)
	}
	if len(rec.Principals.Required) == 0 {
		t.Fatal("Principals.Required is empty, want operating-model-derived close obligation")
	}
	for _, required := range rec.Principals.Required {
		if required.Resolution != "unproven" {
			t.Fatalf("Principals.Required = %+v, adopted profile must not authenticate operating-model roles", rec.Principals.Required)
		}
	}
}

func TestProject_Integration_ProfileNotAdopted(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	rec, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if rec.Principals.ProfileAdopted {
		t.Fatal("Principals.ProfileAdopted = true, want false before adoption")
	}
	if rec.Principals.SelectedProfileID != "" || rec.Principals.SelectedProfileDigest != "" {
		t.Fatalf("selected profile = %q/%q, want empty before adoption", rec.Principals.SelectedProfileID, rec.Principals.SelectedProfileDigest)
	}
	const absent = "no governance profile is adopted at the evaluated revision; role and approver requirements beyond the operating-model obligations are unknown"
	if !containsString(rec.Principals.Disclosures, absent) {
		t.Fatalf("Principals.Disclosures = %v, want %q", rec.Principals.Disclosures, absent)
	}
}

func TestProject_Integration_InvalidPolicyAuthority(t *testing.T) {
	policyFiles := installedPolicyFixtureFiles(t)
	tests := []struct {
		name    string
		files   map[string]string
		wantErr string
	}{
		{
			name: "incomplete adoption",
			files: map[string]string{
				".verdi/policy/policies/go-toolchain.md": policyFiles[".verdi/policy/policies/go-toolchain.md"],
			},
			wantErr: "incomplete adoption",
		},
		{
			name: "malformed authority",
			files: map[string]string{
				".verdi/policy/constitution.md": "---\nschema: [not valid here]\n",
			},
			wantErr: "decoding constitution.md",
		},
		{
			name: "unavailable authority path",
			files: map[string]string{
				".verdi/policy": "not a directory",
			},
			wantErr: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.files[".verdi/specs/active/payments/spec.md"] = testFeatureSpecMD
			repo := buildFactsRepo(t, tt.files)
			cfg := openConfig(t, repo.Dir)

			rec, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Project error = %v, want containing %q", err, tt.wantErr)
			}
			if rec.Schema != "" {
				t.Fatalf("Project record schema = %q, want zero record on invalid policy authority", rec.Schema)
			}
		})
	}
}

// TestProject_Integration_UnknownDefaultBranch proves that an unresolvable
// default branch produces BOTH the default-branch-unresolved blocker and
// the lifecycle-state-unproven blocker (specstate itself cannot derive a
// state without a default branch), Relationship == "unknown", and that
// nothing anywhere invents a branch name or a guessed state.
func TestProject_Integration_UnknownDefaultBranch(t *testing.T) {
	repo := buildFactsRepoNoDefaultBranch(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	rec, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}

	if rec.Repository.DefaultBranch.Known {
		t.Fatalf("DefaultBranch = %+v, want unknown", rec.Repository.DefaultBranch)
	}
	if rec.Repository.DefaultBranch.Name != "" || rec.Repository.DefaultBranch.Ref != "" || rec.Repository.DefaultBranch.Head != "" {
		t.Fatalf("DefaultBranch = %+v, want every field empty (nothing invented)", rec.Repository.DefaultBranch)
	}
	if rec.Repository.Relationship != "unknown" {
		t.Fatalf("Relationship = %q, want unknown", rec.Repository.Relationship)
	}
	if rec.Lifecycle.State != "unproven" {
		t.Fatalf("Lifecycle.State = %q, want unproven", rec.Lifecycle.State)
	}
	if findBlocker(rec.Blockers.Current, "default-branch-unresolved/unknown") == nil {
		t.Errorf("blockers missing default-branch-unresolved/unknown; got %v", blockerIDs(rec.Blockers.Current))
	}
	if findBlocker(rec.Blockers.Current, "lifecycle-state-unproven/unknown") == nil {
		t.Errorf("blockers missing lifecycle-state-unproven/unknown; got %v", blockerIDs(rec.Blockers.Current))
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("Record.Validate: %v", err)
	}
}

// TestProject_Integration_DivergedDirtyStalePosture proves a working-tree
// edit that diverges from the committed default-branch bytes yields
// Dirty == true, Source == "working-tree", Lifecycle.Relation ==
// "diverged", and Posture == "advisory" (never "authoritative" — DC-2's
// wrong-checkout ambiguity the posture rule exists to surface) — the
// "stale posture" fixture the work order names.
func TestProject_Integration_DivergedDirtyStalePosture(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	edited := testFeatureSpecMD + "\n<!-- local, uncommitted edit -->\n"
	if err := os.WriteFile(repo.Dir+"/.verdi/specs/active/payments/spec.md", []byte(edited), 0o644); err != nil {
		t.Fatalf("editing spec.md: %v", err)
	}

	rec, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}

	if !rec.Repository.Dirty.Known || !rec.Repository.Dirty.Value {
		t.Fatalf("Dirty = %+v, want known/true", rec.Repository.Dirty)
	}
	if rec.Repository.Source != "working-tree" {
		t.Fatalf("Source = %q, want working-tree", rec.Repository.Source)
	}
	if rec.Lifecycle.Relation != "diverged" {
		t.Fatalf("Relation = %q, want diverged", rec.Lifecycle.Relation)
	}
	if rec.Lifecycle.Posture != "advisory" {
		t.Fatalf("Posture = %q, want advisory (never authoritative on a diverged working tree)", rec.Lifecycle.Posture)
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("Record.Validate: %v", err)
	}
}

// TestProject_Integration_RemoteOnly proves a spec present at the default
// branch but ABSENT from the working tree resolves Source == "remote-ref"
// and evaluates the default-branch bytes (never a NotFound refusal, and
// never a fabricated empty candidate).
func TestProject_Integration_RemoteOnly(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	if err := os.Remove(repo.Dir + "/.verdi/specs/active/payments/spec.md"); err != nil {
		t.Fatalf("removing working-tree copy: %v", err)
	}

	rec, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if rec.Repository.Source != "remote-ref" {
		t.Fatalf("Source = %q, want remote-ref", rec.Repository.Source)
	}
	if rec.Lifecycle.State != "accepted-pending-build" {
		t.Fatalf("Lifecycle.State = %q, want accepted-pending-build (the default-branch bytes are landed)", rec.Lifecycle.State)
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("Record.Validate: %v", err)
	}
}

func TestProject_Integration_RemoteOnlyStoryProjectsQuality(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/quality-story/spec.md": projectQualityStorySpecMD})
	cfg := openConfig(t, repo.Dir)

	if err := os.Remove(repo.Dir + "/.verdi/specs/active/quality-story/spec.md"); err != nil {
		t.Fatalf("removing working-tree copy: %v", err)
	}

	rec, err := NewProjector().Project(context.Background(), cfg, "spec/quality-story")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if rec.Repository.Source != "remote-ref" {
		t.Fatalf("Source = %q, want remote-ref", rec.Repository.Source)
	}
	if findBlocker(rec.Blockers.Current, "obligation-quality/ac-1/behavioral") == nil {
		t.Fatalf("blockers = %v, want remote story's declared obligation pair projected", blockerIDs(rec.Blockers.Current))
	}
}

const projectQualityStorySpecMD = `---
id: spec/quality-story
kind: spec
class: story
title: "Quality story"
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
story: jira:QUALITY-1
links:
  - { type: implements, ref: "spec/feature#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "behavior is verified", evidence: [behavioral] }
---
# body
`

const projectElaboratedObligationMD = `---
id: obligation/quality-story--ac-1--behavioral
kind: obligation
title: "Quality"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "the behavior holds"
  falsifier: "the behavior does not hold"
  scope: "quality story"
  producer: { kind: checker, ref: "verify:behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "rerun after spec or code change"
links:
  - { type: verifies, ref: "spec/quality-story" }
frozen: { at: 2026-08-10, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Quality

Authored obligation.
`

const projectUnresolvedObligationMD = `---
id: obligation/quality-story--ac-1--behavioral
kind: obligation
title: "Quality"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/quality-story" }
frozen: { at: 2026-08-10, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Quality

<!-- verdi:obligation-unauthored -->
`

func TestProject_Integration_ObligationQualityMatching(t *testing.T) {
	tests := []struct {
		name       string
		producer   string
		wantReason string
	}{
		{name: "elaborated with no evidence blocks", wantReason: "producer-missing"},
		{name: "exact producer source and freshness match", producer: "verify:behavioral"},
		{name: "producer mismatch blocks", producer: "other:behavioral", wantReason: "producer-mismatch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := buildFactsRepo(t, map[string]string{
				".verdi/specs/active/quality-story/spec.md":            projectQualityStorySpecMD,
				".verdi/obligations/quality-story/ac-1--behavioral.md": projectElaboratedObligationMD,
			})
			if tt.producer != "" {
				writeJourneyEvidenceRecord(t, repo.Dir, repo.Head, tt.producer)
			}

			rec, err := NewProjector().Project(context.Background(), openConfig(t, repo.Dir), "spec/quality-story")
			if err != nil {
				t.Fatalf("Project: %v", err)
			}
			blocker := findBlocker(rec.Blockers.Current, "obligation-quality/ac-1/behavioral")
			if tt.wantReason == "" {
				if blocker != nil {
					t.Fatalf("quality blocker = %+v, want none", blocker)
				}
				return
			}
			if blocker == nil {
				t.Fatalf("blockers = %v, want obligation-quality/ac-1/behavioral", blockerIDs(rec.Blockers.Current))
			}
			if blocker.Reason != ReasonObligationDesignUnresolved || blocker.Class != ClassMechanical || blocker.Transition != "build:start" {
				t.Fatalf("quality blocker = %+v, want exact reason/class/action", blocker)
			}
			if len(blocker.Witnesses) != 1 || blocker.Witnesses[0] != ".verdi/obligations/quality-story/ac-1--behavioral.md: elaborated/"+tt.wantReason {
				t.Fatalf("quality blocker witnesses = %v, want stable mismatch witness", blocker.Witnesses)
			}
			if blocker.Owner.Declared != "platform-team" || blocker.Owner.Attribution.PrincipalID != "" {
				t.Fatalf("quality blocker owner = %+v, want declared platform-team and disclosed unauthenticated attribution", blocker.Owner)
			}
			if len(rec.Actions.Safe) != 0 {
				t.Fatalf("Actions.Safe = %+v, quality projection must not invent a build action", rec.Actions.Safe)
			}
		})
	}
}

func TestProject_Integration_ObligationQualityFailuresRemainBlockers(t *testing.T) {
	tests := []struct {
		name       string
		obligation string
		state      string
	}{
		{name: "elaborated", obligation: projectElaboratedObligationMD, state: "elaborated"},
		{name: "missing", state: "missing"},
		{name: "unresolved", obligation: projectUnresolvedObligationMD, state: "unresolved-design-debt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{".verdi/specs/active/quality-story/spec.md": projectQualityStorySpecMD}
			if tt.obligation != "" {
				files[".verdi/obligations/quality-story/ac-1--behavioral.md"] = tt.obligation
			}
			repo := buildFactsRepo(t, files)
			writeJourneyEvidenceRecordWithVerdict(t, repo.Dir, repo.Head, "failing:behavioral", "fail", "ci failure witness")

			rec, err := NewProjector().Project(context.Background(), openConfig(t, repo.Dir), "spec/quality-story")
			if err != nil {
				t.Fatalf("Project: %v", err)
			}
			blocker := findBlocker(rec.Blockers.Current, "obligation-quality/ac-1/behavioral")
			if blocker == nil {
				t.Fatalf("blockers = %v, want obligation-quality/ac-1/behavioral", blockerIDs(rec.Blockers.Current))
			}
			structural := ".verdi/obligations/quality-story/ac-1--behavioral.md: " + tt.state
			if len(blocker.Witnesses) != 2 || blocker.Witnesses[0] != structural || blocker.Witnesses[1] != "ci failure witness" {
				t.Fatalf("quality blocker witnesses = %v, want [%q %q]", blocker.Witnesses, structural, "ci failure witness")
			}
		})
	}
}

func writeJourneyEvidenceRecord(t *testing.T, root, commit, producer string) {
	writeJourneyEvidenceRecordWithVerdict(t, root, commit, producer, "pass", "ci proof")
}

func writeJourneyEvidenceRecordWithVerdict(t *testing.T, root, commit, producer, verdict, witness string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "data", "derived", "spec--quality-story", commit)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"behavioral","verdict":"` + verdict + `","witness":"` + witness + `","producer":"` + producer + `","provenance":{"source":"ci","pipeline":"pipeline-1","job":"verify","commit":"` + commit + `"},"digest":"sha256:` + strings.Repeat("a", 64) + `"}]`
	if err := os.WriteFile(filepath.Join(dir, "verdicts.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestProject_Integration_MissingRef_NotFound proves a ref resolving
// nowhere at all (no working tree, no default branch) surfaces the typed
// *NotFoundError a CLI lane maps to exit 2.
func TestProject_Integration_MissingRef_NotFound(t *testing.T) {
	repo := buildFactsRepo(t, nil)
	cfg := openConfig(t, repo.Dir)

	_, err := NewProjector().Project(context.Background(), cfg, "spec/nowhere")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("Project error = %v, want *NotFoundError", err)
	}
}

// TestProject_Integration_ComponentClass_Error proves a component-class
// target is refused (no story, no acceptance criteria — nothing for a
// journey to project).
func TestProject_Integration_ComponentClass_Error(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/shared-lib/spec.md": testComponentSpecMD})
	cfg := openConfig(t, repo.Dir)

	_, err := NewProjector().Project(context.Background(), cfg, "spec/shared-lib")
	if err == nil {
		t.Fatal("Project: want an error for a component-class target")
	}
}

// TestProject_Integration_Deterministic proves two Project calls over the
// same semantic inputs produce byte-identical Canonical() output —
// fixturegit's fixed identity/date makes the underlying commit shas
// themselves stable, and Project performs no wall-clock- or randomness-
// dependent derivation of its own.
func TestProject_Integration_Deterministic(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	rec1, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project (1): %v", err)
	}
	rec2, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project (2): %v", err)
	}

	out1, err := Canonical(rec1)
	if err != nil {
		t.Fatalf("Canonical (1): %v", err)
	}
	out2, err := Canonical(rec2)
	if err != nil {
		t.Fatalf("Canonical (2): %v", err)
	}
	if string(out1) != string(out2) {
		t.Fatalf("Canonical output differs across two Project calls:\n1: %s\n2: %s", out1, out2)
	}
}

// TestProject_Integration_TwoDistinctRootsByteIdentical is F7: the
// no-default-branch fixture built twice, in two DIFFERENT temp dirs
// (different absolute paths), must Project to byte-identical Canonical()
// output — the same semantic content evaluated from two different
// checkouts must never diverge merely because their absolute filesystem
// paths differ (F1(b)/CO-2/CO-4's machine-independence guarantee). Before
// F1/F2 this test is RED: specstate's own "no default branch could be
// resolved for <root>" disclosure embeds each repo's own absolute temp
// dir path verbatim, so the two records' Lifecycle.Disclosures (and the
// lifecycle-state-unproven blocker's witness drawn from the same
// specstate.Result) differ byte-for-byte between the two roots.
func TestProject_Integration_TwoDistinctRootsByteIdentical(t *testing.T) {
	files := map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD}

	repo1 := buildFactsRepoNoDefaultBranch(t, files)
	cfg1 := openConfig(t, repo1.Dir)
	rec1, err := NewProjector().Project(context.Background(), cfg1, "spec/payments")
	if err != nil {
		t.Fatalf("Project (repo1): %v", err)
	}

	repo2 := buildFactsRepoNoDefaultBranch(t, files)
	cfg2 := openConfig(t, repo2.Dir)
	rec2, err := NewProjector().Project(context.Background(), cfg2, "spec/payments")
	if err != nil {
		t.Fatalf("Project (repo2): %v", err)
	}

	if repo1.Dir == repo2.Dir {
		t.Fatalf("test setup: want two distinct roots, got the same dir twice: %s", repo1.Dir)
	}

	out1, err := Canonical(rec1)
	if err != nil {
		t.Fatalf("Canonical (repo1): %v", err)
	}
	out2, err := Canonical(rec2)
	if err != nil {
		t.Fatalf("Canonical (repo2): %v", err)
	}
	if string(out1) != string(out2) {
		t.Fatalf("Canonical output differs across two distinct-root repos with identical semantic content (a root path leaked into the record):\nrepo1 (%s): %s\nrepo2 (%s): %s", repo1.Dir, out1, repo2.Dir, out2)
	}
}

// TestProject_Integration_ReadOnly proves Project changes neither HEAD nor
// the working tree's status (cmd/verdi/specstate_test.go:56-70's idiom).
func TestProject_Integration_ReadOnly(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	headBefore := currentHead(t, repo.Dir)
	statusBefore := porcelainStatus(t, repo.Dir)

	if _, err := NewProjector().Project(context.Background(), cfg, "spec/payments"); err != nil {
		t.Fatalf("Project: %v", err)
	}

	if headBefore != currentHead(t, repo.Dir) {
		t.Fatalf("HEAD changed: before=%s after=%s", headBefore, currentHead(t, repo.Dir))
	}
	if statusBefore != porcelainStatus(t, repo.Dir) {
		t.Fatalf("git status --porcelain changed: before=%q after=%q", statusBefore, porcelainStatus(t, repo.Dir))
	}
}

// TestProject_Integration_ValidateCanonicalRoundTrip proves a full,
// real-fixturegit-derived record round-trips through Validate, Canonical,
// and Decode: the decoded record equals the original in every field
// Decode itself checks (schema validity and a matching recomputed digest).
func TestProject_Integration_ValidateCanonicalRoundTrip(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	rec, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("Record.Validate: %v", err)
	}

	out, err := Canonical(rec)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}

	decoded, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Digest != rec.Digest && decoded.Digest == "" {
		t.Fatalf("Decode: digest not carried through round trip")
	}
	out2, err := Canonical(decoded)
	if err != nil {
		t.Fatalf("Canonical (decoded): %v", err)
	}
	if string(out) != string(out2) {
		t.Fatalf("round trip is not byte-identical:\noriginal: %s\ndecoded:  %s", out, out2)
	}
}
