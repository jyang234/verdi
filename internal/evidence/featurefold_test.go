package evidence

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// featureSpec builds a minimal, valid feature spec with one AC declaring
// the given evidence kinds, for FoldFeature's synthetic table-driven
// cases (brief: "synthetic record/edge sets for fold cases").
func featureSpec(t *testing.T, acID string, kinds ...artifact.EvidenceKind) *artifact.SpecFrontmatter {
	t.Helper()
	return &artifact.SpecFrontmatter{
		Base:  artifact.Base{ID: "spec/loan-update", Kind: artifact.KindSpec, Title: "Loan update", Owners: []string{"platform-team"}},
		Class: artifact.ClassFeature,
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: acID, Text: "outcome text for " + acID, Evidence: kinds},
		},
	}
}

func ciRecord(kind artifact.EvidenceKind, verdict artifact.EvidenceVerdict, ac string) artifact.Evidence {
	return artifact.Evidence{
		Schema:      "verdi.evidence/v1",
		EvidenceFor: []string{ac},
		Kind:        kind,
		Verdict:     verdict,
		Witness:     "w",
		Provenance:  artifact.EvidenceProvenance{Source: artifact.SourceCI, Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
		Digest:      "sha256:" + strings.Repeat("a", 64),
	}
}

// writeOutcomeAttestation reuses attestations_test.go's writeAttestation
// helper — the outcome attestation is the same artifact kind, keyed by
// the feature's own slug rather than a story slug (R4-I-11).
func writeOutcomeAttestation(t *testing.T, storeRoot, featureSlug, acID string) {
	t.Helper()
	writeAttestation(t, storeRoot, featureSlug, acID, testAttestation)
}

// TestFoldFeature_PerStatus is the exit criterion's "a table-driven case
// per feature-AC status including the outcome-floor bullet" — the outcome
// floor's two satisfying forms (attestation-only, automated-outcome-record)
// both proving evidenced, and an AC missing both staying pending even with
// every implementing story closed.
func TestFoldFeature_PerStatus(t *testing.T) {
	tests := []struct {
		name        string
		kinds       []artifact.EvidenceKind
		stories     []ImplementingStory
		records     []artifact.Evidence
		attest      bool
		wantStatus  Status
		wantSummary string
	}{
		{
			name:       "evidenced via attestation-only floor satisfaction",
			kinds:      []artifact.EvidenceKind{artifact.EvidenceAttestation},
			stories:    []ImplementingStory{{SpecRef: "spec/story-a", Closed: true}},
			attest:     true,
			wantStatus: StatusEvidenced,
		},
		{
			name:       "evidenced via automated-outcome-record floor satisfaction",
			kinds:      []artifact.EvidenceKind{artifact.EvidenceBehavioral, artifact.EvidenceAttestation},
			stories:    []ImplementingStory{{SpecRef: "spec/story-a", Closed: true}},
			records:    []artifact.Evidence{ciRecord(artifact.EvidenceBehavioral, artifact.VerdictPass, "ac-1")},
			attest:     false,
			wantStatus: StatusEvidenced,
		},
		{
			name:       "pending: every implementing story closed but floor missing both forms",
			kinds:      []artifact.EvidenceKind{artifact.EvidenceBehavioral, artifact.EvidenceAttestation},
			stories:    []ImplementingStory{{SpecRef: "spec/story-a", Closed: true}},
			attest:     false,
			wantStatus: StatusPending,
		},
		{
			name:       "pending: floor satisfied but stories not closed or eligible",
			kinds:      []artifact.EvidenceKind{artifact.EvidenceAttestation},
			stories:    []ImplementingStory{{SpecRef: "spec/story-a", Closed: false, Eligible: false}},
			attest:     true,
			wantStatus: StatusPending,
		},
		{
			name:       "pending: story eligible (not closed) satisfies the story-bookkeeping half",
			kinds:      []artifact.EvidenceKind{artifact.EvidenceAttestation},
			stories:    []ImplementingStory{{SpecRef: "spec/story-a", Closed: false, Eligible: true}},
			attest:     false,
			wantStatus: StatusPending,
		},
		{
			name:       "evidenced: story eligible (not closed) plus floor satisfied",
			kinds:      []artifact.EvidenceKind{artifact.EvidenceAttestation},
			stories:    []ImplementingStory{{SpecRef: "spec/story-a", Closed: false, Eligible: true}},
			attest:     true,
			wantStatus: StatusEvidenced,
		},
		{
			name:       "no-signal: no implementing story at all",
			kinds:      []artifact.EvidenceKind{artifact.EvidenceAttestation},
			stories:    nil,
			attest:     false,
			wantStatus: StatusNoSignal,
		},
		{
			name:       "no-signal wins even when a stray outcome attestation exists with no implementing story",
			kinds:      []artifact.EvidenceKind{artifact.EvidenceAttestation},
			stories:    nil,
			attest:     true,
			wantStatus: StatusNoSignal,
		},
		{
			name:       "violated: propagates from an implementing story's violated status",
			kinds:      []artifact.EvidenceKind{artifact.EvidenceAttestation},
			stories:    []ImplementingStory{{SpecRef: "spec/story-a", Closed: true, Violated: true}},
			attest:     true,
			wantStatus: StatusViolated,
		},
		{
			name:       "violated: a failing outcome-level record",
			kinds:      []artifact.EvidenceKind{artifact.EvidenceBehavioral, artifact.EvidenceAttestation},
			stories:    []ImplementingStory{{SpecRef: "spec/story-a", Closed: true}},
			records:    []artifact.Evidence{ciRecord(artifact.EvidenceBehavioral, artifact.VerdictFail, "ac-1")},
			attest:     true,
			wantStatus: StatusViolated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			featureSlug := "loan-update"
			if tc.attest {
				writeOutcomeAttestation(t, root, featureSlug, "ac-1")
			}

			spec := featureSpec(t, "ac-1", tc.kinds...)
			result, err := FoldFeature(FeatureInput{
				Spec:        spec,
				Stories:     map[string][]ImplementingStory{"ac-1": tc.stories},
				Records:     tc.records,
				StoreRoot:   root,
				FeatureSlug: featureSlug,
			})
			if err != nil {
				t.Fatalf("FoldFeature: %v", err)
			}
			if len(result.ACs) != 1 {
				t.Fatalf("ACs = %+v, want exactly 1", result.ACs)
			}
			if got := result.ACs[0].Status; got != tc.wantStatus {
				t.Fatalf("status = %s, want %s (summary=%q)", got, tc.wantStatus, result.ACs[0].Summary)
			}
			wantViolated := tc.wantStatus == StatusViolated
			if result.Violated != wantViolated {
				t.Fatalf("result.Violated = %v, want %v", result.Violated, wantViolated)
			}
		})
	}
}

