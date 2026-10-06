package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/refindex"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/wallbadge"
	"github.com/jyang234/verdi/internal/workbench"
	"github.com/jyang234/verdi/internal/wtmanager"
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
// instance's serving path renders object, stub, reference and sticky
// cards with the obligation rows, evidence slots, attestation chips and
// coverage chips in their existing texts, under its domain refusal (the
// posture ac-1 and ac-2 read), and the production readiness loader, run on
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
			h.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, w.servingPath, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d\n%s", w.servingPath, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			for _, problem := range canvasWallMissing(body, w) {
				t.Error(problem)
			}
			// The serving path's posture, which ac-1 and ac-2 read: an
			// authoring wall under its domain refusal, so no object-card
			// pin, no Correct stub and no sticky Graduate.
			for _, problem := range canvasWallDomainProblems(body, w, false) {
				t.Error(problem)
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

// canvasWallMissing lists every card kind and every receipt of the
// wall-canvas fixture that a rendered wall page lacks: object, stub,
// reference and sticky cards, the coverage chips, the obligation rows and
// the evidence-slot and attestation chips, each in its existing text. An
// empty list is the whole fixture.
func canvasWallMissing(body string, w canvasWall) []string {
	var out []string
	for _, id := range []string{
		"card-ac-1", "card-ac-2", "card-co-1", "card-dc-1", "card-oq-1",
		"stub-card-" + canvasWallStubSlug,
		"ref-card-" + strings.ReplaceAll(canvasWallADRRef, "/", "-"),
		"sticky-" + w.stickyID,
	} {
		if _, ok := testIDText(body, id); !ok {
			out = append(out, "no element data-testid="+id+" on the wall")
		}
	}
	if !strings.Contains(body, `<span class="stub-tab">`+canvasWallStubSlug+`</span>`) {
		out = append(out, "the stub card does not carry its slug "+canvasWallStubSlug)
	}
	if !strings.Contains(body, `<p class="sticky-body">`+canvasWallStickyBody+`</p>`) {
		out = append(out, "the sticky does not carry its body "+canvasWallStickyBody)
	}
	receipts := map[string]string{
		"coverage-ac-1":                    "covered by 1 stub",
		"coverage-ac-2":                    "no stub",
		"obligation-none-ac-1-static":      "no obligation",
		"obligation-none-ac-2-attestation": "no obligation",
		"slot-ac-1-behavioral":             "no record",
		"slot-ac-1-static":                 "1 record",
		"slot-ac-1-attestation":            "attested",
		"slot-ac-2-attestation":            "no attestation",
	}
	for _, id := range slices.Sorted(maps.Keys(receipts)) {
		if got, ok := testIDText(body, id); !ok || got != receipts[id] {
			out = append(out, fmt.Sprintf("receipt %s = %q (present %v), want %q", id, got, ok, receipts[id]))
		}
	}
	if !strings.Contains(body, `>`+canvasWallObligationTitle+`</span>`) {
		out = append(out, "ac-1's authored behavioral obligation row does not show its title "+canvasWallObligationTitle)
	}
	return out
}

// TestCanvasWallMissing: a page carrying the whole fixture yields no
// problem, and each missing card, slug or receipt text is named.
func TestCanvasWallMissing(t *testing.T) {
	w := canvasWalls()[0]
	var whole strings.Builder
	for _, id := range []string{"card-ac-1", "card-ac-2", "card-co-1", "card-dc-1", "card-oq-1", "stub-card-" + canvasWallStubSlug, "ref-card-adr-0001-outbox-events", "sticky-" + w.stickyID} {
		whole.WriteString(`<div data-testid="` + id + `"></div>`)
	}
	whole.WriteString(`<span class="stub-tab">` + canvasWallStubSlug + `</span><p class="sticky-body">` + canvasWallStickyBody + `</p>`)
	for id, text := range map[string]string{
		"coverage-ac-1": "covered by 1 stub", "coverage-ac-2": "no stub",
		"obligation-none-ac-1-static": "no obligation", "obligation-none-ac-2-attestation": "no obligation",
		"slot-ac-1-behavioral": "no record", "slot-ac-1-static": "1 record",
		"slot-ac-1-attestation": "attested", "slot-ac-2-attestation": "no attestation",
	} {
		whole.WriteString(`<span data-testid="` + id + `">` + text + `</span>`)
	}
	whole.WriteString(`<span>` + canvasWallObligationTitle + `</span>`)
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"the whole fixture", whole.String(), 0},
		{"no sticky card", strings.Replace(whole.String(), `data-testid="sticky-`, `data-testid="gone-`, 1), 1},
		{"a receipt in another text", strings.Replace(whole.String(), ">1 record<", ">2 records<", 1), 1},
		{"an empty page", "", 19},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := canvasWallMissing(tc.body, w); len(got) != tc.want {
				t.Errorf("canvasWallMissing = %d problems %v, want %d", len(got), got, tc.want)
			}
		})
	}
}

