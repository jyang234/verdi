package constitutionapp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/constitutionimpact"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

type countingConflictEvaluator struct {
	calls    int
	delegate ConflictEvaluator
	err      error
}

func (e *countingConflictEvaluator) Evaluate(ctx context.Context, root string, request policyconflict.Request) (ConflictEvidence, error) {
	e.calls++
	if e.err != nil {
		return ConflictEvidence{}, e.err
	}
	return e.delegate.Evaluate(ctx, root, request)
}

func commitInventory(t testing.TB, root string, consumers ...constitutionimpact.Consumer) string {
	t.Helper()
	if err := os.MkdirAll(root+"/.verdi/constitution", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root+"/"+constitutionimpact.InventoryPath, fixtureInventory(t, consumers...), 0o644); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", constitutionimpact.InventoryPath)
	runFixtureGit(t, root, "commit", "--quiet", "--no-verify", "-m", "update constitution consumers")
	return strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
}

func checkoutProposalBranch(t testing.TB, root, branch string) {
	t.Helper()
	runFixtureGit(t, root, "checkout", "--quiet", "-b", branch)
}

func hasImpactReason(coverage constitutionimpact.Coverage, code constitutionimpact.ReasonCode) bool {
	for _, reason := range coverage.Reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func TestImpactReview_RemovedConsumerRemainsInExactTreeUnion(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	acceptedHead := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
	svc := testService()
	evaluator := &countingConflictEvaluator{err: errors.New("evaluator deliberately unavailable")}
	svc.Conflict = evaluator

	changed := strings.Replace(readFixtureFile(t, root, "overlays/frontend-go-version.md"),
		`title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (removed consumer)"`, 1)
	if _, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/remove-consumer", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(changed), Expected: Expected{Branch: "policy/remove-consumer"},
	}); typed != nil {
		t.Fatalf("Propose: %v", typed)
	}
	proposedHead := commitInventory(t, root)

	review, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	wantConsumer := fixtureConsumer("spec/operand-feature", policyartifact.Scope{Phases: []string{}, Paths: []string{"cmd/"}, Refs: []string{}})
	wantIdentity, err := wantConsumer.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Coverage.Consumers) != 1 || len(review.Coverage.Evaluations) != 1 || review.Coverage.Evaluations[0].ConsumerIdentity != wantIdentity {
		t.Fatalf("removed accepted consumer was not retained exactly once: %+v", review.Coverage)
	}
	if evaluator.calls != 1 {
		t.Fatalf("evaluator calls = %d, want one for the removed union member", evaluator.calls)
	}
	if review.Coverage.Accepted.Commit != acceptedHead || review.Coverage.Proposed.Commit != proposedHead ||
		review.Coverage.Accepted.Commit != review.Accepted.Ref || review.Coverage.Proposed.Commit != review.Proposed.Ref {
		t.Fatalf("coverage/snapshot identities disagree: coverage=%+v accepted=%q proposed=%q", review.Coverage, review.Accepted.Ref, review.Proposed.Ref)
	}
	wantAcceptedTree := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", acceptedHead+"^{tree}"))
	wantProposedTree := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", proposedHead+"^{tree}"))
	if review.Coverage.Accepted.Tree != wantAcceptedTree || review.Coverage.Proposed.Tree != wantProposedTree {
		t.Fatalf("coverage tree identities = %q/%q, want %q/%q", review.Coverage.Accepted.Tree, review.Coverage.Proposed.Tree, wantAcceptedTree, wantProposedTree)
	}
}

