package lint

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/specstate"
)

// vl015StatuslessPredTmpl is VL-015's merge-signaled (statusless)
// predecessor fixture: the same loan-workflow shape vl015_test.go's own
// frozen-path fixtures use, but carrying NO frozen: stamp and NO status:
// field at all — the ratified merge-signaled lifecycle's "no third state"
// statusless shape (docs/superpowers/specs/2026-08-01-merge-signals-spec-
// acceptance-design.md). co1Text is the one value every case below varies,
// to prove exactly WHICH commit's bytes VL-015's merge-signaled path reads
// its manifest from.
const vl015StatuslessPredTmpl = `---
id: spec/loan-workflow-sl
kind: spec
class: feature
title: "Loan workflow (VL-015 statusless fixture)"
owners: [platform-team]
problem: { text: "loan officers cannot see workflow status changes in real time", anchor: "#problem" }
outcome: { text: "loan officers see workflow status changes within one minute", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "workflow status changes are visible within one minute", evidence: [runtime, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "workflow history is queryable by loan id", evidence: [static, attestation], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: %q, anchor: "#co-1" }
---
# Loan workflow (VL-015 statusless fixture)

## Problem

Loan officers only see workflow status changes on their next manual refresh.

## Outcome

Loan officers see workflow status changes within one minute of the change.

## AC-1

Workflow status changes are visible within one minute.

## AC-2

Workflow history is queryable by loan id.

## CO-1

Constraint text lives in frontmatter; this body section is fixed regardless.
`

// vl015StatuslessSuccessorTmpl is vl015LoanWorkflowV2Tmpl's statusless
// counterpart: the superseding revision names spec/loan-workflow-sl as its
// sole whole-spec predecessor.
const vl015StatuslessSuccessorTmpl = `---
id: spec/loan-workflow-sl-v2
kind: spec
class: feature
title: "Loan workflow v2 (VL-015 statusless fixture, supersedes v1)"
owners: [platform-team]
problem: { text: "loan officers cannot see workflow status changes in real time", anchor: "#problem" }
outcome: { text: "loan officers see workflow status changes within thirty seconds", anchor: "#outcome" }
links:
  - { type: supersedes, ref: spec/loan-workflow-sl }
acceptance_criteria:
  - { id: ac-1, text: "workflow status changes are visible within thirty seconds", evidence: [runtime, attestation], anchor: "#ac-1" }
  - { id: ac-3, text: "workflow status changes emit an audit event", evidence: [static, attestation], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: %q, anchor: "#co-1" }
supersession:
%s
---
# Loan workflow v2 (VL-015 statusless fixture, supersedes v1)

## Problem

Loan officers only see workflow status changes on their next manual refresh.

## Outcome

Loan officers see workflow status changes within thirty seconds of the change.

## AC-1

Workflow status changes are visible within thirty seconds.

## AC-3

Workflow status changes emit an audit event.

## CO-1

Constraint text lives in frontmatter; this body section is fixed regardless.
`

const vl015StatuslessCO1Text = vl015PredecessorCO1Text

const vl015StatuslessPredRelPath = ".verdi/specs/active/loan-workflow-sl/spec.md"

// vl015StatuslessSupersessionBody is the standard happy-path classification
// every case below starts from unless it is deliberately testing a
// different bucket shape.
const vl015StatuslessSupersessionBody = `  carried: [co-1]
  amended: [ { id: ac-1, note: "tightened the visibility threshold" } ]
  amended_advisory: []
  removed: [ { id: ac-2, note: "moved to a separate reporting feature" } ]
  added: [ac-3]`