// canvasWallObjectIDs is the fixture wall's declared object cards, each of
// which carries a yarn handle where the wall's domain is live.
func canvasWallObjectIDs() []string {
	return []string{"ac-1", "ac-2", "co-1", "dc-1", "oq-1"}
}

// stickyCard returns the rendered markup of the sticky card whose testid is
// sticky-<id>, up to its closing tag, or "" when the page has none.
func stickyCard(body, id string) string {
	at := strings.Index(body, `data-testid="sticky-`+id+`"`)
	if at < 0 {
		return ""
	}
	end := strings.Index(body[at:], "</div>")
	if end < 0 {
		return ""
	}
	return body[at : at+end]
}

// canvasWallDomainProblems lists every way a rendered authoring wall page
// departs from the domain posture wantLive names. A live domain (the
// writable path) has no refusal banner and offers an object-card pin on
// every declared object, Correct stub on the stub, and Graduate on the
// sticky. A refused domain (the serving path) shows the refusal banner
// and offers none of them.
func canvasWallDomainProblems(body string, w canvasWall, wantLive bool) []string {
	var out []string
	if !strings.Contains(body, `data-board-mode="authoring"`) {
		out = append(out, "the wall is not in authoring mode")
	}
	check := func(what string, liveShape bool) {
		if liveShape != wantLive {
			out = append(out, fmt.Sprintf("%s: live shape %v, want domain live %v", what, liveShape, wantLive))
		}
	}
	check("no asd-domain-refusal banner", !strings.Contains(body, `data-testid="asd-domain-refusal"`))
	for _, id := range canvasWallObjectIDs() {
		check("object-card yarn-handle-"+id, strings.Contains(body, `data-testid="yarn-handle-`+id+`"`))
	}
	check("Correct stub on "+canvasWallStubSlug, strings.Contains(body, `data-testid="correct-stub-`+canvasWallStubSlug+`"`))
	// The sticky's Graduate is the contextual toolbar's (spec/wall-canvas-v2
	// ac-3; SI-350 (5)): the card says it offers one as data-can-graduate.
	check("sticky Graduate on "+w.stickyID, strings.Contains(stickyCard(body, w.stickyID), `data-can-graduate="sticky"`))
	return out
}

// TestCanvasWallDomainProblems pins the posture check both ways on fixed
// markup: a live wall passes as live and fails as refused, a refused wall
// the reverse, and a live wall missing one pin is named.
func TestCanvasWallDomainProblems(t *testing.T) {
	w := canvasWalls()[0]
	var live strings.Builder
	live.WriteString(`<section data-board-mode="authoring">`)
	for _, id := range canvasWallObjectIDs() {
		live.WriteString(`<button data-testid="yarn-handle-` + id + `"></button>`)
	}
	live.WriteString(`<button data-testid="correct-stub-` + canvasWallStubSlug + `">Correct stub</button>`)
	live.WriteString(`<div data-testid="sticky-` + w.stickyID + `" data-can-graduate="sticky"></div>`)
	refused := `<section data-board-mode="authoring"><div data-testid="asd-domain-refusal"></div><div data-testid="sticky-` + w.stickyID + `"></div>`
	for _, tc := range []struct {
		name      string
		body      string
		wantLive  bool
		wantCount int
	}{
		{"live as live", live.String(), true, 0},
		{"refused as refused", refused, false, 0},
		{"live as refused", live.String(), false, 8},
		{"refused as live", refused, true, 8},
		{"live without dc-1's pin", strings.Replace(live.String(), `yarn-handle-dc-1`, `yarn-handle-xx`, 1), true, 1},
		{"a Graduate outside the sticky", strings.Replace(live.String(), `" data-can-graduate="sticky"></div>`, `"></div><div data-can-graduate="sticky"></div>`, 1), true, 1},
		{"a review-mode wall", strings.Replace(refused, "authoring", "review", 1), false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := canvasWallDomainProblems(tc.body, w, tc.wantLive); len(got) != tc.wantCount {
				t.Errorf("canvasWallDomainProblems = %d problems %v, want %d", len(got), got, tc.wantCount)
			}
		})
	}
}

