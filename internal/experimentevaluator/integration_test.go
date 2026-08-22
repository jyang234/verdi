package experimentevaluator

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
)

func buildEvaluatorFixture(t *testing.T) (string, string) {
	t.Helper()
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
	return bin, digestBytes(binary)
}

func waitForFixturePath(path string, limit time.Duration) error {
	deadline := time.Now().Add(limit)
	for {
		_, err := os.Stat(path)
		if err == nil {
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("fixture path did not appear before cleanup deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestEvaluatorIntegrationUsesProfileCommandAndHermeticHelper(t *testing.T) {
	bin, binaryDigest := buildEvaluatorFixture(t)

	workspace := t.TempDir()
	profile, _, err := execworkspace.BuildProfile(workspace, t.TempDir(), execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantNetwork},
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{bin}},
		{Kind: execworkspace.GrantTimeouts, Seconds: 10},
	}}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	launch := Launch{Directory: workspace, Argv: []string{bin, "--mode=ok", "run"}, Digest: binaryDigest}

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

func TestEvaluatorDeadlineReturnsWhileDescendantRetainsOutput(t *testing.T) {
	bin, binaryDigest := buildEvaluatorFixture(t)
	workspace := t.TempDir()
	release := filepath.Join(t.TempDir(), "release")
	ready := release + ".ready"
	done := release + ".done"

	profile, _, err := execworkspace.BuildProfile(workspace, t.TempDir(), execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantNetwork},
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{bin}},
		{Kind: execworkspace.GrantTimeouts, Seconds: 1},
	}}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	launch := Launch{
		Directory: workspace,
		Argv:      []string{bin, "--mode=retain-output", release, "run"},
		Digest:    binaryDigest,
	}

	type result struct {
		err     error
		elapsed time.Duration
	}
	resultCh := make(chan result, 1)
	start := time.Now()
	go func() {
		_, err := Observe(context.Background(), profile, ObserveInput{
			Launch:  launch,
			Request: validProtocolRequest(experiment.CycleMeasured),
		})
		resultCh <- result{err: err, elapsed: time.Since(start)}
	}()

	if err := waitForFixturePath(ready, 2*time.Second); err != nil {
		if writeErr := os.WriteFile(release, nil, 0o600); writeErr != nil {
			t.Fatalf("wait for retained-output descendant: %v; release fixture: %v", err, writeErr)
		}
		select {
		case got := <-resultCh:
			t.Fatalf("wait for retained-output descendant: %v; Observe returned after %v: %v", err, got.elapsed, got.err)
		case <-time.After(2 * time.Second):
			t.Fatalf("wait for retained-output descendant: %v; Observe also remained blocked after fixture release", err)
		}
	}
	defer func() {
		if err := os.WriteFile(release, nil, 0o600); err != nil {
			t.Errorf("release retained-output descendant: %v", err)
			return
		}
		if err := waitForFixturePath(done, 2*time.Second); err != nil {
			t.Errorf("retained-output descendant cleanup: %v", err)
		}
	}()

	var got result
	select {
	case got = <-resultCh:
	case <-time.After(2 * time.Second):
		if err := os.WriteFile(release, nil, 0o600); err != nil {
			t.Fatalf("Observe remained blocked after its harness deadline; release fixture: %v", err)
		}
		select {
		case got = <-resultCh:
		case <-time.After(2 * time.Second):
			t.Fatal("Observe remained blocked after the retained-output descendant was released")
		}
		t.Fatalf("Observe remained blocked after its harness deadline until the descendant released inherited output; returned after %v: %v", got.elapsed, got.err)
	}
	if !IsOperational(got.err) || !errors.Is(got.err, ErrHarnessDeadline) {
		t.Fatalf("Observe error after %v = %v, want bounded operational ErrHarnessDeadline", got.elapsed, got.err)
	}
}
