package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/wallbadge"
	"github.com/jyang234/verdi/internal/workbench"
)

// TestBadgeSpecDecodes proves every wall-badge fixture instance the
// harness provisions is a VALID spec document (spec/badge-computes ac-5's
// fixtures must reach a live, renderable board — a decode failure would
// 500 the board before any badge could compute): the draft instances and
// the frozen sealed instance all strict-decode and validate, and the
// badge-triggering state (the dangling stub ref, the dangling decision
// link, the dangling top-level link) is present in each.
func TestBadgeSpecDecodes(t *testing.T) {
	cases := []struct {
		name       string
		spec       string
		status     string
		frozenLine string
	}{
		{"authoring draft", badgeWallSpecName, "draft", ""},
		{"review draft", badgeReviewSpecName, "draft", ""},
		{"sealed", badgeSealedSpecName, "accepted-pending-build", "frozen: { at: 2024-01-01, commit: 0123456789abcdef0123456789abcdef01234567 }\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := badgeSpec(tc.spec, tc.status, tc.frozenLine)
			fmBytes, _, err := artifact.SplitFrontmatter([]byte(doc))
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			fm, err := artifact.DecodeSpec(fmBytes)
			if err != nil {
				t.Fatalf("DecodeSpec: %v", err)
			}
			if fm.ID != "spec/"+tc.spec || string(fm.Status) != tc.status {
				t.Errorf("decoded id/status = %q/%q, want spec/%s / %s", fm.ID, fm.Status, tc.spec, tc.status)
			}
			if len(fm.Stubs) != 1 || fm.Stubs[0].Slug != "badge-orphan" || fm.Stubs[0].AcceptanceCriteria[0] != "ac-99" {
				t.Errorf("stubs = %+v, want the dangling badge-orphan → ac-99 fixture", fm.Stubs)
			}
			if len(fm.Links) != 1 || fm.Links[0].Ref != "spec/no-such-parent" {
				t.Errorf("links = %+v, want the dangling spec/no-such-parent depends-on", fm.Links)
			}
			if len(fm.Decisions) != 1 || len(fm.Decisions[0].Links) != 1 || fm.Decisions[0].Links[0].Ref != "adr/0099-no-such-adr" {
				t.Errorf("decisions = %+v, want dc-1 carrying the dangling exempts link", fm.Decisions)
			}
		})
	}
}

func TestSlotWallObligationQualityDecodes(t *testing.T) {
	tests := []struct {
		kind         artifact.EvidenceKind
		producerKind artifact.ObligationProducerKind
		sourceKind   artifact.ObligationSourceKind
	}{
		{artifact.EvidenceStatic, artifact.ObligationProducerChecker, artifact.ObligationSourceCIJob},
		{artifact.EvidenceBehavioral, artifact.ObligationProducerTest, artifact.ObligationSourceCIJob},
		{artifact.EvidenceAttestation, artifact.ObligationProducerAuthenticatedHuman, artifact.ObligationSourceGovernedAttestation},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			fmBytes, _, err := artifact.SplitFrontmatter([]byte(slotWallObligation(tt.kind)))
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			obligation, err := artifact.DecodeObligation(fmBytes)
			if err != nil {
				t.Fatalf("DecodeObligation: %v", err)
			}
			if obligation.ForKind != tt.kind || obligation.Quality == nil {
				t.Fatalf("decoded obligation = %+v, want %s with quality", obligation, tt.kind)
			}
			if obligation.Quality.State != artifact.ObligationQualityElaborated ||
				obligation.Quality.Producer.Kind != tt.producerKind ||
				obligation.Quality.AuthoritativeSource.Kind != tt.sourceKind {
				t.Errorf("quality = %+v, want elaborated %s/%s declaration", obligation.Quality, tt.producerKind, tt.sourceKind)
			}
		})
	}
}