// TestCanvasWallPaths pins each wall-canvas fixture wall's two addresses
// (F2G-1; SI-350 (11)). The serving path is the unprefixed board address,
// served from the serving checkout under the domain refusal; ac-1 and ac-2
// read it. The writable path is the per-branch board address of the
// wall's own namesake design branch, where the domain is live; ac-3 to
// ac-6 write through it. The Playwright files carry their own copies of
// these strings, so each is pinned literally and against the workbench's
// own address constructor. No two walls share a branch or a path: a
// shared branch would let one file's writes reach the other's wall
// (BL-98).
func TestCanvasWallPaths(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{canvasWallServingPath, "/board/spec/decline-canvas-wall"},
		{canvasWallWritablePath, "/b/design%2Fdecline-canvas-wall/board/spec/decline-canvas-wall"},
		{canvasKeysServingPath, "/board/spec/decline-canvas-keys"},
		{canvasKeysWritablePath, "/b/design%2Fdecline-canvas-keys/board/spec/decline-canvas-keys"},
	} {
		if tc.got != tc.want {
			t.Errorf("path constant = %q, want %q (the spec files copy this string)", tc.got, tc.want)
		}
	}
	seen := map[string]string{}
	for _, w := range canvasWalls() {
		if w.branch() != "design/"+w.name {
			t.Errorf("%s's writable branch = %q, want its namesake design/%s (the only branch whose domain is live)", w.name, w.branch(), w.name)
		}
		if w.servingPath != "/board/spec/"+w.name {
			t.Errorf("%s's serving path = %q, want /board/spec/%s", w.name, w.servingPath, w.name)
		}
		if want := workbench.BranchBoardHref(w.branch(), w.name); w.writablePath != want {
			t.Errorf("%s's writable path = %q, want %q", w.name, w.writablePath, want)
		}
		for _, v := range []string{w.branch(), w.servingPath, w.writablePath} {
			if other, dup := seen[v]; dup {
				t.Errorf("%s and %s share %q", other, w.name, v)
			}
			seen[v] = w.name
		}
	}
}

