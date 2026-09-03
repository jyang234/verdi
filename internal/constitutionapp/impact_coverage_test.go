package constitutionapp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/constitutionimpact"
	"github.com/jyang234/verdi/internal/gitx"
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

// transientCheckoutMutationEvaluator substitutes malformed policy bytes in
// the serving checkout for exactly the duration of each evaluation. An impact
// review bound to the proposed commit is unaffected; one evaluated at the
// mutable serving checkout consumes the substituted bytes.
type transientCheckoutMutationEvaluator struct {
	servingRoot string
	roots       []string
}

func (e *transientCheckoutMutationEvaluator) Evaluate(ctx context.Context, root string, request policyconflict.Request) (ConflictEvidence, error) {
	e.roots = append(e.roots, root)
	path := e.servingRoot + "/.verdi/policy/constitution.md"
	original, err := os.ReadFile(path)
	if err != nil {
		return ConflictEvidence{}, err
	}
	if err := os.WriteFile(path, []byte("transient malformed constitution\n"), 0o644); err != nil {
		return ConflictEvidence{}, err
	}
	evidence, evaluateErr := (localConflictEvaluator{}).Evaluate(ctx, root, request)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		return ConflictEvidence{}, err
	}
	return evidence, evaluateErr
}

type recordingCanceledConflictEvaluator struct {
	roots []string
}

func (e *recordingCanceledConflictEvaluator) Evaluate(_ context.Context, root string, _ policyconflict.Request) (ConflictEvidence, error) {
	e.roots = append(e.roots, root)
	return ConflictEvidence{}, context.Canceled
}

type partialEvaluationCheckoutGitReader struct {
	GitReader
	path         string
	removeCalled bool
	removeCtxErr error
}

func (r *partialEvaluationCheckoutGitReader) WorktreeAddDetached(_ context.Context, _, path, _ string) error {
	r.path = path
	if err := os.Mkdir(path, 0o755); err != nil {
		return err
	}
	return context.Canceled
}

func (r *partialEvaluationCheckoutGitReader) WorktreeRemove(ctx context.Context, _, path string) error {
	r.removeCalled = true
	r.removeCtxErr = ctx.Err()
	return os.Remove(path)
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

func TestImpactReview_EvaluatesRegisteredAndSupplementalEvidenceAtProposedCommit(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	svc := testService()
	evaluator := &transientCheckoutMutationEvaluator{servingRoot: root}
	svc.Conflict = evaluator
	proposeChangedPolicy(t, root, svc, "policy/exact-evaluation", KindPolicy, "go-toolchain", "policies/go-toolchain.md", "Go toolchain policy", "Go toolchain policy (exact evaluation)")

	review, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{Targets: []ImpactTarget{unprovenJudgeTarget()}})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	if len(evaluator.roots) != 2 {
		t.Fatalf("evaluation calls = %d, want one registered and one supplemental call", len(evaluator.roots))
	}
	for _, evaluationRoot := range evaluator.roots {
		if evaluationRoot == root {
			t.Fatalf("conflict evidence was evaluated at mutable serving checkout %q", evaluationRoot)
		}
		if _, err := os.Stat(evaluationRoot); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("exact evaluation checkout %q was not removed after use: %v", evaluationRoot, err)
		}
	}
	if len(review.Coverage.Evaluations) != 1 || review.Coverage.Evaluations[0].Report == nil {
		t.Fatalf("registered evaluation consumed transient serving-checkout substitution: %+v", review.Coverage.Evaluations)
	}
	if len(review.Conflicts) != 1 || review.Conflicts[0].Report == nil {
		t.Fatalf("supplemental evaluation consumed transient serving-checkout substitution: %+v", review.Conflicts)
	}
}

func TestImpactReview_RemovesExactEvaluationCheckoutAfterCancellation(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	svc := testService()
	evaluator := &recordingCanceledConflictEvaluator{}
	svc.Conflict = evaluator
	proposeChangedPolicy(t, root, svc, "policy/canceled-exact-evaluation", KindPolicy, "go-toolchain", "policies/go-toolchain.md", "Go toolchain policy", "Go toolchain policy (canceled exact evaluation)")

	_, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{})
	if typed == nil || typed.Classification != ClassificationOperational || !errors.Is(typed, context.Canceled) {
		t.Fatalf("canceled exact evaluation error = %+v, want wrapped operational context.Canceled", typed)
	}
	if len(evaluator.roots) != 1 || evaluator.roots[0] == root {
		t.Fatalf("canceled evaluation roots = %q, want one private exact-tree checkout", evaluator.roots)
	}
	if _, err := os.Stat(evaluator.roots[0]); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("exact evaluation checkout %q was not removed after cancellation: %v", evaluator.roots[0], err)
	}
}

func TestMaterializeEvaluationCheckout_CleansPartialAddAfterCancellation(t *testing.T) {
	git := &partialEvaluationCheckoutGitReader{GitReader: testService().Git}
	svc := testService()
	svc.Git = git
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.materializeEvaluationCheckout(ctx, t.TempDir(), strings.Repeat("a", 40))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("materialize canceled partial checkout error = %v, want context.Canceled", err)
	}
	if !git.removeCalled || git.removeCtxErr != nil {
		t.Fatalf("partial checkout cleanup called=%v context error=%v, want uncanceled owner cleanup", git.removeCalled, git.removeCtxErr)
	}
	if _, statErr := os.Stat(filepath.Dir(git.path)); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("partial exact evaluation checkout parent %q was not removed: %v", filepath.Dir(git.path), statErr)
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

type unavailableExactTreeGitReader struct {
	GitReader
	ref            string
	path           string
	unavailable    error
	materializeErr error
}

