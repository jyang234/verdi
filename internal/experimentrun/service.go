package experimentrun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentevaluator"
)

// WorkspaceMaterializer is the consumer-owned materialization port for one
// candidate identity.
type WorkspaceMaterializer interface {
	Materialize(context.Context, execworkspace.Request) (execworkspace.Result, error)
}

// AttemptEvaluator executes one already-authorized evaluator attempt.
type AttemptEvaluator interface {
	Observe(context.Context, execworkspace.Profile, experimentevaluator.ObserveInput) (experimentevaluator.Attempt, error)
}

// strictAttemptEvaluator is the production composition of the Task 2
// one-attempt evaluator adapter. Tests may replace it through AttemptEvaluator
// without giving the service a second evaluator protocol implementation.
type strictAttemptEvaluator struct{}

func (strictAttemptEvaluator) Observe(ctx context.Context, profile execworkspace.Profile, input experimentevaluator.ObserveInput) (experimentevaluator.Attempt, error) {
	return experimentevaluator.Observe(ctx, profile, input)
}

// ServiceDependencies supplies every authority and side-effect port required
// to start or resume a CSE run. It intentionally has no authorization default.
type ServiceDependencies struct {
	Authorization AuthorizationResolver
	Inputs        InputResolver
	Materializer  WorkspaceMaterializer
	Evaluator     AttemptEvaluator
	Versions      experiment.ReceiptVersions
}

// Service executes receipt-first CSE starts and unchanged-input resumes. It
// owns neither policy resolution, workspace implementation, evaluator
// transport, nor recommendation semantics.
type Service struct {
	authorization AuthorizationResolver
	inputs        InputResolver
	materializer  WorkspaceMaterializer
	evaluator     AttemptEvaluator
	versions      experiment.ReceiptVersions
}

// StartRequest identifies one locked experiment run below a checkout root.
type StartRequest struct {
	Root          string
	ExperimentDir string
	Run           string
	Definition    experiment.Definition
}

// StartResult carries the durable execution facts plus the complete V2 result
// and its whole-result digest. Incomplete executions return an error and never
// carry or publish a result.
type StartResult struct {
	Receipt        experiment.ExecutionReceipt
	Observations   []experiment.Observation
	WarmupFailures []experiment.WarmupFailure
	Result         experiment.Result
	ResultDigest   string
}

// NewService validates that every required authority and workspace-effect port
// is explicit. Its default evaluator is the existing Task 2 adapter.
func NewService(dependencies ServiceDependencies) (*Service, error) {
	if dependencies.Authorization == nil {
		return nil, fmt.Errorf("experimentrun: new service: authorization resolver is nil")
	}
	if dependencies.Inputs == nil {
		return nil, fmt.Errorf("experimentrun: new service: input resolver is nil")
	}
	if dependencies.Materializer == nil {
		return nil, fmt.Errorf("experimentrun: new service: workspace materializer is nil")
	}
	if dependencies.Evaluator == nil {
		dependencies.Evaluator = strictAttemptEvaluator{}
	}
	if err := dependencies.Versions.Validate(); err != nil {
		return nil, fmt.Errorf("experimentrun: new service versions: %w", err)
	}
	return &Service{
		authorization: dependencies.Authorization,
		inputs:        dependencies.Inputs,
		materializer:  dependencies.Materializer,
		evaluator:     dependencies.Evaluator,
		versions:      dependencies.Versions,
	}, nil
}

