package journey

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
)

func TestBlockerValidateRegisteredBuildStartActionOnly(t *testing.T) {
	b := validRecord(t).Blockers.Current[0]
	b.Transition = "build:start"
	if err := b.validate("blocker"); err != nil {
		t.Fatalf("build:start action identity rejected: %v", err)
	}

	for _, invalid := range []string{"build:stop", "close:now", "build:start:again"} {
		t.Run(invalid, func(t *testing.T) {
			b.Transition = invalid
			if err := b.validate("blocker"); err == nil {
				t.Fatalf("transition %q accepted, want only registered build:start action identity", invalid)
			}
		})
	}
}

func TestBlockersValidateObligationQualityDeclarationOrder(t *testing.T) {
	owner := testOwner()
	blocker := func(id string, reason ReasonCode, class BlockerClass, transition string) Blocker {
		return Blocker{ID: id, Reason: reason, Class: class, Witnesses: []string{"w"}, Owner: owner, ClearingCondition: "clear", Transition: transition}
	}
	qualityA := blocker("obligation-quality/ac-2/runtime", ReasonObligationDesignUnresolved, ClassMechanical, "build:start")
	qualityB := blocker("obligation-quality/ac-1/behavioral", ReasonObligationDesignUnresolved, ClassMechanical, "build:start")

	tests := []struct {
		name    string
		current []Blocker
		wantErr string
	}{
		{
			name: "declaration-ordered quality group is valid",
			current: []Blocker{
				blocker("lifecycle-state-unproven/unknown", ReasonLifecycleStateUnproven, ClassUnknown, "unknown"),
				qualityA,
				qualityB,
				blocker("principal-resolution-unproven/close", ReasonPrincipalResolutionUnproven, ClassGovernance, "close"),
			},
		},
		{
			name: "split quality group is invalid",
			current: []Blocker{
				qualityA,
				blocker("principal-resolution-unproven/close", ReasonPrincipalResolutionUnproven, ClassGovernance, "close"),
				qualityB,
			},
			wantErr: "contiguous",
		},
		{
			name: "unsorted non-quality ids remain invalid",
			current: []Blocker{
				blocker("principal-resolution-unproven/close", ReasonPrincipalResolutionUnproven, ClassGovernance, "close"),
				qualityA,
				blocker("lifecycle-state-unproven/unknown", ReasonLifecycleStateUnproven, ClassUnknown, "unknown"),
			},
			wantErr: "ordered",
		},
		{
			name:    "duplicate quality id is invalid",
			current: []Blocker{qualityA, qualityA},
			wantErr: "duplicate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := Blockers{Current: tt.current, Eventual: deriveEventual()}
			err := bs.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validate error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestDeriveObligationQualityBlockers(t *testing.T) {
	owner := testOwner()
	facts := []ObligationQualityFact{
		{ACID: "ac-2", Kind: artifact.EvidenceRuntime, Assessment: evidence.ObligationAssessment{StructuralState: evidence.ObligationUnresolvedDesignDebt, WitnessPath: ".verdi/obligations/story/ac-2--runtime.md"}},
		{ACID: "ac-2", Kind: artifact.EvidenceStatic, Assessment: evidence.ObligationAssessment{StructuralState: evidence.ObligationMissing, WitnessPath: ".verdi/obligations/story/ac-2--static.md"}},
		{ACID: "ac-1", Kind: artifact.EvidenceBehavioral, Assessment: evidence.ObligationAssessment{StructuralState: evidence.ObligationElaborated, MatchState: evidence.ObligationUnproven, Reason: evidence.ObligationReasonProducerMismatch, WitnessPath: ".verdi/obligations/story/ac-1--behavioral.md"}},
		{ACID: "ac-1", Kind: artifact.EvidenceStatic, Assessment: evidence.ObligationAssessment{StructuralState: evidence.ObligationElaborated, MatchState: evidence.ObligationUnproven, Reason: evidence.ObligationReasonProducerMissing, WitnessPath: ".verdi/obligations/story/ac-1--static.md"}},
		{ACID: "ac-3", Kind: artifact.EvidenceStatic, Assessment: evidence.ObligationAssessment{StructuralState: evidence.ObligationElaborated, MatchState: evidence.ObligationMatched, WitnessPath: ".verdi/obligations/story/ac-3--static.md"}},
	}

	got := deriveObligationQualityBlockers(facts, owner)
	wantIDs := []string{
		"obligation-quality/ac-2/runtime",
		"obligation-quality/ac-2/static",
		"obligation-quality/ac-1/behavioral",
		"obligation-quality/ac-1/static",
	}
	if ids := blockerIDs(got); !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("ids = %v, want declaration order %v", ids, wantIDs)
	}
	for _, b := range got {
		if b.Reason != ReasonObligationDesignUnresolved || b.Class != ClassMechanical || b.Transition != "build:start" {
			t.Fatalf("blocker = %+v, want exact reason/class/action", b)
		}
		if !reflect.DeepEqual(b.Owner, owner) || len(b.Witnesses) != 1 || b.Witnesses[0] == "" {
			t.Fatalf("blocker owner/witness = %+v", b)
		}
	}
}

func TestMergeObligationQualityBlockersPreservesQualityGroupOrder(t *testing.T) {
	owner := testOwner()
	base := []Blocker{
		{ID: "forge-facts-unavailable/merge", Reason: ReasonForgeFactsUnavailable, Class: ClassExternalWait, Witnesses: []string{"w"}, Owner: owner, ClearingCondition: "c", Transition: "merge"},
		{ID: "obligation-unknown-kind/close/x/y", Reason: ReasonObligationUnknownKind, Class: ClassUnknown, Witnesses: []string{"w"}, Owner: owner, ClearingCondition: "c", Transition: "close"},
	}
	quality := deriveObligationQualityBlockers([]ObligationQualityFact{
		{ACID: "ac-2", Kind: artifact.EvidenceRuntime, Assessment: evidence.ObligationAssessment{StructuralState: evidence.ObligationMissing, WitnessPath: "a"}},
		{ACID: "ac-1", Kind: artifact.EvidenceStatic, Assessment: evidence.ObligationAssessment{StructuralState: evidence.ObligationMissing, WitnessPath: "b"}},
	}, owner)
	got := mergeObligationQualityBlockers(base, quality)
	want := []string{
		"forge-facts-unavailable/merge",
		"obligation-quality/ac-2/runtime",
		"obligation-quality/ac-1/static",
		"obligation-unknown-kind/close/x/y",
	}
	if ids := blockerIDs(got); !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestEvidenceObligationQualityReader(t *testing.T) {
	root := t.TempDir()
	writeJourneyQualitySpec(t, root)
	writeJourneyQualityObligation(t, root, "ac-2", artifact.EvidenceRuntime, "quality:\n  state: unresolved-design-debt\n")
	writeJourneyQualityObligation(t, root, "ac-1", artifact.EvidenceBehavioral, "")

	reader := evidenceObligationQualityReader{}
	facts, err := reader.Assess(context.Background(), root, ".verdi/specs/active/quality-story/spec.md", "story", "", "", "")
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	wantPairs := []string{"ac-2/runtime", "ac-2/static", "ac-1/behavioral"}
	var gotPairs []string
	for _, fact := range facts {
		gotPairs = append(gotPairs, fact.ACID+"/"+string(fact.Kind))
	}
	if !reflect.DeepEqual(gotPairs, wantPairs) {
		t.Fatalf("pairs = %v, want declaration order %v", gotPairs, wantPairs)
	}
	wantStates := []evidence.ObligationStructuralState{evidence.ObligationUnresolvedDesignDebt, evidence.ObligationMissing, evidence.ObligationLegacyUnelaborated}
	for i := range facts {
		if facts[i].Assessment.StructuralState != wantStates[i] {
			t.Fatalf("facts[%d] state = %q, want %q", i, facts[i].Assessment.StructuralState, wantStates[i])
		}
	}
}

func TestEvidenceObligationQualityReaderMalformedIsOperational(t *testing.T) {
	root := t.TempDir()
	writeJourneyQualitySpec(t, root)
	writeJourneyQualityObligation(t, root, "ac-2", artifact.EvidenceRuntime, "quality:\n  state: unresolved-design-debt\n  unknown: true\n")
	_, err := (evidenceObligationQualityReader{}).Assess(context.Background(), root, ".verdi/specs/active/quality-story/spec.md", "story", "", "", "")
	if err == nil {
		t.Fatal("Assess = nil error, want malformed obligation operational")
	}
}

func writeJourneyQualitySpec(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "specs", "active", "quality-story", "spec.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
id: spec/quality-story
kind: spec
class: story
title: Quality story
owners: [platform-team]
story: jira:QUALITY-1
problem: { text: p, anchor: "#problem" }
outcome: { text: o, anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/feature#ac-1" }
acceptance_criteria:
  - { id: ac-2, text: second, evidence: [runtime, static] }
  - { id: ac-1, text: first, evidence: [behavioral] }
---
# Quality story
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJourneyQualityObligation(t *testing.T, root, acID string, kind artifact.EvidenceKind, quality string) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "obligations", "quality-story", acID+"--"+string(kind)+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "---\nid: obligation/quality-story--" + acID + "--" + string(kind) + "\nkind: obligation\ntitle: Quality\nowners: [platform-team]\nfor_kind: " + string(kind) + "\n" + quality + "links:\n  - { type: verifies, ref: \"spec/quality-story\" }\nfrozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }\n---\n# Quality\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}
