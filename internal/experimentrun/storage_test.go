package experimentrun

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/filelock"
)

func TestRunStorageCreatesMissingWriterLockParent(t *testing.T) {
	root := t.TempDir()
	storage, err := newRunStorage(root, "experiments/comparison", "run-1")
	if err != nil {
		t.Fatalf("newRunStorage: %v", err)
	}

	called := false
	if err := storage.withWriterLock(func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("withWriterLock: %v", err)
	}
	if !called {
		t.Fatal("withWriterLock did not call its operation")
	}
	if _, err := os.Lstat(filepath.Dir(storage.writerLockPath)); err != nil {
		t.Fatalf("writer-lock parent was not created: %v", err)
	}
	if _, err := os.Lstat(storage.writerLockPath); !os.IsNotExist(err) {
		t.Fatalf("writer lock remains after release: %v", err)
	}
}

func TestRunStorageReceiptIsImmutable(t *testing.T) {
	root := t.TempDir()
	storage, err := newRunStorage(root, "experiments/comparison", "run-1")
	if err != nil {
		t.Fatalf("newRunStorage: %v", err)
	}
	receipt := storageReceipt(t, root, "run-1")
	if err := storage.createReceipt(receipt); err != nil {
		t.Fatalf("first createReceipt: %v", err)
	}
	first, err := os.ReadFile(storage.executionPath)
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	if err := storage.createReceipt(receipt); err == nil || !strings.Contains(err.Error(), "already exists; start does not resume") {
		t.Fatalf("same createReceipt error = %v, want immutable no-resume refusal", err)
	}
	if got, err := os.ReadFile(storage.executionPath); err != nil || !bytes.Equal(got, first) {
		t.Fatalf("same receipt changed durable bytes: bytes equal=%t err=%v", bytes.Equal(got, first), err)
	}

	conflict := receipt
	conflict.Versions.Verdi = "v-conflict"
	if err := storage.createReceipt(conflict); err == nil || !strings.Contains(err.Error(), "conflicts with the current run identity") {
		t.Fatalf("conflicting createReceipt error = %v, want immutable conflict", err)
	}
	if got, err := os.ReadFile(storage.executionPath); err != nil || !bytes.Equal(got, first) {
		t.Fatalf("conflicting receipt changed durable bytes: bytes equal=%t err=%v", bytes.Equal(got, first), err)
	}
}

func TestRunStorageRejectsContendedWriterLock(t *testing.T) {
	root := t.TempDir()
	storage, err := newRunStorage(root, "experiments/comparison", "run-1")
	if err != nil {
		t.Fatalf("newRunStorage: %v", err)
	}
	if err := ensureParentTree(root, filepath.Dir(storage.writerLockPath)); err != nil {
		t.Fatalf("ensure writer-lock parent: %v", err)
	}
	lock, err := filelock.Acquire(storage.writerLockPath)
	if err != nil {
		t.Fatalf("acquire test writer lock: %v", err)
	}
	t.Cleanup(func() {
		if releaseErr := filelock.Release(lock, storage.writerLockPath); releaseErr != nil {
			t.Errorf("release test writer lock: %v", releaseErr)
		}
	})

	if err := storage.createReceipt(storageReceipt(t, root, "run-1")); err == nil || !strings.Contains(err.Error(), "acquire checkout writer lock") {
		t.Fatalf("createReceipt while lock held error = %v, want lock contention refusal", err)
	}
}

