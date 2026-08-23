//go:build !linux

package experimentrun

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
)

func TestWave3BJourneyUnsupportedPlatformRefusesBeforeCommandOrEvidence(t *testing.T) {
	root := t.TempDir()
	const experimentDir = "experiments/comparison"
	const run = "run-1"
	def, capabilities, _ := testDefinition(t, []string{"alpha", "beta"}, 0)
	capabilities.RequiresNetwork = false
	capabilitiesBytes, err := canonjson.Marshal(capabilities)
	if err != nil {
		t.Fatal(err)
	}
	def.Evaluator.CapabilitiesDigest = testDigestBytes(capabilitiesBytes)
	def = relockDefinition(t, def)
	writeStartAuthority(t, root, experimentDir, capabilitiesBytes, candidatePatches(t, def))
	writeResolvedInputs(t, root, def)
	paths, err := experiment.PathsForRun(experimentDir, run)
	if err != nil {
		t.Fatal(err)
	}
	materializer := &recordingMaterializer{root: root, receiptPath: filepath.Join(root, filepath.FromSlash(paths.Execution))}
	evaluator := &fixedAttemptEvaluator{}
	service, err := NewService(ServiceDependencies{
		Authorization: staticAuthorization{authorization: testAuthorization(t, def, false)},
		Inputs: staticInputs{values: map[string]ResolvedInput{
			def.Workload.ID:    {ID: def.Workload.ID, Path: "inputs/workload.json", Digest: def.Workload.Digest},
			def.Fixtures[0].ID: {ID: def.Fixtures[0].ID, Path: "fixtures/request-log.json", Digest: def.Fixtures[0].Digest},
			def.Contract.ID:    {ID: def.Contract.ID, Path: "contracts/behavioral.json", Digest: def.Contract.Digest},
		}},
		Materializer: materializer,
		Evaluator:    evaluator,
		Versions:     experiment.ReceiptVersions{Verdi: "v-test", RecommendationEngine: string(def.Algorithm)},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.Start(context.Background(), StartRequest{Root: root, ExperimentDir: experimentDir, Run: run, Definition: def})
	var operational *execworkspace.OperationalError
	if !errors.As(err, &operational) || operational.Op != "isolation-profile: apply-grants" || !strings.Contains(err.Error(), "network (deny unconfigurable on this platform)") {
		t.Fatalf("Start on %s error = %v, want CO-6 operational default-deny refusal", runtime.GOOS, err)
	}
	if len(materializer.requests) != 0 {
		t.Fatalf("unsupported isolation materialized %d candidates before refusal", len(materializer.requests))
	}
	if evaluator.calls != 0 {
		t.Fatalf("unsupported isolation constructed/executed %d evaluator calls before refusal", evaluator.calls)
	}
	for _, path := range []string{paths.Execution, paths.Observations, paths.Result} {
		if _, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(path))); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("unsupported isolation published %q: %v", path, statErr)
		}
	}
}
