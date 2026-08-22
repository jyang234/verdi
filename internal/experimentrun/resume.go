package experimentrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentdecision"
	"github.com/jyang234/verdi/internal/experimentevaluator"
)

// ResumeRequest identifies one previously started durable run. Start remains
// the only mode permitted to create its execution receipt.
type ResumeRequest StartRequest

// Resume rederives every receipt input, validates the exact measured prefix,
// restarts warmups diagnostically, and executes only the missing measured
// tail. A complete result is returned idempotently without re-execution.
func (s *Service) Resume(ctx context.Context, request ResumeRequest) (StartResult, error) {
	if s == nil {
		return StartResult{}, fmt.Errorf("experimentrun: resume: service is nil")
	}
	if ctx == nil {
		return StartResult{}, fmt.Errorf("experimentrun: resume: nil context")
	}
	startRequest := StartRequest(request)
	storage, err := newRunStorage(startRequest.Root, startRequest.ExperimentDir, startRequest.Run)
	if err != nil {
		return StartResult{}, err
	}
	receipt, err := storage.loadReceipt()
	if err != nil {
		return StartResult{}, fmt.Errorf("experimentrun: resume requires an execution receipt: %w", err)
	}
	if err := startRequest.Definition.Validate(); err != nil {
		return StartResult{}, fmt.Errorf("experimentrun: resume definition: %w", err)
	}
	locked, err := experiment.Locked(startRequest.Definition)
	if err != nil {
		return StartResult{}, fmt.Errorf("experimentrun: resume definition lock: %w", err)
	}
	if !locked {
		return StartResult{}, fmt.Errorf("experimentrun: resume requires a locked definition")
	}
	capabilities, capabilitiesBytes, err := storage.loadCapabilities()
	if err != nil {
		return StartResult{}, err
	}
	patches, err := storage.loadCandidatePatches(startRequest.Definition)
	if err != nil {
		return StartResult{}, err
	}
	inputs, err := ResolveInputs(ctx, s.inputs, startRequest.Root, startRequest.Definition)
	if err != nil {
		return StartResult{}, err
	}
	authorized, err := ResolveAuthorization(ctx, s.authorization, startRequest.Definition, capabilities)
	if err != nil {
		return StartResult{}, err
	}
	schedule, err := DeriveSchedule(startRequest.Definition)
	if err != nil {
		return StartResult{}, err
	}
	experimentDigest, err := experiment.DefinitionDigest(startRequest.Definition)
	if err != nil {
		return StartResult{}, fmt.Errorf("experimentrun: resume definition digest: %w", err)
	}
	candidates, fingerprint, enforcement, err := s.planResumeCandidates(startRequest, authorized, capabilities, inputs, patches, experimentDigest)
	if err != nil {
		return StartResult{}, err
	}
	if err := VerifyExecutionReceipt(ReceiptInput{
		Definition:        startRequest.Definition,
		Run:               startRequest.Run,
		Capabilities:      capabilities,
		CapabilitiesBytes: capabilitiesBytes,
		Authorization:     authorized,
		Inputs:            inputs,
		CandidatePatches:  patches,
		Fingerprint:       fingerprint,
		Enforcement:       enforcement,
		Versions:          s.versions,
	}, receipt); err != nil {
		return StartResult{}, fmt.Errorf("experimentrun: resume receipt parity: %w", err)
	}
	observations, err := storage.loadMeasuredPrefix(startRequest.Definition, schedule)
	if err != nil {
		return StartResult{}, fmt.Errorf("experimentrun: resume measured prefix: %w", err)
	}
	storedResult, hasResult, err := storage.loadResult()
	if err != nil {
		return StartResult{}, err
	}
	complete := len(observations) == len(measuredSchedule(schedule))
	if hasResult && !complete {
		return StartResult{}, fmt.Errorf("experimentrun: incomplete run has a result")
	}
	environmentRoots := candidateEnvironmentRoots(startRequest.Definition, candidates)
	if complete {
		if hasResult {
			if err := proveEnvironmentRootsAbsent(startRequest.Root, environmentRoots); err != nil {
				return StartResult{}, err
			}
			if err := experimentdecision.VerifyResult(startRequest.Definition, observations, &receipt, storedResult); err != nil {
				return StartResult{}, fmt.Errorf("experimentrun: verify existing result: %w", err)
			}
			digest, err := experiment.ResultDigest(storedResult)
			if err != nil {
				return StartResult{}, fmt.Errorf("experimentrun: digest existing result: %w", err)
			}
			return StartResult{Receipt: receipt, Observations: observations, WarmupFailures: append([]experiment.WarmupFailure(nil), storedResult.Execution.WarmupDiagnostics.Failures...), Result: storedResult, ResultDigest: digest}, nil
		}
	}

	// Missing roots are retryable before measured evidence and at the unique
	// complete-prefix/no-result cleanup boundary. An incomplete measured prefix
	// still requires every receipt-bound root to persist unchanged.
	rootState := candidateRootsMayBeMissing
	if !complete && len(observations) > 0 {
		rootState = candidateRootsMustExist
	}
	if err := s.prepareCandidateProfiles(ctx, startRequest.Root, startRequest.Definition, candidates, rootState); err != nil {
		return StartResult{}, err
	}

	warmupFailures := []experiment.WarmupFailure{}
	for _, scheduled := range schedule {
		if scheduled.Cycle.Kind != experiment.CycleWarmup {
			continue
		}
		attempt, err := s.observeScheduled(ctx, startRequest, inputs, experimentDigest, candidates[scheduled.Candidate], scheduled)
		if err != nil {
			return StartResult{}, err
		}
		if attempt.Outcome.Kind != experiment.OutcomeCompleted {
			warmupFailures = append(warmupFailures, experiment.WarmupFailure{Candidate: scheduled.Candidate, Warmup: scheduled.Cycle.Number, Kind: attempt.Outcome.Kind, Witness: *attempt.Outcome.Witness})
		}
	}
	measured := measuredSchedule(schedule)
	for _, scheduled := range measured[len(observations):] {
		attempt, err := s.observeScheduled(ctx, startRequest, inputs, experimentDigest, candidates[scheduled.Candidate], scheduled)
		if err != nil {
			return StartResult{}, err
		}
		if err := storage.appendObservation(startRequest.Definition, schedule, *attempt.Observation); err != nil {
			return StartResult{}, err
		}
		observations = append(observations, *attempt.Observation)
	}
	result, digest, err := completeRun(storage, startRequest.Definition, observations, receipt, warmupFailures, environmentRoots)
	if err != nil {
		return StartResult{}, err
	}
	return StartResult{Receipt: receipt, Observations: observations, WarmupFailures: warmupFailures, Result: result, ResultDigest: digest}, nil
}