// TestFoldFeature_UnauthoredOutcomeAttestation_StaysPending proves spec/
// attest-helper dc-3 at this package's own feature-fold call site
// (featurefold.go): an unauthored `verdi attest` scaffold does not satisfy
// the outcome floor (parent spec/closure-ergonomics dc-2 — "not foldable
// until authored"), so an otherwise-closed implementing story with only an
// unauthored outcome attestation and no other floor signal stays PENDING,
// never evidenced.
func TestFoldFeature_UnauthoredOutcomeAttestation_StaysPending(t *testing.T) {
	root := t.TempDir()
	featureSlug := "loan-update"
	writeAttestation(t, root, featureSlug, "ac-1", unauthoredScaffoldFixture)

	spec := featureSpec(t, "ac-1", artifact.EvidenceAttestation)
	result, err := FoldFeature(FeatureInput{
		Spec:        spec,
		Stories:     map[string][]ImplementingStory{"ac-1": {{SpecRef: "spec/story-a", Closed: true}}},
		StoreRoot:   root,
		FeatureSlug: featureSlug,
	})
	if err != nil {
		t.Fatalf("FoldFeature: %v", err)
	}
	if got := result.ACs[0].Status; got != StatusPending {
		t.Fatalf("status = %s, want pending (an unauthored scaffold does not satisfy the outcome floor, dc-3); summary=%q", got, result.ACs[0].Summary)
	}
	if !strings.Contains(result.ACs[0].Summary, "attestation:absent") {
		t.Fatalf("Summary = %q, want it to mention attestation:absent (dc-3: unauthored renders identically to absent)", result.ACs[0].Summary)
	}
}

