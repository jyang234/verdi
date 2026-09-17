package specimport

import (
	"errors"
	"os"
	"testing"
)

// f13Card pins one mechanical-field-map.json "cards" entry: the exact
// selector, its declared display_text (post collapse-whitespace) and the
// RAW (pre-transform) selected span's sha256 (source_text_sha256) — see
// the lane report for the independent Python verification that
// source_text_sha256 hashes the RAW span, not the collapsed text.
type f13Card struct {
	target      string
	start, end  int
	displayText string
	rawSHA256   string
}

// Revised under spec/uat-round-1 ac-7/dc-9 (2026-09-16): the prior eight
// selectors split the source's one comma-joined "Prove" sentence at
// arbitrary points (joining multiple claims, starting mid-clause) and
// selected the imperative "Enumerate every allowed edge..." sentence as
// if it were a criterion. Each selector below now covers exactly one
// complete claim and starts/ends at a clause or sentence boundary; the
// nine Prove-sentence claims are ac-1..ac-9, the already-whole
// blocking-finding sentence is ac-10 (byte-identical to the prior ac-7),
// and the prior two-sentence ac-8 splits into ac-11 and ac-12.
var f13PinnedCards = []f13Card{
	{"ac-1", 1556, 1572, "R0 cannot repeat", "1240c7be155b26d99dd4198dea9d7fe859874c993420d1e76d65930f6cf161d1"},
	{"ac-2", 1574, 1600, "R1 count cannot exceed one", "d99db78b04a90aab45cc504797db96b658d275319d15ae8f77dccaacd7dec11e"},
	{"ac-3", 1602, 1633, "R1 candidate re-enters Aligning", "920335b10f43f0d0d37ecfb777bab7daa43617ac1ae80952820636fe2898aa6e"},
	{"ac-4", 1635, 1681, "R2 cannot produce another automatic correction", "46dca1df0415f80681414e732f2519769e9fb0f356572ea3f081ac654053e4bf"},
	{"ac-5", 1683, 1724, "any tree change invalidates bound results", "7982283e48651335d8f64f83fcceb9b88d981344b9809b7719d2da6cb3b7ea26"},
	{"ac-6", 1726, 1775, "all stories Done moves the feature to AwaitingUAT", "d80e519ed49fe8fbe82e43e445d15f230598dc4aeff6c37fe2cc652556e175aa"},
	{"ac-7", 1777, 1850, "two consecutive operational exits from Align route only that flight to G2", "fc0e6413d8b70d3b1cf421951761cde6df7dea0f70cc72652478b422d2da733f"},
	{"ac-8", 1852, 1891, "provider summaries never satisfy a gate", "16a645629e587b035cdb4b7c25feba5b453e878e96d8daa5a6c3a1b3254d4052"},
	{"ac-9", 1897, 1924, "there is no `Failed` state.", "7626b25e56987e75b00b6591ca921d8abc8069ff52e5fbde1f9adce3e02bd193"},
	{"ac-10", 1981, 2115, "A blocking finding requires nonempty binding-authority cite, reachable-state witness, concrete incorrect result, and threat-model fit.", "4b730b5b58e4a13bbb56860d56a012e49ee78b2ad6303635f393cd80e0855e22"},
	{"ac-11", 2116, 2157, "The author lane adjudicates each finding.", "ac955612933d11b54aabf5b5ee67ccf7eff29463563829a1abc4de75f256180b"},
	{"ac-12", 2158, 2200, "Conflicting blocking findings route to G2.", "8e99c87e643c1f81b8a8dd7d43c6becc6ec811d0296ce461ad85aecb1db0f496"},
}

func readF13Fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/f13/" + name)
	if err != nil {
		t.Fatalf("reading F13 fixture %s: %v", name, err)
	}
	return data
}

func f13Request(t *testing.T, retainUnmapped bool, extraSources ...Source) Request {
	primary := readF13Fixture(t, "primary-f13.md")
	req := Request{
		Schema:         RequestSchema,
		Target:         Target{Slug: "f13-gatekeeper", Class: "feature", Title: "F13 Gatekeeper"},
		Format:         FormatF13Reference,
		Primary:        "primary-f13",
		Sources:        append([]Source{{ID: "primary-f13", Label: "primary-f13.md", Data: primary}}, extraSources...),
		RetainUnmapped: retainUnmapped,
	}
	return req
}

