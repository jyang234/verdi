package evidence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

const qualityBeforeCommit = "1111111111111111111111111111111111111111"
const qualityAfterCommit = "2222222222222222222222222222222222222222"
const qualityLandingCommit = "9999999999999999999999999999999999999999"

func TestFold_ObligationQualityProspectiveSemantics(t *testing.T) {
	tests := []struct {
		name       string
		quality    string
		write      bool
		evaluation string
		git        qualityGitStub
		verdict    artifact.EvidenceVerdict
		want       Status
		state      ObligationStructuralState
		match      ObligationMatchState
	}{
		{"historical legacy pass keeps incumbent behavior", "", true, qualityBeforeCommit, qualityGitStub{answers: map[[2]string]bool{{qualityBeforeCommit, ObligationQualityAdoptionCommit}: true}}, artifact.VerdictPass, StatusEvidenced, ObligationLegacyUnelaborated, ObligationUnproven},
		{"post-adoption proof cannot borrow historical legacy meaning", "", true, qualityBeforeCommit, qualityGitStub{answers: map[[2]string]bool{{qualityBeforeCommit, ObligationQualityAdoptionCommit}: true, {ObligationQualityAdoptionCommit, qualityAfterCommit}: true}}, artifact.VerdictPass, StatusPending, ObligationLegacyUnelaborated, ObligationUnproven},
		{"post-adoption legacy pass is pending", "", true, qualityAfterCommit, qualityGitStub{answers: map[[2]string]bool{{ObligationQualityAdoptionCommit, qualityAfterCommit}: true}}, artifact.VerdictPass, StatusPending, ObligationLegacyUnelaborated, ObligationUnproven},
		{"post-adoption missing pass is pending", "", false, qualityAfterCommit, qualityGitStub{}, artifact.VerdictPass, StatusPending, ObligationMissing, ObligationUnproven},
		{"explicit unresolved is pending even historically", "quality:\n  state: unresolved-design-debt\n", true, qualityBeforeCommit, qualityGitStub{}, artifact.VerdictPass, StatusPending, ObligationUnresolvedDesignDebt, ObligationUnproven},
		{"elaborated exact pass evidences", foldQualityBlock("verify:static", "verify", "[code]"), true, qualityAfterCommit, qualityGitStub{}, artifact.VerdictPass, StatusEvidenced, ObligationElaborated, ObligationMatched},
		{"elaborated producer mismatch is pending", foldQualityBlock("different", "verify", "[code]"), true, qualityAfterCommit, qualityGitStub{}, artifact.VerdictPass, StatusPending, ObligationElaborated, ObligationUnproven},
		{"missing obligation cannot hide failure", "", false, qualityAfterCommit, qualityGitStub{}, artifact.VerdictFail, StatusViolated, ObligationMissing, ObligationViolatedWithWitness},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.write {
				writeFoldQualityObligation(t, root, artifact.EvidenceStatic, tt.quality)
			}
			recordCommit := tt.evaluation
			if tt.name == "post-adoption proof cannot borrow historical legacy meaning" {
				recordCommit = qualityAfterCommit
			}
			record := testEvidence(artifact.EvidenceStatic, tt.verdict, "ac-1",
				withProducer("verify:static"), withJob("verify"), withCommit(recordCommit))
			result, err := Fold(Input{
				Spec: testSpec("jira:TEST-1", ac("ac-1", artifact.EvidenceStatic)), Records: []artifact.Evidence{record},
				StoreRoot: root, StorySlug: "test-1", EvaluationCommit: tt.evaluation,
				SpecLandingCommit: qualityLandingCommit, Git: tt.git,
			})
			if err != nil {
				t.Fatalf("Fold: %v", err)
			}
			got := result.ACs[0]
			if got.Status != tt.want {
				t.Fatalf("status = %q, want %q; result=%+v", got.Status, tt.want, got)
			}
			q := got.Kinds[0].ObligationQuality
			if q.StructuralState != tt.state || q.MatchState != tt.match {
				t.Fatalf("quality = %+v, want state/match %q/%q", q, tt.state, tt.match)
			}
			if q.WitnessPath != ".verdi/obligations/test-story/ac-1--static.md" {
				t.Fatalf("witness = %q", q.WitnessPath)
			}
			if tt.verdict == artifact.VerdictFail && got.Summary != "static:fail" {
				t.Fatalf("failing summary = %q, want incumbent failing witness summary", got.Summary)
			}
		})
	}
}

func TestFold_ObligationQualityWaiverPrecedence(t *testing.T) {
	root := t.TempDir()
	writeWaiver(t, root, "test-1", "ac-1", testActiveWaiver)
	record := testEvidence(artifact.EvidenceStatic, artifact.VerdictFail, "ac-1")
	result, err := Fold(Input{
		Spec: testSpec("jira:TEST-1", ac("ac-1", artifact.EvidenceStatic)), Records: []artifact.Evidence{record},
		StoreRoot: root, StorySlug: "test-1", EvaluationCommit: qualityAfterCommit,
	})
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	if result.ACs[0].Status != StatusWaived {
		t.Fatalf("status = %q, want waived", result.ACs[0].Status)
	}
}

