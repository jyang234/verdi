package policyconflict

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/repositoryfacts"
)

// SI-257 (as amended), lane E4c: the conflict evaluator's reverification of
// an accepted context compares its expected branch with the branch being
// closed resolved from the sealed pair (Repository, CIRef), never from
// Repository.Branch alone. The acceptance-candidate arm is unchanged.

// ciEnvKeys is every CI-provider variable the repository-facts gather may
// read, plus GITHUB_HEAD_REF, which it never reads: each test clears them
// all so a host CI's environment cannot leak into a row.
var ciEnvKeys = []string{
	"GITHUB_ACTIONS", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "GITHUB_HEAD_REF",
	"GITLAB_CI", "CI_COMMIT_REF_NAME", "CI_COMMIT_BRANCH", "CI_COMMIT_TAG",
}

func setReverifyCIEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, key := range ciEnvKeys {
		t.Setenv(key, "")
	}
	for key, value := range env {
		t.Setenv(key, value)
	}
}

func githubRunEnv(branch string) map[string]string {
	return map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "branch", "GITHUB_REF_NAME": branch}
}

// acceptedWithExpected returns a copy of request whose accepted context
// asserts expected (nil for none); request itself is never modified.
func acceptedWithExpected(request Request, expected *contextcompile.Expected) Request {
	accepted := *request.Target.AcceptedContext
	accepted.Expected = expected
	return Request{Schema: request.Schema, Target: Target{Kind: TargetAcceptedContext, AcceptedContext: &accepted}}
}

func TestReverifyConflictViewAcceptedBranchBeingClosed(t *testing.T) {
	const closeBranch = "close/operand-feature"
	setReverifyCIEnv(t, nil)
	repo := operandFixtureRepo(t)
	compileRequest := acceptedOperandRequest("spec/operand-feature")
	base := Request{Schema: RequestSchema, Target: Target{Kind: TargetAcceptedContext, AcceptedContext: &compileRequest}}
	operands, err := ResolveOperands(context.Background(), contextcompile.NewCompiler(), repo.Dir, base, contextcompile.ConflictFacts{})
	if err != nil {
		t.Fatalf("ResolveOperands: %v", err)
	}
	sealed, err := operands.View()
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if !sealed.Snapshot.Repository.Branch.Known || sealed.Snapshot.Repository.Branch.Value != "main" || sealed.Snapshot.CIRef != nil {
		t.Fatalf("test setup: sealed snapshot = branch %+v ci ref %v, want checked-out main and no CI ref", sealed.Snapshot.Repository.Branch, sealed.Snapshot.CIRef)
	}

	detached := repositoryfacts.StringFact{}
	ciRef := func(f repositoryfacts.CIRefFact) *repositoryfacts.CIRefFact { return &f }
	known := func(name string) *repositoryfacts.CIRefFact {
		return ciRef(repositoryfacts.CIRefFact{Known: true, Provider: repositoryfacts.CIProviderGitHub, Name: name})
	}
	expect := func(branch, head string) *contextcompile.Expected {
		return &contextcompile.Expected{Branch: branch, Head: head}
	}
	otherHead := strings.Repeat("0", 40)
	tests := []struct {
		name     string
		branch   repositoryfacts.StringFact
		ciRef    *repositoryfacts.CIRefFact
		expected *contextcompile.Expected
		wantOK   bool
	}{
		{name: "checked-out branch equals expected (control)", branch: sealed.Snapshot.Repository.Branch, expected: expect("main", repo.Head), wantOK: true},
		{name: "checked-out branch differs from expected", branch: sealed.Snapshot.Repository.Branch, expected: expect(closeBranch, repo.Head)},
		{name: "checked-out branch wins over a sealed CI ref", branch: sealed.Snapshot.Repository.Branch, ciRef: known(closeBranch), expected: expect(closeBranch, repo.Head)},
		{name: "detached, sealed CI ref equals expected", branch: detached, ciRef: known(closeBranch), expected: expect(closeBranch, repo.Head), wantOK: true},
		{name: "detached, sealed CI ref names another branch", branch: detached, ciRef: known("close/other"), expected: expect(closeBranch, repo.Head)},
		{name: "detached, no sealed CI ref", branch: detached, expected: expect(closeBranch, repo.Head)},
		{name: "detached, sealed CI ref unknown", branch: detached, ciRef: ciRef(repositoryfacts.CIRefFact{Reason: repositoryfacts.CIRefReasonRemoteTrackingUnresolved}), expected: expect(closeBranch, repo.Head)},
		{name: "detached, sealed CI ref invalid", branch: detached, ciRef: known("close~1"), expected: expect("close~1", repo.Head)},
		{name: "detached, CI ref matches but HEAD differs", branch: detached, ciRef: known(closeBranch), expected: expect(closeBranch, otherHead)},
		{name: "detached, no expectation", branch: detached, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := sealed // the returned View is caller-owned; mutating it is safe
			view.Snapshot.Repository.Branch = tt.branch
			view.Snapshot.CIRef = tt.ciRef
			err := reverifyConflictView(view, acceptedWithExpected(base, tt.expected))
			if tt.wantOK && err != nil {
				t.Fatalf("reverifyConflictView: %v", err)
			}
			if !tt.wantOK && (err == nil || !strings.Contains(err.Error(), "accepted expected repository identity does not match sealed snapshot")) {
				t.Fatalf("reverifyConflictView error = %v, want the accepted expected-repository refusal", err)
			}
		})
	}
}

