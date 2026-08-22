//go:build linux

package experimentrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentdecision"
	"github.com/jyang234/verdi/internal/experimentevaluator"
	"github.com/jyang234/verdi/internal/fixturegit"
)

var errJourneyInterrupted = errors.New("journey interrupted after measured prefix")

func TestWave3BJourneyDefaultDenyResumeAndAtRestProof(t *testing.T) {
	evaluator, evaluatorDigest := buildJourneyEvaluator(t)
	fixture := newJourneyFixture(t, evaluator, evaluatorDigest, "completed", "2s", 1, 2, true)
	interrupt := &journeyEvaluator{
		interrupt: &interruptPoint{kind: experiment.CycleMeasured, candidate: "alpha", round: 1, err: errJourneyInterrupted},
	}
	service := fixture.service(t, fixture.authorization, fixture.inputs, interrupt)

	if _, err := service.Start(context.Background(), fixture.request); !errors.Is(err, errJourneyInterrupted) {
		t.Fatalf("Start interruption error = %v, want errors.Is(_, %v)", err, errJourneyInterrupted)
	}
	if fixture.materializer.calls != 2 {
		t.Fatalf("receipt-first base+patch materializations = %d, want 2", fixture.materializer.calls)
	}
	assertMaterializedCandidates(t, fixture.materializer.paths)
	prefixBytes := readJourneyArtifact(t, fixture.request.Root, fixture.paths.Observations)
	prefix, err := experiment.DecodeObservations(prefixBytes)
	if err != nil {
		t.Fatalf("decode interrupted observations: %v", err)
	}
	if len(prefix) != 1 || prefix[0].Candidate != "beta" || prefix[0].Round != 1 {
		t.Fatalf("interrupted measured prefix = %+v, want beta round 1 only", prefix)
	}
	assertJourneyArtifactAbsent(t, fixture.request.Root, fixture.paths.Result)

	resumeEvaluator := &journeyEvaluator{}
	resumed := fixture.service(t, fixture.authorization, fixture.inputs, resumeEvaluator)
	result, err := resumed.Resume(context.Background(), ResumeRequest(fixture.request))
	if err != nil {
		t.Fatalf("Resume unchanged run: %v", err)
	}
	if got, want := renderJourneyCalls(resumeEvaluator.calls), "warmup-1:alpha,warmup-1:beta,measured-1:alpha,measured-2:alpha,measured-2:beta"; got != want {
		t.Fatalf("Resume calls = %q, want only restarted warmups plus missing measured tail %q", got, want)
	}
	if result.Result.Schema != experiment.ResultSchemaV2 || result.Result.Decision == nil || result.Result.Execution == nil {
		t.Fatalf("Resume result = %+v, want complete V2 envelope", result.Result)
	}
	if result.Result.Decision.Verdict != experiment.VerdictProvenWinner || result.Result.Decision.Winner != "beta" {
		t.Fatalf("Resume decision = %s winner %q, want proven beta", result.Result.Decision.Verdict, result.Result.Decision.Winner)
	}
	if result.ResultDigest == "" {
		t.Fatal("Resume result digest is empty")
	}
	if result.Receipt.Network.Mode != experiment.NetworkDeny || !result.Receipt.Network.Configured {
		t.Fatalf("receipt network = %+v, want configured default deny", result.Receipt.Network)
	}
	if got := result.Result.Execution.Isolation.Disclosures; len(got) != 0 {
		t.Fatalf("default-deny result disclosures = %v, want empty", got)
	}

	receiptBytes := readJourneyArtifact(t, fixture.request.Root, fixture.paths.Execution)
	atRestReceipt, err := experiment.DecodeExecutionReceipt(receiptBytes)
	if err != nil {
		t.Fatalf("decode at-rest receipt: %v", err)
	}
	observationBytes := readJourneyArtifact(t, fixture.request.Root, fixture.paths.Observations)
	atRestObservations, err := experiment.DecodeObservations(observationBytes)
	if err != nil {
		t.Fatalf("decode at-rest observations: %v", err)
	}
	resultBytes := readJourneyArtifact(t, fixture.request.Root, fixture.paths.Result)
	atRestResult, err := experiment.DecodeResult(resultBytes)
	if err != nil {
		t.Fatalf("decode at-rest result: %v", err)
	}
	if err := experimentdecision.VerifyResult(fixture.request.Definition, atRestObservations, &atRestReceipt, atRestResult); err != nil {
		t.Fatalf("at-rest receipt/recompute proof: %v", err)
	}
	atRestDigest, err := experiment.ResultDigest(atRestResult)
	if err != nil {
		t.Fatalf("digest at-rest result: %v", err)
	}
	if atRestDigest != result.ResultDigest {
		t.Fatalf("at-rest result digest = %q, returned explicit digest %q", atRestDigest, result.ResultDigest)
	}
	receiptDigest, err := experiment.ExecutionReceiptDigest(atRestReceipt)
	if err != nil {
		t.Fatalf("digest at-rest receipt: %v", err)
	}
	if receiptDigest != atRestResult.Execution.ExecutionDigest {
		t.Fatalf("at-rest receipt digest = %q, result annex = %q", receiptDigest, atRestResult.Execution.ExecutionDigest)
	}
}

