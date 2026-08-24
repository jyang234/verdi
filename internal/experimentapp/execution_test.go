package experimentapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentpolicy"
	"github.com/jyang234/verdi/internal/experimentrun"
)

type recordedExecutionCall struct {
	request       experimentrun.StartRequest
	authorization experimentrun.ExecutionAuthorization
}

type recordingExecutionRunner struct {
	starts  []recordedExecutionCall
	resumes []recordedExecutionCall
	verdict experiment.Verdict
	err     error
	write   bool
}

func (r *recordingExecutionRunner) Start(_ context.Context, request experimentrun.StartRequest, authorization experimentrun.ExecutionAuthorization) (experimentrun.StartResult, error) {
	r.starts = append(r.starts, recordedExecutionCall{request: request, authorization: cloneExecutionAuthorization(authorization)})
	return r.result(request)
}

func (r *recordingExecutionRunner) Resume(_ context.Context, request experimentrun.ResumeRequest, authorization experimentrun.ExecutionAuthorization) (experimentrun.StartResult, error) {
	start := experimentrun.StartRequest(request)
	r.resumes = append(r.resumes, recordedExecutionCall{request: start, authorization: cloneExecutionAuthorization(authorization)})
	return r.result(start)
}

func (r *recordingExecutionRunner) result(request experimentrun.StartRequest) (experimentrun.StartResult, error) {
	if r.err != nil {
		return experimentrun.StartResult{}, r.err
	}
	if r.write {
		name := filepath.Join(request.Root, filepath.FromSlash(request.ExperimentDir), "runs", request.Run, "machine-evidence.txt")
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return experimentrun.StartResult{}, err
		}
		if err := os.WriteFile(name, []byte("machine evidence\n"), 0o600); err != nil {
			return experimentrun.StartResult{}, err
		}
	}
	digest, err := experiment.DefinitionDigest(request.Definition)
	if err != nil {
		return experimentrun.StartResult{}, err
	}
	result := testExecutionResult(testedVerdict(r.verdict), request.Run, digest)
	resultDigest, err := experiment.ResultDigest(result)
	if err != nil {
		return experimentrun.StartResult{}, err
	}
	return experimentrun.StartResult{
		Receipt: experiment.ExecutionReceipt{Run: request.Run}, Result: result, ResultDigest: resultDigest,
	}, nil
}

func testedVerdict(verdict experiment.Verdict) experiment.Verdict {
	if verdict == "" {
		return experiment.VerdictProvenWinner
	}
	return verdict
}

func testExecutionResult(verdict experiment.Verdict, run, definitionDigest string) experiment.Result {
	decision := experiment.ResultDecision{
		Experiment: "request-path-v2", DefinitionDigest: definitionDigest, Run: run,
		Algorithm: experiment.AlgorithmV1, Verdict: verdict,
		Candidates: []experiment.DecisionCandidate{
			{ID: "baseline", Baseline: true, Eligible: true, ExecutionFailures: []experiment.CandidateExecutionFailure{}},
			{ID: "cache", Eligible: true, ExecutionFailures: []experiment.CandidateExecutionFailure{}},
		},
		ObservationsDigest: rawDigest([]byte("observations")),
	}
	if verdict == experiment.VerdictProvenWinner {
		decision.Winner = "cache"
	} else {
		decision.Reasons = []experiment.Reason{{Code: experiment.ReasonPracticalTie, Candidate: "cache", Detail: "registered evidence does not separate the candidates"}}
	}
	result, err := experiment.NewResultV2(decision, experiment.ResultExecution{
		ExecutionDigest: rawDigest([]byte("receipt")),
		Isolation: experiment.ResultIsolation{
			Network:     experiment.ReceiptNetwork{Mode: experiment.NetworkDeny, Configured: true, Reason: "test default deny"},
			Disclosures: []experiment.IsolationDisclosure{},
		},
		WarmupDiagnostics: experiment.WarmupDiagnostics{
			Authority: experiment.WarmupAuthorityNonDecisionDiagnostic,
			Scope:     experiment.WarmupScopeFinalInvocation, Failures: []experiment.WarmupFailure{},
		},
	})
	if err != nil {
		panic(err)
	}
	return result
}