// TestFoldFeature_Precedence proves violated beats every other status even
// when the floor is otherwise fully satisfied and every story is closed —
// 03's total precedence, violated > evidenced > pending > no-signal.
func TestFoldFeature_Precedence(t *testing.T) {
	root := t.TempDir()
	writeOutcomeAttestation(t, root, "loan-update", "ac-1")

	spec := featureSpec(t, "ac-1", artifact.EvidenceAttestation)
	result, err := FoldFeature(FeatureInput{
		Spec: spec,
		Stories: map[string][]ImplementingStory{
			"ac-1": {
				{SpecRef: "spec/story-a", Closed: true},
				{SpecRef: "spec/story-b", Closed: true, Violated: true},
			},
		},
		StoreRoot:   root,
		FeatureSlug: "loan-update",
	})
	if err != nil {
		t.Fatalf("FoldFeature: %v", err)
	}
	if result.ACs[0].Status != StatusViolated {
		t.Fatalf("status = %s, want violated (one violated implementing story marks the AC violated regardless of siblings)", result.ACs[0].Status)
	}
}

// TestFoldFeature_ImplementingStoriesDisclosed proves the AC result
// discloses which stories implement it, in the order given.
func TestFoldFeature_ImplementingStoriesDisclosed(t *testing.T) {
	spec := featureSpec(t, "ac-1", artifact.EvidenceAttestation)
	result, err := FoldFeature(FeatureInput{
		Spec: spec,
		Stories: map[string][]ImplementingStory{
			"ac-1": {{SpecRef: "spec/story-a"}, {SpecRef: "spec/story-b"}},
		},
		StoreRoot:   t.TempDir(),
		FeatureSlug: "loan-update",
	})
	if err != nil {
		t.Fatalf("FoldFeature: %v", err)
	}
	want := []string{"spec/story-a", "spec/story-b"}
	got := result.ACs[0].ImplementingStories
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ImplementingStories = %v, want %v", got, want)
	}
}

// --- Negative paths ---

func TestFoldFeature_Negative(t *testing.T) {
	t.Run("nil spec", func(t *testing.T) {
		_, err := FoldFeature(FeatureInput{StoreRoot: t.TempDir()})
		if err == nil {
			t.Fatal("FoldFeature(nil spec): want error, got nil")
		}
	})

	t.Run("wrong class", func(t *testing.T) {
		spec := featureSpec(t, "ac-1", artifact.EvidenceAttestation)
		spec.Class = artifact.ClassStory
		_, err := FoldFeature(FeatureInput{Spec: spec, StoreRoot: t.TempDir()})
		if err == nil {
			t.Fatal("FoldFeature(story-class spec): want error, got nil")
		}
	})

	t.Run("no acceptance criteria", func(t *testing.T) {
		spec := featureSpec(t, "ac-1", artifact.EvidenceAttestation)
		spec.AcceptanceCriteria = nil
		_, err := FoldFeature(FeatureInput{Spec: spec, StoreRoot: t.TempDir()})
		if err == nil {
			t.Fatal("FoldFeature(no ACs): want error, got nil")
		}
	})

	t.Run("dangling binding: record evidence_for names an unknown AC", func(t *testing.T) {
		spec := featureSpec(t, "ac-1", artifact.EvidenceAttestation)
		_, err := FoldFeature(FeatureInput{
			Spec:        spec,
			Records:     []artifact.Evidence{ciRecord(artifact.EvidenceBehavioral, artifact.VerdictPass, "ac-999")},
			StoreRoot:   t.TempDir(),
			FeatureSlug: "loan-update",
		})
		if err == nil {
			t.Fatal("FoldFeature(dangling binding): want error, got nil")
		}
		if !strings.Contains(err.Error(), "ac-999") {
			t.Fatalf("error = %q, want it to name the dangling AC id", err)
		}
	})
}