func (s *Service) observeScheduled(ctx context.Context, request StartRequest, inputs ResolvedInputs, experimentDigest string, candidate *candidatePlan, scheduled ScheduledAttempt) (experimentevaluator.Attempt, error) {
	attempt, err := s.evaluateAttempt(ctx, candidate.profile, experimentevaluator.ObserveInput{
		Launch: experimentevaluator.Launch{
			Directory: candidate.workspacePath,
			Argv:      append([]string(nil), request.Definition.Evaluator.Argv...),
			Digest:    request.Definition.Evaluator.Digest,
		},
		Request: experiment.EvaluatorRequest{
			Schema: experiment.EvaluatorProtocolSchema, ExperimentDigest: experimentDigest, Run: request.Run,
			Candidate: scheduled.Candidate, Cycle: scheduled.Cycle, Workload: inputs.Workload,
			Fixtures: append([]experiment.ResolvedArtifact(nil), inputs.Fixtures...), Contract: inputs.Contract,
		},
	})
	if err != nil {
		return experimentevaluator.Attempt{}, fmt.Errorf("experimentrun: evaluate %s %q: %w", scheduled.Cycle.Kind, scheduled.Candidate, err)
	}
	return attempt, nil
}

func (s *Service) activateResumeCandidate(ctx context.Context, root string, candidate *candidatePlan, hasMeasured bool) error {
	materialized, err := s.materializer.Materialize(ctx, execworkspace.Request{Identity: candidate.identity, PatchBytes: append([]byte(nil), candidate.patch...)})
	if err != nil {
		return fmt.Errorf("experimentrun: resume materialize workspace: %w", err)
	}
	wantWorkspaceID, err := candidate.identity.WorkspaceID()
	if err != nil {
		return fmt.Errorf("experimentrun: resume candidate workspace id: %w", err)
	}
	if materialized.WorkspaceID != wantWorkspaceID || materialized.Path != candidate.workspacePath {
		return fmt.Errorf("experimentrun: resume materializer identity/path does not match derived candidate plan")
	}
	if err := validateResumeEnvironmentRoot(root, candidate.environment, hasMeasured); err != nil {
		return err
	}
	profile, err := candidate.planned.Activate()
	if err != nil {
		return fmt.Errorf("experimentrun: resume activate candidate profile: %w", err)
	}
	candidate.profile = profile
	candidate.activated = true
	return nil
}