func TestRunStorageAppendObservationPublishesExactPrefix(t *testing.T) {
	root := t.TempDir()
	storage, err := newRunStorage(root, "experiments/comparison", "run-1")
	if err != nil {
		t.Fatalf("newRunStorage: %v", err)
	}
	def, schedule := storageDefinition(t)
	measured := measuredSchedule(schedule)
	first := storageObservation(t, def, "run-1", measured[0])
	second := storageObservation(t, def, "run-1", measured[1])
	firstBytes, err := experiment.EncodeObservation(first)
	if err != nil {
		t.Fatalf("encode first observation: %v", err)
	}
	secondBytes, err := experiment.EncodeObservation(second)
	if err != nil {
		t.Fatalf("encode second observation: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(storage.observationsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storage.observationsPath, firstBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := storage.appendObservation(def, schedule, second); err != nil {
		t.Fatalf("append observation: %v", err)
	}
	got, err := os.ReadFile(storage.observationsPath)
	if err != nil {
		t.Fatalf("read observations: %v", err)
	}
	want := append(append([]byte(nil), firstBytes...), secondBytes...)
	if !bytes.Equal(got, want) {
		t.Fatalf("observations bytes = %q, want exact prior prefix plus one canonical line %q", got, want)
	}
}

func TestRunStoragePreservesMeasuredCandidateFailure(t *testing.T) {
	root := t.TempDir()
	storage, err := newRunStorage(root, "experiments/comparison", "run-1")
	if err != nil {
		t.Fatalf("newRunStorage: %v", err)
	}
	def, schedule := storageDefinition(t)
	scheduled := measuredSchedule(schedule)[0]
	witness := "candidate timed out after 30s"
	outcome := experiment.CandidateOutcome{Kind: experiment.OutcomeCandidateTimeout, Witness: &witness}
	observation := storageObservation(t, def, "run-1", scheduled)
	observation.Outcome = &outcome
	observation.Guards = []experiment.GuardResult{}
	observation.Measurements = []experiment.Measurement{{
		ID:     experiment.EvaluatorWallDurationMetricID,
		Value:  experiment.NumberValue("30"),
		Unit:   "ns",
		Source: experiment.SourceHarnessMeasured,
	}}
	observation.Disclosures = []string{experiment.PeakRSSUnavailableDisclosure}

	if err := storage.appendObservation(def, schedule, observation); err != nil {
		t.Fatalf("append candidate failure: %v", err)
	}
	data, err := os.ReadFile(storage.observationsPath)
	if err != nil {
		t.Fatalf("read observations: %v", err)
	}
	observations, err := experiment.DecodeObservations(data)
	if err != nil {
		t.Fatalf("decode observations: %v", err)
	}
	if len(observations) != 1 || observations[0].Outcome == nil || observations[0].Outcome.Kind != experiment.OutcomeCandidateTimeout || observations[0].Outcome.Witness == nil || *observations[0].Outcome.Witness != witness {
		t.Fatalf("persisted candidate failure = %#v, want exact timeout evidence", observations)
	}
}

func TestRunStorageRejectsInvalidObservationPrefixWithoutMutation(t *testing.T) {
	root := t.TempDir()
	storage, err := newRunStorage(root, "experiments/comparison", "run-1")
	if err != nil {
		t.Fatalf("newRunStorage: %v", err)
	}
	def, schedule := storageDefinition(t)
	measured := measuredSchedule(schedule)
	if err := os.MkdirAll(filepath.Dir(storage.observationsPath), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name     string
		existing []byte
		append   experiment.Observation
		want     string
	}{
		{
			name:     "corrupt existing bytes",
			existing: []byte(`{"schema":`),
			append:   storageObservation(t, def, "run-1", measured[0]),
			want:     "decode observations",
		},
		{
			name:     "wrong measured schedule prefix",
			existing: []byte{},
			append:   storageObservation(t, def, "run-1", measured[1]),
			want:     "next observation",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(storage.observationsPath, test.existing, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := storage.appendObservation(def, schedule, test.append); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("appendObservation error = %v, want %q", err, test.want)
			}
			got, err := os.ReadFile(storage.observationsPath)
			if err != nil || !bytes.Equal(got, test.existing) {
				t.Fatalf("invalid append mutated prior bytes: equal=%t err=%v", bytes.Equal(got, test.existing), err)
			}
		})
	}
}

func TestRunStorageConcurrentMeasuredPublicationHasOneWinner(t *testing.T) {
	root := t.TempDir()
	storage, err := newRunStorage(root, "experiments/comparison", "run-1")
	if err != nil {
		t.Fatalf("newRunStorage: %v", err)
	}
	def, schedule := storageDefinition(t)
	first := storageObservation(t, def, "run-1", measuredSchedule(schedule)[0])

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			errs <- storage.appendObservation(def, schedule, first)
		}()
	}
	close(start)
	wait.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent publications successes = %d, want exactly one", successes)
	}
	bytes, err := os.ReadFile(storage.observationsPath)
	if err != nil {
		t.Fatalf("read observations: %v", err)
	}
	observations, err := experiment.DecodeObservations(bytes)
	if err != nil || len(observations) != 1 {
		t.Fatalf("concurrent observations = %#v err=%v, want one valid record", observations, err)
	}
}