func TestWave3BJourneyRejectsChangedResumeIdentity(t *testing.T) {
	evaluator, evaluatorDigest := buildJourneyEvaluator(t)
	for _, test := range []struct {
		name   string
		change func(*testing.T, *journeyFixture) (ExecutionAuthorization, staticInputs)
		want   string
	}{
		{
			name: "environment identity",
			change: func(_ *testing.T, fixture *journeyFixture) (ExecutionAuthorization, staticInputs) {
				authorization := cloneAuthorization(fixture.authorization)
				authorization.DeclaredEnv["LANG"] = "C.UTF-8"
				return authorization, fixture.inputs
			},
			want: "receipt parity",
		},
		{
			name: "input identity",
			change: func(t *testing.T, fixture *journeyFixture) (ExecutionAuthorization, staticInputs) {
				writeTestFile(t, fixture.request.Root, "inputs/workload.json", "changed workload\n")
				return fixture.authorization, fixture.inputs
			},
			want: "digest",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newJourneyFixture(t, evaluator, evaluatorDigest, "completed", "2s", 0, 2, false)
			interrupt := &journeyEvaluator{
				interrupt: &interruptPoint{kind: experiment.CycleMeasured, candidate: "beta", round: 1, err: errJourneyInterrupted},
			}
			service := fixture.service(t, fixture.authorization, fixture.inputs, interrupt)
			if _, err := service.Start(context.Background(), fixture.request); !errors.Is(err, errJourneyInterrupted) {
				t.Fatalf("Start interruption: %v", err)
			}
			before := readJourneyArtifact(t, fixture.request.Root, fixture.paths.Observations)
			authorization, inputs := test.change(t, fixture)
			resumeEvaluator := &journeyEvaluator{}
			resume := fixture.service(t, authorization, inputs, resumeEvaluator)

			_, err := resume.Resume(context.Background(), ResumeRequest(fixture.request))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Resume changed %s error = %v, want operational %q refusal", test.name, err, test.want)
			}
			if len(resumeEvaluator.calls) != 0 {
				t.Fatalf("Resume changed %s executed evaluator calls: %+v", test.name, resumeEvaluator.calls)
			}
			after := readJourneyArtifact(t, fixture.request.Root, fixture.paths.Observations)
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("Resume changed %s mutated measured prefix", test.name)
			}
			assertJourneyArtifactAbsent(t, fixture.request.Root, fixture.paths.Result)
		})
	}
}