// TestProvisionCanvasWallBranches: on a minimal store sitting on the
// serving branch, each wall gets its own local namesake branch cut at the
// serving branch's tip, its managed worktree pre-cut at wtmanager's
// deterministic path, and that worktree's untracked half (the sticky and
// the static record) seeded. The serving checkout neither moves nor
// dirties. Provisioning twice, or outside a repository, fails loudly.
func TestProvisionCanvasWallBranches(t *testing.T) {
	ctx := t.Context()
	const commit = "0123456789abcdef0123456789abcdef01234567"
	storeRoot := newDraftBoardsTestStore(t)
	// The walls' commit analogue: the serving branch moves past main, so a
	// branch cut anywhere but its tip is told apart.
	wallRel := filepath.Join(".verdi", "specs", "active", canvasWallSpecName, "spec.md")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(storeRoot, wallRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeRoot, wallRel), []byte(canvasWallSpec(canvasWallSpecName)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGit(ctx, storeRoot, nil, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(ctx, storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "design: canvas wall (test analogue)"); err != nil {
		t.Fatal(err)
	}
	tip, err := gitOutput(ctx, storeRoot, "rev-parse", designBranch)
	if err != nil {
		t.Fatal(err)
	}
	if mainTip, _ := gitOutput(ctx, storeRoot, "rev-parse", "main"); mainTip == tip {
		t.Fatal("the serving branch did not move past main: the cut point would be untested")
	}
	if err := provisionCanvasWallBranches(ctx, storeRoot, commit); err != nil {
		t.Fatalf("provisionCanvasWallBranches: %v", err)
	}
	for _, w := range canvasWalls() {
		if got, err := gitOutput(ctx, storeRoot, "rev-parse", "refs/heads/"+w.branch()); err != nil || got != tip {
			t.Errorf("%s = %q (%v), want the serving branch's tip %s", w.branch(), got, err, tip)
		}
		if err := runGit(ctx, storeRoot, nil, "rev-parse", "--quiet", "--verify", "refs/remotes/origin/"+w.branch()); err == nil {
			t.Errorf("%s was pushed; the writable branch is local only", w.branch())
		}
		wt := wtmanager.WorktreePath(storeRoot, w.branch())
		if got, err := gitOutput(ctx, wt, "rev-parse", "--abbrev-ref", "HEAD"); err != nil || got != w.branch() {
			t.Errorf("worktree %s is on %q (%v), want %s", wt, got, err, w.branch())
		}
		annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(wt))
		if err != nil {
			t.Fatal(err)
		}
		if len(annotations) != 1 || annotations[0].ID != w.stickyID || annotations[0].Board == nil || annotations[0].Board.Story != w.name {
			t.Errorf("%s's worktree annotations = %+v, want only its own sticky %s", w.name, annotations, w.stickyID)
		}
		verdicts := filepath.Join(wt, filepath.FromSlash(store.DerivedSpecRelDir(store.RefSlug("spec/"+w.name))), commit, "verdicts.json")
		if _, err := os.Stat(verdicts); err != nil {
			t.Errorf("%s's worktree has no static record: %v", w.name, err)
		}
	}
	if got, _ := gitOutput(ctx, storeRoot, "rev-parse", "--abbrev-ref", "HEAD"); got != designBranch {
		t.Errorf("serving checkout on %q, want %s unmoved", got, designBranch)
	}
	if got, err := gitOutput(ctx, storeRoot, "status", "--porcelain", "--untracked-files=all"); err != nil || got != "" {
		t.Errorf("serving checkout porcelain = %q (%v), want clean: the worktrees live in the ignored data zone", got, err)
	}

	if err := provisionCanvasWallBranches(ctx, storeRoot, commit); err == nil {
		t.Error("provisioning the writable branches twice succeeded, want a refusal")
	}
	if err := provisionCanvasWallBranches(ctx, t.TempDir(), commit); err == nil {
		t.Error("provisionCanvasWallBranches outside a repository succeeded, want an error")
	}
}

// canvasWallServe is one real `verdi serve` over a store provisioned by
// the shared store's own sequence, with the shared serve's environment.
type canvasWallServe struct {
	base  string // http://127.0.0.1:<port>, no trailing slash
	store sharedStore
}

// startCanvasWallServe starts the SHIPPED binary over a fresh shared-shape
// store built from this module (startCanvasWallServeAt), failing the test
// if the start fails.
func startCanvasWallServe(t *testing.T) canvasWallServe {
	t.Helper()
	serve, err := startCanvasWallServeAt(t, absModuleRoot(t))
	if err != nil {
		t.Fatalf("starting verdi serve over a shared-shape store: %v", err)
	}
	return serve
}

// startCanvasWallServeAt starts the SHIPPED binary over a fresh
// shared-shape store through readinessPilotFixture's sequence:
// provisionSharedStore, buildBinary, then `verdi serve` under
// sharedServeEnv, the exact posture the e2e harness gives its own serve.
// moduleRoot locates the corpus and the tree the binary is built from.
//
// The fixture makes its scratch with os.MkdirTemp("", ...) and never
// removes it, and a failed start never returns its path. So before the
// start, this makes a directory of its own, registers its removal, and
// points TMPDIR at it: every scratch the start makes, the store and its
// managed worktrees included, lands there and is removed on every path,
// a failed start included. The serve's stop is registered after that
// removal, so it runs first. The directory sits directly under the
// ambient temp root, not under a t.TempDir: serve puts its MCP socket
// under TMPDIR, and a t.TempDir path would push the socket past the
// 103-byte unix sun_path ceiling (I-29).
func startCanvasWallServeAt(t *testing.T, moduleRoot string) (canvasWallServe, error) {
	t.Helper()
	tmp, err := os.MkdirTemp("", "cw-")
	if err != nil {
		t.Fatalf("making the serve's temp root: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmp); err != nil {
			t.Errorf("removing the serve's temp root %s: %v", tmp, err)
		}
	})
	t.Setenv("TMPDIR", tmp)
	f := newReadinessPilotFixture(moduleRoot, "http://127.0.0.1:9/openmrs")
	t.Cleanup(f.stop)
	url, err := f.ensureStarted(t.Context())
	if err != nil {
		return canvasWallServe{}, err
	}
	f.mu.Lock()
	st := f.serve.store
	f.mu.Unlock()
	return canvasWallServe{base: strings.TrimSuffix(url, "/"), store: st}, nil
}