func TestRunStoragePreflightRejectsUnsafeDurablePaths(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(t *testing.T, storage runStorage, outside string)
	}{
		{
			name: "symlinked run parent",
			setup: func(t *testing.T, storage runStorage, outside string) {
				t.Helper()
				parent := filepath.Dir(filepath.Dir(storage.executionPath))
				if err := os.MkdirAll(parent, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(parent, "runs")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "nonregular execution file",
			setup: func(t *testing.T, storage runStorage, _ string) {
				t.Helper()
				if err := os.MkdirAll(storage.executionPath, 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "symlinked observations file",
			setup: func(t *testing.T, storage runStorage, outside string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(storage.observationsPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, storage.observationsPath); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "symlinked writer lock",
			setup: func(t *testing.T, storage runStorage, outside string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(storage.writerLockPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, storage.writerLockPath); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			storage, err := newRunStorage(root, "experiments/comparison", "run-1")
			if err != nil {
				t.Fatalf("newRunStorage: %v", err)
			}
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
				t.Fatal(err)
			}
			test.setup(t, storage, outside)
			if err := storage.preflightStart(); err == nil {
				t.Fatal("preflightStart() = nil error for unsafe durable path")
			}
		})
	}
}

func storageDefinition(t *testing.T) (experiment.Definition, []ScheduledAttempt) {
	t.Helper()
	def, _, _ := testDefinition(t, []string{"alpha", "beta"}, 1)
	schedule, err := DeriveSchedule(def)
	if err != nil {
		t.Fatalf("DeriveSchedule: %v", err)
	}
	return def, schedule
}

func storageObservation(t *testing.T, def experiment.Definition, run string, scheduled ScheduledAttempt) experiment.Observation {
	t.Helper()
	digest, err := experiment.DefinitionDigest(def)
	if err != nil {
		t.Fatal(err)
	}
	outcome := experiment.CandidateOutcome{Kind: experiment.OutcomeCompleted}
	return experiment.Observation{
		Schema:           experiment.ObservationSchemaV2,
		ExperimentDigest: digest,
		Run:              run,
		Candidate:        scheduled.Candidate,
		Round:            scheduled.Cycle.Number,
		Outcome:          &outcome,
		Guards:           []experiment.GuardResult{{ID: "correctness", Verdict: experiment.GuardVerdictPass}},
		Measurements: []experiment.Measurement{
			{ID: "latency", Value: experiment.NumberValue("10"), Unit: "ms", Source: experiment.SourceEvaluatorMeasured},
			{ID: "memory", Value: experiment.NumberValue("1"), Unit: "bytes", Source: experiment.SourceEvaluatorMeasured},
		},
		Disclosures: []string{},
	}
}

func storageReceipt(t *testing.T, root, run string) experiment.ExecutionReceipt {
	t.Helper()
	def, capabilities, _ := testDefinition(t, []string{"alpha", "beta"}, 0)
	capabilities.RequiresNetwork = true
	capabilitiesBytes, err := canonjson.Marshal(capabilities)
	if err != nil {
		t.Fatal(err)
	}
	def.Evaluator.CapabilitiesDigest = testDigestBytes(capabilitiesBytes)
	def = relockDefinition(t, def)
	authorization := testAuthorization(t, def, true)
	authorized := mustResolveAuthorization(t, def, capabilities, authorization)
	inputs := writeResolvedInputs(t, root, def)
	planned, report, err := execworkspace.PlanProfile(filepath.Join(root, "planned-workspace"), filepath.Join(root, "planned-workspace", environmentRootName), authorized.Grants, authorization.DeclaredEnv)
	if err != nil {
		t.Fatalf("PlanProfile: %v", err)
	}
	if len(planned.Env()) == 0 || report == nil {
		t.Fatal("PlanProfile did not return the receipt-bound planned profile/report")
	}
	receipt, err := buildExecutionReceipt(ReceiptInput{
		Definition:        def,
		Run:               run,
		Capabilities:      capabilities,
		CapabilitiesBytes: capabilitiesBytes,
		Authorization:     authorized,
		Inputs:            inputs,
		CandidatePatches:  candidatePatches(t, def),
		Fingerprint:       testFingerprint(t, def, capabilities, authorization, inputs),
		Enforcement:       *report,
		Versions: experiment.ReceiptVersions{
			Verdi:                "v-test",
			RecommendationEngine: string(def.Algorithm),
		},
	}, linuxHostRuntimeFacts())
	if err != nil {
		t.Fatalf("buildExecutionReceipt: %v", err)
	}
	return receipt
}
