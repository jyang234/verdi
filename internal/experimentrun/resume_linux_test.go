//go:build linux

package experimentrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentevaluator"
)

func TestStartPreservesFixturePresenceInEvaluatorRequests(t *testing.T) {
	for _, fixtureCount := range []int{0, 1} {
		t.Run(fmt.Sprintf("%d fixtures", fixtureCount), func(t *testing.T) {
			fixture := newRunFixture(t, "run-1", 0)
			if fixtureCount == 0 {
				makeRunFixtureFixtureless(t, &fixture)
			}
			evaluator := &recordingEvaluator{}
			service := newResumeTestService(t, fixture, evaluator)

			if _, err := service.Start(context.Background(), fixture.request); err != nil {
				t.Fatalf("Start: %v", err)
			}
			assertEvaluatorRequestFixtures(t, evaluator.requests, fixture.request.Definition.Fixtures)
		})
	}
}

func TestResumePreservesFixturePresenceInEvaluatorRequests(t *testing.T) {
	for _, fixtureCount := range []int{0, 1} {
		t.Run(fmt.Sprintf("%d fixtures", fixtureCount), func(t *testing.T) {
			fixture := newRunFixture(t, "run-1", 0)
			if fixtureCount == 0 {
				makeRunFixtureFixtureless(t, &fixture)
			}
			interruption := errors.New("interrupt before first measured observation")
			starter := newResumeTestService(t, fixture, &interruptingEvaluator{
				delegate:  &recordingEvaluator{},
				kind:      experiment.CycleMeasured,
				candidate: "alpha",
				round:     1,
				err:       interruption,
			})
			if _, err := starter.Start(context.Background(), fixture.request); !errors.Is(err, interruption) {
				t.Fatalf("Start interruption error = %v, want %v", err, interruption)
			}

			evaluator := &recordingEvaluator{}
			service := newResumeTestService(t, fixture, evaluator)
			if _, err := service.Resume(context.Background(), ResumeRequest(fixture.request)); err != nil {
				t.Fatalf("Resume: %v", err)
			}
			assertEvaluatorRequestFixtures(t, evaluator.requests, fixture.request.Definition.Fixtures)
		})
	}
}

func TestStartZeroWarmupsActivatesEveryCandidateBeforeFirstObservation(t *testing.T) {
	fixture := newRunFixture(t, "run-1", 0)
	interruption := errors.New("interrupt after first measured append")
	evaluator := &allCandidatesActiveEvaluator{
		delegate:       &recordingEvaluator{},
		root:           fixture.request.Root,
		materializer:   fixture.materializer,
		wantCandidates: len(fixture.request.Definition.Candidates),
		interrupt: interruptPoint{
			kind:      experiment.CycleMeasured,
			candidate: "beta",
			round:     1,
			err:       interruption,
		},
	}
	service := newResumeTestService(t, fixture, evaluator)

	if _, err := service.Start(context.Background(), fixture.request); !errors.Is(err, interruption) {
		t.Fatalf("Start interruption error = %v, want %v", err, interruption)
	}
	storage, err := newRunStorage(fixture.request.Root, fixture.request.ExperimentDir, fixture.request.Run)
	if err != nil {
		t.Fatal(err)
	}
	prefix, err := storage.loadMeasuredPrefix(fixture.request.Definition, mustSchedule(t, fixture.request.Definition))
	if err != nil {
		t.Fatal(err)
	}
	if len(prefix) != 1 || prefix[0].Candidate != "alpha" || prefix[0].Round != 1 {
		t.Fatalf("interrupted measured prefix = %#v, want only alpha@1", prefix)
	}
	if len(fixture.materializer.paths) != len(fixture.request.Definition.Candidates) {
		t.Fatalf("candidate materializations before first append = %d, want %d", len(fixture.materializer.paths), len(fixture.request.Definition.Candidates))
	}

	resumeEvaluator := &recordingEvaluator{}
	service = newResumeTestService(t, fixture, resumeEvaluator)
	result, err := service.Resume(context.Background(), ResumeRequest(fixture.request))
	if err != nil {
		t.Fatalf("Resume unchanged zero-warmup interruption: %v", err)
	}
	if len(resumeEvaluator.requests) != 3 {
		t.Fatalf("resumed evaluator calls = %d, want three missing measured attempts", len(resumeEvaluator.requests))
	}
	if len(result.Observations) != 4 || result.Result.Schema != experiment.ResultSchemaV2 {
		t.Fatalf("resumed result observations/schema = %d/%q, want 4/%q", len(result.Observations), result.Result.Schema, experiment.ResultSchemaV2)
	}
}