func TestWave3BJourneyClassifiesCandidateAndEvaluatorFailures(t *testing.T) {
	evaluator, evaluatorDigest := buildJourneyEvaluator(t)
	for _, test := range []struct {
		name string
		mode experiment.OutcomeKind
	}{
		{name: "candidate crash", mode: experiment.OutcomeCandidateCrash},
		{name: "candidate timeout", mode: experiment.OutcomeCandidateTimeout},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newJourneyFixture(t, evaluator, evaluatorDigest, string(test.mode), "2s", 0, 1, false)
			service := fixture.service(t, fixture.authorization, fixture.inputs, nil)
			result, err := service.Start(context.Background(), fixture.request)
			if err != nil {
				t.Fatalf("Start %s: %v; candidate failure must remain result data", test.mode, err)
			}
			if result.Result.Decision == nil || result.Result.Decision.Verdict != experiment.VerdictViolatedWithWitness {
				t.Fatalf("%s decision = %+v, want violated result data", test.mode, result.Result.Decision)
			}
			if len(result.Observations) != 2 {
				t.Fatalf("%s observations = %d, want one per measured candidate", test.mode, len(result.Observations))
			}
			for _, observation := range result.Observations {
				if observation.Outcome == nil || observation.Outcome.Kind != test.mode || len(observation.Guards) != 0 {
					t.Fatalf("%s observation = %+v", test.mode, observation)
				}
				for _, measurement := range observation.Measurements {
					if measurement.Source != experiment.SourceHarnessMeasured {
						t.Fatalf("%s observation admitted non-harness measurement %+v", test.mode, measurement)
					}
				}
			}
			for _, candidate := range result.Result.Decision.Candidates {
				if candidate.Eligible || len(candidate.ExecutionFailures) != 1 || candidate.ExecutionFailures[0].Kind != test.mode {
					t.Fatalf("%s candidate result = %+v, want ineligible failure data", test.mode, candidate)
				}
			}
			if _, err := os.Lstat(filepath.Join(fixture.request.Root, filepath.FromSlash(fixture.paths.Result))); err != nil {
				t.Fatalf("%s did not publish completed result: %v", test.mode, err)
			}
		})
	}

	for _, test := range []struct {
		name    string
		mode    string
		timeout string
		want    error
	}{
		{name: "malformed evaluator response", mode: "malformed", timeout: "2s", want: experimentevaluator.ErrProtocol},
		{name: "evaluator crash", mode: "evaluator-crash", timeout: "2s", want: experimentevaluator.ErrEvaluatorExit},
		{name: "evaluator timeout", mode: "evaluator-timeout", timeout: "1s", want: experimentevaluator.ErrHarnessDeadline},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newJourneyFixture(t, evaluator, evaluatorDigest, test.mode, test.timeout, 0, 1, false)
			service := fixture.service(t, fixture.authorization, fixture.inputs, nil)
			_, err := service.Start(context.Background(), fixture.request)
			if !experimentevaluator.IsOperational(err) || !errors.Is(err, test.want) {
				t.Fatalf("Start %s error = %v, want operational errors.Is(_, %v)", test.mode, err, test.want)
			}
			if _, statErr := os.Lstat(filepath.Join(fixture.request.Root, filepath.FromSlash(fixture.paths.Execution))); statErr != nil {
				t.Fatalf("%s lost immutable receipt: %v", test.mode, statErr)
			}
			assertJourneyArtifactAbsent(t, fixture.request.Root, fixture.paths.Observations)
			assertJourneyArtifactAbsent(t, fixture.request.Root, fixture.paths.Result)
		})
	}
}

type journeyFixture struct {
	request       StartRequest
	authorization ExecutionAuthorization
	inputs        staticInputs
	materializer  *receiptFirstMaterializer
	paths         experiment.RunPaths
	versions      experiment.ReceiptVersions
}