// buildVL015StatuslessRepo builds a two-layer REAL git history for VL-015's
// merge-signaled path: layer 1 commits the statusless predecessor with
// landingCO1Text — this layer's own SHA is the "landing commit" every case
// below points its fake resolver's Baseline.LandingCommit at, so
// gitx.Show(landing, ...) reads REAL history, only the projector's
// State/Baseline DECISION is faked (engine.go's SpecStateResolver doc
// comment: some proof shapes only a fake can reach; this repo shape is not
// one of them, but reusing the same seam keeps every case here isolated
// from specstate's own resolution logic, which is proven separately in
// internal/specstate). Layer 2 then commits the predecessor a SECOND time
// at headCO1Text — modeling working-tree/HEAD drift after landing
// (headCO1Text may equal landingCO1Text when a case needs no drift) —
// alongside the statusless successor built from successorCO1Text and
// supersessionBody. Returns the repo (checked out at layer 2 / HEAD) and
// the landing commit SHA.
func buildVL015StatuslessRepo(t *testing.T, landingCO1Text, headCO1Text, successorCO1Text, supersessionBody string) (*fixturegit.Repo, string) {
	t.Helper()

	layer1 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml":        setupManifestYAML,
			".gitattributes":           setupGitAttributes,
			vl015StatuslessPredRelPath: fmt.Sprintf(vl015StatuslessPredTmpl, landingCO1Text),
		},
		Message: "vl015 statusless layer 1: loan-workflow-sl landed",
	}
	repo1 := fixturegit.Build(t, []fixturegit.Layer{layer1})
	landingSHA := repo1.Head

	layer2 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml":                               setupManifestYAML,
			".gitattributes":                                  setupGitAttributes,
			vl015StatuslessPredRelPath:                        fmt.Sprintf(vl015StatuslessPredTmpl, headCO1Text),
			".verdi/specs/active/loan-workflow-sl-v2/spec.md": fmt.Sprintf(vl015StatuslessSuccessorTmpl, successorCO1Text, supersessionBody),
		},
		Message: "vl015 statusless layer 2: loan-workflow-sl HEAD + loan-workflow-sl-v2",
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{layer1, layer2})
	provisionMutableZone(t, repo.Dir)
	return repo, landingSHA
}

// vl015FakeRunInput builds a *RunInput over repo's checked-out working tree
// (BuildSnapshot — real Documents, real predecessor/successor decode) wired
// with fakeResolver in place of a real specstate.Projector — the same
// direct-construction seam vl004_test.go's fakeStateResolver/RunInput
// wiring uses.
func vl015FakeRunInput(t *testing.T, repo *fixturegit.Repo, fakeResolver SpecStateResolver) *RunInput {
	t.Helper()
	snap, err := BuildSnapshot(repo.Dir, Options{})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	return &RunInput{Ctx: context.Background(), Root: repo.Dir, Snapshot: snap, LintCtx: Context{}, Opts: Options{}, Projector: fakeResolver}
}

// vl015StatuslessAdHocRunInput builds a *RunInput over a plain (non-git)
// temp directory carrying the standard statusless predecessor/successor
// pair, wired with fakeResolver — used for cases where the fake resolver's
// own Result already fails VL-015's acceptance condition, so checkOne's
// read step (gitx.Show against real git history) is never reached at all;
// no real git repository is needed to prove those cases.
func vl015StatuslessAdHocRunInput(t *testing.T, fakeResolver SpecStateResolver) *RunInput {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, filepath.FromSlash(vl015StatuslessPredRelPath)), fmt.Sprintf(vl015StatuslessPredTmpl, vl015StatuslessCO1Text))
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "loan-workflow-sl-v2", "spec.md"), fmt.Sprintf(vl015StatuslessSuccessorTmpl, vl015StatuslessCO1Text, vl015StatuslessSupersessionBody))

	snap, err := BuildSnapshot(dir, Options{})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	return &RunInput{Ctx: context.Background(), Root: dir, Snapshot: snap, LintCtx: Context{}, Opts: Options{}, Projector: fakeResolver}
}

// vl015ProvenBaseline returns a specstate.Result shape that satisfies
// VL-015's acceptance condition for state, wired to landingSHA.
func vl015ProvenBaseline(state specstate.State, landingSHA string) specstate.Result {
	return specstate.Result{
		State:    state,
		Relation: specstate.RelationExact,
		Baseline: &specstate.Baseline{
			Path:          vl015StatuslessPredRelPath,
			Blob:          "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			LandingCommit: landingSHA,
		},
	}
}

