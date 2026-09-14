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

var f13PinnedCards = []f13Card{
	{"ac-1", 1485, 1549, "Enumerate every allowed edge and representative forbidden edges.", "c2c2bc453bc856ac53dc29837c8ea7a06fe2239a7c3864d16094d5ab9f55b940"},
	{"ac-2", 1556, 1681, "R0 cannot repeat, R1 count cannot exceed one, R1 candidate re-enters Aligning, R2 cannot produce another automatic correction", "61663084bdd0399181e8d5da50ecb502c3ac6a07339f873f4c34045ce8ef3af3"},
	{"ac-3", 1683, 1724, "any tree change invalidates bound results", "7982283e48651335d8f64f83fcceb9b88d981344b9809b7719d2da6cb3b7ea26"},
	{"ac-4", 1726, 1775, "all stories Done moves the feature to AwaitingUAT", "d80e519ed49fe8fbe82e43e445d15f230598dc4aeff6c37fe2cc652556e175aa"},
	{"ac-5", 1777, 1850, "two consecutive operational exits from Align route only that flight to G2", "fc0e6413d8b70d3b1cf421951761cde6df7dea0f70cc72652478b422d2da733f"},
	{"ac-6", 1852, 1924, "provider summaries never satisfy a gate, and there is no `Failed` state.", "7396c7b0427133ac28f732b44031d0076d31ad96fcc5033af709c054155d4331"},
	{"ac-7", 1981, 2115, "A blocking finding requires nonempty binding-authority cite, reachable-state witness, concrete incorrect result, and threat-model fit.", "4b730b5b58e4a13bbb56860d56a012e49ee78b2ad6303635f393cd80e0855e22"},
	{"ac-8", 2116, 2200, "The author lane adjudicates each finding. Conflicting blocking findings route to G2.", "d8794640a3b6b09260b9ae1f8620e6af3e13db6841aa3ea27a37cbccf0c46e61"},
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

func TestNormalize_F13ProfileProducesEightPinnedFields(t *testing.T) {
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
	if cov.MappedBytes != 642 {
		t.Errorf("MappedBytes = %d, want 642", cov.MappedBytes)
	}
	if cov.RetainedBytes != 1767 {
		t.Errorf("RetainedBytes = %d, want 1767", cov.RetainedBytes)
	}
	if cov.UnresolvedBytes != 0 {
		t.Errorf("UnresolvedBytes = %d, want 0", cov.UnresolvedBytes)
	}
	if got := cov.MappedBytes + cov.RetainedBytes + cov.UnresolvedBytes; got != cov.TotalBytes {
		t.Errorf("mapped+retained+unresolved = %d, want TotalBytes %d", got, cov.TotalBytes)
	}
	if len(cov.Intervals) != 17 {
		t.Errorf("len(Intervals) = %d, want 17 (8 mapped + 9 retained-only)", len(cov.Intervals))
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
	if mapped != 8 || retained != 9 {
		t.Errorf("mapped intervals = %d, retained intervals = %d, want 8 and 9", mapped, retained)
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
	// source-inventory.json's required_value_gap_count is 10: 2 statements
	// + 8 AC evidence declarations.
	gaps := 0
	for _, f := range plan.Findings {
		if f.Code == FindingMissingStatement || f.Code == FindingMissingEvidence {
			gaps++
		}
	}
	if gaps != 10 {
		t.Errorf("missing-statement + missing-evidence findings = %d, want 10", gaps)
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