func TestImpactReview_InventoryFailureClassification(t *testing.T) {
	t.Run("missing is disclosed unproven", func(t *testing.T) {
		root := buildFixtureRepo(t)
		checkoutProposalBranch(t, root, "policy/missing-inventory")
		if err := os.Remove(root + "/" + constitutionimpact.InventoryPath); err != nil {
			t.Fatal(err)
		}
		runFixtureGit(t, root, "add", "-A")
		runFixtureGit(t, root, "commit", "--quiet", "--no-verify", "-m", "remove constitution inventory")

		review, typed := testService().ImpactReview(context.Background(), root, ImpactReviewRequest{})
		if typed != nil {
			t.Fatalf("ImpactReview: %v", typed)
		}
		if review.Coverage.State != constitutionimpact.StateDisclosedUnproven || review.Coverage.Proposed.Presence != constitutionimpact.PresenceMissing ||
			!hasImpactReason(review.Coverage, constitutionimpact.ReasonProposedInventoryMissing) {
			t.Fatalf("missing inventory classification = %+v", review.Coverage)
		}
	})

	t.Run("malformed present bytes are operational", func(t *testing.T) {
		root := buildFixtureRepo(t)
		checkoutProposalBranch(t, root, "policy/malformed-inventory")
		if err := os.WriteFile(root+"/"+constitutionimpact.InventoryPath, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runFixtureGit(t, root, "add", constitutionimpact.InventoryPath)
		runFixtureGit(t, root, "commit", "--quiet", "--no-verify", "-m", "malform constitution inventory")

		_, typed := testService().ImpactReview(context.Background(), root, ImpactReviewRequest{})
		if typed == nil || typed.Classification != ClassificationOperational || typed.Code != "impact-evidence-invalid" {
			t.Fatalf("malformed inventory error = %+v, want operational impact-evidence-invalid", typed)
		}
	})
}

func TestImpactReview_CatalogOnlyChangeTriggersCoverage(t *testing.T) {
	root := buildFixtureRepo(t)
	checkoutProposalBranch(t, root, "policy/catalog-only")
	path := root + "/.verdi/policy/constitution.md"
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(content), "action: [make-verify]", "action: [make-lint, make-verify]", 1)
	if changed == string(content) {
		t.Fatal("constitution fixture did not contain the action catalog entry")
	}
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", ".verdi/policy/constitution.md")
	runFixtureGit(t, root, "commit", "--quiet", "--no-verify", "-m", "extend constitution action catalog")

	review, typed := testService().ImpactReview(context.Background(), root, ImpactReviewRequest{})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	if len(review.Layers) != 1 || review.Layers[0].Kind != policyartifact.KindConstitution || review.Layers[0].Change != "changed" {
		t.Fatalf("catalog-only layer delta = %+v", review.Layers)
	}
	if review.Coverage.State != constitutionimpact.StateDisclosedUnproven || !hasImpactReason(review.Coverage, constitutionimpact.ReasonConsumerUniverseEmpty) {
		t.Fatalf("catalog-only empty-universe coverage = %+v", review.Coverage)
	}
}

func TestImpactReview_DirtyCheckoutRefusesCanonicalEvaluation(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	svc := testService()
	evaluator := &countingConflictEvaluator{delegate: localConflictEvaluator{}}
	svc.Conflict = evaluator
	changed := strings.Replace(readFixtureFile(t, root, "overlays/frontend-go-version.md"),
		`title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (dirty)"`, 1)
	if _, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/dirty-impact", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(changed), Expected: Expected{Branch: "policy/dirty-impact"},
	}); typed != nil {
		t.Fatalf("Propose: %v", typed)
	}
	if err := os.WriteFile(root+"/untracked-impact-input", []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	review, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	if !review.Identity.Dirty || evaluator.calls != 0 || review.Coverage.State != constitutionimpact.StateDisclosedUnproven ||
		len(review.Coverage.Evaluations) != 1 || review.Coverage.Evaluations[0].Refusal == nil ||
		review.Coverage.Evaluations[0].Refusal.Code != constitutionimpact.ReasonEvaluationUnresolved {
		t.Fatalf("dirty-checkout coverage = %+v; evaluator calls=%d", review.Coverage, evaluator.calls)
	}
}