// TestStartCanvasWallServeAt_FailedStartLeavesNoScratch is the helper's
// negative path: a start over a module root with no corpus fails after
// the fixture has made its scratch, and once the test that started it
// ends, no scratch is left behind.
func TestStartCanvasWallServeAt_FailedStartLeavesNoScratch(t *testing.T) {
	var tmp string
	t.Run("start", func(t *testing.T) {
		if _, err := startCanvasWallServeAt(t, t.TempDir()); err == nil {
			t.Fatal("a start over a module root with no corpus succeeded")
		}
		tmp = os.Getenv("TMPDIR")
		entries, err := os.ReadDir(tmp)
		if err != nil || len(entries) == 0 {
			t.Fatalf("the failed start made no scratch under %s (%v): the witness would be vacuous", tmp, err)
		}
	})
	if tmp == "" {
		t.Fatal("the start subtest recorded no TMPDIR")
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("the failed start left %s behind (stat: %v)", tmp, err)
	}
}

// storeFileDigests maps every regular file under root to its sha256, keyed
// by its slash path relative to root. It skips git's administrative files
// (any .git directory or file: a read-only git status may refresh an
// index's stat cache) and the subtree skip ("" skips nothing).
func storeFileDigests(t *testing.T, root, skip string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" || (skip != "" && path == skip) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("digesting %s: %v", root, err)
	}
	return out
}

// digestDrift lists every path whose digest differs between before and
// after, added and removed paths included, in path order.
func digestDrift(before, after map[string]string) []string {
	var out []string
	for p, sum := range before {
		if after[p] != sum {
			out = append(out, p)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func TestDigestDrift(t *testing.T) {
	before := map[string]string{"a": "1", "b": "2", "c": "3"}
	after := map[string]string{"a": "1", "b": "9", "d": "4"}
	if got := digestDrift(before, after); !slices.Equal(got, []string{"b", "c", "d"}) {
		t.Errorf("digestDrift = %v, want [b c d]", got)
	}
	if got := digestDrift(before, maps.Clone(before)); got != nil {
		t.Errorf("digestDrift of equal maps = %v, want none", got)
	}
}

// worktreePaths lists root's registered git worktrees, each path resolved
// through symlinks, in git's order.
func worktreePaths(t *testing.T, root string) []string {
	t.Helper()
	out, err := gitOutput(t.Context(), root, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			paths = append(paths, realPath(t, p))
		}
	}
	return paths
}

// realPath resolves p through symlinks (macOS's /var is /private/var).
func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("resolving %s: %v", p, err)
	}
	return r
}

// treeSize is the file count and byte total under dir.
func treeSize(t *testing.T, dir string) (files int, bytes int64) {
	t.Helper()
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files++
		bytes += info.Size()
		return nil
	})
	if err != nil {
		t.Fatalf("sizing %s: %v", dir, err)
	}
	return files, bytes
}

// boardSnapshot is the part of a board snapshot a typed write rides on.
type boardSnapshot struct {
	BaseDigest  string            `json:"base_digest"`
	BaseSpecB64 string            `json:"base_spec_b64"`
	Expected    map[string]string `json:"expected"`
}

// getBoardSnapshot reads the board snapshot at path (a board address).
func getBoardSnapshot(t *testing.T, base, path string) boardSnapshot {
	t.Helper()
	status, body := httpGetBody(t, base+path+"/snapshot")
	if status != http.StatusOK {
		t.Fatalf("GET %s/snapshot = %d\n%s", path, status, body)
	}
	var snap boardSnapshot
	if err := json.Unmarshal([]byte(body), &snap); err != nil {
		t.Fatalf("decoding %s/snapshot: %v", path, err)
	}
	return snap
}