func TestNormalize_F13ProfileProducesTwelvePinnedFields(t *testing.T) {
	plan, err := Normalize(f13Request(t, true))
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	primaryData := readF13Fixture(t, "primary-f13.md")

	for _, card := range f13PinnedCards {
		field, ok := fieldByTarget(plan.Fields, card.target)
		if !ok {
			t.Fatalf("no field for %s: %+v", card.target, plan.Fields)
		}
		if field.Text != card.displayText {
			t.Errorf("%s.Text = %q, want %q", card.target, field.Text, card.displayText)
		}
		if field.Origin != OriginCopiedSource {
			t.Errorf("%s.Origin = %q, want %q", card.target, field.Origin, OriginCopiedSource)
		}
		if len(field.Spans) != 1 {
			t.Fatalf("%s has %d spans, want 1", card.target, len(field.Spans))
		}
		sp := field.Spans[0]
		if sp.SourceID != "primary-f13" || sp.Start != card.start || sp.End != card.end {
			t.Errorf("%s span = %+v, want source primary-f13 [%d,%d)", card.target, sp, card.start, card.end)
		}
		rawGot := sha256Hex(primaryData[card.start:card.end])
		if rawGot != card.rawSHA256 {
			t.Errorf("%s raw span sha256 = %s, want %s (mechanical-field-map.json source_text_sha256)", card.target, rawGot, card.rawSHA256)
		}
	}
}

func TestNormalize_F13ProfileCoverageAccountsEveryByteOnce(t *testing.T) {
	plan, err := Normalize(f13Request(t, true))
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	cov := coverageForSource(t, plan, "primary-f13")

	if cov.TotalBytes != 2409 {
		t.Errorf("TotalBytes = %d, want 2409", cov.TotalBytes)
	}
	if cov.MappedBytes != 565 {
		t.Errorf("MappedBytes = %d, want 565", cov.MappedBytes)
	}
	if cov.RetainedBytes != 1844 {
		t.Errorf("RetainedBytes = %d, want 1844", cov.RetainedBytes)
	}
	if cov.UnresolvedBytes != 0 {
		t.Errorf("UnresolvedBytes = %d, want 0", cov.UnresolvedBytes)
	}
	if got := cov.MappedBytes + cov.RetainedBytes + cov.UnresolvedBytes; got != cov.TotalBytes {
		t.Errorf("mapped+retained+unresolved = %d, want TotalBytes %d", got, cov.TotalBytes)
	}
	if len(cov.Intervals) != 25 {
		t.Errorf("len(Intervals) = %d, want 25 (12 mapped + 13 retained-only)", len(cov.Intervals))
	}
	mapped, retained := 0, 0
	for _, iv := range cov.Intervals {
		switch iv.Disposition {
		case DispositionMapped:
			mapped++
		case DispositionRetained:
			retained++
		default:
			t.Errorf("unexpected disposition %q in F13 primary coverage", iv.Disposition)
		}
	}
	if mapped != 12 || retained != 13 {
		t.Errorf("mapped intervals = %d, retained intervals = %d, want 12 and 13", mapped, retained)
	}
}

func coverageForSource(t *testing.T, plan Plan, sourceID string) Coverage {
	t.Helper()
	for _, c := range plan.Coverage {
		if c.SourceID == sourceID {
			return c
		}
	}
	t.Fatalf("no coverage entry for source %q: %+v", sourceID, plan.Coverage)
	return Coverage{}
}

func TestNormalize_F13ProfileMissingStatementsAndAllEvidenceGaps(t *testing.T) {
	plan, err := Normalize(f13Request(t, true))
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	for _, target := range []string{"problem", "outcome"} {
		if _, ok := findingForTarget(plan.Findings, FindingMissingStatement, target); !ok {
			t.Errorf("no missing-statement finding for %s", target)
		}
		if _, ok := fieldByTarget(plan.Fields, target); ok {
			t.Errorf("%s field present despite F13 having no explicit label", target)
		}
	}
	for _, card := range f13PinnedCards {
		if _, ok := findingForTarget(plan.Findings, FindingMissingEvidence, card.target); !ok {
			t.Errorf("no missing-evidence finding for %s", card.target)
		}
	}
	// mechanical-field-map.json's required_value_gap_count is 14: 2
	// statements + 12 AC evidence declarations.
	gaps := 0
	for _, f := range plan.Findings {
		if f.Code == FindingMissingStatement || f.Code == FindingMissingEvidence {
			gaps++
		}
	}
	if gaps != 14 {
		t.Errorf("missing-statement + missing-evidence findings = %d, want 14", gaps)
	}
}

