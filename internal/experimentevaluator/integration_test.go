package experimentevaluator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
)

func TestEvaluatorIntegrationUsesProfileCommandAndHermeticHelper(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fixture-evaluator")
	build := exec.Command("go", "build", "-o", bin, "./testdata/evaluator")
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fixture evaluator: %v\n%s", err, out)
	}
	binary, err := os.ReadFile(bin)
	if err != nil {
		t.Fatalf("read fixture evaluator: %v", err)
	}

	workspace := t.TempDir()
	profile, _, err := execworkspace.BuildProfile(workspace, t.TempDir(), execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantNetwork},
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{bin}},
		{Kind: execworkspace.GrantTimeouts, Seconds: 10},
	}}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	launch := Launch{Directory: workspace, Argv: []string{bin, "--mode=ok", "run"}, Digest: digestBytes(binary)}

	discovery, err := Discover(context.Background(), profile, DiscoverInput{
		Launch:             launch,
		CapabilitiesDigest: digestBytes([]byte(canonicalCapabilities)),
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if string(discovery.Bytes) != canonicalCapabilities {
		t.Fatalf("Discover bytes = %q, want canned canonical capabilities", discovery.Bytes)
	}

	attempt, err := Observe(context.Background(), profile, ObserveInput{Launch: launch, Request: validProtocolRequest(experiment.CycleMeasured)})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if attempt.Observation == nil {
		t.Fatal("Observe returned nil measured observation")
	}
	if err := attempt.Observation.Validate(); err != nil {
		t.Fatalf("measured observation Validate: %v", err)
	}
	if len(attempt.ProcessMeasurements) == 0 || attempt.ProcessMeasurements[0].ID != experiment.EvaluatorWallDurationMetricID {
		t.Fatalf("process measurements = %+v, want fixed wall duration", attempt.ProcessMeasurements)
	}
}
