package specimport

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/designscaffold"
)

// f13DeferredNoMappingRequest is the owner's exact reproduced selection:
// the pinned F13 primary, both statements explicitly deferred, unmapped
// bytes explicitly retained, and ZERO Mappings — precisely the state
// spec-import-contract.md anticipates ("The preview has a missing-evidence
// finding until an explicit Mapping supplies kinds"). Every automatic
// acceptance criterion therefore reaches Compose with no evidence kind.
func f13DeferredNoMappingRequest(t *testing.T) Request {
	t.Helper()
	req := f13Request(t, true)
	req.DeferStatements = true
	return req
}

// TestCompose_F13NoMappings_EvidenceGapIsMissingEvidence pins the one
// classification this state is allowed to produce. The primary's structure
// is fully recognized (eight pinned selectors, zero unresolved bytes), so
// the only defect is the absent evidence the contract already names
// missing-evidence; reporting it as unsupported-structure asserts a
// structural defect the engine never computed and, through the browser's
// code-keyed guidance, tells the operator to abandon a format that parsed
// perfectly.
//
// Each criterion is reported exactly once, no finding carries the internal
// splice package name, and the candidate stays nil while incomplete.
func TestCompose_F13NoMappings_EvidenceGapIsMissingEvidence(t *testing.T) {
	root := minimalStoreRoot(t)
	req := f13DeferredNoMappingRequest(t)
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: want findings, not an error: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose returned candidate bytes for an incomplete import:\n%s", candidate)
	}

	byTarget := map[string]int{}
	for _, f := range findings {
		if f.Code == FindingUnsupportedStructure {
			t.Errorf("unsupported-structure reported for a fully recognized primary whose only gap is evidence: %+v", f)
		}
		if strings.Contains(f.Message, "splice:") {
			t.Errorf("finding forwards the internal splice package name to the user: %+v", f)
		}
		if f.Code == FindingMissingEvidence {
			if !f.Blocking {
				t.Errorf("missing-evidence finding is not blocking: %+v", f)
			}
			byTarget[f.Target]++
		}
	}
	for _, card := range f13PinnedCards {
		if byTarget[card.target] != 1 {
			t.Errorf("missing-evidence findings for %s = %d, want exactly 1: %+v", card.target, byTarget[card.target], findings)
		}
	}
	if len(byTarget) != len(f13PinnedCards) {
		t.Errorf("missing-evidence targets = %v, want exactly the %d pinned criteria: %+v", byTarget, len(f13PinnedCards), findings)
	}
}

// TestPreview_F13NoMappings_EvidenceGapsAndDeferralsOnly is the same state
// at the surface the operator actually sees: eight blocking
// missing-evidence findings, the two nonblocking statements-deferred
// disclosures, and nothing else — no extra fabricated structural defect and
// no duplicate entry for the criterion Compose happened to reach first.
// Fields and coverage are asserted in the same pass: they are computed by
// assemblePreview from the plan, independent of Compose's classification,
// and must stay exactly as they are.
func TestPreview_F13NoMappings_EvidenceGapsAndDeferralsOnly(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := f13DeferredNoMappingRequest(t)

	result, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if result.Ready {
		t.Fatalf("Ready = true for an import with no evidence at all: %+v", result.Findings)
	}
	if result.Candidate != nil {
		t.Fatalf("Preview returned candidate bytes for an incomplete import:\n%s", result.Candidate)
	}

	missing := map[string]int{}
	deferred := map[string]int{}
	for _, f := range result.Findings {
		switch f.Code {
		case FindingMissingEvidence:
			if !f.Blocking {
				t.Errorf("missing-evidence finding is not blocking: %+v", f)
			}
			missing[f.Target]++
		case FindingStatementsDeferred:
			if f.Blocking {
				t.Errorf("statements-deferred disclosure is blocking: %+v", f)
			}
			deferred[f.Target]++
		default:
			t.Errorf("unexpected %s finding in a state whose only gaps are evidence and deferral: %+v", f.Code, f)
		}
	}
	for _, card := range f13PinnedCards {
		if missing[card.target] != 1 {
			t.Errorf("missing-evidence findings for %s = %d, want exactly 1: %+v", card.target, missing[card.target], result.Findings)
		}
	}
	if len(missing) != len(f13PinnedCards) {
		t.Errorf("missing-evidence targets = %v, want exactly the %d pinned criteria", missing, len(f13PinnedCards))
	}
	for _, target := range []string{"problem", "outcome"} {
		if deferred[target] != 1 {
			t.Errorf("statements-deferred disclosures for %s = %d, want exactly 1: %+v", target, deferred[target], result.Findings)
		}
	}

	wantFields := []string{"problem", "outcome"}
	for _, card := range f13PinnedCards {
		wantFields = append(wantFields, card.target)
	}
	if len(result.Fields) != len(wantFields) {
		t.Fatalf("Fields = %d entries, want %d: %+v", len(result.Fields), len(wantFields), result.Fields)
	}
	for i, want := range wantFields {
		if result.Fields[i].Target != want {
			t.Errorf("Fields[%d].Target = %q, want %q", i, result.Fields[i].Target, want)
		}
	}

	primary := readF13Fixture(t, "primary-f13.md")
	var sawPrimaryCoverage bool
	for _, cov := range result.Coverage {
		if cov.SourceID != "primary-f13" {
			continue
		}
		sawPrimaryCoverage = true
		if cov.TotalBytes != len(primary) {
			t.Errorf("primary TotalBytes = %d, want %d", cov.TotalBytes, len(primary))
		}
		if cov.UnresolvedBytes != 0 {
			t.Errorf("primary UnresolvedBytes = %d, want 0 under retain_unmapped", cov.UnresolvedBytes)
		}
	}
	if !sawPrimaryCoverage {
		t.Fatalf("no coverage record for the primary: %+v", result.Coverage)
	}
}