func TestImpactReview_OperationDistinctConsumersReuseEvidenceWithoutCollapsingRows(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	first := fixtureConsumer("spec/operand-feature", policyartifact.Scope{Phases: []string{}, Paths: []string{"cmd/"}, Refs: []string{}})
	second := first
	second.GovernedOperations = []string{}
	commitInventory(t, root, first, second)

	svc := testService()
	evaluator := &countingConflictEvaluator{delegate: localConflictEvaluator{}}
	svc.Conflict = evaluator
	changed := strings.Replace(readFixtureFile(t, root, "overlays/frontend-go-version.md"),
		`title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (shared evidence)"`, 1)
	if _, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/shared-impact-evidence", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(changed), Expected: Expected{Branch: "policy/shared-impact-evidence"},
	}); typed != nil {
		t.Fatalf("Propose: %v", typed)
	}

	review, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	if evaluator.calls != 1 {
		t.Fatalf("evaluator calls = %d, want one shared request/environment evaluation", evaluator.calls)
	}
	if len(review.Coverage.Consumers) != 2 || len(review.Coverage.Evaluations) != 2 ||
		review.Coverage.Evaluations[0].ConsumerIdentity == review.Coverage.Evaluations[1].ConsumerIdentity {
		t.Fatalf("operation-distinct rows were collapsed or omitted: %+v", review.Coverage.Evaluations)
	}
	for _, row := range review.Coverage.Evaluations {
		if row.Report == nil {
			t.Fatalf("shared evidence did not complete row %s: %+v", row.ConsumerIdentity, row)
		}
	}
}

func TestImpactReview_DistinctRequestEnvironmentDoesNotReuseEvidence(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	local := fixtureConsumer("spec/operand-feature", policyartifact.Scope{Phases: []string{}, Paths: []string{"cmd/"}, Refs: []string{}})
	production := local
	production.Request.Scope.Environments = []string{"production"}
	production.Environment = "production"
	commitInventory(t, root, local, production)

	svc := testService()
	evaluator := &countingConflictEvaluator{err: errors.New("evaluator deliberately unavailable")}
	svc.Conflict = evaluator
	changed := strings.Replace(readFixtureFile(t, root, "overlays/frontend-go-version.md"),
		`title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (distinct evidence)"`, 1)
	if _, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/distinct-impact-evidence", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(changed), Expected: Expected{Branch: "policy/distinct-impact-evidence"},
	}); typed != nil {
		t.Fatalf("Propose: %v", typed)
	}

	review, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	if evaluator.calls != 2 {
		t.Fatalf("evaluator calls = %d, want distinct calls for distinct canonical request/environment operands", evaluator.calls)
	}
	if len(review.Coverage.Evaluations) != 2 {
		t.Fatalf("coverage evaluations = %d, want two required rows", len(review.Coverage.Evaluations))
	}
}

func TestSubmitPreparation_SupplementalEvaluationCannotRepairEmptyCanonicalUniverse(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	commitInventory(t, root)
	svc := testService()
	changed := strings.Replace(readFixtureFile(t, root, "overlays/frontend-go-version.md"),
		`title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (supplemental only)"`, 1)
	if _, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/supplemental-only", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(changed), Expected: Expected{Branch: "policy/supplemental-only"},
	}); typed != nil {
		t.Fatalf("Propose: %v", typed)
	}
	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{Targets: []ImpactTarget{unprovenJudgeTarget()}})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if len(prep.ImpactReview.Conflicts) != 1 || prep.ImpactReview.Conflicts[0].Report == nil ||
		len(prep.ImpactReview.Coverage.SupplementalTargets) != 1 {
		t.Fatalf("supplemental target did not complete successfully: %+v", prep.ImpactReview)
	}
	if target := prep.ImpactReview.Coverage.SupplementalTargets[0]; target.Result == nil {
		t.Fatalf("supplemental coverage target = %+v; refusal=%+v", target, target.Refusal)
	}
	if prep.ReadyForSubmission || prep.ImpactReview.Coverage.State != constitutionimpact.StateDisclosedUnproven ||
		!hasImpactReason(prep.ImpactReview.Coverage, constitutionimpact.ReasonConsumerUniverseEmpty) {
		t.Fatalf("supplemental result repaired canonical coverage: %+v", prep)
	}
}