func TestResumeUnchangedRunRestartsWarmupsAndExecutesOnlyMissingMeasuredTail(t *testing.T) {
	fixture := newInterruptedRun(t, "run-1")
	resumeEvaluator := &recordingEvaluator{}
	service := newResumeTestService(t, fixture, resumeEvaluator)

	result, err := service.Resume(context.Background(), ResumeRequest(fixture.request))
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if len(resumeEvaluator.requests) != 5 {
		t.Fatalf("resume evaluator calls = %d, want two restarted warmups plus three missing measured attempts", len(resumeEvaluator.requests))
	}
	for _, request := range resumeEvaluator.requests {
		if request.Request.Cycle.Kind == experiment.CycleMeasured && request.Request.Candidate == "beta" && request.Request.Cycle.Number == 1 {
			t.Fatal("resume re-executed the already-published beta@1 observation")
		}
	}
	if result.Result.Schema != experiment.ResultSchemaV2 || result.ResultDigest == "" {
		t.Fatalf("resume result = %#v digest=%q", result.Result, result.ResultDigest)
	}
	if got := result.Result.Execution.WarmupDiagnostics.Failures; len(got) != 1 || got[0].Candidate != "beta" || got[0].Witness != "warmup candidate timed out" {
		t.Fatalf("final-invocation warmup diagnostics = %#v", got)
	}
	if result.Result.Execution.WarmupDiagnostics.Failures[0].Witness == "initial invocation timeout" {
		t.Fatal("result retained a warmup diagnostic from the interrupted invocation")
	}
	if len(result.Observations) != 4 {
		t.Fatalf("complete observations = %d, want 4", len(result.Observations))
	}
	for _, path := range fixture.materializer.paths {
		if _, statErr := os.Lstat(filepath.Join(path, environmentRootName)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("completed resume left environment root below %q: %v", path, statErr)
		}
	}
	paths, _ := experiment.PathsForRun(fixture.request.ExperimentDir, fixture.request.Run)
	if _, statErr := os.Lstat(filepath.Join(fixture.request.Root, filepath.FromSlash(paths.Result))); statErr != nil {
		t.Fatalf("completed resume did not publish result: %v", statErr)
	}
}

func TestResumeCompleteResultIsIdempotentWithoutSelectingOrReexecuting(t *testing.T) {
	fixture := newInterruptedRun(t, "run-1")
	firstEvaluator := &recordingEvaluator{}
	service := newResumeTestService(t, fixture, firstEvaluator)
	first, err := service.Resume(context.Background(), ResumeRequest(fixture.request))
	if err != nil {
		t.Fatal(err)
	}

	secondEvaluator := &recordingEvaluator{}
	service = newResumeTestService(t, fixture, secondEvaluator)
	second, err := service.Resume(context.Background(), ResumeRequest(fixture.request))
	if err != nil {
		t.Fatalf("idempotent Resume: %v", err)
	}
	if len(secondEvaluator.requests) != 0 {
		t.Fatalf("complete idempotent resume executed %d attempts", len(secondEvaluator.requests))
	}
	firstBytes, _ := experiment.EncodeResult(first.Result)
	secondBytes, _ := experiment.EncodeResult(second.Result)
	if string(firstBytes) != string(secondBytes) || first.ResultDigest != second.ResultDigest {
		t.Fatalf("idempotent result differs:\nfirst=%s%s\nsecond=%s%s", firstBytes, first.ResultDigest, secondBytes, second.ResultDigest)
	}
}

func TestResumeCompleteResultRequiresEnvironmentRootsToRemainAbsent(t *testing.T) {
	fixture := newInterruptedRun(t, "run-1")
	service := newResumeTestService(t, fixture, &recordingEvaluator{})
	if _, err := service.Resume(context.Background(), ResumeRequest(fixture.request)); err != nil {
		t.Fatal(err)
	}
	environment := filepath.Join(fixture.materializer.paths[0], environmentRootName)
	if err := os.MkdirAll(environment, 0o700); err != nil {
		t.Fatal(err)
	}
	evaluator := &recordingEvaluator{}
	service = newResumeTestService(t, fixture, evaluator)
	if _, err := service.Resume(context.Background(), ResumeRequest(fixture.request)); err == nil {
		t.Fatal("Resume complete result with recreated environment root = nil error")
	}
	if len(evaluator.requests) != 0 {
		t.Fatalf("complete result with present environment executed %d attempts", len(evaluator.requests))
	}
}