// TestVL015_StatuslessPredecessor_ProvenBaseline_Clean is the PASS case: a
// statusless predecessor with a proven Git-derived accepted baseline and a
// complete, byte-identical successor manifest produces zero VL-015
// findings.
func TestVL015_StatuslessPredecessor_ProvenBaseline_Clean(t *testing.T) {
	for _, state := range []specstate.State{specstate.AcceptedPendingBuild, specstate.Superseded, specstate.Closed} {
		t.Run(string(state), func(t *testing.T) {
			repo, landingSHA := buildVL015StatuslessRepo(t, vl015StatuslessCO1Text, vl015StatuslessCO1Text, vl015StatuslessCO1Text, vl015StatuslessSupersessionBody)
			in := vl015FakeRunInput(t, repo, fakeStateResolver{result: vl015ProvenBaseline(state, landingSHA)})

			findings := (vl015{}).Check(in)
			if len(findings) != 0 {
				t.Fatalf("VL-015 fired on a proven statusless baseline (state=%s) with a complete, byte-identical manifest:\n%s", state, findingsString(findings))
			}
		})
	}
}

// TestVL015_StatuslessPredecessor_ReadsLandingCommit_NotWorkingTree is the
// HISTORY-NOT-WORKING-TREE case: the predecessor's working-tree bytes
// drifted AFTER landing, but the successor's manifest matches the LANDING
// COMMIT's original objects. A clean result here witnesses that the
// manifest is read at the Git-derived landing commit, never from later
// working-tree bytes.
//
// Disclosure: this drifted-bytes-with-a-proven-baseline shape is reachable
// ONLY through the fake resolver seam and exists purely to witness the read
// point — it is not a supported real-world green. Through the REAL
// projector the drifted bytes would not match the default branch and would
// project Proposed/RelationDiverged, which VL-015 fails closed on (see
// TestVL015_StatuslessPredecessor_FailClosed's diverged case).
func TestVL015_StatuslessPredecessor_ReadsLandingCommit_NotWorkingTree(t *testing.T) {
	const driftedAfterLanding = "must not add new SYNCHRONOUS cross-service calls (later working-tree drift)"

	repo, landingSHA := buildVL015StatuslessRepo(t, vl015StatuslessCO1Text, driftedAfterLanding, vl015StatuslessCO1Text, vl015StatuslessSupersessionBody)
	in := vl015FakeRunInput(t, repo, fakeStateResolver{result: vl015ProvenBaseline(specstate.AcceptedPendingBuild, landingSHA)})

	findings := (vl015{}).Check(in)
	if len(findings) != 0 {
		t.Fatalf("VL-015 fired even though the successor's manifest matches the Git-derived LANDING COMMIT (working-tree bytes drifted after landing, which must never be read):\n%s", findingsString(findings))
	}
}

// TestVL015_StatuslessPredecessor_CarriedByteDrift_Reports is the
// statusless-path MISMATCH case: the landing commit and HEAD agree, but the
// successor's own carried co-1 text has drifted from the predecessor's
// landed text — a verdict failure, exactly like the existing frozen-path
// carried-byte-drift case.
func TestVL015_StatuslessPredecessor_CarriedByteDrift_Reports(t *testing.T) {
	const successorDrift = "must not add new SYNCHRONOUS cross-service calls (successor drift)"

	repo, landingSHA := buildVL015StatuslessRepo(t, vl015StatuslessCO1Text, vl015StatuslessCO1Text, successorDrift, vl015StatuslessSupersessionBody)
	in := vl015FakeRunInput(t, repo, fakeStateResolver{result: vl015ProvenBaseline(specstate.AcceptedPendingBuild, landingSHA)})

	findings := (vl015{}).Check(in)
	onlyRule(t, findings, "VL-015")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "byte-identical") {
		t.Fatalf("finding message = %q, want the carried byte-identity violation wording", findings[0].Message)
	}
}