// TestNormalize_F13ProfileStatementsMappedFromASupportSource pins the F13
// half of the same reconciliation: the profile's bound primary genuinely
// has no labeled Problem/Outcome, and a user may map them from a supporting
// source. Both facts must survive — the statements resolve, and the
// profile's structural disclosure stays visible as nonblocking.
func TestNormalize_F13ProfileStatementsMappedFromASupportSource(t *testing.T) {
	support := Source{ID: "notes", Label: "notes.md", Data: []byte("The gate cannot be audited.\n\nEvery gate decision is reviewable.\n")}
	req := f13Request(t, true, support)
	req.Mappings = []Mapping{
		{Target: "problem", SourceID: "notes", Start: 0, End: 27, Transform: TransformIdentity},
		{Target: "outcome", SourceID: "notes", Start: 29, End: 63, Transform: TransformIdentity},
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	for _, target := range []string{"problem", "outcome"} {
		field, ok := fieldByTarget(plan.Fields, target)
		if !ok || field.Text == "" {
			t.Fatalf("%s = %+v ok=%v, want the explicitly mapped statement", target, field, ok)
		}
		f, ok := findingForTarget(plan.Findings, FindingMissingStatement, target)
		if !ok {
			t.Fatalf("the profile's structural disclosure for %s was cleared entirely: %+v", target, plan.Findings)
		}
		if f.Blocking {
			t.Errorf("missing-statement for the explicitly mapped %s is still blocking: %+v", target, f)
		}
	}
	// The twelve evidence gaps are untouched, and the primary's pinned byte
	// accounting is unaffected by mapping statements from another source.
	for _, card := range f13PinnedCards {
		if f, ok := findingForTarget(plan.Findings, FindingMissingEvidence, card.target); !ok || !f.Blocking {
			t.Errorf("missing-evidence for %s = %+v ok=%v, want it still blocking", card.target, f, ok)
		}
	}
	cov := coverageForSource(t, plan, "primary-f13")
	if cov.TotalBytes != 2409 || cov.MappedBytes != 565 || cov.RetainedBytes != 1844 || len(cov.Intervals) != 25 {
		t.Errorf("primary coverage changed: %+v", cov)
	}
}

func TestNormalize_F13ProfileRefusesAlteredPrimaryBytes(t *testing.T) {
	primary := readF13Fixture(t, "primary-f13.md")
	altered := append([]byte(nil), primary...)
	altered[0] = 'X' // one changed byte anywhere invalidates the pinned SHA-256.

	req := Request{
		Schema:         RequestSchema,
		Target:         Target{Slug: "f13-gatekeeper", Class: "feature", Title: "F13 Gatekeeper"},
		Format:         FormatF13Reference,
		Primary:        "primary-f13",
		Sources:        []Source{{ID: "primary-f13", Label: "primary-f13.md", Data: altered}},
		RetainUnmapped: true,
	}
	if _, err := Normalize(req); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("Normalize with an altered F13 primary: got err %v, want ErrUnsupportedFormat", err)
	}
}

func TestNormalize_F13ProfileSupportingSourcesAreWholeRetainedUnits(t *testing.T) {
	supports := []Source{
		{ID: "review-validation", Label: "review-validation.md", Data: readF13Fixture(t, "review-validation.md")},
		{ID: "transition-core", Label: "transition-core.md", Data: readF13Fixture(t, "transition-core.md")},
		{ID: "review-journal", Label: "review-journal.md", Data: readF13Fixture(t, "review-journal.md")},
	}
	plan, err := Normalize(f13Request(t, true, supports...))
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	for _, s := range supports {
		cov := coverageForSource(t, plan, s.ID)
		if len(cov.Intervals) != 1 {
			t.Fatalf("support %s has %d intervals, want exactly 1 (a whole retained-only unit): %+v", s.ID, len(cov.Intervals), cov.Intervals)
		}
		iv := cov.Intervals[0]
		if iv.Disposition != DispositionRetained || iv.Start != 0 || iv.End != len(s.Data) {
			t.Fatalf("support %s interval = %+v, want one retained-only [0,%d)", s.ID, iv, len(s.Data))
		}
		if cov.TotalBytes != len(s.Data) || cov.RetainedBytes != len(s.Data) || cov.MappedBytes != 0 || cov.UnresolvedBytes != 0 {
			t.Fatalf("support %s coverage = %+v, want a single fully-retained unit", s.ID, cov)
		}
	}
}

