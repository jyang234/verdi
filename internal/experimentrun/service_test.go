package experimentrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentevaluator"
)

func TestNewServiceDefaultsToStrictEvaluatorAdapter(t *testing.T) {
	service, err := NewService(ServiceDependencies{
		Authorization: staticAuthorization{},
		Inputs:        staticInputs{},
		Materializer:  &recordingMaterializer{},
		Versions: experiment.ReceiptVersions{
			Verdi:                "v-test",
			RecommendationEngine: string(experiment.AlgorithmV1),
		},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, ok := service.evaluator.(strictAttemptEvaluator); !ok {
		t.Fatalf("default evaluator = %T, want strict experimentevaluator adapter", service.evaluator)
	}
}

func TestValidateEvaluatorAttemptFailsClosed(t *testing.T) {
	witness := "candidate timed out"
	completedWithWitness := experiment.CandidateOutcome{Kind: experiment.OutcomeCompleted, Witness: &witness}
	timeout := experiment.CandidateOutcome{Kind: experiment.OutcomeCandidateTimeout, Witness: &witness}
	completed := experiment.CandidateOutcome{Kind: experiment.OutcomeCompleted}

	for _, test := range []struct {
		name      string
		scheduled ScheduledAttempt
		attempt   experimentevaluator.Attempt
		want      string
	}{
		{
			name:      "invalid warmup outcome",
			scheduled: ScheduledAttempt{Candidate: "alpha", Cycle: experiment.EvaluatorCycle{Kind: experiment.CycleWarmup, Number: 1}},
			attempt:   experimentevaluator.Attempt{Outcome: completedWithWitness},
			want:      "outcome",
		},
		{
			name:      "warmup observation",
			scheduled: ScheduledAttempt{Candidate: "alpha", Cycle: experiment.EvaluatorCycle{Kind: experiment.CycleWarmup, Number: 1}},
			attempt:   experimentevaluator.Attempt{Outcome: completed, Observation: &experiment.Observation{}},
			want:      "warmup",
		},
		{
			name:      "missing measured observation",
			scheduled: ScheduledAttempt{Candidate: "alpha", Cycle: experiment.EvaluatorCycle{Kind: experiment.CycleMeasured, Number: 1}},
			attempt:   experimentevaluator.Attempt{Outcome: completed},
			want:      "no observation",
		},
		{
			name:      "mismatched measured outcome",
			scheduled: ScheduledAttempt{Candidate: "alpha", Cycle: experiment.EvaluatorCycle{Kind: experiment.CycleMeasured, Number: 1}},
			attempt: experimentevaluator.Attempt{
				Outcome: timeout,
				Observation: &experiment.Observation{
					Outcome: &completed,
				},
			},
			want: "does not match",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateEvaluatorAttempt(test.scheduled, test.attempt); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateEvaluatorAttempt error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestActivateCandidateRejectsMaterializerIdentityMismatch(t *testing.T) {
	root := t.TempDir()
	identity, err := execworkspace.NewPatchIdentity("run-1", strings.Repeat("a", 40), []byte("diff --git a/a b/a\n"))
	if err != nil {
		t.Fatal(err)
	}
	workspaceID, err := identity.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	workspace := execworkspace.UnitPath(root, workspaceID)
	environment := filepath.Join(workspace, environmentRootName)
	planned, _, err := execworkspace.PlanProfile(workspace, environment, testGrants(t, true, "./tools/evaluator", 30), map[string]string{"LANG": "C"})
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{materializer: fixedMaterializer{result: execworkspace.Result{
		WorkspaceID: "wrong-workspace-id",
		Path:        workspace,
		Outcome:     execworkspace.OutcomeMaterialized,
	}}}

	_, err = service.activateCandidate(context.Background(), &candidatePlan{
		identity:      identity,
		workspacePath: workspace,
		environment:   environment,
		planned:       planned,
	})
	if err == nil || !strings.Contains(err.Error(), "workspace id") {
		t.Fatalf("activateCandidate error = %v, want materializer identity mismatch", err)
	}
}

func TestActivateCandidateRejectsEnvironmentCollisionAfterMaterialization(t *testing.T) {
	root := t.TempDir()
	identity, err := execworkspace.NewPatchIdentity("run-1", strings.Repeat("a", 40), []byte("diff --git a/a b/a\n"))
	if err != nil {
		t.Fatal(err)
	}
	workspaceID, err := identity.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	workspace := execworkspace.UnitPath(root, workspaceID)
	environment := filepath.Join(workspace, environmentRootName)
	planned, _, err := execworkspace.PlanProfile(workspace, environment, testGrants(t, true, "./tools/evaluator", 30), map[string]string{"LANG": "C"})
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{materializer: fixedMaterializer{
		result: execworkspace.Result{WorkspaceID: workspaceID, Path: workspace, Outcome: execworkspace.OutcomeMaterialized},
		create: environment,
	}}

	_, err = service.activateCandidate(context.Background(), &candidatePlan{
		identity:      identity,
		workspacePath: workspace,
		environment:   environment,
		planned:       planned,
	})
	if err == nil || !strings.Contains(err.Error(), "reserved environment root") {
		t.Fatalf("activateCandidate error = %v, want post-materialization environment collision", err)
	}
}

func TestStartRejectsExistingCandidateEnvironmentBeforeReceipt(t *testing.T) {
	root := t.TempDir()
	const experimentDir = "experiments/comparison"
	const run = "run-1"

	def, capabilities, _ := testDefinition(t, []string{"alpha", "beta"}, 1)
	capabilities.RequiresNetwork = true
	capabilitiesBytes, err := canonjson.Marshal(capabilities)
	if err != nil {
		t.Fatal(err)
	}
	def.Evaluator.CapabilitiesDigest = testDigestBytes(capabilitiesBytes)
	def = relockDefinition(t, def)
	patches := candidatePatches(t, def)
	writeStartAuthority(t, root, experimentDir, capabilitiesBytes, patches)
	writeResolvedInputs(t, root, def)

	digest, err := experiment.DefinitionDigest(def)
	if err != nil {
		t.Fatal(err)
	}
	identityRunID, err := experiment.WorkspaceRunID(digest, run, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := execworkspace.NewPatchIdentity(identityRunID, def.Candidates[0].Base, patches["alpha"])
	if err != nil {
		t.Fatal(err)
	}
	workspaceID, err := identity.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(execworkspace.UnitPath(root, workspaceID), environmentRootName), 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := experiment.PathsForRun(experimentDir, run)
	if err != nil {
		t.Fatal(err)
	}
	materializer := &recordingMaterializer{root: root, receiptPath: filepath.Join(root, filepath.FromSlash(paths.Execution))}
	service, err := NewService(ServiceDependencies{
		Authorization: staticAuthorization{authorization: testAuthorization(t, def, true)},
		Inputs: staticInputs{values: map[string]ResolvedInput{
			def.Workload.ID:    {ID: def.Workload.ID, Path: "inputs/workload.json", Digest: def.Workload.Digest},
			def.Fixtures[0].ID: {ID: def.Fixtures[0].ID, Path: "fixtures/request-log.json", Digest: def.Fixtures[0].Digest},
			def.Contract.ID:    {ID: def.Contract.ID, Path: "contracts/behavioral.json", Digest: def.Contract.Digest},
		}},
		Materializer: materializer,
		Evaluator:    &recordingEvaluator{},
		Versions: experiment.ReceiptVersions{
			Verdi:                "v-test",
			RecommendationEngine: string(def.Algorithm),
		},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.Start(context.Background(), StartRequest{Root: root, ExperimentDir: experimentDir, Run: run, Definition: def})
	if err == nil || !strings.Contains(err.Error(), "reserved environment root") {
		t.Fatalf("Start() error = %v, want existing environment collision before receipt", err)
	}
	if len(materializer.requests) != 0 {
		t.Fatalf("Start() materialized despite pre-receipt collision: %d requests", len(materializer.requests))
	}
	if _, statErr := os.Lstat(materializer.receiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Start() created receipt despite pre-receipt collision: %v", statErr)
	}
}

func TestStartPublishesReceiptBeforeMaterializationAndMeasuredPrefix(t *testing.T) {
	root := t.TempDir()
	const experimentDir = "experiments/comparison"
	const run = "run-1"

	def, capabilities, _ := testDefinition(t, []string{"alpha", "beta"}, 1)
	capabilities.RequiresNetwork = true
	capabilitiesBytes, err := canonjson.Marshal(capabilities)
	if err != nil {
		t.Fatal(err)
	}
	def.Evaluator.CapabilitiesDigest = testDigestBytes(capabilitiesBytes)
	def = relockDefinition(t, def)
	patches := candidatePatches(t, def)
	writeStartAuthority(t, root, experimentDir, capabilitiesBytes, patches)
	writeResolvedInputs(t, root, def)

	inputs := staticInputs{values: map[string]ResolvedInput{
		def.Workload.ID:    {ID: def.Workload.ID, Path: "inputs/workload.json", Digest: def.Workload.Digest},
		def.Fixtures[0].ID: {ID: def.Fixtures[0].ID, Path: "fixtures/request-log.json", Digest: def.Fixtures[0].Digest},
		def.Contract.ID:    {ID: def.Contract.ID, Path: "contracts/behavioral.json", Digest: def.Contract.Digest},
	}}
	paths, err := experiment.PathsForRun(experimentDir, run)
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(root, filepath.FromSlash(paths.Execution))
	materializer := &recordingMaterializer{root: root, receiptPath: receiptPath}
	evaluator := &recordingEvaluator{}
	service, err := NewService(ServiceDependencies{
		Authorization: staticAuthorization{authorization: testAuthorization(t, def, true)},
		Inputs:        inputs,
		Materializer:  materializer,
		Evaluator:     evaluator,
		Versions: experiment.ReceiptVersions{
			Verdi:                "v-test",
			RecommendationEngine: string(def.Algorithm),
		},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	result, err := service.Start(context.Background(), StartRequest{
		Root:          root,
		ExperimentDir: experimentDir,
		Run:           run,
		Definition:    def,
	})
	if runtime.GOOS != "linux" {
		if err == nil || !strings.Contains(err.Error(), "authoritative CSE execution requires linux") {
			t.Fatalf("Start() error = %v, want unsupported-platform operational refusal", err)
		}
		if len(materializer.requests) != 0 || len(evaluator.requests) != 0 {
			t.Fatalf("Start() on unsupported host materialized or evaluated: materializations=%d evaluations=%d", len(materializer.requests), len(evaluator.requests))
		}
		if _, statErr := os.Lstat(receiptPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Start() on unsupported host created receipt: lstat error = %v", statErr)
		}
		return
	}
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if result.Receipt.Run != run {
		t.Fatalf("receipt run = %q, want %q", result.Receipt.Run, run)
	}
	if len(materializer.requests) != len(def.Candidates) {
		t.Fatalf("materializations = %d, want %d", len(materializer.requests), len(def.Candidates))
	}
	schedule, err := DeriveSchedule(def)
	if err != nil {
		t.Fatal(err)
	}
	if len(evaluator.requests) != len(schedule) {
		t.Fatalf("evaluations = %d, want complete schedule %d", len(evaluator.requests), len(schedule))
	}
	if len(result.Observations) != len(def.Candidates)*def.Execution.Rounds {
		t.Fatalf("measured observations = %d, want %d with warmups excluded", len(result.Observations), len(def.Candidates)*def.Execution.Rounds)
	}
	if got := result.WarmupFailures; len(got) != 1 || got[0].Candidate != "beta" || got[0].Warmup != 1 || got[0].Kind != experiment.OutcomeCandidateTimeout {
		t.Fatalf("warmup failures = %#v, want beta timeout diagnostic", got)
	}
	for i, want := range []string{"alpha@1", "beta@1", "beta@2", "alpha@2"} {
		got := fmt.Sprintf("%s@%d", result.Observations[i].Candidate, result.Observations[i].Round)
		if got != want {
			t.Fatalf("observation %d = %q, want exact measured schedule prefix %q", i, got, want)
		}
	}

	observationsPath := filepath.Join(root, filepath.FromSlash(paths.Observations))
	observationBytes, err := os.ReadFile(observationsPath)
	if err != nil {
		t.Fatalf("read observations: %v", err)
	}
	persisted, err := experiment.DecodeObservations(observationBytes)
	if err != nil {
		t.Fatalf("decode observations: %v", err)
	}
	if len(persisted) != len(result.Observations) {
		t.Fatalf("persisted observations = %d, want %d", len(persisted), len(result.Observations))
	}
	if _, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(paths.Result))); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Task 4 emitted result.json: lstat error = %v", statErr)
	}
	for _, materialized := range materializer.paths {
		if _, statErr := os.Stat(filepath.Join(materialized, environmentRootName)); statErr != nil {
			t.Fatalf("activated environment root below %q: %v", materialized, statErr)
		}
	}
}

type recordingMaterializer struct {
	root        string
	receiptPath string
	requests    []execworkspace.Request
	paths       []string
}

type fixedMaterializer struct {
	result execworkspace.Result
	err    error
	create string
}

func (m fixedMaterializer) Materialize(_ context.Context, _ execworkspace.Request) (execworkspace.Result, error) {
	if m.err != nil {
		return execworkspace.Result{}, m.err
	}
	if err := os.MkdirAll(m.result.Path, 0o755); err != nil {
		return execworkspace.Result{}, err
	}
	if m.create != "" {
		if err := os.MkdirAll(m.create, 0o755); err != nil {
			return execworkspace.Result{}, err
		}
	}
	return m.result, nil
}

func (m *recordingMaterializer) Materialize(_ context.Context, request execworkspace.Request) (execworkspace.Result, error) {
	if _, err := os.Lstat(m.receiptPath); err != nil {
		return execworkspace.Result{}, fmt.Errorf("receipt was not published before materialization: %w", err)
	}
	workspaceID, err := request.Identity.WorkspaceID()
	if err != nil {
		return execworkspace.Result{}, err
	}
	path := execworkspace.UnitPath(m.root, workspaceID)
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return execworkspace.Result{}, fmt.Errorf("workspace %q existed before materializer effect: %w", path, err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return execworkspace.Result{}, err
	}
	m.requests = append(m.requests, request)
	m.paths = append(m.paths, path)
	return execworkspace.Result{WorkspaceID: workspaceID, Path: path, Outcome: execworkspace.OutcomeMaterialized}, nil
}

type recordingEvaluator struct {
	requests []experimentevaluator.ObserveInput
}

func (e *recordingEvaluator) Observe(ctx context.Context, profile execworkspace.Profile, input experimentevaluator.ObserveInput) (experimentevaluator.Attempt, error) {
	cmd, _, cancel, err := profile.Command(ctx, input.Launch.Argv[0])
	if err != nil {
		return experimentevaluator.Attempt{}, fmt.Errorf("profile must be activated before evaluator use: %w", err)
	}
	if cmd == nil {
		cancel()
		return experimentevaluator.Attempt{}, errors.New("activated profile returned nil command")
	}
	cancel()
	e.requests = append(e.requests, input)
	if input.Request.Cycle.Kind == experiment.CycleWarmup && input.Request.Candidate == "beta" {
		witness := "warmup candidate timed out"
		return experimentevaluator.Attempt{Outcome: experiment.CandidateOutcome{Kind: experiment.OutcomeCandidateTimeout, Witness: &witness}}, nil
	}
	outcome := experiment.CandidateOutcome{Kind: experiment.OutcomeCompleted}
	if input.Request.Cycle.Kind == experiment.CycleWarmup {
		return experimentevaluator.Attempt{Outcome: outcome}, nil
	}
	observation := experiment.Observation{
		Schema:           experiment.ObservationSchemaV2,
		ExperimentDigest: input.Request.ExperimentDigest,
		Run:              input.Request.Run,
		Candidate:        input.Request.Candidate,
		Round:            input.Request.Cycle.Number,
		Outcome:          &outcome,
		Guards:           []experiment.GuardResult{{ID: "correctness", Verdict: experiment.GuardVerdictPass}},
		Measurements: []experiment.Measurement{
			{ID: "latency", Value: experiment.NumberValue("10"), Unit: "ms", Source: experiment.SourceEvaluatorMeasured},
			{ID: "memory", Value: experiment.NumberValue("1"), Unit: "bytes", Source: experiment.SourceEvaluatorMeasured},
		},
		Disclosures: []string{},
	}
	return experimentevaluator.Attempt{Outcome: outcome, Observation: &observation}, nil
}

func writeStartAuthority(t *testing.T, root, experimentDir string, capabilities []byte, patches map[string][]byte) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(experimentDir))
	if err := os.MkdirAll(filepath.Join(dir, "candidates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evaluator-capabilities.json"), capabilities, 0o600); err != nil {
		t.Fatal(err)
	}
	for candidate, patch := range patches {
		if err := os.WriteFile(filepath.Join(dir, "candidates", candidate+".patch"), patch, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