// TestVL015_StatuslessPredecessor_FailClosed is the FAIL CLOSED / FAIL
// CLOSED HONESTLY table: every shape the brief's acceptance condition rules
// out — an unproven Git-derived state, a proven state with an incomplete
// baseline, a proven state whose baseline names a DIFFERENT spec path than
// the predecessor whose manifest is being read, a proposed (new or diverged)
// predecessor, and a resolver operational error — must produce exactly one
// VL-015 finding naming the observed state/relation and carrying specstate's
// own Disclosures (joined "; ") when present. A Result whose Relation is the
// zero value must still render a readable relation word, never a ragged
// "(relation )".
func TestVL015_StatuslessPredecessor_FailClosed(t *testing.T) {
	cases := []struct {
		name         string
		fakeResolver SpecStateResolver
		want         []string
	}{
		{
			name: "unproven: no first-parent landing commit could be proven",
			fakeResolver: fakeStateResolver{result: specstate.Result{
				State:    specstate.Unproven,
				Relation: specstate.RelationUnproven,
				Disclosures: []string{
					"specstate: " + vl015StatuslessPredRelPath + " matches the default branch's bytes (blob deadbeef) but no first-parent landing commit could be proven on main",
				},
			}},
			want: []string{"unproven", "no first-parent landing commit could be proven"},
		},
		{
			name: "unproven with multiple disclosures joined",
			fakeResolver: fakeStateResolver{result: specstate.Result{
				State:       specstate.Unproven,
				Relation:    specstate.RelationUnproven,
				Disclosures: []string{"first witness", "second witness"},
			}},
			want: []string{"first witness; second witness"},
		},
		{
			name: "proven state, nil baseline (only a fake can reach this shape)",
			fakeResolver: fakeStateResolver{result: specstate.Result{
				State:    specstate.AcceptedPendingBuild,
				Relation: specstate.RelationExact,
				Baseline: nil,
			}},
			want: []string{"accepted-pending-build", "incomplete"},
		},
		{
			name: "unproven with a zero relation still names a relation word",
			fakeResolver: fakeStateResolver{result: specstate.Result{
				State:       specstate.Unproven,
				Disclosures: []string{"no witness at all"},
			}},
			want: []string{"unproven", "relation unknown", "no witness at all"},
		},
		{
			name: "proven state, complete baseline, but it names a DIFFERENT spec path",
			fakeResolver: fakeStateResolver{result: specstate.Result{
				State:    specstate.AcceptedPendingBuild,
				Relation: specstate.RelationExact,
				Baseline: &specstate.Baseline{
					Path:          ".verdi/specs/active/some-other-spec/spec.md",
					Blob:          "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					LandingCommit: "0123456789abcdef0123456789abcdef01234567",
				},
			}},
			want: []string{
				".verdi/specs/active/some-other-spec/spec.md",
				vl015StatuslessPredRelPath,
			},
		},
		{
			name: "proven state, incomplete baseline (missing landing commit)",
			fakeResolver: fakeStateResolver{result: specstate.Result{
				State:    specstate.Superseded,
				Relation: specstate.RelationExact,
				Baseline: &specstate.Baseline{Path: vl015StatuslessPredRelPath, Blob: "deadbeef"},
			}},
			want: []string{"superseded", "incomplete"},
		},
		{
			name: "proven state, incomplete baseline (missing blob)",
			fakeResolver: fakeStateResolver{result: specstate.Result{
				State:    specstate.Closed,
				Relation: specstate.RelationExact,
				Baseline: &specstate.Baseline{Path: vl015StatuslessPredRelPath, LandingCommit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
			}},
			want: []string{"closed", "incomplete"},
		},
		{
			name: "diverged predecessor: a live proposal, not a landed baseline",
			fakeResolver: fakeStateResolver{result: specstate.Result{
				State:    specstate.Proposed,
				Relation: specstate.RelationDiverged,
				Baseline: &specstate.Baseline{Path: vl015StatuslessPredRelPath, Blob: "deadbeef"},
			}},
			want: []string{"proposed", "diverged"},
		},
		{
			name: "new predecessor: never yet reachable from the default branch",
			fakeResolver: fakeStateResolver{result: specstate.Result{
				State:    specstate.Proposed,
				Relation: specstate.RelationNew,
			}},
			want: []string{"proposed", "new"},
		},
		{
			name:         "resolver operational error",
			fakeResolver: fakeStateResolver{err: fmt.Errorf("boom: git exec failed")},
			want:         []string{"boom: git exec failed"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := vl015StatuslessAdHocRunInput(t, tc.fakeResolver)
			findings := (vl015{}).Check(in)
			onlyRule(t, findings, "VL-015")
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
			}
			for _, want := range tc.want {
				if !strings.Contains(findings[0].Message, want) {
					t.Fatalf("finding message = %q, want it to contain %q", findings[0].Message, want)
				}
			}
		})
	}
}