// TestFoldFeature_FloorBreakdown proves FeatureACResult.Floor projects the
// fold's OWN outcome-floor evaluation — the OR-across-signals semantics a
// disclosure consumer (spec/close-preflight) must render, never the story
// fold's AND-across-declared-kinds (ADJ-56 finding 2). It is a projection of
// the same fold that produced Status, so a floor cleared by one signal reads
// Satisfied even while the AC stays pending for another reason, and a
// violated floor carries its failing witness (finding 3).
func TestFoldFeature_FloorBreakdown(t *testing.T) {
	const slug = "loan-update"

	foldAC1 := func(t *testing.T, root string, kinds []artifact.EvidenceKind, stories []ImplementingStory, records []artifact.Evidence) FeatureACResult {
		t.Helper()
		res, err := FoldFeature(FeatureInput{
			Spec:        featureSpec(t, "ac-1", kinds...),
			Stories:     map[string][]ImplementingStory{"ac-1": stories},
			Records:     records,
			StoreRoot:   root,
			FeatureSlug: slug,
		})
		if err != nil {
			t.Fatalf("FoldFeature: %v", err)
		}
		return res.ACs[0]
	}

	t.Run("floor satisfied via one disjunct while the AC stays pending (finding 2)", func(t *testing.T) {
		got := foldAC1(t, t.TempDir(),
			[]artifact.EvidenceKind{artifact.EvidenceBehavioral, artifact.EvidenceAttestation},
			[]ImplementingStory{{SpecRef: "spec/story-a", Closed: false, Eligible: false}}, // open + ineligible -> pending
			[]artifact.Evidence{ciRecord(artifact.EvidenceBehavioral, artifact.VerdictPass, "ac-1")},
		)
		if got.Status != StatusPending {
			t.Fatalf("Status = %q, want pending (open ineligible story)", got.Status)
		}
		if !got.Floor.Satisfied {
			t.Fatal("Floor.Satisfied = false, want true (a passing behavioral outcome record clears the OR floor even with no attestation)")
		}
		if !got.Floor.DeclaresAttestation || got.Floor.Attestation != AttestationAbsent || got.Floor.Violating != nil {
			t.Fatalf("Floor = %+v, want DeclaresAttestation=true, Attestation=absent, no violation", got.Floor)
		}
	})

	t.Run("floor satisfied via authored outcome attestation", func(t *testing.T) {
		root := t.TempDir()
		writeOutcomeAttestation(t, root, slug, "ac-1")
		got := foldAC1(t, root,
			[]artifact.EvidenceKind{artifact.EvidenceBehavioral, artifact.EvidenceAttestation},
			[]ImplementingStory{{SpecRef: "spec/story-a", Closed: true}},
			nil,
		)
		if !got.Floor.Satisfied || got.Floor.Attestation != AttestationAuthored {
			t.Fatalf("Floor = %+v, want Satisfied=true, Attestation=authored", got.Floor)
		}
	})

	t.Run("floor unsatisfied names both disjuncts' state", func(t *testing.T) {
		got := foldAC1(t, t.TempDir(),
			[]artifact.EvidenceKind{artifact.EvidenceBehavioral, artifact.EvidenceAttestation},
			[]ImplementingStory{{SpecRef: "spec/story-a", Closed: true}},
			nil,
		)
		if got.Status != StatusPending {
			t.Fatalf("Status = %q, want pending (floor unmet)", got.Status)
		}
		if got.Floor.Satisfied || !got.Floor.DeclaresAttestation || got.Floor.Attestation != AttestationAbsent {
			t.Fatalf("Floor = %+v, want Satisfied=false, DeclaresAttestation=true, Attestation=absent", got.Floor)
		}
	})

	t.Run("floor violated names its failing witness (finding 3)", func(t *testing.T) {
		got := foldAC1(t, t.TempDir(),
			[]artifact.EvidenceKind{artifact.EvidenceBehavioral},
			[]ImplementingStory{{SpecRef: "spec/story-a", Closed: true}},
			[]artifact.Evidence{ciRecord(artifact.EvidenceBehavioral, artifact.VerdictFail, "ac-1")},
		)
		if got.Status != StatusViolated {
			t.Fatalf("Status = %q, want violated", got.Status)
		}
		if got.Floor.Violating == nil || got.Floor.Violating.Witness != "w" {
			t.Fatalf("Floor.Violating = %+v, want the failing outcome record named", got.Floor.Violating)
		}
	})

	t.Run("AC declaring no attestation kind reports DeclaresAttestation=false", func(t *testing.T) {
		got := foldAC1(t, t.TempDir(),
			[]artifact.EvidenceKind{artifact.EvidenceBehavioral},
			[]ImplementingStory{{SpecRef: "spec/story-a", Closed: true}},
			nil,
		)
		if got.Floor.DeclaresAttestation {
			t.Fatalf("Floor.DeclaresAttestation = true for an AC declaring only [behavioral]: %+v", got.Floor)
		}
	})
}