type canceledConflictEvaluator struct{}

func (canceledConflictEvaluator) Evaluate(context.Context, string, policyconflict.Request) (ConflictEvidence, error) {
	return ConflictEvidence{}, context.Canceled
}

func TestImpactReview_ContextCancellationIsOperationalNotCoverageEvidence(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	svc := testService()
	svc.Conflict = canceledConflictEvaluator{}
	changed := strings.Replace(readFixtureFile(t, root, "overlays/frontend-go-version.md"),
		`title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (canceled)"`, 1)
	if _, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/canceled-impact", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(changed), Expected: Expected{Branch: "policy/canceled-impact"},
	}); typed != nil {
		t.Fatalf("Propose: %v", typed)
	}

	_, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{})
	if typed == nil || typed.Classification != ClassificationOperational || typed.Code != "evaluation-canceled" || !errors.Is(typed, context.Canceled) {
		t.Fatalf("cancellation error = %+v, want operational evaluation-canceled wrapping context.Canceled", typed)
	}
}

func TestImpactReview_RefusesStaleHead(t *testing.T) {
	root := buildFixtureRepo(t)
	calls := 0
	svc := testService()
	svc.Git = headShiftingGitReader{GitReader: svc.Git, calls: &calls, shiftAt: 2}

	_, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{})
	if typed == nil || typed.Classification != ClassificationOperational || typed.Code != "identity-shifted" {
		t.Fatalf("stale-head error = %+v, want operational identity-shifted", typed)
	}
}

func TestExactTreeAt_PreservesExecutableModeAndRefusesInventorySymlink(t *testing.T) {
	root := buildFixtureRepo(t)
	checkoutProposalBranch(t, root, "policy/exact-tree-modes")
	inventoryPath := root + "/" + constitutionimpact.InventoryPath
	if err := os.Chmod(inventoryPath, 0o755); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", constitutionimpact.InventoryPath)
	runFixtureGit(t, root, "commit", "--quiet", "--no-verify", "-m", "make inventory executable")
	head := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))

	exact, err := testService().exactTreeAt(context.Background(), root, head)
	if err != nil {
		t.Fatalf("exactTreeAt executable inventory: %v", err)
	}
	wantTree := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", head+"^{tree}"))
	if exact.Commit != head || exact.Tree != wantTree {
		t.Fatalf("exact identities = %q/%q, want %q/%q", exact.Commit, exact.Tree, head, wantTree)
	}
	info, err := fs.Stat(exact.FS, constitutionimpact.InventoryPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("exact inventory mode = %04o, want Git executable mode 0755", got)
	}

	if err := os.Remove(inventoryPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../policy/constitution.md", inventoryPath); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", "-A")
	runFixtureGit(t, root, "commit", "--quiet", "--no-verify", "-m", "replace inventory with symlink")
	symlinkHead := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
	if _, err := testService().exactTreeAt(context.Background(), root, symlinkHead); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("exactTreeAt inventory symlink error = %v", err)
	}
}

type invalidTreeOIDGitReader struct{ GitReader }

func (r invalidTreeOIDGitReader) RevParse(ctx context.Context, root, rev string) (string, error) {
	if strings.HasSuffix(rev, "^{tree}") {
		return "not-a-full-object-id", nil
	}
	return r.GitReader.RevParse(ctx, root, rev)
}

func TestExactTreeAt_ValidatesTreeIdentity(t *testing.T) {
	root := buildFixtureRepo(t)
	head := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
	svc := testService()
	svc.Git = invalidTreeOIDGitReader{GitReader: svc.Git}
	if _, err := svc.exactTreeAt(context.Background(), root, head); err == nil || !strings.Contains(err.Error(), "exact tree identity") {
		t.Fatalf("invalid tree identity error = %v", err)
	}
}