// TestCompose_MissingEvidence_TemplateRefusalStillReported is the
// counterexample that keeps the correction from becoming a global early
// return on the plan's blocking findings. A store override declaring a stub
// of its own cannot express an import at all, and that refusal is reported
// today from inside applyCandidateEdits, long before the evidence gate —
// with zero evidence mapped, the operator must still learn about it now
// rather than after selecting all eight evidence sets.
func TestCompose_MissingEvidence_TemplateRefusalStillReported(t *testing.T) {
	root := minimalStoreRoot(t)
	canonical, err := designscaffold.LoadTemplate(root, "feature.md")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	override := strings.Replace(string(canonical), placeholderStubSlug, "custom-child", 1)
	if !strings.Contains(override, "custom-child") {
		t.Fatal("test fixture assumption broken: the canonical feature template no longer declares the placeholder stub slug")
	}
	writeStoreFile(t, root, ".verdi/templates/feature.md", override)

	req := f13DeferredNoMappingRequest(t)
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: want a blocking finding, not an error: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose imported against a template it cannot express:\n%s", candidate)
	}
	var sawRefusal bool
	for _, f := range findings {
		if f.Blocking && f.Code == FindingUnsupportedStructure && f.Target == "template" &&
			strings.Contains(f.Message, "custom-child") {
			sawRefusal = true
		}
	}
	if !sawRefusal {
		t.Fatalf("want the template refusal still reported while evidence is missing, got: %+v", findings)
	}
}

// TestCompose_AppendObjectFailure_StillUnsupportedStructure proves the
// correction narrows exactly one condition at the pass-4 call site and
// nothing else: a field whose id belongs to no object block is a genuine
// structural defect of the candidate this template/model must express, and
// it keeps its unsupported-structure code, its target and splice's own
// message. Compose is exported over an arbitrary Plan, so this is reached
// the same way any other caller would reach it.
func TestCompose_AppendObjectFailure_StillUnsupportedStructure(t *testing.T) {
	root := minimalStoreRoot(t)
	req := minimalRequest()
	plan := Plan{Fields: []Field{
		{Target: "problem", Text: "First line.", Origin: OriginUserAdded},
		{Target: "outcome", Text: "Users get value.", Origin: OriginUserAdded},
		{Target: "ac-1", Text: "Criterion one.", Origin: OriginUserAdded, Evidence: []string{"static", "attestation"}},
		{Target: "zz-1", Text: "No object block owns this id.", Origin: OriginUserAdded},
	}}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: want a blocking finding, not an error: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose composed a candidate carrying a field no object block owns:\n%s", candidate)
	}
	var sawUnsupported bool
	for _, f := range findings {
		if f.Blocking && f.Code == FindingUnsupportedStructure && f.Target == "zz-1" &&
			strings.Contains(f.Message, "no known block prefix") {
			sawUnsupported = true
		}
	}
	if !sawUnsupported {
		t.Fatalf("want the unrelated append failure still reported as %s on zz-1, got: %+v", FindingUnsupportedStructure, findings)
	}
}