// buildVL015StatuslessRepoWithLandingContent builds the same two-layer
// history buildVL015StatuslessRepo builds, except layer 1 (the landing
// commit) holds landingPredContent VERBATIM at the predecessor's path —
// content that need not be a well-formed spec at all. Layer 2 (the working
// tree BuildSnapshot reads) holds the normal, valid predecessor plus the
// standard successor, so the ONLY unreadable bytes in the repository are the
// ones at the landing commit: any read failure a case sees therefore
// witnesses that VL-015 read the landing commit and nothing else.
func buildVL015StatuslessRepoWithLandingContent(t *testing.T, landingPredContent string) (*fixturegit.Repo, string) {
	t.Helper()

	layer1 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml":        setupManifestYAML,
			".gitattributes":           setupGitAttributes,
			vl015StatuslessPredRelPath: landingPredContent,
		},
		Message: "vl015 statusless layer 1: loan-workflow-sl landed (unreadable bytes)",
	}
	repo1 := fixturegit.Build(t, []fixturegit.Layer{layer1})
	landingSHA := repo1.Head

	layer2 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml":                               setupManifestYAML,
			".gitattributes":                                  setupGitAttributes,
			vl015StatuslessPredRelPath:                        fmt.Sprintf(vl015StatuslessPredTmpl, vl015StatuslessCO1Text),
			".verdi/specs/active/loan-workflow-sl-v2/spec.md": fmt.Sprintf(vl015StatuslessSuccessorTmpl, vl015StatuslessCO1Text, vl015StatuslessSupersessionBody),
		},
		Message: "vl015 statusless layer 2: readable loan-workflow-sl + loan-workflow-sl-v2",
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{layer1, layer2})
	provisionMutableZone(t, repo.Dir)
	return repo, landingSHA
}

// TestVL015_StatuslessPredecessor_UnreadableBaseline_FailsClosed covers
// readPredecessorManifestAt's three error branches — the git read itself
// failing, the frontmatter not splitting, and the frontmatter not
// strict-decoding — on the merge-signaled path, where the selected commit is
// result.Baseline.LandingCommit.
//
// The first case is the direct witness of the READ POINT: the repository's
// working tree and every real commit hold a perfectly readable predecessor,
// and the only thing wrong is the landing commit the fake resolver names, so
// the resulting finding can only come from a read at
// Baseline.LandingCommit. The other two cases put the unreadable bytes AT
// the landing commit while the working tree stays valid, which witnesses the
// same read point from the other direction.
func TestVL015_StatuslessPredecessor_UnreadableBaseline_FailsClosed(t *testing.T) {
	// A well-formed but never-created 40-hex object id: Frozen/baseline
	// shape checks pass, `git show` fails.
	const missingSHA = "0123456789abcdef0123456789abcdef01234567"

	const noFrontmatter = `# Loan workflow (VL-015 statusless fixture)

This landing-commit revision carries no frontmatter fence at all.
`

	const unknownKey = `---
id: spec/loan-workflow-sl
kind: spec
class: feature
title: "Loan workflow (VL-015 statusless fixture)"
owners: [platform-team]
totally_unknown_key: "strict decoding must reject this"
---
# Loan workflow (VL-015 statusless fixture)
`

	cases := []struct {
		name string
		// setup returns the repo to lint and the landing commit the fake
		// resolver's proven baseline will name.
		setup func(t *testing.T) (*fixturegit.Repo, string)
		want  []string
	}{
		{
			name: "landing commit does not exist: the git read fails",
			setup: func(t *testing.T) (*fixturegit.Repo, string) {
				repo, _ := buildVL015StatuslessRepo(t, vl015StatuslessCO1Text, vl015StatuslessCO1Text, vl015StatuslessCO1Text, vl015StatuslessSupersessionBody)
				return repo, missingSHA
			},
			want: []string{
				"reading predecessor spec/loan-workflow-sl",
				"at its Git-derived accepted baseline's landing commit " + missingSHA,
			},
		},
		{
			name: "landing commit content has no frontmatter to split",
			setup: func(t *testing.T) (*fixturegit.Repo, string) {
				return buildVL015StatuslessRepoWithLandingContent(t, noFrontmatter)
			},
			want: []string{
				"predecessor spec/loan-workflow-sl frontmatter at its Git-derived accepted baseline's landing commit does not split",
			},
		},
		{
			name: "landing commit content does not strict-decode",
			setup: func(t *testing.T) (*fixturegit.Repo, string) {
				return buildVL015StatuslessRepoWithLandingContent(t, unknownKey)
			},
			want: []string{
				"predecessor spec/loan-workflow-sl frontmatter at its Git-derived accepted baseline's landing commit does not decode",
				"totally_unknown_key",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, landingSHA := tc.setup(t)
			in := vl015FakeRunInput(t, repo, fakeStateResolver{result: vl015ProvenBaseline(specstate.AcceptedPendingBuild, landingSHA)})

			findings := (vl015{}).Check(in)
			onlyRule(t, findings, "VL-015")
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
			}
			for _, want := range tc.want {
				if !strings.Contains(findings[0].Message, want) {
					t.Fatalf("finding message = %q, want it to contain %q", findings[0].Message, want)
				}
			}
		})
	}
}