func (r unavailableExactTreeGitReader) LsTreeEntriesIncludingTrees(ctx context.Context, root, ref string) ([]gitx.TreeEntry, error) {
	if ref == r.ref && r.path == "" {
		return nil, r.unavailable
	}
	return r.GitReader.LsTreeEntriesIncludingTrees(ctx, root, ref)
}

func (r unavailableExactTreeGitReader) Show(ctx context.Context, root, ref, path string) ([]byte, error) {
	if ref == r.ref && path == r.path {
		return nil, r.unavailable
	}
	return r.GitReader.Show(ctx, root, ref, path)
}

func (r unavailableExactTreeGitReader) WorktreeAddDetached(ctx context.Context, root, path, commit string) error {
	if r.materializeErr != nil {
		return r.materializeErr
	}
	return r.GitReader.WorktreeAddDetached(ctx, root, path, commit)
}

func TestSubmitPreparation_ExactTreeReadUnavailabilityIsDisclosedAndBlocking(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "enumeration unavailable"},
		{name: "inventory blob unavailable", path: constitutionimpact.InventoryPath},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := buildConflictFixtureRepo(t)
			svc := testService()
			proposeChangedPolicy(t, root, svc, "policy/unavailable-exact-tree", KindPolicy, "go-toolchain", "policies/go-toolchain.md", "Go toolchain policy", "Go toolchain policy (unavailable exact tree)")
			proposedHead := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
			svc.Git = unavailableExactTreeGitReader{
				GitReader: svc.Git, ref: proposedHead, path: test.path,
				unavailable:    errors.New("exact tree deliberately unavailable"),
				materializeErr: errors.New("unavailable tree must not be materialized"),
			}

			prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{Targets: []ImpactTarget{unprovenJudgeTarget()}})
			if typed != nil {
				t.Fatalf("SubmitPreparation classified exact-tree unavailability as operational: %v", typed)
			}
			if prep.ReadyForSubmission || prep.ImpactReview.Coverage.State != constitutionimpact.StateDisclosedUnproven ||
				prep.ImpactReview.Coverage.Proposed.Presence != constitutionimpact.PresenceUnavailable ||
				!hasImpactReason(prep.ImpactReview.Coverage, constitutionimpact.ReasonProposedTreeUnavailable) {
				t.Fatalf("exact-tree unavailability did not remain disclosed and blocking: %+v", prep)
			}
		})
	}
}

func TestImpactReview_ExactTreeCancellationRemainsOperational(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "enumeration canceled"},
		{name: "blob read canceled", path: constitutionimpact.InventoryPath},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := buildConflictFixtureRepo(t)
			svc := testService()
			proposeChangedPolicy(t, root, svc, "policy/canceled-exact-tree", KindPolicy, "go-toolchain", "policies/go-toolchain.md", "Go toolchain policy", "Go toolchain policy (canceled exact tree)")
			proposedHead := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
			svc.Git = unavailableExactTreeGitReader{
				GitReader: svc.Git, ref: proposedHead, path: test.path, unavailable: context.Canceled,
			}

			_, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{})
			if typed == nil || typed.Classification != ClassificationOperational || !errors.Is(typed, context.Canceled) {
				t.Fatalf("exact-tree cancellation error = %+v, want wrapped operational context.Canceled", typed)
			}
		})
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

func TestImpactReview_RefusesEveryUnknownConstitutionSiblingKind(t *testing.T) {
	for _, test := range []struct {
		name     string
		wantPath string
		stage    func(testing.TB, string)
	}{
		{
			name:     "regular file",
			wantPath: ".verdi/constitution/unknown.json",
			stage: func(t testing.TB, root string) {
				t.Helper()
				if err := os.WriteFile(root+"/.verdi/constitution/unknown.json", []byte("{}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				runFixtureGit(t, root, "add", ".verdi/constitution/unknown.json")
			},
		},
		{
			name:     "symlink",
			wantPath: ".verdi/constitution/unknown",
			stage: func(t testing.TB, root string) {
				t.Helper()
				if err := os.Symlink("../policy/constitution.md", root+"/.verdi/constitution/unknown"); err != nil {
					t.Fatal(err)
				}
				runFixtureGit(t, root, "add", ".verdi/constitution/unknown")
			},
		},
		{
			name:     "tree",
			wantPath: ".verdi/constitution/unknown",
			stage: func(t testing.TB, root string) {
				t.Helper()
				if err := os.MkdirAll(root+"/.verdi/constitution/unknown", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(root+"/.verdi/constitution/unknown/child", []byte("unknown\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				runFixtureGit(t, root, "add", ".verdi/constitution/unknown/child")
			},
		},
		{
			name:     "gitlink",
			wantPath: ".verdi/constitution/unknown",
			stage: func(t testing.TB, root string) {
				t.Helper()
				head := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
				runFixtureGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+head+",.verdi/constitution/unknown")
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := buildFixtureRepo(t)
			checkoutProposalBranch(t, root, "policy/unknown-constitution-"+strings.ReplaceAll(test.name, " ", "-"))
			test.stage(t, root)
			runFixtureGit(t, root, "commit", "--quiet", "--no-verify", "-m", "add unknown constitution sibling")

			_, typed := testService().ImpactReview(context.Background(), root, ImpactReviewRequest{})
			if typed == nil || typed.Classification != ClassificationOperational ||
				!strings.HasSuffix(typed.Error(), test.wantPath) {
				t.Fatalf("unknown constitution %s error = %+v, want operational closed-grammar refusal", test.name, typed)
			}
		})
	}
}