// Start validates every locked execution fact, publishes the immutable receipt
// under the checkout writer lock, then materializes and executes the complete
// deterministic schedule, removes only the reserved profile roots, and
// publishes the verified complete V2 result.
func (s *Service) Start(ctx context.Context, request StartRequest) (StartResult, error) {
	if s == nil {
		return StartResult{}, fmt.Errorf("experimentrun: start: service is nil")
	}
	if ctx == nil {
		return StartResult{}, fmt.Errorf("experimentrun: start: nil context")
	}
	if err := request.Definition.Validate(); err != nil {
		return StartResult{}, fmt.Errorf("experimentrun: start definition: %w", err)
	}
	locked, err := experiment.Locked(request.Definition)
	if err != nil {
		return StartResult{}, fmt.Errorf("experimentrun: start definition lock: %w", err)
	}
	if !locked {
		return StartResult{}, fmt.Errorf("experimentrun: start requires a locked definition")
	}
	storage, err := newRunStorage(request.Root, request.ExperimentDir, request.Run)
	if err != nil {
		return StartResult{}, err
	}
	if err := storage.preflightStart(); err != nil {
		return StartResult{}, fmt.Errorf("experimentrun: start storage preflight: %w", err)
	}
	capabilities, capabilitiesBytes, err := storage.loadCapabilities()
	if err != nil {
		return StartResult{}, err
	}
	patches, err := storage.loadCandidatePatches(request.Definition)
	if err != nil {
		return StartResult{}, err
	}
	inputs, err := ResolveInputs(ctx, s.inputs, request.Root, request.Definition)
	if err != nil {
		return StartResult{}, err
	}
	authorized, err := ResolveAuthorization(ctx, s.authorization, request.Definition, capabilities)
	if err != nil {
		return StartResult{}, err
	}
	schedule, err := DeriveSchedule(request.Definition)
	if err != nil {
		return StartResult{}, err
	}
	experimentDigest, err := experiment.DefinitionDigest(request.Definition)
	if err != nil {
		return StartResult{}, fmt.Errorf("experimentrun: start definition digest: %w", err)
	}
	candidates, fingerprint, enforcement, err := s.planCandidates(request, authorized, capabilities, inputs, patches, experimentDigest)
	if err != nil {
		return StartResult{}, err
	}
	receipt, err := BuildExecutionReceipt(ReceiptInput{
		Definition:        request.Definition,
		Run:               request.Run,
		Capabilities:      capabilities,
		CapabilitiesBytes: capabilitiesBytes,
		Authorization:     authorized,
		Inputs:            inputs,
		CandidatePatches:  patches,
		Fingerprint:       fingerprint,
		Enforcement:       enforcement,
		Versions:          s.versions,
	})
	if err != nil {
		return StartResult{}, err
	}
	if err := storage.createReceipt(receipt); err != nil {
		return StartResult{}, err
	}
	result := StartResult{
		Receipt:        receipt,
		Observations:   make([]experiment.Observation, 0, len(measuredSchedule(schedule))),
		WarmupFailures: []experiment.WarmupFailure{},
	}
	for _, scheduled := range schedule {
		candidate, ok := candidates[scheduled.Candidate]
		if !ok {
			return StartResult{}, fmt.Errorf("experimentrun: schedule names unplanned candidate %q", scheduled.Candidate)
		}
		profile, err := s.activateCandidate(ctx, candidate)
		if err != nil {
			return StartResult{}, err
		}
		observeInput := experimentevaluator.ObserveInput{
			Launch: experimentevaluator.Launch{
				Directory: candidate.workspacePath,
				Argv:      append([]string(nil), request.Definition.Evaluator.Argv...),
				Digest:    request.Definition.Evaluator.Digest,
			},
			Request: experiment.EvaluatorRequest{
				Schema:           experiment.EvaluatorProtocolSchema,
				ExperimentDigest: experimentDigest,
				Run:              request.Run,
				Candidate:        scheduled.Candidate,
				Cycle:            scheduled.Cycle,
				Workload:         inputs.Workload,
				Fixtures:         append([]experiment.ResolvedArtifact(nil), inputs.Fixtures...),
				Contract:         inputs.Contract,
			},
		}
		attempt, err := s.evaluateAttempt(ctx, profile, observeInput)
		if err != nil {
			return StartResult{}, fmt.Errorf("experimentrun: evaluate %s %q: %w", scheduled.Cycle.Kind, scheduled.Candidate, err)
		}
		if scheduled.Cycle.Kind == experiment.CycleWarmup {
			if attempt.Outcome.Kind != experiment.OutcomeCompleted {
				result.WarmupFailures = append(result.WarmupFailures, experiment.WarmupFailure{
					Candidate: scheduled.Candidate,
					Warmup:    scheduled.Cycle.Number,
					Kind:      attempt.Outcome.Kind,
					Witness:   *attempt.Outcome.Witness,
				})
			}
			continue
		}
		if err := storage.appendObservation(request.Definition, schedule, *attempt.Observation); err != nil {
			return StartResult{}, err
		}
		result.Observations = append(result.Observations, *attempt.Observation)
	}
	result.Result, result.ResultDigest, err = completeRun(storage, request.Definition, result.Observations, receipt, result.WarmupFailures, candidateEnvironmentRoots(request.Definition, candidates))
	if err != nil {
		return StartResult{}, err
	}
	return result, nil
}

