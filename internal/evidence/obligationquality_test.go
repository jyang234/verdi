package evidence

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

type qualityGitStub struct {
	answers map[[2]string]bool
	err     error
}

func (g qualityGitStub) IsAncestor(_ context.Context, _ string, ancestor, descendant string) (bool, error) {
	if g.err != nil {
		return false, g.err
	}
	return g.answers[[2]string{ancestor, descendant}], nil
}

func TestClassifyObligationEvaluation(t *testing.T) {
	const before = "1111111111111111111111111111111111111111"
	const after = "2222222222222222222222222222222222222222"
	tests := []struct {
		name string
		eval string
		git  qualityGitStub
		want ObligationEvaluationClass
	}{
		{"equal", ObligationQualityAdoptionCommit, qualityGitStub{}, ObligationEvaluationHistorical},
		{"ancestor", before, qualityGitStub{answers: map[[2]string]bool{{before, ObligationQualityAdoptionCommit}: true}}, ObligationEvaluationHistorical},
		{"after", after, qualityGitStub{answers: map[[2]string]bool{{ObligationQualityAdoptionCommit, after}: true}}, ObligationEvaluationPostAdoption},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClassifyObligationEvaluation(context.Background(), tt.git, t.TempDir(), tt.eval)
			if err != nil {
				t.Fatalf("ClassifyObligationEvaluation: %v", err)
			}
			if got != tt.want {
				t.Fatalf("class = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyObligationEvaluation_Negative(t *testing.T) {
	tests := []struct {
		name string
		git  qualityGitStub
	}{
		{"divergent", qualityGitStub{answers: map[[2]string]bool{}}},
		{"git failure", qualityGitStub{err: errors.New("ancestry unavailable")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ClassifyObligationEvaluation(context.Background(), tt.git, t.TempDir(), "3333333333333333333333333333333333333333"); err == nil {
				t.Fatal("ClassifyObligationEvaluation = nil error, want operational refusal")
			}
		})
	}
}

func TestAssessObligation_StructuralStates(t *testing.T) {
	tests := []struct {
		name    string
		quality string
		body    string
		write   bool
		want    ObligationStructuralState
	}{
		{"missing", "", "", false, ObligationMissing},
		{"legacy", "", "authored prose", true, ObligationLegacyUnelaborated},
		{"legacy marker", "", UnauthoredObligationMarker, true, ObligationUnresolvedDesignDebt},
		{"explicit unresolved", "quality:\n  state: unresolved-design-debt\n", "authored prose", true, ObligationUnresolvedDesignDebt},
		{"elaborated", elaboratedQualityYAML(), "authored prose", true, ObligationElaborated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.write {
				writeQualityObligation(t, root, artifact.EvidenceBehavioral, tt.quality, tt.body)
			}
			got, err := AssessObligation(context.Background(), ObligationAssessmentInput{
				StoreRoot: root, SpecName: "loan-refi", ACID: "ac-2", Kind: artifact.EvidenceBehavioral,
			})
			if err != nil {
				t.Fatalf("AssessObligation: %v", err)
			}
			if got.StructuralState != tt.want {
				t.Fatalf("StructuralState = %q, want %q", got.StructuralState, tt.want)
			}
			wantPath := filepath.ToSlash(filepath.Join(".verdi", "obligations", "loan-refi", "ac-2--behavioral.md"))
			if got.WitnessPath != wantPath {
				t.Fatalf("WitnessPath = %q, want %q", got.WitnessPath, wantPath)
			}
		})
	}
}

func TestAssessObligation_ExactMatching(t *testing.T) {
	root := t.TempDir()
	writeQualityObligation(t, root, artifact.EvidenceBehavioral, elaboratedQualityYAML(), "authored")
	base := artifact.Evidence{
		Kind: artifact.EvidenceBehavioral, Verdict: artifact.VerdictPass,
		Producer:   "verify:behavioral",
		Provenance: artifact.EvidenceProvenance{Source: artifact.SourceCI, Job: "verify", Commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	tests := []struct {
		name   string
		mutate func(*artifact.Evidence)
		git    qualityGitStub
		want   ObligationMatchState
		reason ObligationMatchReason
	}{
		{"matched", func(*artifact.Evidence) {}, qualityGitStub{answers: map[[2]string]bool{{"9999999999999999999999999999999999999999", base.Provenance.Commit}: true}}, ObligationMatched, ""},
		{"producer missing", func(e *artifact.Evidence) { e.Producer = "" }, qualityGitStub{}, ObligationUnproven, ObligationReasonProducerMissing},
		{"producer mismatch", func(e *artifact.Evidence) { e.Producer = "other" }, qualityGitStub{}, ObligationUnproven, ObligationReasonProducerMismatch},
		{"source mismatch", func(e *artifact.Evidence) { e.Provenance.Source = artifact.SourceLocal }, qualityGitStub{}, ObligationUnproven, ObligationReasonSourceMismatch},
		{"source ref missing", func(e *artifact.Evidence) { e.Provenance.Job = "" }, qualityGitStub{}, ObligationUnproven, ObligationReasonSourceRefMissing},
		{"source ref mismatch", func(e *artifact.Evidence) { e.Provenance.Job = "other" }, qualityGitStub{}, ObligationUnproven, ObligationReasonSourceRefMismatch},
		{"code stale", func(e *artifact.Evidence) { e.Provenance.Commit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" }, qualityGitStub{}, ObligationUnproven, ObligationReasonFreshnessStale},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := base
			tt.mutate(&record)
			got, err := AssessObligation(context.Background(), ObligationAssessmentInput{
				StoreRoot: root, SpecName: "loan-refi", ACID: "ac-2", Kind: artifact.EvidenceBehavioral,
				Record: &record, EvaluationCommit: base.Provenance.Commit,
				SpecLandingCommit: "9999999999999999999999999999999999999999", Git: tt.git,
			})
			if err != nil {
				t.Fatalf("AssessObligation: %v", err)
			}
			if got.MatchState != tt.want || got.Reason != tt.reason {
				t.Fatalf("match = %q/%q, want %q/%q", got.MatchState, got.Reason, tt.want, tt.reason)
			}
		})
	}
}

func TestAssessObligation_CodeEvaluationCommitUnavailableIsOperational(t *testing.T) {
	root := t.TempDir()
	writeQualityObligation(t, root, artifact.EvidenceBehavioral, elaboratedQualityYAML(), "authored")
	record := artifact.Evidence{
		Kind: artifact.EvidenceBehavioral, Verdict: artifact.VerdictPass,
		Producer:   "verify:behavioral",
		Provenance: artifact.EvidenceProvenance{Source: artifact.SourceCI, Job: "verify", Commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	_, err := AssessObligation(context.Background(), ObligationAssessmentInput{
		StoreRoot: root, SpecName: "loan-refi", ACID: "ac-2", Kind: artifact.EvidenceBehavioral,
		Record: &record, SpecLandingCommit: record.Provenance.Commit,
	})
	if err == nil {
		t.Fatal("AssessObligation = nil error, want unavailable code evaluation commit to be operational")
	}
}

func TestAssessObligation_PreservesFailingWitness(t *testing.T) {
	record := artifact.Evidence{Kind: artifact.EvidenceBehavioral, Verdict: artifact.VerdictFail, Witness: "failure witness"}
	got, err := AssessObligation(context.Background(), ObligationAssessmentInput{
		StoreRoot: t.TempDir(), SpecName: "loan-refi", ACID: "ac-2", Kind: artifact.EvidenceBehavioral, Record: &record,
	})
	if err != nil {
		t.Fatalf("AssessObligation: %v", err)
	}
	if got.StructuralState != ObligationMissing || got.MatchState != ObligationViolatedWithWitness || got.Violating == nil || got.Violating.Witness != "failure witness" {
		t.Fatalf("assessment = %+v, want missing plus preserved violation", got)
	}
}

func TestAssessObligation_AttestationAndUnprovableFreshness(t *testing.T) {
	tests := []struct {
		name       string
		kind       artifact.EvidenceKind
		quality    string
		wantReason ObligationMatchReason
	}{
		{"attestation identity unavailable", artifact.EvidenceAttestation, attestationQualityYAML(), ObligationReasonSourceRefMissing},
		{"dependency receipt unavailable", artifact.EvidenceBehavioral, dependencyQualityYAML(), ObligationReasonFreshnessUnproven},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeQualityObligation(t, root, tt.kind, tt.quality, "authored")
			record := artifact.Evidence{Kind: tt.kind, Verdict: artifact.VerdictPass, Producer: "verify:behavioral", Provenance: artifact.EvidenceProvenance{Source: artifact.SourceCI, Job: "verify", Commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
			got, err := AssessObligation(context.Background(), ObligationAssessmentInput{StoreRoot: root, SpecName: "loan-refi", ACID: "ac-2", Kind: tt.kind, Record: &record, EvaluationCommit: record.Provenance.Commit})
			if err != nil {
				t.Fatalf("AssessObligation: %v", err)
			}
			if got.MatchState != ObligationUnproven || got.Reason != tt.wantReason {
				t.Fatalf("match = %q/%q, want unproven/%q", got.MatchState, got.Reason, tt.wantReason)
			}
		})
	}
}

func writeQualityObligation(t *testing.T, root string, kind artifact.EvidenceKind, quality, body string) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "obligations", "loan-refi", "ac-2--"+string(kind)+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "---\n" + obligationYAMLForQuality(kind, quality) + "---\n# Foo\n\n" + body + "\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func obligationYAMLForQuality(kind artifact.EvidenceKind, quality string) string {
	return "id: obligation/loan-refi--ac-2--" + string(kind) + "\nkind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: " + string(kind) + "\n" + quality + "links:\n  - { type: verifies, ref: \"spec/loan-refi\" }\nfrozen: { at: 2026-07-13, commit: 3e91ab2 }\n"
}

func elaboratedQualityYAML() string {
	return "quality:\n  state: elaborated\n  claim: claim\n  falsifier: falsifier\n  scope: scope\n  producer: { kind: checker, ref: \"verify:behavioral\" }\n  authoritative_source: { kind: ci-job, ref: \"verify\" }\n  freshness:\n    invalidated_by: [spec, code]\n    rule: rerun\n"
}

func attestationQualityYAML() string {
	return "quality:\n  state: elaborated\n  claim: claim\n  falsifier: falsifier\n  scope: scope\n  producer: { kind: authenticated-human, ref: \"role:owner\" }\n  authoritative_source: { kind: governed-attestation, ref: \"approval:owner\" }\n  freshness:\n    invalidated_by: [spec]\n    rule: rerun\n"
}

func dependencyQualityYAML() string {
	return "quality:\n  state: elaborated\n  claim: claim\n  falsifier: falsifier\n  scope: scope\n  producer: { kind: checker, ref: \"verify:behavioral\" }\n  authoritative_source: { kind: ci-job, ref: \"verify\" }\n  freshness:\n    invalidated_by: [dependency]\n    rule: rerun\n"
}