func proveEnvironmentRootsAbsent(root string, paths []string) error {
	for _, path := range paths {
		if err := validateParentTree(root, filepath.Dir(path)); err != nil {
			return err
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			if err == nil {
				return fmt.Errorf("experimentrun: completed result has a present environment root %q", path)
			}
			return fmt.Errorf("experimentrun: prove completed environment root %q absent: %w", path, err)
		}
	}
	return nil
}

func validateResumeEnvironmentRoot(root, path string, hasMeasuredObservations bool) error {
	if !filepath.IsAbs(path) || filepath.Base(path) != environmentRootName {
		return fmt.Errorf("experimentrun: reserved environment root %q is not an exact absolute %s path", path, environmentRootName)
	}
	if err := validateParentTree(root, filepath.Dir(path)); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if hasMeasuredObservations {
			return fmt.Errorf("experimentrun: reserved environment root %q is missing after measured evidence", path)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("experimentrun: lstat reserved environment root %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("experimentrun: reserved environment root %q is not a non-symlink directory", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("experimentrun: read reserved environment root %q: %w", path, err)
	}
	for _, entry := range entries {
		if entry.Name() != ".home" && entry.Name() != ".tmp" {
			return fmt.Errorf("experimentrun: reserved environment root %q has foreign top-level entry %q", path, entry.Name())
		}
	}
	for _, relative := range []string{filepath.Join(".home"), filepath.Join(".home", ".config"), filepath.Join(".home", ".cache"), ".tmp"} {
		required := filepath.Join(path, relative)
		requiredInfo, statErr := os.Lstat(required)
		if statErr != nil || requiredInfo.Mode()&os.ModeSymlink != 0 || !requiredInfo.IsDir() {
			return fmt.Errorf("experimentrun: reserved environment root %q does not have the exact activated profile shape at %q", path, relative)
		}
	}
	return nil
}

func cleanupEnvironmentRoots(root string, paths []string) error {
	for _, path := range paths {
		if !filepath.IsAbs(path) || filepath.Base(path) != environmentRootName {
			return fmt.Errorf("experimentrun: refuse cleanup of non-reserved environment path %q", path)
		}
		if err := validateParentTree(root, filepath.Dir(path)); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("experimentrun: lstat environment root %q before cleanup: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("experimentrun: refuse cleanup of environment root collision %q", path)
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("experimentrun: remove environment root %q: %w", path, err)
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			if err == nil {
				return fmt.Errorf("experimentrun: environment root %q remains after cleanup", path)
			}
			return fmt.Errorf("experimentrun: prove environment root %q absent after cleanup: %w", path, err)
		}
	}
	return nil
}
