package experimentrun

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

func TestInputBindingCodecIsStrictCanonicalAndPresenceSensitive(t *testing.T) {
	digestA := "sha256:" + strings.Repeat("a", 64)
	digestB := "sha256:" + strings.Repeat("b", 64)
	digestC := "sha256:" + strings.Repeat("c", 64)
	want := []byte(`{"inputs":[{"digest":"` + digestA + `","id":"contract","path":"contracts/behavioral.json","slot":"contract"},{"digest":"` + digestB + `","id":"fixture-one","path":"fixtures/one.json","slot":"fixture:fixture-one"},{"digest":"` + digestC + `","id":"workload","path":"inputs/workload.json","slot":"workload"}],"schema":"verdi.experiment-input-bindings/v1"}` + "\n")

	decoded, err := DecodeInputBindings(want)
	if err != nil {
		t.Fatalf("DecodeInputBindings(canonical): %v", err)
	}
	encoded, err := EncodeInputBindings(decoded)
	if err != nil {
		t.Fatalf("EncodeInputBindings(decoded): %v", err)
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("EncodeInputBindings() = %s, want %s", encoded, want)
	}

	tests := []struct {
		name string
		raw  []byte
	}{
		{name: "invalid UTF-8", raw: append([]byte(`{"inputs":[]}`), 0xff)},
		{name: "invalid JSON", raw: []byte(`{"inputs":`)},
		{name: "missing schema", raw: []byte(`{"inputs":[]}` + "\n")},
		{name: "null schema", raw: []byte(`{"inputs":[],"schema":null}` + "\n")},
		{name: "missing inputs", raw: []byte(`{"schema":"verdi.experiment-input-bindings/v1"}` + "\n")},
		{name: "null inputs", raw: []byte(`{"inputs":null,"schema":"verdi.experiment-input-bindings/v1"}` + "\n")},
		{name: "unknown top-level field", raw: bytes.Replace(want, []byte(`{"inputs"`), []byte(`{"ambient":{},"inputs"`), 1)},
		{name: "duplicate top-level field", raw: bytes.Replace(want, []byte(`"schema":`), []byte(`"schema":"verdi.experiment-input-bindings/v1","schema":`), 1)},
		{name: "unknown entry field", raw: bytes.Replace(want, []byte(`{"digest"`), []byte(`{"ambient":"x","digest"`), 1)},
		{name: "duplicate entry field", raw: bytes.Replace(want, []byte(`"slot":"contract"`), []byte(`"slot":"contract","slot":"contract"`), 1)},
		{name: "missing entry field", raw: bytes.Replace(want, []byte(`,"path":"contracts/behavioral.json"`), nil, 1)},
		{name: "null entry field", raw: bytes.Replace(want, []byte(`"path":"contracts/behavioral.json"`), []byte(`"path":null`), 1)},
		{name: "trailing data", raw: append(append([]byte(nil), want...), []byte("{}")...)},
		{name: "noncanonical whitespace", raw: bytes.Replace(want, []byte(`{"inputs"`), []byte(`{ "inputs"`), 1)},
		{name: "noncanonical key order", raw: []byte(`{"schema":"verdi.experiment-input-bindings/v1","inputs":[]}` + "\n")},
		{name: "missing canonical newline", raw: bytes.TrimSuffix(want, []byte("\n"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeInputBindings(tt.raw); err == nil {
				t.Fatal("DecodeInputBindings() error = nil, want strict/canonical refusal")
			}
		})
	}
}

func TestInputBindingSemanticValidationAndDeepCopy(t *testing.T) {
	base := InputBindings{
		Schema: InputBindingSchema,
		Inputs: []InputBinding{
			{Slot: InputSlotContract, ID: "contract", Digest: digestText("contract"), Path: "contracts/behavioral.json"},
			{Slot: FixtureInputSlot("fixture"), ID: "fixture", Digest: digestText("fixture"), Path: "fixtures/request-log.json"},
			{Slot: InputSlotWorkload, ID: "workload", Digest: digestText("workload"), Path: "inputs/workload.json"},
		},
	}
	if _, err := EncodeInputBindings(base); err != nil {
		t.Fatalf("EncodeInputBindings(valid): %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*InputBindings)
	}{
		{name: "unknown schema", mutate: func(b *InputBindings) { b.Schema = "verdi.experiment-input-bindings/v2" }},
		{name: "absent inputs", mutate: func(b *InputBindings) { b.Inputs = nil }},
		{name: "unsorted slots", mutate: func(b *InputBindings) { b.Inputs[0], b.Inputs[1] = b.Inputs[1], b.Inputs[0] }},
		{name: "duplicate slot", mutate: func(b *InputBindings) { b.Inputs[1].Slot = b.Inputs[0].Slot }},
		{name: "unknown slot", mutate: func(b *InputBindings) { b.Inputs[1].Slot = "fixture" }},
		{name: "empty fixture suffix", mutate: func(b *InputBindings) { b.Inputs[1].Slot = "fixture:" }},
		{name: "noncanonical fixture suffix", mutate: func(b *InputBindings) { b.Inputs[1].Slot = "fixture:fixture/one" }},
		{name: "duplicate path", mutate: func(b *InputBindings) { b.Inputs[1].Path = b.Inputs[0].Path }},
		{name: "invalid id", mutate: func(b *InputBindings) { b.Inputs[1].ID = "Fixture" }},
		{name: "invalid digest", mutate: func(b *InputBindings) { b.Inputs[1].Digest = "sha256:no" }},
		{name: "noncanonical path", mutate: func(b *InputBindings) { b.Inputs[1].Path = "fixtures/../fixture.json" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bindings := base.Clone()
			tt.mutate(&bindings)
			if _, err := EncodeInputBindings(bindings); err == nil {
				t.Fatal("EncodeInputBindings() error = nil, want semantic refusal")
			}
		})
	}

	clone := base.Clone()
	base.Inputs[0].Path = "changed"
	if clone.Inputs[0].Path != "contracts/behavioral.json" {
		t.Fatalf("Clone() retained caller alias: %+v", clone.Inputs[0])
	}
}

func TestInputBindingExactDefinitionCorrespondenceIncludingZeroFixtures(t *testing.T) {
	def, _, _ := testDefinition(t, []string{"alpha", "beta"}, 0)
	valid := bindingsForDefinition(def)
	if _, err := NewBoundInputResolver(def, valid); err != nil {
		t.Fatalf("NewBoundInputResolver(valid): %v", err)
	}
	unlocked := def
	unlocked.Lock = nil
	if _, err := NewBoundInputResolver(unlocked, valid); err == nil {
		t.Fatal("NewBoundInputResolver(unlocked definition) error = nil")
	}

	fixtureless := def
	fixtureless.Fixtures = []experiment.ArtifactRef{}
	fixtureless = relockDefinition(t, fixtureless)
	fixturelessBindings := bindingsForDefinition(fixtureless)
	if len(fixturelessBindings.Inputs) != 2 {
		t.Fatalf("fixtureless binding count = %d, want workload and contract", len(fixturelessBindings.Inputs))
	}
	if _, err := NewBoundInputResolver(fixtureless, fixturelessBindings); err != nil {
		t.Fatalf("NewBoundInputResolver(zero fixtures): %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*InputBindings)
	}{
		{name: "missing slot", mutate: func(b *InputBindings) { b.Inputs = b.Inputs[1:] }},
		{name: "extra slot", mutate: func(b *InputBindings) {
			b.Inputs = append(b.Inputs, InputBinding{Slot: FixtureInputSlot("other"), ID: "other", Digest: digestText("other"), Path: "fixtures/other.json"})
			sort.Slice(b.Inputs, func(i, j int) bool { return b.Inputs[i].Slot < b.Inputs[j].Slot })
		}},
		{name: "workload identity mismatch", mutate: func(b *InputBindings) { bindingBySlot(b, InputSlotWorkload).ID = "other" }},
		{name: "contract digest mismatch", mutate: func(b *InputBindings) { bindingBySlot(b, InputSlotContract).Digest = digestText("other") }},
		{name: "fixture slot id mismatch", mutate: func(b *InputBindings) { bindingBySlot(b, FixtureInputSlot(def.Fixtures[0].ID)).ID = "other" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bindings := valid.Clone()
			tt.mutate(&bindings)
			if _, err := NewBoundInputResolver(def, bindings); err == nil {
				t.Fatal("NewBoundInputResolver() error = nil, want exact-correspondence refusal")
			}
		})
	}
}

func TestResolveInputBindingIdenticalIDsRemainDistinctBySlot(t *testing.T) {
	def, _, _ := testDefinition(t, []string{"alpha", "beta"}, 0)
	root := t.TempDir()
	sharedDigest := digestText("shared\n")
	def.Workload = experiment.ArtifactRef{ID: "shared", Digest: sharedDigest}
	def.Contract = experiment.ArtifactRef{ID: "shared", Digest: sharedDigest}
	def = relockDefinition(t, def)
	writeTestFile(t, root, "inputs/workload.json", "shared\n")
	writeTestFile(t, root, "fixtures/request-log.json", "fixture")
	writeTestFile(t, root, "contracts/behavioral.json", "shared\n")

	bindings := bindingsForDefinition(def)
	resolver, err := NewBoundInputResolver(def, bindings)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveInputs(context.Background(), resolver, root, def)
	if err != nil {
		t.Fatalf("ResolveInputs() with identical workload/contract ids: %v", err)
	}
	if resolved.Workload.Path != "inputs/workload.json" || resolved.Contract.Path != "contracts/behavioral.json" {
		t.Fatalf("resolved paths = workload %q contract %q, want slot-specific paths", resolved.Workload.Path, resolved.Contract.Path)
	}

	bindings.Inputs[0].Path = "changed-after-construction"
	got, err := resolver.ResolveExperimentInput(context.Background(), root, ResolveInputRequest{Slot: InputSlotContract, Ref: def.Contract})
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "contracts/behavioral.json" {
		t.Fatalf("bound resolver retained caller alias: %+v", got)
	}
}

func TestInputBindingPathProofRejectsUnsafeOrChangedFiles(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, root string, def *experiment.Definition, bindings *InputBindings)
	}{
		{name: "unprotected", mutate: func(t *testing.T, root string, _ *experiment.Definition, bindings *InputBindings) {
			writeTestFile(t, root, "other/workload.json", "workload")
			bindingBySlot(bindings, InputSlotWorkload).Path = "other/workload.json"
		}},
		{name: "raw digest mismatch", mutate: func(t *testing.T, root string, _ *experiment.Definition, _ *InputBindings) {
			writeTestFile(t, root, "inputs/workload.json", "changed")
		}},
		{name: "symlink", mutate: func(t *testing.T, root string, def *experiment.Definition, bindings *InputBindings) {
			path := "inputs/workload-link.json"
			if err := os.Symlink(filepath.Join(root, "inputs", "workload.json"), filepath.Join(root, filepath.FromSlash(path))); err != nil {
				t.Fatal(err)
			}
			def.ProtectedPaths = append(def.ProtectedPaths, path)
			bindingBySlot(bindings, InputSlotWorkload).Path = path
		}},
		{name: "nonregular", mutate: func(t *testing.T, root string, def *experiment.Definition, bindings *InputBindings) {
			path := "inputs/workload-dir"
			if err := os.Mkdir(filepath.Join(root, filepath.FromSlash(path)), 0o755); err != nil {
				t.Fatal(err)
			}
			def.ProtectedPaths = append(def.ProtectedPaths, path)
			bindingBySlot(bindings, InputSlotWorkload).Path = path
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, _, _ := testDefinition(t, []string{"alpha", "beta"}, 0)
			root := t.TempDir()
			writeTestFile(t, root, "inputs/workload.json", "workload")
			writeTestFile(t, root, "fixtures/request-log.json", "fixture")
			writeTestFile(t, root, "contracts/behavioral.json", "contract")
			bindings := bindingsForDefinition(def)
			tt.mutate(t, root, &def, &bindings)
			def = relockDefinition(t, def)
			resolver, err := NewBoundInputResolver(def, bindings)
			if err != nil {
				t.Fatalf("NewBoundInputResolver(): %v", err)
			}
			if _, err := ResolveInputs(context.Background(), resolver, root, def); err == nil {
				t.Fatal("ResolveInputs() error = nil, want protected regular raw-byte proof refusal")
			}
		})
	}
}

func TestInputBindingFailurePrecedesExecutionAndDurablePublication(t *testing.T) {
	root := t.TempDir()
	const experimentDir = "experiments/comparison"
	const run = "run-binding-refusal"
	def, _, capabilitiesBytes := testDefinition(t, []string{"alpha", "beta"}, 0)
	writeStartAuthority(t, root, experimentDir, capabilitiesBytes, candidatePatches(t, def))
	writeResolvedInputs(t, root, def)
	writeTestFile(t, root, "inputs/workload.json", "changed-after-binding")
	resolver, err := NewBoundInputResolver(def, bindingsForDefinition(def))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := experiment.PathsForRun(experimentDir, run)
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(root, filepath.FromSlash(paths.Execution))
	materializer := &recordingMaterializer{root: root, receiptPath: receiptPath}
	service, err := NewService(ServiceDependencies{
		Authorization: staticAuthorization{authorization: testAuthorization(t, def, false)},
		Inputs:        resolver,
		Materializer:  materializer,
		Evaluator:     &recordingEvaluator{},
		Versions: experiment.ReceiptVersions{
			Verdi: "v-test", RecommendationEngine: string(def.Algorithm),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Start(context.Background(), StartRequest{
		Root: root, ExperimentDir: experimentDir, Run: run, Definition: def,
	})
	if err == nil || !strings.Contains(err.Error(), "raw-byte digest") {
		t.Fatalf("Start() error = %v, want input-binding digest refusal", err)
	}
	if len(materializer.requests) != 0 {
		t.Fatalf("materializer calls = %d, want no execution after binding refusal", len(materializer.requests))
	}
	if _, statErr := os.Lstat(receiptPath); !os.IsNotExist(statErr) {
		t.Fatalf("input-binding refusal published receipt: %v", statErr)
	}
}

func TestInputBindingResolverRequestCarriesClosedSlotAndReference(t *testing.T) {
	def, _, _ := testDefinition(t, []string{"alpha", "beta"}, 0)
	root := t.TempDir()
	writeTestFile(t, root, "inputs/workload.json", "workload")
	writeTestFile(t, root, "fixtures/request-log.json", "fixture")
	writeTestFile(t, root, "contracts/behavioral.json", "contract")
	recorder := &recordingSlotResolver{values: bindingsForDefinition(def)}
	if _, err := ResolveInputs(context.Background(), recorder, root, def); err != nil {
		t.Fatalf("ResolveInputs(): %v", err)
	}
	want := []ResolveInputRequest{
		{Slot: InputSlotWorkload, Ref: def.Workload},
		{Slot: FixtureInputSlot(def.Fixtures[0].ID), Ref: def.Fixtures[0]},
		{Slot: InputSlotContract, Ref: def.Contract},
	}
	if !reflect.DeepEqual(recorder.requests, want) {
		t.Fatalf("resolver requests = %+v, want %+v", recorder.requests, want)
	}
}

type recordingSlotResolver struct {
	values   InputBindings
	requests []ResolveInputRequest
}

func (r *recordingSlotResolver) ResolveExperimentInput(_ context.Context, _ string, request ResolveInputRequest) (ResolvedInput, error) {
	r.requests = append(r.requests, request)
	binding := bindingBySlot(&r.values, request.Slot)
	return ResolvedInput{ID: binding.ID, Path: binding.Path, Digest: binding.Digest}, nil
}

func bindingsForDefinition(def experiment.Definition) InputBindings {
	inputs := []InputBinding{
		{Slot: InputSlotWorkload, ID: def.Workload.ID, Digest: def.Workload.Digest, Path: "inputs/workload.json"},
		{Slot: InputSlotContract, ID: def.Contract.ID, Digest: def.Contract.Digest, Path: "contracts/behavioral.json"},
	}
	for _, fixture := range def.Fixtures {
		inputs = append(inputs, InputBinding{Slot: FixtureInputSlot(fixture.ID), ID: fixture.ID, Digest: fixture.Digest, Path: "fixtures/request-log.json"})
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Slot < inputs[j].Slot })
	return InputBindings{Schema: InputBindingSchema, Inputs: inputs}
}

func bindingBySlot(bindings *InputBindings, slot InputSlot) *InputBinding {
	for index := range bindings.Inputs {
		if bindings.Inputs[index].Slot == slot {
			return &bindings.Inputs[index]
		}
	}
	panic("missing test binding for " + string(slot))
}