func TestResumeRejectsMissingEnvironmentAfterMeasuredEvidenceWithoutResult(t *testing.T) {
	fixture := newInterruptedRun(t, "run-1")
	if err := os.RemoveAll(filepath.Join(fixture.materializer.paths[0], environmentRootName)); err != nil {
		t.Fatal(err)
	}
	evaluator := &recordingEvaluator{}
	service := newResumeTestService(t, fixture, evaluator)
	if _, err := service.Resume(context.Background(), ResumeRequest(fixture.request)); err == nil {
		t.Fatal("Resume with missing measured environment root = nil error")
	}
	if len(evaluator.requests) != 0 {
		t.Fatalf("changed environment executed %d attempts", len(evaluator.requests))
	}
	paths, _ := experiment.PathsForRun(fixture.request.ExperimentDir, fixture.request.Run)
	if _, statErr := os.Lstat(filepath.Join(fixture.request.Root, filepath.FromSlash(paths.Result))); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("changed environment emitted result: %v", statErr)
	}
}

func TestResumeWithoutMeasuredEvidenceRecreatesOnlyMissingEnvironmentAndRestartsWarmups(t *testing.T) {
	fixture := newInterruptedRunAt(t, "run-1", experiment.CycleWarmup, "alpha", 1)
	if len(fixture.materializer.paths) != 2 {
		t.Fatalf("warmup interruption materialized %d candidates, want every candidate before the first warmup", len(fixture.materializer.paths))
	}
	evaluator := &recordingEvaluator{}
	service := newResumeTestService(t, fixture, evaluator)
	result, err := service.Resume(context.Background(), ResumeRequest(fixture.request))
	if err != nil {
		t.Fatalf("Resume after warmup interruption: %v", err)
	}
	if len(result.Observations) != 4 || len(evaluator.requests) != 6 {
		t.Fatalf("resumed execution observations/calls = %d/%d, want 4/6", len(result.Observations), len(evaluator.requests))
	}
}

func TestResumeWithoutMeasuredEvidenceRejectsEmptyEnvironmentCollision(t *testing.T) {
	fixture := newInterruptedRunAt(t, "run-1", experiment.CycleWarmup, "alpha", 1)
	digest, err := experiment.DefinitionDigest(fixture.request.Definition)
	if err != nil {
		t.Fatal(err)
	}
	workspaceRunID, err := experiment.WorkspaceRunID(digest, fixture.request.Run, "beta")
	if err != nil {
		t.Fatal(err)
	}
	patch := candidatePatches(t, fixture.request.Definition)["beta"]
	identity, err := execworkspace.NewPatchIdentity(workspaceRunID, fixture.request.Definition.Candidates[1].Base, patch)
	if err != nil {
		t.Fatal(err)
	}
	workspaceID, err := identity.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	collision := filepath.Join(execworkspace.UnitPath(fixture.request.Root, workspaceID), environmentRootName)
	if err := os.RemoveAll(collision); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(collision, 0o700); err != nil {
		t.Fatal(err)
	}
	evaluator := &recordingEvaluator{}
	service := newResumeTestService(t, fixture, evaluator)
	if _, err := service.Resume(context.Background(), ResumeRequest(fixture.request)); err == nil {
		t.Fatal("Resume empty environment collision = nil error")
	}
	if len(evaluator.requests) != 0 {
		t.Fatalf("environment collision executed %d attempts", len(evaluator.requests))
	}
}

func TestResumeCompleteSetRefusesCleanupCollisionAndEmitsNoResult(t *testing.T) {
	fixture := newInterruptedRun(t, "run-1")
	storage, err := newRunStorage(fixture.request.Root, fixture.request.ExperimentDir, fixture.request.Run)
	if err != nil {
		t.Fatal(err)
	}
	schedule := mustSchedule(t, fixture.request.Definition)
	prefix, err := storage.loadMeasuredPrefix(fixture.request.Definition, schedule)
	if err != nil {
		t.Fatal(err)
	}
	for _, scheduled := range measuredSchedule(schedule)[len(prefix):] {
		if err := storage.appendObservation(fixture.request.Definition, schedule, storageObservation(t, fixture.request.Definition, fixture.request.Run, scheduled), experimentevaluator.HardResponseBytes); err != nil {
			t.Fatal(err)
		}
	}
	environment := filepath.Join(fixture.materializer.paths[0], environmentRootName)
	if err := os.RemoveAll(environment); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(environment, []byte("cleanup collision"), 0o600); err != nil {
		t.Fatal(err)
	}
	evaluator := &recordingEvaluator{}
	service := newResumeTestService(t, fixture, evaluator)
	if _, err := service.Resume(context.Background(), ResumeRequest(fixture.request)); err == nil {
		t.Fatal("Resume cleanup collision = nil error")
	}
	if len(evaluator.requests) != 0 {
		t.Fatalf("complete set executed %d attempts before cleanup refusal", len(evaluator.requests))
	}
	if _, present, err := storage.loadResult(); err != nil || present {
		t.Fatalf("cleanup failure result = present %t, error %v", present, err)
	}
}