func newJourneyFixture(t *testing.T, evaluator, evaluatorDigest, mode, timeout string, warmups, rounds int, discover bool) *journeyFixture {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			"candidate.txt":             "base\n",
			"contracts/behavioral.json": "contract\n",
			"fixtures/request-log.json": "fixture\n",
			"inputs/workload.json":      "workload\n",
		},
		Message: "build CSE journey base",
	}})
	patches := map[string][]byte{
		"alpha": buildJourneyPatch(t, repo.Dir, "alpha\n"),
		"beta":  buildJourneyPatch(t, repo.Dir, "beta\n"),
	}
	capabilities := experiment.Capabilities{
		Schema:           experiment.CapabilitiesSchemaV2,
		EvaluatorVersion: "journey-evaluator/v1",
		ProtocolVersions: []string{experiment.EvaluatorProtocolSchema, experiment.ObservationSchemaV2},
		Metrics: []experiment.CapabilityMetric{{
			ID: "latency", Type: experiment.MetricDuration, Unit: "ms", Direction: experiment.DirectionLower,
		}},
		Guards:      []string{"correctness"},
		Environment: []string{"LANG"},
	}
	capabilitiesBytes, err := canonjson.Marshal(capabilities)
	if err != nil {
		t.Fatal(err)
	}
	capabilitiesPath := filepath.Join(t.TempDir(), "capabilities.json")
	if err := os.WriteFile(capabilitiesPath, capabilitiesBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	argv := []string{evaluator, "--capabilities=" + capabilitiesPath, "--mode=" + mode, "run"}
	if discover {
		discoveryEnvironment := filepath.Join(repo.Dir, ".journey-discovery-environment")
		profile, report, err := execworkspace.BuildProfile(repo.Dir, discoveryEnvironment, journeyGrants(t, evaluator, timeout), map[string]string{"LANG": "C"})
		if err != nil {
			t.Fatalf("build default-deny describe profile: %v", err)
		}
		if report.Network.Mode != execworkspace.NetworkDeny || !report.Network.Configured {
			t.Fatalf("describe profile network = %+v, want configured default deny", report.Network)
		}
		discovery, err := experimentevaluator.Discover(context.Background(), profile, experimentevaluator.DiscoverInput{
			Launch:             experimentevaluator.Launch{Directory: repo.Dir, Argv: argv, Digest: evaluatorDigest},
			CapabilitiesDigest: journeyDigest(capabilitiesBytes),
		})
		if err != nil {
			t.Fatalf("strict evaluator describe: %v", err)
		}
		if !reflect.DeepEqual(discovery.Bytes, capabilitiesBytes) {
			t.Fatalf("strict describe bytes differ from registered authority")
		}
		capabilitiesBytes = discovery.Bytes
		if err := os.RemoveAll(discoveryEnvironment); err != nil {
			t.Fatalf("remove discovery profile: %v", err)
		}
	}

	relative := 0.1
	separation := 0.05
	def := experiment.Definition{
		Schema: experiment.DefinitionSchema, ID: "comparison", Spike: "spec/comparison", Question: "spec/question#oq-one", BaseCommit: repo.Head,
		Candidates: []experiment.Candidate{
			{ID: "alpha", Patch: "candidates/alpha.patch", Digest: journeyDigest(patches["alpha"]), Base: repo.Head},
			{ID: "beta", Patch: "candidates/beta.patch", Digest: journeyDigest(patches["beta"]), Base: repo.Head},
		},
		Evaluator: experiment.Evaluator{Argv: argv, Digest: evaluatorDigest, CapabilitiesDigest: journeyDigest(capabilitiesBytes)},
		Workload:  experiment.ArtifactRef{ID: "workload", Digest: journeyDigest([]byte("workload\n"))},
		Fixtures:  []experiment.ArtifactRef{{ID: "fixture", Digest: journeyDigest([]byte("fixture\n"))}},
		Contract:  experiment.ArtifactRef{ID: "contract", Digest: journeyDigest([]byte("contract\n"))},
		Decision: experiment.DecisionSpec{
			PrimaryMetric: experiment.PrimaryMetric{ID: "latency", Type: experiment.MetricDuration, Unit: "ms", Aggregation: experiment.AggregationP95, Direction: experiment.DirectionLower},
			Baseline:      "alpha", BaselineImprovement: experiment.Threshold{Relative: &relative}, CandidateSeparation: experiment.Threshold{Relative: &separation},
			Guards: []experiment.Guard{{ID: "correctness"}},
		},
		Execution: experiment.Execution{Warmups: warmups, Rounds: rounds, Order: experiment.OrderDeterministicRotation, TimeoutPerRound: timeout, EnvironmentPolicy: "isolated-v1"},
		Algorithm: experiment.AlgorithmV1, RetentionPolicy: "standard",
		ProtectedPaths: []string{"inputs/workload.json", "fixtures/request-log.json", "contracts/behavioral.json"},
	}
	def = relockDefinition(t, def)
	locked, err := experiment.Locked(def)
	if err != nil || !locked {
		t.Fatalf("journey definition lock = %t, %v", locked, err)
	}
	const experimentDir = "experiments/comparison"
	const run = "run-1"
	writeStartAuthority(t, repo.Dir, experimentDir, capabilitiesBytes, patches)
	inputs := staticInputs{values: map[string]ResolvedInput{
		"workload": {ID: "workload", Path: "inputs/workload.json", Digest: def.Workload.Digest},
		"fixture":  {ID: "fixture", Path: "fixtures/request-log.json", Digest: def.Fixtures[0].Digest},
		"contract": {ID: "contract", Path: "contracts/behavioral.json", Digest: def.Contract.Digest},
	}}
	grantBytes, err := execworkspace.EncodeGrantSet(journeyGrants(t, evaluator, timeout))
	if err != nil {
		t.Fatal(err)
	}
	authorization := ExecutionAuthorization{
		EnvironmentPolicy: def.Execution.EnvironmentPolicy,
		AuthorityDigest:   journeyDigest([]byte("journey authority")),
		GrantBytes:        grantBytes,
		DeclaredEnv:       map[string]string{"LANG": "C"},
	}
	paths, err := experiment.PathsForRun(experimentDir, run)
	if err != nil {
		t.Fatal(err)
	}
	materializer, err := execworkspace.NewMaterializer(repo.Dir, repo.Dir, execworkspace.NewGitReconciler(repo.Dir))
	if err != nil {
		t.Fatalf("new real journey materializer: %v", err)
	}
	return &journeyFixture{
		request:       StartRequest{Root: repo.Dir, ExperimentDir: experimentDir, Run: run, Definition: def},
		authorization: authorization,
		inputs:        inputs,
		materializer:  &receiptFirstMaterializer{delegate: materializer, receiptPath: filepath.Join(repo.Dir, filepath.FromSlash(paths.Execution))},
		paths:         paths,
		versions:      experiment.ReceiptVersions{Verdi: "v-test", RecommendationEngine: string(def.Algorithm)},
	}
}