// TestSlotWallStaticRecordReadsFreshnessStale pins the harness's static slot
// to the reason e2e/tests/42-matrix-preview.spec.ts asserts
// (`static:pending(obligation-quality:elaborated/freshness-stale)`): the
// provisioned CI record matches the obligation's producer and its declared
// authoritative source (SI-229: `job_name`, never the `job` ordering id), so
// only the serving checkout moving past the record's commit keeps it pending.
// It drives the production matcher on the harness's own obligation and
// verdicts text, so a fixture that stops carrying `job_name` reads
// source-ref-missing here, before Playwright ever runs.
func TestSlotWallStaticRecordReadsFreshnessStale(t *testing.T) {
	const (
		recordCommit = "1111111111111111111111111111111111111111"
		laterCommit  = "2222222222222222222222222222222222222222"
	)
	fmBytes, _, err := artifact.SplitFrontmatter([]byte(slotWallObligation(artifact.EvidenceStatic)))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	obligation, err := artifact.DecodeObligation(fmBytes)
	if err != nil {
		t.Fatalf("DecodeObligation: %v", err)
	}
	tests := []struct {
		name             string
		evaluationCommit string
		edit             func(*artifact.Evidence)
		wantState        evidence.ObligationMatchState
		wantReason       evidence.ObligationMatchReason
	}{
		{"checkout moved past the record", laterCommit, nil,
			evidence.ObligationUnproven, evidence.ObligationReasonFreshnessStale},
		{"checkout at the record's commit", recordCommit, nil,
			evidence.ObligationMatched, ""},
		{"job ordering id alone never names the source", laterCommit,
			func(r *artifact.Evidence) { r.Provenance.JobName = "" },
			evidence.ObligationUnproven, evidence.ObligationReasonSourceRefMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The fold's own reader path: a JSON array whose every element
			// strict-decodes through artifact.DecodeEvidence.
			var raw []json.RawMessage
			if err := json.Unmarshal([]byte(slotWallVerdicts(recordCommit)), &raw); err != nil {
				t.Fatalf("decoding slotWallVerdicts: %v", err)
			}
			if len(raw) != 1 {
				t.Fatalf("slotWallVerdicts carries %d records, want exactly the static one", len(raw))
			}
			record, err := artifact.DecodeEvidence(raw[0])
			if err != nil {
				t.Fatalf("DecodeEvidence: %v", err)
			}
			if tt.edit != nil {
				tt.edit(record)
			}
			got, err := evidence.MatchObligation(context.Background(),
				evidence.ObligationAssessment{StructuralState: evidence.ObligationElaborated, Quality: obligation.Quality},
				evidence.ObligationAssessmentInput{Kind: artifact.EvidenceStatic, Record: record, EvaluationCommit: tt.evaluationCommit})
			if err != nil {
				t.Fatalf("MatchObligation: %v", err)
			}
			if got.MatchState != tt.wantState || got.Reason != tt.wantReason {
				t.Errorf("static slot = %s/%s, want %s/%s", got.MatchState, got.Reason, tt.wantState, tt.wantReason)
			}
		})
	}
}

// TestACCountSpecDecodesAndStraddlesTheThreshold proves the size-smell
// fixture pair (spec/case-file-flags ac-2/ac-3) is valid and HONEST:
// both instances strict-decode with exactly their declared AC counts,
// and the counts genuinely straddle dc-1's threshold as computed from
// the SAME declared constants the compute reads — so a future amendment
// of the layout geometry or the reference constant fails here, loudly,
// instead of silently hollowing out the Playwright proof.
func TestACCountSpecDecodesAndStraddlesTheThreshold(t *testing.T) {
	cases := []struct {
		spec  string
		count int
		over  bool
	}{
		{sizeSmellWallSpecName, sizeSmellACCount, true},
		{sizeFitWallSpecName, sizeFitACCount, false},
	}
	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			doc := acCountSpec(tc.spec, tc.count)
			fmBytes, _, err := artifact.SplitFrontmatter([]byte(doc))
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			fm, err := artifact.DecodeSpec(fmBytes)
			if err != nil {
				t.Fatalf("DecodeSpec: %v", err)
			}
			if fm.ID != "spec/"+tc.spec || len(fm.AcceptanceCriteria) != tc.count {
				t.Errorf("decoded id/AC count = %q/%d, want spec/%s with %d ACs", fm.ID, len(fm.AcceptanceCriteria), tc.spec, tc.count)
			}
			estimate := boardlayout.ZoneOriginY + tc.count*boardlayout.RowPitch
			if got := estimate > wallbadge.ReferenceViewportHeight; got != tc.over {
				t.Errorf("dc-1 estimate for %d ACs = %d vs reference %d: over=%v, want %v — the fixture no longer straddles the threshold", tc.count, estimate, wallbadge.ReferenceViewportHeight, got, tc.over)
			}
		})
	}
}