func TestResumeCompleteMeasuredPrefixRerunsWarmupsWithFinalInvocationDiagnostics(t *testing.T) {
	fixture := newInterruptedRun(t, "run-1")
	completeMeasuredPrefix(t, fixture)
	evaluator := &recordingEvaluator{warmupWitness: "final cleanup-boundary timeout"}
	service := newResumeTestService(t, fixture, evaluator)

	result, err := service.Resume(context.Background(), ResumeRequest(fixture.request))
	if err != nil {
		t.Fatalf("Resume complete prefix without result: %v", err)
	}
	if len(evaluator.requests) != 2 {
		t.Fatalf("complete-prefix evaluator calls = %d, want two warmups and no measured attempts", len(evaluator.requests))
	}
	for _, request := range evaluator.requests {
		if request.Request.Cycle.Kind != experiment.CycleWarmup {
			t.Fatalf("complete-prefix Resume executed non-warmup attempt %#v", request.Request.Cycle)
		}
	}
	wantWitness := "final cleanup-boundary timeout"
	if got := result.Result.Execution.WarmupDiagnostics.Failures; len(got) != 1 || got[0].Candidate != "beta" || got[0].Witness != wantWitness {
		t.Fatalf("final-invocation warmup diagnostics = %#v, want beta witness %q", got, wantWitness)
	}
	if got := result.WarmupFailures; len(got) != 1 || got[0].Witness != wantWitness {
		t.Fatalf("returned warmup diagnostics = %#v, want exact final invocation", got)
	}
}

func TestResumeCompleteMeasuredPrefixRecreatesMissingProfilesForCleanupRetry(t *testing.T) {
	for _, test := range []struct {
		name          string
		removeIndexes []int
	}{
		{name: "all roots absent", removeIndexes: []int{0, 1}},
		{name: "mixed absent and activated roots", removeIndexes: []int{0}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newInterruptedRun(t, "run-1")
			completeMeasuredPrefix(t, fixture)
			for _, index := range test.removeIndexes {
				if err := os.RemoveAll(filepath.Join(fixture.materializer.paths[index], environmentRootName)); err != nil {
					t.Fatal(err)
				}
			}
			evaluator := &recordingEvaluator{warmupWitness: "cleanup retry timeout"}
			service := newResumeTestService(t, fixture, evaluator)

			result, err := service.Resume(context.Background(), ResumeRequest(fixture.request))
			if err != nil {
				t.Fatalf("Resume cleanup retry: %v", err)
			}
			if len(evaluator.requests) != 2 {
				t.Fatalf("cleanup-retry evaluator calls = %d, want two warmups", len(evaluator.requests))
			}
			if got := result.Result.Execution.WarmupDiagnostics.Failures; len(got) != 1 || got[0].Witness != "cleanup retry timeout" {
				t.Fatalf("cleanup-retry diagnostics = %#v", got)
			}
			for _, path := range fixture.materializer.paths[:len(fixture.request.Definition.Candidates)] {
				if _, statErr := os.Lstat(filepath.Join(path, environmentRootName)); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("cleanup retry left environment root below %q: %v", path, statErr)
				}
			}
		})
	}
}