func candidateEnvironmentRoots(def experiment.Definition, candidates map[string]*candidatePlan) []string {
	paths := make([]string, 0, len(def.Candidates))
	for _, candidate := range def.Candidates {
		if plan, ok := candidates[candidate.ID]; ok {
			paths = append(paths, plan.environment)
		}
	}
	return paths
}

func (s *Service) evaluateAttempt(ctx context.Context, profile execworkspace.Profile, input experimentevaluator.ObserveInput) (experimentevaluator.Attempt, error) {
	attempt, err := s.evaluator.Observe(ctx, profile, input)
	if err != nil {
		return experimentevaluator.Attempt{}, err
	}
	if err := experimentevaluator.ValidateAttempt(input, attempt); err != nil {
		return experimentevaluator.Attempt{}, fmt.Errorf("experimentrun: validate evaluator attempt: %w", err)
	}
	return attempt, nil
}

type candidatePlan struct {
	identity      execworkspace.Identity
	patch         []byte
	workspacePath string
	environment   string
	planned       execworkspace.Profile
	profile       execworkspace.Profile
	activated     bool
}

func (s *Service) planCandidates(request StartRequest, authorized AuthorizedExecution, capabilities experiment.Capabilities, inputs ResolvedInputs, patches map[string][]byte, experimentDigest string) (map[string]*candidatePlan, []byte, execworkspace.EnforcementReport, error) {
	return s.deriveCandidatePlans(request, authorized, capabilities, inputs, patches, experimentDigest, true)
}

func (s *Service) planResumeCandidates(request StartRequest, authorized AuthorizedExecution, capabilities experiment.Capabilities, inputs ResolvedInputs, patches map[string][]byte, experimentDigest string) (map[string]*candidatePlan, []byte, execworkspace.EnforcementReport, error) {
	return s.deriveCandidatePlans(request, authorized, capabilities, inputs, patches, experimentDigest, false)
}

func (s *Service) deriveCandidatePlans(request StartRequest, authorized AuthorizedExecution, capabilities experiment.Capabilities, inputs ResolvedInputs, patches map[string][]byte, experimentDigest string, preflightExisting bool) (map[string]*candidatePlan, []byte, execworkspace.EnforcementReport, error) {
	plans := make(map[string]*candidatePlan, len(request.Definition.Candidates))
	var fingerprint []byte
	var enforcement execworkspace.EnforcementReport
	for index, candidate := range request.Definition.Candidates {
		identityRunID, err := experiment.WorkspaceRunID(experimentDigest, request.Run, candidate.ID)
		if err != nil {
			return nil, nil, execworkspace.EnforcementReport{}, err
		}
		identity, err := execworkspace.NewPatchIdentity(identityRunID, candidate.Base, patches[candidate.ID])
		if err != nil {
			return nil, nil, execworkspace.EnforcementReport{}, fmt.Errorf("experimentrun: plan candidate %q identity: %w", candidate.ID, err)
		}
		workspaceID, err := identity.WorkspaceID()
		if err != nil {
			return nil, nil, execworkspace.EnforcementReport{}, fmt.Errorf("experimentrun: plan candidate %q workspace id: %w", candidate.ID, err)
		}
		workspacePath := execworkspace.UnitPath(request.Root, workspaceID)
		environmentPath := filepath.Join(workspacePath, environmentRootName)
		if preflightExisting {
			if err := preflightExistingEnvironmentRoot(workspacePath); err != nil {
				return nil, nil, execworkspace.EnforcementReport{}, fmt.Errorf("experimentrun: plan candidate %q environment: %w", candidate.ID, err)
			}
		}
		planned, report, err := execworkspace.PlanProfile(workspacePath, environmentPath, authorized.Grants, authorized.Authorization.DeclaredEnv)
		if err != nil {
			return nil, nil, execworkspace.EnforcementReport{}, fmt.Errorf("experimentrun: plan candidate %q profile: %w", candidate.ID, err)
		}
		if report == nil {
			return nil, nil, execworkspace.EnforcementReport{}, fmt.Errorf("experimentrun: plan candidate %q returned no enforcement report", candidate.ID)
		}
		if index == 0 {
			enforcement = *report
			fingerprint, err = execworkspace.CollectFingerprint(planned, fingerprintInputs(request.Definition, capabilities, inputs, s.versions))
			if err != nil {
				return nil, nil, execworkspace.EnforcementReport{}, fmt.Errorf("experimentrun: collect planned fingerprint: %w", err)
			}
		}
		plans[candidate.ID] = &candidatePlan{
			identity:      identity,
			patch:         append([]byte(nil), patches[candidate.ID]...),
			workspacePath: workspacePath,
			environment:   environmentPath,
			planned:       planned,
		}
	}
	return plans, fingerprint, enforcement, nil
}