// TestSweepFixturesDecode proves the judged-sweep fixtures the harness
// provisions (spec/derivation-drawer ac-3) are VALID artifacts: both spec
// revisions strict-decode as feature drafts declaring dc-1/dc-2, and the
// report — fresh-shaped and partial-shaped alike — strict-decodes through
// the same artifact.DecodeDecisionConflict the wall itself uses, carrying
// one dispositioned and one undispositioned judged finding plus the
// sweep_provenance block.
func TestSweepFixturesDecode(t *testing.T) {
	for _, outcome := range []string{sweepOutcomeV1, sweepOutcomeStaleV2} {
		fmBytes, _, err := artifact.SplitFrontmatter([]byte(sweepSpec(sweepFreshSpecName, outcome)))
		if err != nil {
			t.Fatalf("SplitFrontmatter(%q): %v", outcome, err)
		}
		fm, err := artifact.DecodeSpec(fmBytes)
		if err != nil {
			t.Fatalf("DecodeSpec(%q): %v", outcome, err)
		}
		if len(fm.Decisions) != 2 || fm.Decisions[0].ID != "dc-1" || fm.Decisions[1].ID != "dc-2" {
			t.Errorf("decisions = %+v, want declared dc-1 and dc-2 (the comparison operand)", fm.Decisions)
		}
	}

	const sha = "0123456789abcdef0123456789abcdef01234567"
	for name, scanned := range map[string]string{
		"full":    "spec/x#dc-1, spec/x#dc-2",
		"partial": "spec/x#dc-1",
	} {
		t.Run(name, func(t *testing.T) {
			fmBytes, _, err := artifact.SplitFrontmatter([]byte(sweepReport(sha, scanned)))
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			report, err := artifact.DecodeDecisionConflict(fmBytes)
			if err != nil {
				t.Fatalf("DecodeDecisionConflict: %v", err)
			}
			if report.Covers != sha {
				t.Errorf("covers = %q, want the pinned sha", report.Covers)
			}
			if len(report.Findings) != 2 || !report.Findings[0].Dispositioned() || report.Findings[1].Dispositioned() {
				t.Errorf("findings = %+v, want one dispositioned + one undispositioned", report.Findings)
			}
			if report.SweepProvenance == nil || len(report.SweepProvenance.DecisionsScanned) == 0 {
				t.Error("sweep_provenance block missing from the fixture report")
			}
		})
	}
}

// TestBadgeSpecSealedRequiresFrozen is the negative path: the sealed
// status without its frozen stamp must FAIL validation (the frozenLine
// parameter is load-bearing, not decoration).
func TestBadgeSpecSealedRequiresFrozen(t *testing.T) {
	doc := badgeSpec(badgeSealedSpecName, "accepted-pending-build", "")
	fmBytes, _, err := artifact.SplitFrontmatter([]byte(doc))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	_, err = artifact.DecodeSpec(fmBytes)
	if err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Fatalf("DecodeSpec of an unfrozen sealed fixture: err = %v, want a frozen-stamp refusal", err)
	}
}