func TestNormalize_F13ProfileUnassignedSupportUnresolvedWhenNotRetained(t *testing.T) {
	supports := []Source{
		{ID: "review-validation", Label: "review-validation.md", Data: readF13Fixture(t, "review-validation.md")},
	}
	plan, err := Normalize(f13Request(t, false, supports...))
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	cov := coverageForSource(t, plan, "review-validation")
	if len(cov.Intervals) != 1 || cov.Intervals[0].Disposition != DispositionUnresolved {
		t.Fatalf("support coverage with retain_unmapped=false = %+v, want one unresolved unit", cov)
	}
	if _, ok := findingForTarget(plan.Findings, FindingUnresolvedCoverage, "review-validation"); !ok {
		t.Fatalf("no unresolved-coverage finding for the unassigned support source: %+v", plan.Findings)
	}
}

// TestNormalize_LineSelectionParityMatchesStageplanExtraction pins review
// residual O11: the pinned ATC stage-plan lines 602-671 and the
// pre-extracted primary-f13.md snapshot must normalize to the identical
// selected digest, while their original-file digests differ (they are
// different files of different sizes).
func TestNormalize_LineSelectionParityMatchesStageplanExtraction(t *testing.T) {
	stagePlan := readF13Fixture(t, "stage-plan-c346c005.md")
	primarySnapshot := readF13Fixture(t, "primary-f13.md")

	req := Request{
		Schema:  RequestSchema,
		Target:  Target{Slug: "f13-gatekeeper", Class: "feature", Title: "F13 Gatekeeper"},
		Format:  FormatManualV1,
		Primary: "from-stage-plan",
		Sources: []Source{
			{ID: "from-stage-plan", Label: "stage-plan.md", Data: stagePlan, StartLine: 602, EndLine: 671},
			{ID: "from-snapshot", Label: "primary-f13.md", Data: primarySnapshot},
		},
		RetainUnmapped: true,
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}

	const pinnedSelectedDigest = "7c6d95d4aa516cf682e6d4a33d1c260f1c861d938d46559241a7c764bffca577"
	const pinnedStagePlanDigest = "55c8e7ccc6df6c04418c1a217b2f15e332b1e1127934d2af70afd82ad1b4a8de"

	var fromStagePlan, fromSnapshot Snapshot
	for _, s := range plan.Sources {
		switch s.ID {
		case "from-stage-plan":
			fromStagePlan = s
		case "from-snapshot":
			fromSnapshot = s
		}
	}
	if fromStagePlan.Digest != pinnedSelectedDigest {
		t.Errorf("stage-plan lines 602-671 selected digest = %s, want %s", fromStagePlan.Digest, pinnedSelectedDigest)
	}
	if fromSnapshot.Digest != pinnedSelectedDigest {
		t.Errorf("primary-f13.md snapshot selected digest = %s, want %s", fromSnapshot.Digest, pinnedSelectedDigest)
	}
	if fromStagePlan.OriginalDigest != pinnedStagePlanDigest {
		t.Errorf("stage-plan original digest = %s, want %s", fromStagePlan.OriginalDigest, pinnedStagePlanDigest)
	}
	if fromSnapshot.OriginalDigest != pinnedSelectedDigest {
		t.Errorf("primary-f13.md snapshot original digest = %s, want %s (it IS already the whole selection)", fromSnapshot.OriginalDigest, pinnedSelectedDigest)
	}
	if fromStagePlan.OriginalDigest == fromSnapshot.OriginalDigest {
		t.Error("the two sources' original digests must differ (different files, different sizes); only their SELECTED digest is asserted equal")
	}
}