// preflightExistingEnvironmentRoot preserves the receipt-first order for a
// fresh workspace while refusing to reuse a pre-existing candidate workspace
// whose reserved profile root already exists. PreflightEnvironmentRoot itself
// rejects an absent workspace because it validates an exact root, so absence
// is the one expected no-effect case here.
func preflightExistingEnvironmentRoot(workspacePath string) error {
	if _, err := os.Lstat(workspacePath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("lstat candidate workspace %q: %w", workspacePath, err)
	}
	if _, err := PreflightEnvironmentRoot(workspacePath); err != nil {
		return err
	}
	return nil
}

func fingerprintInputs(def experiment.Definition, capabilities experiment.Capabilities, inputs ResolvedInputs, versions experiment.ReceiptVersions) execworkspace.FingerprintInputs {
	inputDigests := map[string]string{
		"evaluator:" + def.Evaluator.Argv[0]: strings.TrimPrefix(def.Evaluator.Digest, "sha256:"),
		inputs.Workload.Path:                 strings.TrimPrefix(inputs.Workload.Digest, "sha256:"),
		inputs.Contract.Path:                 strings.TrimPrefix(inputs.Contract.Digest, "sha256:"),
	}
	for _, fixture := range inputs.Fixtures {
		inputDigests[fixture.Path] = strings.TrimPrefix(fixture.Digest, "sha256:")
	}
	return execworkspace.FingerprintInputs{
		ToolVersions: map[string]string{
			"evaluator":             capabilities.EvaluatorVersion,
			"recommendation-engine": string(def.Algorithm),
			"runtime":               runtime.Version(),
			"verdi":                 versions.Verdi,
		},
		EnvVarNames:  append([]string(nil), capabilities.Environment...),
		InputDigests: inputDigests,
	}
}

func (s *Service) activateCandidate(ctx context.Context, candidate *candidatePlan) (execworkspace.Profile, error) {
	if candidate.activated {
		return candidate.profile, nil
	}
	materialized, err := s.materializer.Materialize(ctx, execworkspace.Request{
		Identity:   candidate.identity,
		PatchBytes: append([]byte(nil), candidate.patch...),
	})
	if err != nil {
		return execworkspace.Profile{}, fmt.Errorf("experimentrun: materialize workspace: %w", err)
	}
	wantWorkspaceID, err := candidate.identity.WorkspaceID()
	if err != nil {
		return execworkspace.Profile{}, fmt.Errorf("experimentrun: candidate workspace id: %w", err)
	}
	if materialized.WorkspaceID != wantWorkspaceID {
		return execworkspace.Profile{}, fmt.Errorf("experimentrun: materializer workspace id %q does not match planned workspace id %q", materialized.WorkspaceID, wantWorkspaceID)
	}
	if materialized.Path != candidate.workspacePath {
		return execworkspace.Profile{}, fmt.Errorf("experimentrun: materializer path %q does not match planned path %q", materialized.Path, candidate.workspacePath)
	}
	environment, err := PreflightEnvironmentRoot(materialized.Path)
	if err != nil {
		return execworkspace.Profile{}, err
	}
	if environment != candidate.environment {
		return execworkspace.Profile{}, fmt.Errorf("experimentrun: preflight environment root %q does not match planned root %q", environment, candidate.environment)
	}
	profile, err := candidate.planned.Activate()
	if err != nil {
		return execworkspace.Profile{}, fmt.Errorf("experimentrun: activate candidate profile: %w", err)
	}
	candidate.profile = profile
	candidate.activated = true
	return profile, nil
}
