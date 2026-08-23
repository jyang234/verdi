//go:build linux

package experimentrun

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
)

func TestStartOperationalFailuresPublishNoObservationOrResult(t *testing.T) {
	for _, test := range []struct {
		name                 string
		configure            func(error, *recordingMaterializer) (WorkspaceMaterializer, AttemptEvaluator)
		want                 error
		wantError            string
		wantMaterializations int
	}{
		{
			name: "materializer failure",
			configure: func(failure error, _ *recordingMaterializer) (WorkspaceMaterializer, AttemptEvaluator) {
				return failingMaterializer{err: failure}, &recordingEvaluator{}
			},
			want: errFailureSentinel,
		},
		{
			name: "evaluator operational failure",
			configure: func(failure error, materializer *recordingMaterializer) (WorkspaceMaterializer, AttemptEvaluator) {
				return materializer, &fixedAttemptEvaluator{err: failure}
			},
			want:                 errFailureSentinel,
			wantMaterializations: 2,
		},
		{
			name: "evaluator cancellation",
			configure: func(_ error, materializer *recordingMaterializer) (WorkspaceMaterializer, AttemptEvaluator) {
				return materializer, &fixedAttemptEvaluator{err: context.Canceled}
			},
			want:                 context.Canceled,
			wantMaterializations: 2,
		},
		{
			name: "invalid injected attempt identity",
			configure: func(_ error, materializer *recordingMaterializer) (WorkspaceMaterializer, AttemptEvaluator) {
				request := validServiceEvaluatorRequest()
				attempt := validServiceAttempt(request)
				return materializer, &fixedAttemptEvaluator{attempt: attempt}
			},
			wantError:            "experiment digest",
			wantMaterializations: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			const experimentDir = "experiments/comparison"
			const run = "run-1"
			def, capabilities, _ := testDefinition(t, []string{"alpha", "beta"}, 0)
			capabilities.RequiresNetwork = true
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
			recording := &recordingMaterializer{
				root:        root,
				receiptPath: filepath.Join(root, filepath.FromSlash(paths.Execution)),
			}
			materializer, evaluator := test.configure(errFailureSentinel, recording)
			service, err := NewService(ServiceDependencies{
				Authorization: staticAuthorization{authorization: testAuthorization(t, def, true)},
				Inputs: staticInputs{values: map[string]ResolvedInput{
					def.Workload.ID:    {ID: def.Workload.ID, Path: "inputs/workload.json", Digest: def.Workload.Digest},
					def.Fixtures[0].ID: {ID: def.Fixtures[0].ID, Path: "fixtures/request-log.json", Digest: def.Fixtures[0].Digest},
					def.Contract.ID:    {ID: def.Contract.ID, Path: "contracts/behavioral.json", Digest: def.Contract.Digest},
				}},
				Materializer: materializer,
				Evaluator:    evaluator,
				Versions: experiment.ReceiptVersions{
					Verdi:                "v-test",
					RecommendationEngine: string(def.Algorithm),
				},
			})
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}

			_, err = service.Start(context.Background(), StartRequest{Root: root, ExperimentDir: experimentDir, Run: run, Definition: def})
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("Start error = %v, want errors.Is(_, %v)", err, test.want)
			}
			if test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("Start error = %v, want %q", err, test.wantError)
			}
			if _, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(paths.Execution))); statErr != nil {
				t.Fatalf("receipt should remain as the pre-execution proof locus: %v", statErr)
			}
			for _, path := range []string{paths.Observations, paths.Result} {
				if _, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(path))); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("operational failure published %q: %v", path, statErr)
				}
			}
			if got := len(recording.requests); got != test.wantMaterializations {
				t.Fatalf("materializations after failure = %d, want %d", got, test.wantMaterializations)
			}
		})
	}
}

var errFailureSentinel = errors.New("injected operational failure")

type failingMaterializer struct {
	err error
}

func (f failingMaterializer) Materialize(context.Context, execworkspace.Request) (execworkspace.Result, error) {
	return execworkspace.Result{}, f.err
}
