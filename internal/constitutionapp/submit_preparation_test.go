package constitutionapp

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// unprovenJudgeTarget is the one declared target buildConflictFixtureRepo
// resolves: mechanically clean, but requiring a semantic evaluation this
// package wires no judge for, so policyconflict itself returns
// VerdictBlockedUnproven/ReasonJudgeUnavailable.
func unprovenJudgeTarget() ImpactTarget {
	return ImpactTarget{
		Spec:    "spec/operand-feature",
		Phase:   contextcompile.PhaseDesign,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{"cmd/"}, Refs: []string{}},
	}
}

func TestSubmitPreparation_BlockedOnProvenConflict(t *testing.T) {
	root := buildUnauthorizedExemptionFixtureRepo(t)
	svc := testService()

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{
		Targets: []ImpactTarget{{
			Spec:    "spec/operand-feature",
			Phase:   contextcompile.PhaseDesign,
			Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		}},
	})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if prep.ReadyForSubmission {
		t.Fatal("expected ReadyForSubmission == false when a target has a mechanically proven conflict")
	}
	if len(prep.BlockingReasons) == 0 {
		t.Fatal("expected at least one disclosed blocking reason")
	}
	if !prep.Validation.Snapshot.Adopted {
		t.Fatal("the proposal itself should still validate cleanly")
	}
}

// TestSubmitPreparation_BlockedOnUnprovenJudge pins SI-178's chosen
// semantics (c): every VerdictBlockedUnproven target maps to
// ready_for_submission: false and is named in blocking_reasons with its own
// policyconflict reason code. The kernel's vocabulary calls this state
// blocking (DC-6/DC-7); the packet must never read affirmatively clean over
// it. Merge and approval stay outside this operation either way — a human
// still acts on the packet's complete witnesses through the normal
// pull-request review.
func TestSubmitPreparation_BlockedOnUnprovenJudge(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	svc := testService()

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{
		Targets: []ImpactTarget{unprovenJudgeTarget()},
	})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if prep.ReadyForSubmission {
		t.Fatal("expected ReadyForSubmission == false for a target whose conflict verdict is blocked-unproven")
	}
	joined := strings.Join(prep.BlockingReasons, "\n")
	if !strings.Contains(joined, "spec/operand-feature") {
		t.Fatalf("expected the unproven target to be named in blocking_reasons, got %v", prep.BlockingReasons)
	}
	if !strings.Contains(joined, string(policyconflict.ReasonJudgeUnavailable)) {
		t.Fatalf("expected the policyconflict reason %q in blocking_reasons, got %v", policyconflict.ReasonJudgeUnavailable, prep.BlockingReasons)
	}
}

// TestSubmitPreparation_UnprovenTargetsDisclosedOnPacket proves the
// packet-level disclosure SI-178(c) requires: a MECHANICALLY CLEAN proposal
// whose semantic evaluation is merely unproven still cannot serialize a
// record that reads affirmatively clean — the versioned record itself names
// every unproven target and reports ready_for_submission: false.
func TestSubmitPreparation_UnprovenTargetsDisclosedOnPacket(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	svc := testService()

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{
		Targets: []ImpactTarget{unprovenJudgeTarget()},
	})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}

	// Precondition: nothing is mechanically proven-violated here, so the
	// ONLY thing standing between this packet and a clean reading is the
	// unproven semantic evaluation.
	report := prep.ImpactReview.Conflicts[0].Report
	if report == nil {
		t.Fatal("expected a completed conflict report")
	}
	if report.Verdict != policyconflict.VerdictBlockedUnproven {
		t.Fatalf("verdict = %q, want %q", report.Verdict, policyconflict.VerdictBlockedUnproven)
	}
	for _, row := range report.Mechanical {
		if row.State == policyconflict.ProofViolatedWithWitness {
			t.Fatalf("this fixture must be mechanically clean, got a violated row: %+v", row)
		}
	}

	record, err := EncodeResult(prep)
	if err != nil {
		t.Fatalf("EncodeResult: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(record, &decoded); err != nil {
		t.Fatalf("decoding the packet record: %v", err)
	}
	if decoded["ready_for_submission"] != false {
		t.Fatalf("ready_for_submission = %v, want false", decoded["ready_for_submission"])
	}
	if got, want := decoded["unproven_targets"], []any{"spec/operand-feature"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unproven_targets = %#v, want %#v — the versioned record must name every unproven target", got, want)
	}
}

// TestSubmitPreparation_BlockedWhenNothingIsAdopted proves the honest shape
// M-6 asks for: Validate carries no constant-true "proven" field, so the
// one non-clean state a clean Validate result can still describe — a store
// that has adopted no constitution at all (or adopted one incompletely) —
// is what blocks preparation. There is nothing to submit, and the packet
// says so rather than reporting an affirmatively ready empty store.
func TestSubmitPreparation_BlockedWhenNothingIsAdopted(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"README.md": "no constitution here\n"}, Message: "init"},
	})
	svc := testService()

	prep, typed := svc.SubmitPreparation(context.Background(), repo.Dir, SubmitPreparationRequest{})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if prep.ReadyForSubmission {
		t.Fatal("expected ReadyForSubmission == false over a store that has adopted no constitution")
	}
	if !strings.Contains(strings.Join(prep.BlockingReasons, "\n"), "adopted no constitution") {
		t.Fatalf("expected a disclosed not-adopted blocking reason, got %v", prep.BlockingReasons)
	}
	if prep.Validation.Snapshot.Reason == "" {
		t.Fatal("expected the underlying not-adopted reason to stay visible on the validation snapshot")
	}
}

// headShiftingGitReader is a real gitxReader whose nth RevParse("HEAD") call
// reports a DIFFERENT commit — the observable shape of a checkout that moves
// (another process committing, switching, or resetting) between
// SubmitPreparation's two identity resolutions.
type headShiftingGitReader struct {
	GitReader
	calls   *int
	shiftAt int
}

func (r headShiftingGitReader) RevParse(ctx context.Context, root, rev string) (string, error) {
	if rev == "HEAD" {
		*r.calls++
		if *r.calls == r.shiftAt {
			return "0000000000000000000000000000000000000000", nil
		}
	}
	return r.GitReader.RevParse(ctx, root, rev)
}

// TestSubmitPreparation_RefusesWhenCheckoutMovesMidOperation proves the
// packet is never composed from two DIFFERENT repository states: Validate
// and ImpactReview each resolve identity, and a disagreement between them
// is a typed operational refusal, never a packet silently stapling one
// commit's validation onto another commit's impact review.
func TestSubmitPreparation_RefusesWhenCheckoutMovesMidOperation(t *testing.T) {
	root := buildFixtureRepo(t)
	calls := 0
	svc := testService()
	svc.Git = headShiftingGitReader{GitReader: svc.Git, calls: &calls, shiftAt: 2}

	_, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{})
	if typed == nil {
		t.Fatal("expected a refusal when the checkout moves between the two identity resolutions")
	}
	if typed.Classification != ClassificationOperational || typed.Code != "identity-shifted" {
		t.Fatalf("expected operational/identity-shifted, got %+v", typed)
	}
}

func TestSubmitPreparation_InputInvalid(t *testing.T) {
	svc := testService()
	if _, typed := svc.SubmitPreparation(context.Background(), "", SubmitPreparationRequest{}); typed == nil || typed.Classification != ClassificationVerdict {
		t.Fatalf("expected verdict for missing root, got %+v", typed)
	}
}