// TestVL015_StatuslessPredecessor_RealGitIntegration_Clean is the HERMETIC
// REAL-GIT INTEGRATION positive case: a statusless predecessor genuinely
// lands on the resolved default branch ("main"), the successor (with a
// complete, byte-identical manifest) sits in the working tree, and the full
// lint engine — real specstate.NewProjector(), no fakes — reports zero
// VL-015 findings. The successor is left UNCOMMITTED deliberately: were it
// committed and reachable, the predecessor's real Git-derived state would
// become Superseded rather than AcceptedPendingBuild — both are in VL-015's
// accepted set, but AcceptedPendingBuild through the real projector is the
// more direct proof that this path (not just the fake-driven ones above)
// actually reaches gitx.Show at the real landing commit.
func TestVL015_StatuslessPredecessor_RealGitIntegration_Clean(t *testing.T) {
	layer1 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml":        setupManifestYAML,
			".gitattributes":           setupGitAttributes,
			vl015StatuslessPredRelPath: fmt.Sprintf(vl015StatuslessPredTmpl, vl015StatuslessCO1Text),
		},
		Message: "vl015 real-git layer 1: loan-workflow-sl landed on main",
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{layer1})
	provisionMutableZone(t, repo.Dir)

	writeTestFile(t, filepath.Join(repo.Dir, ".verdi", "specs", "active", "loan-workflow-sl-v2", "spec.md"), fmt.Sprintf(vl015StatuslessSuccessorTmpl, vl015StatuslessCO1Text, vl015StatuslessSupersessionBody))

	t.Setenv("CI_DEFAULT_BRANCH", "main")
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-015" {
			t.Fatalf("VL-015 fired on a genuinely landed statusless predecessor with a complete, byte-identical successor manifest, through the real specstate.Projector: %s", f.String())
		}
	}
}

// TestVL015_StatuslessPredecessor_RealGitIntegration_NeverLanded_FailsClosed
// is the HERMETIC REAL-GIT INTEGRATION negative case: a statusless
// predecessor that never landed on the resolved default branch at all
// (working-tree-only, no frozen stamp) must fail VL-015 closed, through the
// real specstate.NewProjector(), naming the proposed/new Git-derived state.
func TestVL015_StatuslessPredecessor_RealGitIntegration_NeverLanded_FailsClosed(t *testing.T) {
	layer1 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml": setupManifestYAML,
			".gitattributes":    setupGitAttributes,
		},
		Message: "vl015 real-git negative layer 1: bare store, no predecessor landed",
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{layer1})
	provisionMutableZone(t, repo.Dir)

	writeTestFile(t, filepath.Join(repo.Dir, filepath.FromSlash(vl015StatuslessPredRelPath)), fmt.Sprintf(vl015StatuslessPredTmpl, vl015StatuslessCO1Text))
	writeTestFile(t, filepath.Join(repo.Dir, ".verdi", "specs", "active", "loan-workflow-sl-v2", "spec.md"), fmt.Sprintf(vl015StatuslessSuccessorTmpl, vl015StatuslessCO1Text, vl015StatuslessSupersessionBody))

	t.Setenv("CI_DEFAULT_BRANCH", "main")
	findings := runLint(t, repo.Dir, Context{}, Options{})

	var got *Finding
	for i := range findings {
		if findings[i].Rule == "VL-015" {
			got = &findings[i]
		}
	}
	if got == nil {
		t.Fatalf("no VL-015 finding for a statusless predecessor that never landed on the default branch:\n%s", findingsString(findings))
	}
	if !strings.Contains(got.Message, "proposed") || !strings.Contains(got.Message, "new") {
		t.Fatalf("finding message = %q, want it to name the proposed/new Git-derived state (never landed)", got.Message)
	}
}