// setProblem posts one typed set-problem transaction through the board's
// mutate path at path, the browser's own transport, and returns the
// response.
func setProblem(t *testing.T, base, path, name, text string, snap boardSnapshot) (int, string) {
	t.Helper()
	return httpPostJSON(t, base+path+"/api/mutate_draft", map[string]any{
		"request": map[string]any{
			"schema":        "verdi.draftmutation/v1",
			"spec":          "spec/" + name,
			"base_digest":   snap.BaseDigest,
			"base_spec_b64": snap.BaseSpecB64,
			"expected":      snap.Expected,
			"operations":    []map[string]string{{"op": "set-problem", "text": text, "anchor": "#problem"}},
		},
	})
}

// refsOf lists every ref in root with the object it names.
func refsOf(t *testing.T, root string) string {
	t.Helper()
	out, err := gitOutput(t.Context(), root, "for-each-ref", "--format=%(refname) %(objectname)")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCanvasWallWritablePath_DomainLiveAndWritesIsolated is F2G-1's claim,
// proved through the SHIPPED binary over a store the shared store's own
// sequence provisions. For each wall-canvas fixture wall:
//   - the writable path renders the whole wall with its domain live: no
//     refusal banner, a pin on every object card, Correct stub, and the
//     sticky's Graduate;
//   - its first request reuses the worktree provisioning pre-cut, so no
//     request pays a cut, and every worktree lives inside the store's own
//     scratch directory, so it goes when that scratch goes (main.go's on
//     exit; this test's through startCanvasWallServeAt);
//   - one typed write through the existing mutate path lands in that
//     wall's own worktree only: the serving branch's copy, the other wall
//     and every other file of the store are byte-identical, and no ref
//     moves;
//   - the serving path still renders as before, under its domain refusal,
//     and refuses the same write.
//
// Each wall's namesake branch also adds one row to the whole-store
// directory, linked to the writable path, and one entry to every board's
// branch menu; both effects are pinned here.
func TestCanvasWallWritablePath_DomainLiveAndWritesIsolated(t *testing.T) {
	neutralizeCIEnvForTest(t)
	serve := startCanvasWallServe(t)
	root := serve.store.storeRoot
	scratch := realPath(t, filepath.Dir(root))

	worktrees := worktreePaths(t, root)
	for _, p := range worktrees {
		if !strings.HasPrefix(p, scratch+string(filepath.Separator)) {
			t.Errorf("worktree %s lies outside the store's scratch %s, so removing the scratch would not remove it", p, scratch)
		}
	}
	for _, w := range canvasWalls() {
		wt := realPath(t, wtmanager.WorktreePath(root, w.branch()))
		if !slices.Contains(worktrees, wt) {
			t.Fatalf("%s's managed worktree %s is not pre-cut; registered: %v", w.name, wt, worktrees)
		}
		files, bytes := treeSize(t, wt)
		t.Logf("cost: %s's pre-cut worktree holds %d files, %d bytes", w.name, files, bytes)
	}

	for _, w := range canvasWalls() {
		t.Run(w.name, func(t *testing.T) {
			wt := wtmanager.WorktreePath(root, w.branch())
			marker := "writable-path probe for " + w.name

			// The writable path, on its first request.
			start := time.Now()
			status, page := httpGetBody(t, serve.base+w.writablePath)
			t.Logf("cost: first GET %s took %s", w.writablePath, time.Since(start))
			if status != http.StatusOK {
				t.Fatalf("GET %s = %d\n%s", w.writablePath, status, page)
			}
			for _, problem := range canvasWallMissing(page, w) {
				t.Errorf("writable path: %s", problem)
			}
			for _, problem := range canvasWallDomainProblems(page, w, true) {
				t.Errorf("writable path: %s", problem)
			}
			if got := worktreePaths(t, root); !slices.Equal(got, worktrees) {
				t.Errorf("the first request changed the worktrees: %v, want %v", got, worktrees)
			}

			// The serving path still renders as before, and refuses a typed
			// write with its domain refusal.
			status, page = httpGetBody(t, serve.base+w.servingPath)
			if status != http.StatusOK {
				t.Fatalf("GET %s = %d\n%s", w.servingPath, status, page)
			}
			for _, problem := range canvasWallMissing(page, w) {
				t.Errorf("serving path: %s", problem)
			}
			for _, problem := range canvasWallDomainProblems(page, w, false) {
				t.Errorf("serving path: %s", problem)
			}
			refsBefore := refsOf(t, root)
			digests := storeFileDigests(t, root, "")
			status, body := setProblem(t, serve.base, w.servingPath, w.name, marker, getBoardSnapshot(t, serve.base, w.servingPath))
			if status != http.StatusForbidden || !strings.Contains(body, "is not its mutable design branch") {
				t.Errorf("a typed write on the serving path = %d %s, want 403 with the domain refusal", status, body)
			}
			if drift := digestDrift(digests, storeFileDigests(t, root, "")); len(drift) > 0 {
				t.Errorf("the refused write changed the store: %v", drift)
			}

			// One typed write through the writable path.
			status, body = setProblem(t, serve.base, w.writablePath, w.name, marker, getBoardSnapshot(t, serve.base, w.writablePath))
			if status != http.StatusOK || !strings.Contains(body, `"result"`) || strings.Contains(body, `"failure"`) {
				t.Fatalf("a typed write on the writable path = %d %s, want 200 with a result", status, body)
			}
			outside := maps.Clone(digests)
			maps.DeleteFunc(outside, func(p string, _ string) bool {
				return strings.HasPrefix(p, filepath.ToSlash(mustRel(t, root, wt))+"/")
			})
			if drift := digestDrift(outside, storeFileDigests(t, root, wt)); len(drift) > 0 {
				t.Errorf("the write reached past %s's own worktree: %v", w.name, drift)
			}
			if got := refsOf(t, root); got != refsBefore {
				t.Errorf("the write moved a ref:\n%s\nwant\n%s", got, refsBefore)
			}
			// Inside the wall's own worktree the write is exactly the
			// kernel's: the spec rewritten, and its design-provenance
			// record beside it.
			specDir := ".verdi/specs/active/" + w.name
			specRel := specDir + "/spec.md"
			want := "M " + specRel + "\n?? " + specDir + "/design-provenance.jsonl"
			if got, err := gitOutput(t.Context(), wt, "status", "--porcelain", "--untracked-files=all"); err != nil || got != want {
				t.Errorf("%s's worktree porcelain = %q (%v), want %q", w.name, got, err, want)
			}
			if spec, err := os.ReadFile(filepath.Join(wt, filepath.FromSlash(specRel))); err != nil || !strings.Contains(string(spec), marker) {
				t.Errorf("the write did not land in %s's worktree spec (%v)", w.name, err)
			}
			if _, page := httpGetBody(t, serve.base+w.writablePath); !strings.Contains(page, marker) {
				t.Errorf("the writable path does not show its own write")
			}
			if _, page := httpGetBody(t, serve.base+w.servingPath); strings.Contains(page, marker) || len(canvasWallDomainProblems(page, w, false)) > 0 {
				t.Errorf("the serving path changed after the writable path's write")
			}
		})
	}

	// The count effects of the namesake branches: one directory row each,
	// a local design-branch draft linked to the writable path, and one
	// branch-menu entry each on a board served from the serving checkout.
	status, home := httpGetBody(t, serve.base+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / = %d", status)
	}
	status, board := httpGetBody(t, serve.base+canvasWallServingPath)
	if status != http.StatusOK {
		t.Fatalf("GET %s = %d", canvasWallServingPath, status)
	}
	for _, w := range canvasWalls() {
		row := `data-testid="dir-entry-` + w.name + `" data-source="` + string(refindex.SourceLocal) + `"`
		if !strings.Contains(home, row) {
			t.Errorf("the directory has no local design-branch row for %s", w.name)
		}
		if !strings.Contains(home, `<a class="dir-board" href="`+w.writablePath+`">spec/`+w.name+`</a>`) {
			t.Errorf("%s's directory row does not link its writable path %s", w.name, w.writablePath)
		}
		if !strings.Contains(board, `data-branch="`+w.branch()+`"`) {
			t.Errorf("the serving checkout's branch menu does not list %s", w.branch())
		}
	}
}

// mustRel is filepath.Rel that fails the test on error.
func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	rel, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatal(err)
	}
	return rel
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