func (f *journeyFixture) service(t *testing.T, authorization ExecutionAuthorization, inputs staticInputs, evaluator AttemptEvaluator) *Service {
	t.Helper()
	service, err := NewService(ServiceDependencies{
		Authorization: staticAuthorization{authorization: authorization}, Inputs: inputs, Materializer: f.materializer,
		Evaluator: evaluator, Versions: f.versions,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return service
}

type receiptFirstMaterializer struct {
	delegate    *execworkspace.Materializer
	receiptPath string
	calls       int
	paths       []string
}

func (m *receiptFirstMaterializer) Materialize(ctx context.Context, request execworkspace.Request) (execworkspace.Result, error) {
	if _, err := os.Lstat(m.receiptPath); err != nil {
		return execworkspace.Result{}, fmt.Errorf("receipt absent before real materialization: %w", err)
	}
	m.calls++
	result, err := m.delegate.Materialize(ctx, request)
	if err == nil {
		m.paths = append(m.paths, result.Path)
	}
	return result, err
}

type journeyEvaluator struct {
	interrupt *interruptPoint
	calls     []experiment.EvaluatorRequest
}

func (e *journeyEvaluator) Observe(ctx context.Context, profile execworkspace.Profile, input experimentevaluator.ObserveInput) (experimentevaluator.Attempt, error) {
	e.calls = append(e.calls, input.Request)
	if e.interrupt != nil && input.Request.Cycle.Kind == e.interrupt.kind && input.Request.Candidate == e.interrupt.candidate && input.Request.Cycle.Number == e.interrupt.round {
		return experimentevaluator.Attempt{}, e.interrupt.err
	}
	return experimentevaluator.Observe(ctx, profile, input)
}

func buildJourneyEvaluator(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "journey-evaluator")
	cmd := exec.Command("go", "build", "-o", path, "./testdata/evaluator")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build journey evaluator: %v\n%s", err, output)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, journeyDigest(data)
}

func journeyGrants(t *testing.T, evaluator, timeout string) execworkspace.GrantSet {
	t.Helper()
	seconds := 2
	if timeout == "1s" {
		seconds = 1
	}
	return execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{evaluator}},
		{Kind: execworkspace.GrantTimeouts, Seconds: seconds},
	}}
}

func buildJourneyPatch(t *testing.T, repo, content string) []byte {
	t.Helper()
	path := filepath.Join(repo, "candidate.txt")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", repo, "diff", "--", "candidate.txt")
	patch, err := cmd.Output()
	if restoreErr := os.WriteFile(path, original, 0o644); err != nil || restoreErr != nil {
		t.Fatalf("build candidate patch: diff error %v, restore error %v", err, restoreErr)
	}
	if len(patch) == 0 {
		t.Fatal("candidate patch is empty")
	}
	return patch
}

func journeyDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func assertMaterializedCandidates(t *testing.T, paths []string) {
	t.Helper()
	if len(paths) != 2 {
		t.Fatalf("materialized candidate paths = %d, want 2", len(paths))
	}
	contents := make([]string, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(path, "candidate.txt"))
		if err != nil {
			t.Fatalf("read materialized candidate marker: %v", err)
		}
		contents = append(contents, string(data))
	}
	sort.Strings(contents)
	if !reflect.DeepEqual(contents, []string{"alpha\n", "beta\n"}) {
		t.Fatalf("materialized base+patch candidate contents = %q", contents)
	}
}

func renderJourneyCalls(calls []experiment.EvaluatorRequest) string {
	parts := make([]string, 0, len(calls))
	for _, call := range calls {
		parts = append(parts, fmt.Sprintf("%s-%d:%s", call.Cycle.Kind, call.Cycle.Number, call.Candidate))
	}
	return strings.Join(parts, ",")
}

func readJourneyArtifact(t *testing.T, root, relative string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read journey artifact %q: %v", relative, err)
	}
	return data
}

func assertJourneyArtifactAbsent(t *testing.T, root, relative string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(relative))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journey artifact %q exists or cannot be proven absent: %v", relative, err)
	}
}