func TestFold_ObligationQualityMisbindingIsOperational(t *testing.T) {
	root := t.TempDir()
	writeFoldQualityObligation(t, root, artifact.EvidenceStatic, foldQualityBlock("verify:static", "verify", "[code]"))
	path := filepath.Join(root, ".verdi", "obligations", "test-story", "ac-1--static.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "obligation/test-story--", "obligation/other-story--", 1))
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	record := testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-1",
		withProducer("verify:static"), withJob("verify"), withCommit(qualityAfterCommit))

	result, err := Fold(Input{
		Spec: testSpec("jira:TEST-1", ac("ac-1", artifact.EvidenceStatic)), Records: []artifact.Evidence{record},
		StoreRoot: root, StorySlug: "test-1", EvaluationCommit: qualityAfterCommit,
		SpecLandingCommit: qualityLandingCommit, Git: qualityGitStub{},
	})
	if err == nil {
		t.Fatalf("Fold = %+v, nil error; want operational misbinding refusal", result)
	}
	if result.Eligible {
		t.Fatalf("Fold result = %+v, misbound obligation authorized the story", result)
	}
}

func TestFold_ObligationQualityAttestationAndFreshnessRemainUnproven(t *testing.T) {
	tests := []struct {
		name    string
		kind    artifact.EvidenceKind
		quality string
		reason  ObligationMatchReason
	}{
		{"attestation identity receipt absent", artifact.EvidenceAttestation, attestationQualityYAML(), ObligationReasonSourceRefMissing},
		{"dependency receipt absent", artifact.EvidenceStatic, foldQualityBlock("verify:static", "verify", "[dependency]"), ObligationReasonFreshnessUnproven},
		{"environment receipt absent", artifact.EvidenceStatic, foldQualityBlock("verify:static", "verify", "[environment]"), ObligationReasonFreshnessUnproven},
		{"policy receipt absent", artifact.EvidenceStatic, foldQualityBlock("verify:static", "verify", "[policy]"), ObligationReasonFreshnessUnproven},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeFoldQualityObligation(t, root, tt.kind, tt.quality)
			if tt.kind == artifact.EvidenceAttestation {
				writeAttestation(t, root, "test-1", "ac-1", testAttestation)
			}
			record := testEvidence(tt.kind, artifact.VerdictPass, "ac-1", withProducer("verify:static"), withJob("verify"), withCommit(qualityAfterCommit))
			result, err := Fold(Input{Spec: testSpec("jira:TEST-1", ac("ac-1", tt.kind)), Records: []artifact.Evidence{record}, StoreRoot: root, StorySlug: "test-1", EvaluationCommit: qualityAfterCommit})
			if err != nil {
				t.Fatalf("Fold: %v", err)
			}
			q := result.ACs[0].Kinds[0].ObligationQuality
			if result.ACs[0].Status != StatusPending || q.MatchState != ObligationUnproven || q.Reason != tt.reason {
				t.Fatalf("result = %+v, want pending unproven/%s", result.ACs[0], tt.reason)
			}
		})
	}
}

func TestFold_ObligationQualityPreviewAndOperationalFailures(t *testing.T) {
	t.Run("preview displays same debt", func(t *testing.T) {
		root := t.TempDir()
		writeFoldQualityObligation(t, root, artifact.EvidenceStatic, "quality:\n  state: unresolved-design-debt\n")
		record := testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-1", withSource(artifact.SourceLocal))
		result, err := Fold(Input{Spec: testSpec("jira:TEST-1", ac("ac-1", artifact.EvidenceStatic)), Records: []artifact.Evidence{record}, Preview: true, StoreRoot: root, StorySlug: "test-1", EvaluationCommit: qualityAfterCommit})
		if err != nil {
			t.Fatalf("Fold: %v", err)
		}
		if result.ACs[0].Status != StatusPending || !strings.Contains(result.ACs[0].Summary, "obligation-quality:unresolved-design-debt") {
			t.Fatalf("preview result = %+v", result.ACs[0])
		}
	})

	t.Run("malformed obligation is operational", func(t *testing.T) {
		root := t.TempDir()
		writeFoldQualityObligation(t, root, artifact.EvidenceStatic, "quality:\n  state: unresolved-design-debt\n  unknown: true\n")
		_, err := Fold(Input{Spec: testSpec("jira:TEST-1", ac("ac-1", artifact.EvidenceStatic)), StoreRoot: root, StorySlug: "test-1", EvaluationCommit: qualityAfterCommit})
		if err == nil {
			t.Fatal("Fold = nil error, want malformed obligation operational")
		}
	})

	t.Run("legacy ancestry failure is operational", func(t *testing.T) {
		root := t.TempDir()
		writeFoldQualityObligation(t, root, artifact.EvidenceStatic, "")
		_, err := Fold(Input{Spec: testSpec("jira:TEST-1", ac("ac-1", artifact.EvidenceStatic)), StoreRoot: root, StorySlug: "test-1", EvaluationCommit: qualityAfterCommit, Git: qualityGitStub{err: context.Canceled}})
		if err == nil {
			t.Fatal("Fold = nil error, want ancestry failure operational")
		}
	})
}

func writeFoldQualityObligation(t *testing.T, root string, kind artifact.EvidenceKind, quality string) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "obligations", "test-story", "ac-1--"+string(kind)+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "---\nid: obligation/test-story--ac-1--" + string(kind) + "\nkind: obligation\ntitle: Quality\nowners: [platform-team]\nfor_kind: " + string(kind) + "\n" + quality + "links:\n  - { type: verifies, ref: \"spec/test-story\" }\nfrozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }\n---\n# Quality\n\nAuthored.\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func foldQualityBlock(producer, job, invalidators string) string {
	return "quality:\n  state: elaborated\n  claim: claim\n  falsifier: falsifier\n  scope: scope\n  producer: { kind: checker, ref: \"" + producer + "\" }\n  authoritative_source: { kind: ci-job, ref: \"" + job + "\" }\n  freshness:\n    invalidated_by: " + invalidators + "\n    rule: rerun\n"
}