func registeredExecutionService(t *testing.T, runner ExecutionRunner) (string, *Service, *fakePolicyResolver, Identity) {
	t.Helper()
	root, service := mutationTestService(t)
	identity := testIdentity(t, root, "request-path-v2")
	identity.Actor = authenticatedHuman(t)
	review := service.ReviewRegistration(context.Background(), identity)
	if review.Outcome.Classification != ClassificationClean {
		t.Fatalf("ReviewRegistration() outcome = %+v", review.Outcome)
	}
	proposal := service.ProposeRegistration(context.Background(), identity, RegistrationInput{ReviewPacketDigest: review.PacketDigest})
	if proposal.Outcome.Classification != ClassificationClean {
		t.Fatalf("ProposeRegistration() outcome = %+v", proposal.Outcome)
	}
	policy := service.policy.(*fakePolicyResolver)
	for index, request := range policy.requests {
		if request.AcceptedCommit != "" {
			t.Fatalf("proposal policy request %d accepted commit = %q, want empty", index, request.AcceptedCommit)
		}
	}
	service.git = gitFromExperimentDir(t, root, "request-path-v2")
	service.runner = runner
	policy.calls = 0
	policy.requests = nil
	identity.Actor = testActor(t)
	return root, service, policy, identity
}

func TestStartResumeUseAcceptedRegistrationCommitAuthorizationAndExactRun(t *testing.T) {
	runner := &recordingExecutionRunner{write: true}
	root, service, policy, identity := registeredExecutionService(t, runner)
	provenancePath := filepath.Join(filepath.Dir(mutationDefinitionPath(root)), experiment.ProvenanceFile)
	provenanceBefore := mustReadFile(t, provenancePath)

	started := service.Start(context.Background(), identity, ExecutionInput{Run: "run-caller-owned"})
	resumed := service.Resume(context.Background(), identity, ExecutionInput{Run: "run-caller-owned"})
	if started.Outcome.Classification != ClassificationClean || resumed.Outcome.Classification != ClassificationClean {
		t.Fatalf("Start/Resume outcomes = %+v / %+v", started.Outcome, resumed.Outcome)
	}
	if len(runner.starts) != 1 || len(runner.resumes) != 1 {
		t.Fatalf("runner calls start=%d resume=%d", len(runner.starts), len(runner.resumes))
	}
	start, resume := runner.starts[0], runner.resumes[0]
	if start.request.Run != "run-caller-owned" || resume.request.Run != "run-caller-owned" || started.Run != "run-caller-owned" || resumed.Run != "run-caller-owned" {
		t.Fatalf("run identity changed: start=%+v resume=%+v", started, resumed)
	}
	if !reflect.DeepEqual(start.request.Definition, resume.request.Definition) || !reflect.DeepEqual(start.authorization, resume.authorization) {
		t.Fatalf("start/resume accepted definition or authorization differ")
	}
	if policy.calls != 2 || len(policy.requests) != 2 {
		t.Fatalf("policy calls = %d requests=%d, want one per operation", policy.calls, len(policy.requests))
	}
	for index, request := range policy.requests {
		if request.AcceptedCommit != testHead {
			t.Fatalf("policy request %d accepted commit = %q, want %q", index, request.AcceptedCommit, testHead)
		}
	}
	expectedAuthorization, err := experimentpolicy.Authorize(policy.decision, experimentpolicy.AuthorizationInput{
		Definition: policy.requests[0].Definition, Capabilities: policy.requests[0].Capabilities,
		ExperimentPath: policy.requests[0].ExperimentPath, CandidatePaths: policy.requests[0].CandidatePaths,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(start.authorization, expectedAuthorization) {
		t.Fatalf("runner authorization = %+v, want exact policy projection %+v", start.authorization, expectedAuthorization)
	}
	if after := mustReadFile(t, provenancePath); !bytes.Equal(after, provenanceBefore) {
		t.Fatalf("machine execution appended mutation provenance")
	}
	worktreeFiles, err := readProposedArtifactFiles(root, started.ExperimentPath)
	if err != nil {
		t.Fatal(err)
	}
	worktreeDigest, err := artifactSetDigest(worktreeFiles, started.ExperimentPath)
	if err != nil {
		t.Fatal(err)
	}
	accepted := service.AcceptedRegistration(context.Background(), identity)
	if worktreeDigest != accepted.ArtifactDigest {
		t.Fatalf("machine evidence changed mutation digest: worktree=%s accepted=%s", worktreeDigest, accepted.ArtifactDigest)
	}
}

func TestStartRequiresAcceptedLockAndExactWorktreeParityBeforeRunner(t *testing.T) {
	runner := &recordingExecutionRunner{}
	root, service := mutationTestService(t)
	identity := testIdentity(t, root, "request-path-v2")
	result := service.Start(context.Background(), identity, ExecutionInput{Run: "run-1"})
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "registration-not-accepted" || len(runner.starts) != 0 {
		t.Fatalf("Start(unaccepted) = %+v runner calls=%d", result, len(runner.starts))
	}

	root, service, _, identity = registeredExecutionService(t, runner)
	direct := filepath.Join(filepath.Dir(mutationDefinitionPath(root)), "direct-after-lock.txt")
	if err := os.WriteFile(direct, []byte("changed locked inputs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result = service.Start(context.Background(), identity, ExecutionInput{Run: "run-1"})
	if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "locked-input-mismatch" || len(runner.starts) != 0 {
		t.Fatalf("Start(mismatched worktree) = %+v runner calls=%d", result, len(runner.starts))
	}
}

func TestStartResumePreserveVerdictAndOperationalClassification(t *testing.T) {
	tests := []struct {
		name   string
		runner *recordingExecutionRunner
		policy error
		resume bool
		want   Classification
		code   string
		exit   int
	}{
		{name: "completed unproven", runner: &recordingExecutionRunner{verdict: experiment.VerdictDisclosedUnproven}, want: ClassificationVerdict, code: "comparison-inconclusive", exit: 1},
		{name: "runner failure", runner: &recordingExecutionRunner{err: errors.New("evaluator unavailable")}, want: ClassificationOperational, code: "runner-failed", exit: 2},
		{name: "policy backend failure", runner: &recordingExecutionRunner{}, policy: errors.New("policy source unavailable"), want: ClassificationOperational, code: "policy-resolution-failed", exit: 2},
		{name: "resume runner failure", runner: &recordingExecutionRunner{err: errors.New("unsafe receipt")}, resume: true, want: ClassificationOperational, code: "runner-failed", exit: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, service, policy, identity := registeredExecutionService(t, tt.runner)
			policy.err = tt.policy
			var result ExecutionResult
			if tt.resume {
				result = service.Resume(context.Background(), identity, ExecutionInput{Run: "run-1"})
			} else {
				result = service.Start(context.Background(), identity, ExecutionInput{Run: "run-1"})
			}
			if result.Outcome.Classification != tt.want || result.Outcome.Code != tt.code || result.Outcome.ExitCode() != tt.exit {
				t.Fatalf("execution outcome = %+v, want %s/%s exit %d", result.Outcome, tt.want, tt.code, tt.exit)
			}
		})
	}
}

func TestStartRejectsRunnerRunIdentityDrift(t *testing.T) {
	runner := &recordingExecutionRunner{}
	_, service, _, identity := registeredExecutionService(t, runner)
	runner.err = fmt.Errorf("runner refused caller identity")
	result := service.Start(context.Background(), identity, ExecutionInput{Run: "run-exact"})
	if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "runner-failed" {
		t.Fatalf("Start(identity refusal) = %+v", result.Outcome)
	}
}