// TestCompose_EvidencelessCriterion_WithoutBlockingPlanFinding_StillBlocks
// proves the suppression cannot fabricate readiness. Compose is exported
// and takes an arbitrary Plan, so a caller can present an evidence-less
// acceptance criterion whose plan carries no blocking missing-evidence
// entry for it — because it carries none at all, or carries only a
// nonblocking one. In both cases Compose must report the gap itself and
// leave the import blocked with nil bytes; only a duplicate of an existing
// BLOCKING missing-evidence entry for the same target is ever suppressed.
func TestCompose_EvidencelessCriterion_WithoutBlockingPlanFinding_StillBlocks(t *testing.T) {
	fields := []Field{
		{Target: "problem", Text: "First line.", Origin: OriginUserAdded},
		{Target: "outcome", Text: "Users get value.", Origin: OriginUserAdded},
		{Target: "ac-1", Text: "Criterion one.", Origin: OriginUserAdded},
	}
	cases := []struct {
		name     string
		findings []Finding
	}{
		{name: "no finding at all"},
		{name: "nonblocking finding only", findings: []Finding{{
			Code:     FindingMissingEvidence,
			Target:   "ac-1",
			Message:  "ac-1 has no evidence kind declared; an explicit mapping must supply one before creation",
			Blocking: false,
		}}},
		{name: "blocking finding for another target", findings: []Finding{{
			Code:     FindingMissingEvidence,
			Target:   "ac-2",
			Message:  "ac-2 has no evidence kind declared; an explicit mapping must supply one before creation",
			Blocking: true,
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := minimalStoreRoot(t)
			plan := Plan{Fields: fields, Findings: tc.findings}

			candidate, findings, err := Compose(context.Background(), root, minimalRequest(), plan)
			if err != nil {
				t.Fatalf("Compose: want a blocking finding, not an error: %v", err)
			}
			if candidate != nil {
				t.Fatalf("Compose composed a candidate whose criterion has no evidence:\n%s", candidate)
			}
			var sawBlocking bool
			for _, f := range findings {
				if f.Blocking && f.Code == FindingMissingEvidence && f.Target == "ac-1" {
					sawBlocking = true
				}
			}
			if !sawBlocking {
				t.Fatalf("want a blocking missing-evidence finding for ac-1, got: %+v", findings)
			}
		})
	}
}

// TestPreview_AttestationFloorUnchanged pins the rule owner the correction
// must not acquire. VL-006's feature evidence floor stays with existing
// lint (spec-import-contract.md: "feature evidence floors ... have one
// owner, never copied into the importer"): criteria mapped with static
// alone still reach the candidate and are still refused there, under
// invalid-candidate rather than the importer's own missing-evidence, and
// criteria mapped with attestation still preview ready.
func TestPreview_AttestationFloorUnchanged(t *testing.T) {
	t.Run("static only is refused by the lint floor", func(t *testing.T) {
		repo := buildImportRepo(t)
		svc := testService(t)
		req := minimalRequest()
		req.Mappings = []Mapping{
			{Target: "ac-1", Evidence: []string{"static"}},
			{Target: "ac-2", Evidence: []string{"static"}},
		}

		result, err := svc.Preview(context.Background(), repo.Dir, req)
		if err != nil {
			t.Fatalf("Preview: %v", err)
		}
		if result.Ready {
			t.Fatalf("Ready = true for criteria that miss the outcome floor: %+v", result.Findings)
		}
		if result.Candidate != nil {
			t.Fatalf("Preview returned candidate bytes for a refused import:\n%s", result.Candidate)
		}
		var sawFloor bool
		for _, f := range result.Findings {
			if f.Code == FindingMissingEvidence {
				t.Errorf("the importer claimed the lint floor's condition as its own missing-evidence gap: %+v", f)
			}
			if f.Blocking && f.Code == FindingInvalidCandidate && strings.Contains(f.Message, "VL-006") &&
				strings.Contains(f.Message, "attestation") {
				sawFloor = true
			}
		}
		if !sawFloor {
			t.Fatalf("want the VL-006 attestation floor reported through the candidate lint, got: %+v", result.Findings)
		}
	})

	t.Run("attestation reaches ready", func(t *testing.T) {
		repo := buildImportRepo(t)
		svc := testService(t)
		req := minimalRequest()
		req.Mappings = []Mapping{
			{Target: "ac-1", Evidence: []string{"attestation"}},
			{Target: "ac-2", Evidence: []string{"attestation"}},
		}

		result, err := svc.Preview(context.Background(), repo.Dir, req)
		if err != nil {
			t.Fatalf("Preview: %v", err)
		}
		if !result.Ready {
			t.Fatalf("Ready = false for criteria that satisfy the outcome floor: %+v", result.Findings)
		}
		if result.Candidate == nil {
			t.Fatal("Preview returned nil candidate bytes alongside ready:true")
		}
	})
}