// TestReverifyConflictViewCandidateArmIgnoresCIRef: the design-phase
// candidate arm still compares its mandatory expected branch with the
// recorded Repository.Branch only, so a CI ref never satisfies it.
func TestReverifyConflictViewCandidateArmIgnoresCIRef(t *testing.T) {
	setReverifyCIEnv(t, nil)
	files := operandPolicyStoreFiles(t)
	files[".verdi/specs/active/operand-candidate/spec.md"] = operandCandidateSpec
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "candidate"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	request := Request{
		Schema: RequestSchema,
		Target: Target{Kind: TargetAcceptanceCandidate, AcceptanceCandidate: &AcceptanceCandidate{
			Adapter:  contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Expected: contextcompile.Expected{Branch: "main", Head: repo.Head},
			Grants:   execworkspace.GrantSet{}, Scope: universalScope(), Spec: "spec/operand-candidate",
		}},
	}
	operands, err := ResolveOperands(context.Background(), contextcompile.NewCompiler(), repo.Dir, request, contextcompile.ConflictFacts{})
	if err != nil {
		t.Fatalf("ResolveOperands: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if err := reverifyConflictView(view, request); err != nil {
		t.Fatalf("reverifyConflictView(control): %v", err)
	}
	view.Snapshot.Repository.Branch = repositoryfacts.StringFact{}
	view.Snapshot.CIRef = &repositoryfacts.CIRefFact{Known: true, Provider: repositoryfacts.CIProviderGitHub, Name: "main"}
	err = reverifyConflictView(view, request)
	if err == nil || !strings.Contains(err.Error(), "candidate expected repository identity does not match sealed snapshot") {
		t.Fatalf("reverifyConflictView error = %v, want the candidate expected-repository refusal", err)
	}
}

// TestServiceEvaluateDetachedCheckoutWithValidatedCIRef re-creates lane
// E4a's gap witness (logs-e4a/gap-witness-policyconflict-reverify.log) as a
// committed test on the path readinessload.NewConflictProvider builds: a
// policyconflict Service over contextcompile.NewCompiler(), whose real
// repository-facts gather reads the process environment and a real
// checkout. The control runs on the checked-out default branch; the CI rows
// run on a detached checkout of the same commit, as the CI close does.
//
// It also records the report each passing run produces, so the lane report
// can state, as evidence, what the detached run's verdict and disclosures
// are, and whether repository-branch-unknown (which contextcompile folds
// branch-detached into) makes the verdict blocking.
func TestServiceEvaluateDetachedCheckoutWithValidatedCIRef(t *testing.T) {
	const closeBranch = "close/operand-feature"
	type outcome struct {
		verdict     Verdict
		disclosures []string
		report      Report
	}
	run := func(t *testing.T, phase contextcompile.Phase, detach, tracking bool, env map[string]string, expectedBranch string) (outcome, error) {
		t.Helper()
		setReverifyCIEnv(t, nil)
		repo, request, actors, _, _ := serviceDispositionRepo(t)
		if tracking {
			runOperandGit(t, repo.Dir, "update-ref", "refs/remotes/origin/"+closeBranch, repo.Head)
		}
		if detach {
			runOperandGit(t, repo.Dir, "checkout", "--quiet", "--detach", "HEAD")
		}
		setReverifyCIEnv(t, env)
		service := NewService(repo.Dir, ServiceDeps{
			Compiler: contextcompile.NewCompiler(), Refs: noCallResolver{t: t}, Primary: serviceNoConflictJudge(),
			Dates: serviceDateSource("2026-08-12"), Actors: actors,
		})
		request = acceptedWithExpected(request, &contextcompile.Expected{Branch: expectedBranch, Head: repo.Head})
		request.Target.AcceptedContext.Phase = phase
		result, err := service.Evaluate(context.Background(), request)
		if err != nil {
			return outcome{}, err
		}
		if _, err := DecodeReport(result.ReportBytes); err != nil {
			t.Fatalf("DecodeReport(result bytes): %v", err)
		}
		codes := make([]string, len(result.Report.Disclosures))
		for i, d := range result.Report.Disclosures {
			codes[i] = string(d.Code)
		}
		sort.Strings(codes)
		return outcome{verdict: result.Report.Verdict, disclosures: codes, report: result.Report}, nil
	}

	control, err := run(t, contextcompile.PhaseDesign, false, true, nil, "main")
	if err != nil {
		t.Fatalf("control (checked-out main): Evaluate: %v", err)
	}
	t.Logf("EVIDENCE control (checked-out main): verdict=%s disclosures=%v", control.verdict, control.disclosures)
	if control.report.Input.CIRef != nil || !control.report.Input.Repository.Branch.Known {
		t.Fatalf("control input = branch %+v ci ref %v, want the checked-out branch and no CI ref", control.report.Input.Repository.Branch, control.report.Input.CIRef)
	}

	t.Run("detached with a validated CI ref equal to the expected branch", func(t *testing.T) {
		got, err := run(t, contextcompile.PhaseDesign, true, true, githubRunEnv(closeBranch), closeBranch)
		if err != nil {
			t.Fatalf("Evaluate: %v (the E4a gap: reverification must accept the sealed CI ref)", err)
		}
		t.Logf("EVIDENCE detached + validated CI ref: verdict=%s disclosures=%v input.repository.branch=%+v input.ci_ref=%+v",
			got.verdict, got.disclosures, got.report.Input.Repository.Branch, *got.report.Input.CIRef)
		if want := (repositoryfacts.CIRefFact{Known: true, Provider: repositoryfacts.CIProviderGitHub, Name: closeBranch}); got.report.Input.CIRef == nil || *got.report.Input.CIRef != want {
			t.Fatalf("report input.ci_ref = %v, want %+v", got.report.Input.CIRef, want)
		}
		if got.report.Input.Repository.Branch.Known {
			t.Fatalf("report input.repository.branch = %+v, want unknown: the detached checkout stays recorded", got.report.Input.Repository.Branch)
		}
		if !containsString(got.disclosures, string(contextcompile.DisclosureRepositoryBranchUnknown)) {
			t.Fatalf("disclosures = %v, want %q for the detached checkout", got.disclosures, contextcompile.DisclosureRepositoryBranchUnknown)
		}
		if blockingCompilerDisclosure(contextcompile.DisclosureRepositoryBranchUnknown) {
			t.Fatalf("%q classifies as blocking", contextcompile.DisclosureRepositoryBranchUnknown)
		}
		if got.verdict != control.verdict {
			t.Fatalf("detached verdict = %s, control verdict = %s: the detached state changed the verdict", got.verdict, control.verdict)
		}
		if extra := stringsMinus(got.disclosures, control.disclosures); len(extra) != 1 || extra[0] != string(contextcompile.DisclosureRepositoryBranchUnknown) {
			t.Fatalf("disclosures beyond the control = %v, want exactly [%s]", extra, contextcompile.DisclosureRepositoryBranchUnknown)
		}
	})

	for _, tt := range []struct {
		name     string
		tracking bool
		env      map[string]string
	}{
		{name: "detached with the remote-tracking ref missing (CI ref unknown)", tracking: false, env: githubRunEnv(closeBranch)},
		{name: "detached outside CI", tracking: true, env: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := run(t, contextcompile.PhaseDesign, true, tt.tracking, tt.env, closeBranch)
			if err == nil || !IsOperational(err) || !errors.Is(err, contextcompile.ErrExpectedRepositoryMismatch) {
				t.Fatalf("Evaluate error = %v, want the operational expected-repository refusal", err)
			}
		})
	}

	// The CI close evaluates a review-phase request. Its compile adds the
	// review-stage disclosures whatever the checkout's state, so the pair
	// below compares a detached review-phase run with its own checked-out
	// control: the detached state may add only repository-branch-unknown and
	// must leave the verdict unchanged.
	t.Run("review phase: detached with a validated CI ref against its control", func(t *testing.T) {
		reviewControl, err := run(t, contextcompile.PhaseReview, false, true, nil, "main")
		if err != nil {
			t.Fatalf("review control: Evaluate: %v", err)
		}
		got, err := run(t, contextcompile.PhaseReview, true, true, githubRunEnv(closeBranch), closeBranch)
		if err != nil {
			t.Fatalf("review detached: Evaluate: %v", err)
		}
		t.Logf("EVIDENCE review control (checked-out main): verdict=%s disclosures=%v", reviewControl.verdict, reviewControl.disclosures)
		t.Logf("EVIDENCE review detached + validated CI ref: verdict=%s disclosures=%v", got.verdict, got.disclosures)
		if got.verdict != reviewControl.verdict {
			t.Fatalf("review detached verdict = %s, control = %s: the detached state changed the verdict", got.verdict, reviewControl.verdict)
		}
		if extra := stringsMinus(got.disclosures, reviewControl.disclosures); len(extra) != 1 || extra[0] != string(contextcompile.DisclosureRepositoryBranchUnknown) {
			t.Fatalf("review disclosures beyond the control = %v, want exactly [%s]", extra, contextcompile.DisclosureRepositoryBranchUnknown)
		}
	})
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// stringsMinus returns the values of a that b does not contain.
func stringsMinus(a, b []string) []string {
	var out []string
	for _, v := range a {
		if !containsString(b, v) {
			out = append(out, v)
		}
	}
	return out
}