// TestStatuslessSpecDecodes proves the statusless-lifecycle pair
// (merge-signaled acceptance) is valid: each instance strict-decodes with
// NO persisted status at all — the shape whose board mode and displayed
// status must come entirely from git-derived effective state.
func TestStatuslessSpecDecodes(t *testing.T) {
	for _, name := range []string{statuslessDraftSpecName, statuslessSealedSpecName} {
		t.Run(name, func(t *testing.T) {
			doc := statuslessSpec(name)
			fmBytes, _, err := artifact.SplitFrontmatter([]byte(doc))
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			fm, err := artifact.DecodeSpec(fmBytes)
			if err != nil {
				t.Fatalf("DecodeSpec: %v", err)
			}
			if fm.ID != "spec/"+name {
				t.Errorf("id = %q, want spec/%s", fm.ID, name)
			}
			if string(fm.Status) != "" {
				t.Errorf("status = %q, want the field ABSENT (the statusless scaffold shape)", fm.Status)
			}
			if fm.Class != artifact.ClassFeature {
				t.Errorf("class = %q, want feature", fm.Class)
			}
		})
	}
}

// testIDText returns the text content of the leaf element whose opening
// tag carries data-testid="id" in body, and whether that element exists.
func testIDText(body, id string) (string, bool) {
	at := strings.Index(body, `data-testid="`+id+`"`)
	if at < 0 {
		return "", false
	}
	open := strings.Index(body[at:], ">")
	if open < 0 {
		return "", false
	}
	rest := body[at+open+1:]
	end := strings.Index(rest, "<")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

func TestTestIDText(t *testing.T) {
	const body = `<div data-testid="card-ac-1" class="x"><span data-testid="coverage-ac-1" data-coverage="1">covered by 1 stub</span></div>`
	for _, tc := range []struct {
		id, want string
		ok       bool
	}{
		{"coverage-ac-1", "covered by 1 stub", true},
		{"card-ac-1", "", true},
		{"coverage-ac-2", "", false},
	} {
		got, ok := testIDText(body, tc.id)
		if got != tc.want || ok != tc.ok {
			t.Errorf("testIDText(%q) = %q, %v; want %q, %v", tc.id, got, ok, tc.want, tc.ok)
		}
	}
	if _, ok := testIDText(`<span data-testid="x"`, "x"); ok {
		t.Error("an unterminated opening tag reported an element")
	}
}

// TestCanvasWallFixture_CarriesEveryCardAndReceipt is the wall-canvas
// fixture's whole claim (spec/wall-canvas-v2 ac-1's obligation: "a
// fixture wall carrying every card kind and every receipt"; SI-350 (11)),
// proved on the shared store the harness itself provisions — the exact
// sequence `verdi serve` is pointed at — rather than a look-alike: each
// instance renders object, stub, reference and sticky cards with the
// obligation rows, evidence slots, attestation chips and coverage chips
// in their existing texts, and the production readiness loader, run on
// the serving checkout, names one of its cards by a "no stub" concern
// and others by unresolved concerns (the readiness mark's input, SI-338
// (2), SI-345 (1) — the store's own state, no mark code involved).
func TestCanvasWallFixture_CarriesEveryCardAndReceipt(t *testing.T) {
	// The harness serves with the CI environment cleared (main.go's
	// neutralizeCIEnv); so does this proof, or a CI runner's own
	// default-branch and PR-boundary variables would speak for the store.
	neutralizeCIEnvForTest(t)
	ctx := t.Context()
	shared, err := provisionSharedStore(ctx, absModuleRoot(t), t.TempDir(), nil)
	if err != nil {
		t.Fatalf("provisionSharedStore: %v", err)
	}
	root := shared.storeRoot
	branch, err := gitOutput(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	head, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	h := workbench.NewHandlerWith(root, workbench.Deps{})

	walls := canvasWalls()
	if len(walls) != 2 || walls[0].name == walls[1].name || walls[0].stickyID == walls[1].stickyID {
		t.Fatalf("canvasWalls = %+v, want two distinct instances (one per spec file, BL-98)", walls)
	}
	for _, w := range walls {
		t.Run(w.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/board/spec/"+w.name, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET /board/spec/%s = %d\n%s", w.name, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()

			// Every card kind.
			for _, id := range []string{
				"card-ac-1", "card-ac-2", "card-co-1", "card-dc-1", "card-oq-1",
				"stub-card-" + canvasWallStubSlug,
				"ref-card-" + strings.ReplaceAll(canvasWallADRRef, "/", "-"),
				"sticky-" + w.stickyID,
			} {
				if _, ok := testIDText(body, id); !ok {
					t.Errorf("no element data-testid=%q on the wall", id)
				}
			}
			if !strings.Contains(body, `<span class="stub-tab">`+canvasWallStubSlug+`</span>`) {
				t.Errorf("the stub card does not carry its slug %q", canvasWallStubSlug)
			}
			if !strings.Contains(body, `<p class="sticky-body">`+canvasWallStickyBody+`</p>`) {
				t.Errorf("the sticky does not carry its body %q", canvasWallStickyBody)
			}

			// Every receipt, in its existing text.
			for id, want := range map[string]string{
				"coverage-ac-1":                    "covered by 1 stub",
				"coverage-ac-2":                    "no stub",
				"obligation-none-ac-1-static":      "no obligation",
				"obligation-none-ac-2-attestation": "no obligation",
				"slot-ac-1-behavioral":             "no record",
				"slot-ac-1-static":                 "1 record",
				"slot-ac-1-attestation":            "attested",
				"slot-ac-2-attestation":            "no attestation",
			} {
				got, ok := testIDText(body, id)
				if !ok || got != want {
					t.Errorf("receipt %s = %q (present %v), want %q", id, got, ok, want)
				}
			}
			if !strings.Contains(body, `>`+canvasWallObligationTitle+`</span>`) {
				t.Errorf("ac-1's authored behavioral obligation row does not show its title %q", canvasWallObligationTitle)
			}

			// The readiness mark's input: the production loader on the
			// serving checkout, the same branch and head the wall serves.
			snap, err := readinessload.Load(ctx, root, "spec/"+w.name, readinessload.Options{BoardHref: workbench.BranchBoardHref})
			if err != nil {
				t.Fatalf("readinessload.Load: %v", err)
			}
			if snap.Branch != branch || snap.Head != head {
				t.Fatalf("snapshot at %s@%s, the wall at %s@%s: a mark would be disclosed, never drawn (SI-338)", snap.Branch, snap.Head, branch, head)
			}
			byID := map[string]readinesspilot.Concern{}
			unresolvedObjects := map[string]bool{}
			for _, c := range snap.Attention {
				byID[c.ID] = c
				if c.Object != "" && !strings.HasPrefix(c.ID, "success/coverage/") {
					unresolvedObjects[c.Object] = true
				}
			}
			if c, ok := byID["success/coverage/ac-2"]; !ok || c.Object != "ac-2" {
				t.Errorf("no Focus next concern success/coverage/ac-2 with Object ac-2 (the \"no stub\" card): %+v", c)
			}
			if _, ok := byID["success/coverage/ac-1"]; ok {
				t.Error("covered ac-1 carries a coverage concern")
			}
			if !unresolvedObjects["oq-1"] {
				t.Errorf("no unresolved Focus next concern names oq-1; objects named: %v", unresolvedObjects)
			}
			if _, ok := byID["review/blocker/stub-unreconciled/"+canvasWallStubSlug]; !ok {
				t.Errorf("no Focus next concern names the stub %s", canvasWallStubSlug)
			}
		})
	}
}

// TestCanvasWallFiles: each instance's committed files strict-decode as
// the artifacts they claim to be, bound to their own spec; a malformed
// name or commit is refused before any file is shaped.
func TestCanvasWallFiles(t *testing.T) {
	const commit = "0123456789abcdef0123456789abcdef01234567"
	for _, w := range canvasWalls() {
		t.Run(w.name, func(t *testing.T) {
			files, err := canvasWallFiles(w.name, commit)
			if err != nil {
				t.Fatalf("canvasWallFiles: %v", err)
			}
			specRel := filepath.Join(".verdi", "specs", "active", w.name, "spec.md")
			obligationRel := filepath.Join(".verdi", "obligations", w.name, "ac-1--behavioral.md")
			attestationRel := filepath.Join(".verdi", "attestations", w.name, "ac-1.md")
			if len(files) != 3 {
				t.Fatalf("files = %v, want the spec, one obligation and one attestation", files)
			}
			fmBytes, _, err := artifact.SplitFrontmatter([]byte(files[specRel]))
			if err != nil {
				t.Fatal(err)
			}
			spec, err := artifact.DecodeSpec(fmBytes)
			if err != nil {
				t.Fatalf("DecodeSpec: %v", err)
			}
			if spec.ID != "spec/"+w.name || spec.Class != artifact.ClassFeature || len(spec.Stubs) != 1 || spec.Stubs[0].Slug != canvasWallStubSlug {
				t.Errorf("spec = %s %s %+v, want the feature wall with stub %s", spec.ID, spec.Class, spec.Stubs, canvasWallStubSlug)
			}
			fmBytes, _, err = artifact.SplitFrontmatter([]byte(files[obligationRel]))
			if err != nil {
				t.Fatal(err)
			}
			obligation, err := artifact.DecodeObligation(fmBytes)
			if err != nil {
				t.Fatalf("DecodeObligation: %v", err)
			}
			if obligation.ForKind != artifact.EvidenceBehavioral || obligation.Title != canvasWallObligationTitle {
				t.Errorf("obligation = %+v", obligation)
			}
			fmBytes, _, err = artifact.SplitFrontmatter([]byte(files[attestationRel]))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := artifact.DecodeAttestation(fmBytes); err != nil {
				t.Fatalf("DecodeAttestation: %v", err)
			}
		})
	}
	for _, tc := range []struct{ name, commit string }{
		{"Not-A-Name", commit},
		{"", commit},
		{canvasWallSpecName, "main"},
		{canvasWallSpecName, strings.Repeat("g", 40)},
	} {
		if _, err := canvasWallFiles(tc.name, tc.commit); err == nil {
			t.Errorf("canvasWallFiles(%q, %q) succeeded, want a refusal", tc.name, tc.commit)
		}
	}
}

// TestWriteCanvasWallScratch: the untracked half — one static record in
// the derived tree keyed by commit and one open board sticky — lands
// where the fold and the board read it, strict-decodable; an unwritable
// store fails loudly.
func TestWriteCanvasWallScratch(t *testing.T) {
	const commit = "0123456789abcdef0123456789abcdef01234567"
	root := t.TempDir()
	w := canvasWalls()[0]
	if err := writeCanvasWallScratch(root, commit, w); err != nil {
		t.Fatalf("writeCanvasWallScratch: %v", err)
	}
	verdicts, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "derived", "spec--"+w.name, commit, "verdicts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(verdicts, &raw); err != nil || len(raw) != 1 {
		t.Fatalf("verdicts.json = %s (%v), want one record", verdicts, err)
	}
	record, err := artifact.DecodeEvidence(raw[0])
	if err != nil {
		t.Fatalf("DecodeEvidence: %v", err)
	}
	if record.Kind != artifact.EvidenceStatic || len(record.EvidenceFor) != 1 || record.EvidenceFor[0] != "ac-1" || record.Provenance.Commit != commit {
		t.Errorf("record = %+v, want one static record for ac-1 at %s", record, commit)
	}
	annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(annotations) != 1 || annotations[0].ID != w.stickyID || annotations[0].Board == nil || annotations[0].Board.Story != w.name ||
		annotations[0].Body != canvasWallStickyBody || annotations[0].Status != artifact.AnnotationOpen {
		t.Errorf("annotations = %+v, want the one open board sticky %s", annotations, w.stickyID)
	}

	blocked := t.TempDir()
	if err := os.WriteFile(filepath.Join(blocked, ".verdi"), []byte("a file, not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCanvasWallScratch(blocked, commit, w); err == nil {
		t.Error("writeCanvasWallScratch succeeded under a file where .verdi/ must be a directory")
	}
	if err := writeCanvasWallScratch(t.TempDir(), commit, canvasWall{name: w.name, stickyID: "not-an-id"}); err == nil {
		t.Error("writeCanvasWallScratch accepted a malformed sticky id")
	}
}