func TestResumeCompleteMeasuredPrefixRefusesMalformedExistingRoots(t *testing.T) {
	for _, test := range []struct {
		name string
		make func(*testing.T, string)
	}{
		{name: "empty", make: func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "partial", make: func(t *testing.T, path string) {
			if err := os.MkdirAll(filepath.Join(path, ".home"), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "symlink", make: func(t *testing.T, path string) {
			if err := os.Symlink(t.TempDir(), path); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "non-directory", make: func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("collision"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "foreign top-level entry", make: func(t *testing.T, path string) {
			makeActivatedEnvironmentRoot(t, path)
			if err := os.WriteFile(filepath.Join(path, "foreign"), []byte("collision"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newInterruptedRun(t, "run-1")
			completeMeasuredPrefix(t, fixture)
			environment := filepath.Join(fixture.materializer.paths[0], environmentRootName)
			if err := os.RemoveAll(environment); err != nil {
				t.Fatal(err)
			}
			test.make(t, environment)
			evaluator := &recordingEvaluator{}
			service := newResumeTestService(t, fixture, evaluator)

			if _, err := service.Resume(context.Background(), ResumeRequest(fixture.request)); err == nil {
				t.Fatal("Resume malformed cleanup-boundary root = nil error")
			}
			if len(evaluator.requests) != 0 {
				t.Fatalf("malformed cleanup-boundary root executed %d attempts", len(evaluator.requests))
			}
			if _, statErr := os.Lstat(environment); statErr != nil {
				t.Fatalf("malformed cleanup-boundary root was changed or removed: %v", statErr)
			}
			storage, err := newRunStorage(fixture.request.Root, fixture.request.ExperimentDir, fixture.request.Run)
			if err != nil {
				t.Fatal(err)
			}
			if _, present, err := storage.loadResult(); err != nil || present {
				t.Fatalf("malformed root result = present %t, error %v", present, err)
			}
		})
	}
}

func TestResumeRejectsChangedReceiptInputsBeforeExecution(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, *interruptedRunFixture)
	}{
		{name: "definition", mutate: func(t *testing.T, f *interruptedRunFixture) {
			f.request.Definition.Question = "spec/question#oq-two"
			f.request.Definition = relockDefinition(t, f.request.Definition)
		}},
		{name: "capabilities", mutate: func(t *testing.T, f *interruptedRunFixture) {
			f.capabilities.EvaluatorVersion = "evaluator-test/changed"
			data, err := canonjson.Marshal(f.capabilities)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(f.request.Root, filepath.FromSlash(f.request.ExperimentDir), "evaluator-capabilities.json")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "authorization", mutate: func(_ *testing.T, f *interruptedRunFixture) {
			f.authorization.AuthorityDigest = digestText("changed-authority")
		}},
		{name: "grants", mutate: func(t *testing.T, f *interruptedRunFixture) {
			data, err := execworkspace.EncodeGrantSet(testGrants(t, true, f.request.Definition.Evaluator.Argv[0], 29))
			if err != nil {
				t.Fatal(err)
			}
			f.authorization.GrantBytes = data
		}},
		{name: "declared environment", mutate: func(_ *testing.T, f *interruptedRunFixture) {
			f.authorization.DeclaredEnv["LANG"] = "changed"
		}},
		{name: "fingerprint version", mutate: func(_ *testing.T, f *interruptedRunFixture) {
			f.versions.Verdi = "v-changed"
		}},
		{name: "schedule", mutate: func(t *testing.T, f *interruptedRunFixture) {
			f.request.Definition.Execution.Warmups++
			f.request.Definition = relockDefinition(t, f.request.Definition)
		}},
		{name: "resolved input bytes", mutate: func(t *testing.T, f *interruptedRunFixture) {
			if err := os.WriteFile(filepath.Join(f.request.Root, "inputs", "workload.json"), []byte("changed"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "candidate identity", mutate: func(t *testing.T, f *interruptedRunFixture) {
			patch := []byte("diff --git a/beta-changed b/beta-changed\n")
			f.request.Definition.Candidates[1].Digest = testDigestBytes(patch)
			f.request.Definition = relockDefinition(t, f.request.Definition)
			path := filepath.Join(f.request.Root, filepath.FromSlash(f.request.ExperimentDir), "candidates", "beta.patch")
			if err := os.WriteFile(path, patch, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newInterruptedRun(t, "run-1")
			test.mutate(t, &fixture)
			evaluator := &recordingEvaluator{}
			service := newResumeTestService(t, fixture, evaluator)
			if _, err := service.Resume(context.Background(), ResumeRequest(fixture.request)); err == nil {
				t.Fatal("Resume changed receipt input = nil error")
			}
			if len(evaluator.requests) != 0 {
				t.Fatalf("changed receipt input executed %d attempts", len(evaluator.requests))
			}
			paths, _ := experiment.PathsForRun(fixture.request.ExperimentDir, fixture.request.Run)
			if _, statErr := os.Lstat(filepath.Join(fixture.request.Root, filepath.FromSlash(paths.Result))); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("changed receipt input emitted result: %v", statErr)
			}
		})
	}
}

func TestResumeRejectsChangedInputSlotPathsWhenDigestsEqual(t *testing.T) {
	fixture := newRunFixture(t, "run-1", 0)
	sharedBytes := []byte("shared workload and contract bytes")
	sharedDigest := testDigestBytes(sharedBytes)
	fixture.request.Definition.Workload.Digest = sharedDigest
	fixture.request.Definition.Contract.Digest = sharedDigest
	fixture.request.Definition = relockDefinition(t, fixture.request.Definition)
	fixture.authorization = testAuthorization(t, fixture.request.Definition, true)
	for _, relative := range []string{"inputs/workload.json", "contracts/behavioral.json"} {
		if err := os.WriteFile(filepath.Join(fixture.request.Root, filepath.FromSlash(relative)), sharedBytes, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fixture.inputs.values[fixture.request.Definition.Workload.ID] = ResolvedInput{
		ID: fixture.request.Definition.Workload.ID, Path: "inputs/workload.json", Digest: sharedDigest,
	}
	fixture.inputs.values[fixture.request.Definition.Contract.ID] = ResolvedInput{
		ID: fixture.request.Definition.Contract.ID, Path: "contracts/behavioral.json", Digest: sharedDigest,
	}

	interruption := errors.New("interrupt before the first measured observation")
	starter := newResumeTestService(t, fixture, &interruptingEvaluator{
		delegate: &recordingEvaluator{}, kind: experiment.CycleMeasured,
		candidate: "alpha", round: 1, err: interruption,
	})
	if _, err := starter.Start(context.Background(), fixture.request); !errors.Is(err, interruption) {
		t.Fatalf("Start interruption error = %v, want %v", err, interruption)
	}

	fixture.inputs.values[fixture.request.Definition.Workload.ID] = ResolvedInput{
		ID: fixture.request.Definition.Workload.ID, Path: "contracts/behavioral.json", Digest: sharedDigest,
	}
	fixture.inputs.values[fixture.request.Definition.Contract.ID] = ResolvedInput{
		ID: fixture.request.Definition.Contract.ID, Path: "inputs/workload.json", Digest: sharedDigest,
	}
	evaluator := &recordingEvaluator{}
	service := newResumeTestService(t, fixture, evaluator)
	if _, err := service.Resume(context.Background(), ResumeRequest(fixture.request)); err == nil {
		t.Fatal("Resume with workload/contract paths exchanged under one shared digest = nil error")
	}
	if len(evaluator.requests) != 0 {
		t.Fatalf("changed slot-to-path custody executed %d attempts", len(evaluator.requests))
	}
}

func TestStartKeepsCompleteRerunsSeparateWithoutPreferredPointer(t *testing.T) {
	root := t.TempDir()
	const experimentDir = "experiments/comparison"
	def, capabilities, _ := testDefinition(t, []string{"alpha", "beta"}, 1)
	capabilities.RequiresNetwork = true
	capabilitiesBytes, err := canonjson.Marshal(capabilities)
	if err != nil {
		t.Fatal(err)
	}
	def.Evaluator.CapabilitiesDigest = testDigestBytes(capabilitiesBytes)
	def = relockDefinition(t, def)
	writeStartAuthority(t, root, experimentDir, capabilitiesBytes, candidatePatches(t, def))
	writeResolvedInputs(t, root, def)
	inputs := staticInputs{values: map[string]ResolvedInput{
		def.Workload.ID:    {ID: def.Workload.ID, Path: "inputs/workload.json", Digest: def.Workload.Digest},
		def.Fixtures[0].ID: {ID: def.Fixtures[0].ID, Path: "fixtures/request-log.json", Digest: def.Fixtures[0].Digest},
		def.Contract.ID:    {ID: def.Contract.ID, Path: "contracts/behavioral.json", Digest: def.Contract.Digest},
	}}
	var workspaceRunIDs []string
	for _, run := range []string{"run-2", "run-1"} {
		paths, _ := experiment.PathsForRun(experimentDir, run)
		materializer := &resumeMaterializer{root: root, receiptPath: filepath.Join(root, filepath.FromSlash(paths.Execution))}
		service, err := NewService(ServiceDependencies{
			Authorization: staticAuthorization{authorization: testAuthorization(t, def, true)}, Inputs: inputs,
			Materializer: materializer, Evaluator: &recordingEvaluator{},
			Versions: experiment.ReceiptVersions{Verdi: "v-test", RecommendationEngine: string(def.Algorithm)},
		})
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.Start(context.Background(), StartRequest{Root: root, ExperimentDir: experimentDir, Run: run, Definition: def})
		if err != nil {
			t.Fatalf("Start(%s): %v", run, err)
		}
		workspaceRunIDs = append(workspaceRunIDs, result.Receipt.Candidates[0].WorkspaceRunID)
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(paths.Result))); err != nil {
			t.Fatalf("result for %s: %v", run, err)
		}
	}
	if workspaceRunIDs[0] == workspaceRunIDs[1] {
		t.Fatalf("reruns share candidate workspace run identity %q", workspaceRunIDs[0])
	}
	runsPath := filepath.Join(root, filepath.FromSlash(experimentDir), "runs")
	entries, err := os.ReadDir(runsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "run-1" || entries[1].Name() != "run-2" {
		t.Fatalf("canonical run enumeration = %#v", entries)
	}
	for _, forbidden := range []string{"latest", "preferred"} {
		if _, err := os.Lstat(filepath.Join(runsPath, forbidden)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("service created forbidden %s pointer: %v", forbidden, err)
		}
	}
}

type interruptedRunFixture struct {
	request       StartRequest
	capabilities  experiment.Capabilities
	authorization ExecutionAuthorization
	inputs        staticInputs
	materializer  *resumeMaterializer
	versions      experiment.ReceiptVersions
}

func newInterruptedRun(t *testing.T, run string) interruptedRunFixture {
	return newInterruptedRunAt(t, run, experiment.CycleMeasured, "alpha", 1)
}

func newInterruptedRunAt(t *testing.T, run string, kind experiment.CycleKind, candidate string, round int) interruptedRunFixture {
	t.Helper()
	fixture := newRunFixture(t, run, 1)
	interrupt := &interruptingEvaluator{delegate: &recordingEvaluator{warmupWitness: "initial invocation timeout"}, kind: kind, candidate: candidate, round: round, err: errors.New("interrupt execution")}
	service := newResumeTestService(t, fixture, interrupt)
	if _, err := service.Start(context.Background(), fixture.request); err == nil {
		t.Fatal("Start interruption = nil error")
	}
	storage, _ := newRunStorage(fixture.request.Root, fixture.request.ExperimentDir, fixture.request.Run)
	prefix, err := storage.loadMeasuredPrefix(fixture.request.Definition, mustSchedule(t, fixture.request.Definition))
	wantPrefix := 1
	if kind == experiment.CycleWarmup {
		wantPrefix = 0
	}
	if err != nil || len(prefix) != wantPrefix {
		t.Fatalf("interrupted measured prefix = %#v, %v; want %d rows", prefix, err, wantPrefix)
	}
	if _, present, err := storage.loadResult(); err != nil || present {
		t.Fatalf("incomplete run result = present %t, error %v; want no result", present, err)
	}
	return fixture
}

func newRunFixture(t *testing.T, run string, warmups int) interruptedRunFixture {
	t.Helper()
	root := t.TempDir()
	const experimentDir = "experiments/comparison"
	def, capabilities, _ := testDefinition(t, []string{"alpha", "beta"}, warmups)
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
	paths, _ := experiment.PathsForRun(experimentDir, run)
	materializer := &resumeMaterializer{root: root, receiptPath: filepath.Join(root, filepath.FromSlash(paths.Execution))}
	fixture := interruptedRunFixture{
		request:       StartRequest{Root: root, ExperimentDir: experimentDir, Run: run, Definition: def},
		capabilities:  capabilities,
		authorization: testAuthorization(t, def, true),
		inputs:        inputs,
		materializer:  materializer,
		versions:      experiment.ReceiptVersions{Verdi: "v-test", RecommendationEngine: string(def.Algorithm)},
	}
	return fixture
}

func makeRunFixtureFixtureless(t *testing.T, fixture *interruptedRunFixture) {
	t.Helper()
	def := fixture.request.Definition
	fixtureID := def.Fixtures[0].ID
	def.Fixtures = []experiment.ArtifactRef{}
	protected := make([]string, 0, len(def.ProtectedPaths)-1)
	for _, path := range def.ProtectedPaths {
		if path != "fixtures/request-log.json" {
			protected = append(protected, path)
		}
	}
	def.ProtectedPaths = protected
	def = relockDefinition(t, def)
	fixture.request.Definition = def
	fixture.authorization = testAuthorization(t, def, true)
	delete(fixture.inputs.values, fixtureID)
}

func assertEvaluatorRequestFixtures(t *testing.T, requests []experimentevaluator.ObserveInput, want []experiment.ArtifactRef) {
	t.Helper()
	if len(requests) == 0 {
		t.Fatal("evaluator received no requests")
	}
	for i, request := range requests {
		got := request.Request.Fixtures
		if got == nil {
			t.Fatalf("evaluator request %d fixtures = nil, want present array", i)
		}
		if len(got) != len(want) {
			t.Fatalf("evaluator request %d fixture count = %d, want %d", i, len(got), len(want))
		}
		for fixtureIndex := range want {
			if got[fixtureIndex].ID != want[fixtureIndex].ID || got[fixtureIndex].Digest != want[fixtureIndex].Digest {
				t.Fatalf("evaluator request %d fixture %d = %#v, want id/digest %#v", i, fixtureIndex, got[fixtureIndex], want[fixtureIndex])
			}
		}
	}
}

func completeMeasuredPrefix(t *testing.T, fixture interruptedRunFixture) {
	t.Helper()
	storage, err := newRunStorage(fixture.request.Root, fixture.request.ExperimentDir, fixture.request.Run)
	if err != nil {
		t.Fatal(err)
	}
	schedule := mustSchedule(t, fixture.request.Definition)
	prefix, err := storage.loadMeasuredPrefix(fixture.request.Definition, schedule)
	if err != nil {
		t.Fatal(err)
	}
	for _, scheduled := range measuredSchedule(schedule)[len(prefix):] {
		if err := storage.appendObservation(fixture.request.Definition, schedule, storageObservation(t, fixture.request.Definition, fixture.request.Run, scheduled), experimentevaluator.HardResponseBytes); err != nil {
			t.Fatal(err)
		}
	}
}

func makeActivatedEnvironmentRoot(t *testing.T, path string) {
	t.Helper()
	for _, relative := range []string{filepath.Join(".home", ".config"), filepath.Join(".home", ".cache"), ".tmp"} {
		if err := os.MkdirAll(filepath.Join(path, relative), 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func newResumeTestService(t *testing.T, fixture interruptedRunFixture, evaluator AttemptEvaluator) *Service {
	t.Helper()
	service, err := NewService(ServiceDependencies{
		Authorization: staticAuthorization{authorization: fixture.authorization},
		Inputs:        fixture.inputs,
		Materializer:  fixture.materializer,
		Evaluator:     evaluator,
		Versions:      fixture.versions,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type interruptingEvaluator struct {
	delegate  AttemptEvaluator
	kind      experiment.CycleKind
	candidate string
	round     int
	err       error
}

type interruptPoint struct {
	kind      experiment.CycleKind
	candidate string
	round     int
	err       error
}

type allCandidatesActiveEvaluator struct {
	delegate       AttemptEvaluator
	root           string
	materializer   *resumeMaterializer
	wantCandidates int
	interrupt      interruptPoint
}

func (e *allCandidatesActiveEvaluator) Observe(ctx context.Context, profile execworkspace.Profile, input experimentevaluator.ObserveInput) (experimentevaluator.Attempt, error) {
	if len(e.materializer.paths) != e.wantCandidates {
		return experimentevaluator.Attempt{}, fmt.Errorf("candidate profiles are still lazy: materialized %d of %d before %s %s@%d", len(e.materializer.paths), e.wantCandidates, input.Request.Cycle.Kind, input.Request.Candidate, input.Request.Cycle.Number)
	}
	for _, workspace := range e.materializer.paths {
		environment := filepath.Join(workspace, environmentRootName)
		if err := validateResumeEnvironmentRoot(e.root, environment, true); err != nil {
			return experimentevaluator.Attempt{}, fmt.Errorf("candidate profile %q not activated before observation: %w", environment, err)
		}
	}
	if input.Request.Cycle.Kind == e.interrupt.kind && input.Request.Candidate == e.interrupt.candidate && input.Request.Cycle.Number == e.interrupt.round {
		return experimentevaluator.Attempt{}, e.interrupt.err
	}
	return e.delegate.Observe(ctx, profile, input)
}

func (e *interruptingEvaluator) Observe(ctx context.Context, profile execworkspace.Profile, input experimentevaluator.ObserveInput) (experimentevaluator.Attempt, error) {
	if input.Request.Cycle.Kind == e.kind && input.Request.Candidate == e.candidate && input.Request.Cycle.Number == e.round {
		return experimentevaluator.Attempt{}, e.err
	}
	return e.delegate.Observe(ctx, profile, input)
}

type resumeMaterializer struct {
	root        string
	receiptPath string
	paths       []string
}

func (m *resumeMaterializer) Materialize(_ context.Context, request execworkspace.Request) (execworkspace.Result, error) {
	if _, err := os.Lstat(m.receiptPath); err != nil {
		return execworkspace.Result{}, fmt.Errorf("receipt absent before materialization: %w", err)
	}
	workspaceID, err := request.Identity.WorkspaceID()
	if err != nil {
		return execworkspace.Result{}, err
	}
	path := execworkspace.UnitPath(m.root, workspaceID)
	outcome := execworkspace.OutcomeReused
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return execworkspace.Result{}, err
		}
		outcome = execworkspace.OutcomeMaterialized
	} else if err != nil {
		return execworkspace.Result{}, err
	}
	seen := false
	for _, existing := range m.paths {
		if existing == path {
			seen = true
			break
		}
	}
	if !seen {
		m.paths = append(m.paths, path)
	}
	return execworkspace.Result{WorkspaceID: workspaceID, Path: path, Outcome: outcome}, nil
}

func mustSchedule(t *testing.T, def experiment.Definition) []ScheduledAttempt {
	t.Helper()
	schedule, err := DeriveSchedule(def)
	if err != nil {
		t.Fatal(err)
	}
	return schedule
}
