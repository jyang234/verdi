package experimentdecision

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

// verifiableRun returns a locked definition, its complete observation set,
// and the closed engine's own Result for that evidence — the one (def,
// obs, res) triple VerifyResult must accept.
func verifiableRun(t *testing.T) (experiment.Definition, []experiment.Observation, experiment.Result) {
	t.Helper()
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	return def, obs, mustEvaluate(t, def, obs)
}

// TestVerifyResultAcceptsEngineOutput is SI-42's positive arm: the engine's
// own output for a locked definition and complete observation set verifies,
// and does so without any environment attestation — recompute-equality is
// an AT-REST check over artifacts, not an emission.
func TestVerifyResultAcceptsEngineOutput(t *testing.T) {
	def, obs, res := verifiableRun(t)
	if err := VerifyResult(def, obs, res); err != nil {
		t.Fatalf("VerifyResult() on the engine's own output: %v", err)
	}
}

// TestVerifyResultAcceptsCommittedFixtures runs the same check over the
// committed end-to-end fixtures, decoded from their bytes rather than
// built in memory.
func TestVerifyResultAcceptsCommittedFixtures(t *testing.T) {
	for _, name := range []string{"caching-proven", "caching-inconclusive"} {
		t.Run(name, func(t *testing.T) {
			def, obs := goldenExperiment(t, name)
			res, err := experiment.DecodeResult(goldenBytes(t, name, "result.json"))
			if err != nil {
				t.Fatalf("DecodeResult(%s): %v", name, err)
			}
			if err := VerifyResult(def, obs, res); err != nil {
				t.Fatalf("VerifyResult(%s) on the committed result.json: %v", name, err)
			}
		})
	}
}

// TestVerifyResultRejects is the negative table. Every case here is
// shape-, enum-, and digest-valid — each one DECODES cleanly through
// experiment.DecodeResult's checks — and is rejected only because it is
// not what the closed engine computes from the evidence (SI-42's whole
// point: a forgeable document is not authority).
func TestVerifyResultRejects(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, def *experiment.Definition, obs *[]experiment.Observation, res *experiment.Result)
	}{
		{"forged winner", func(t *testing.T, _ *experiment.Definition, _ *[]experiment.Observation, res *experiment.Result) {
			// Hand the win to the baseline's runner-up shape by claiming a
			// different eligible candidate won: shape-valid, engine-false.
			res.Winner = "baseline"
		}},
		{"forged verdict", func(t *testing.T, _ *experiment.Definition, _ *[]experiment.Observation, res *experiment.Result) {
			res.Verdict = experiment.VerdictDisclosedUnproven
			res.Winner = ""
			res.Reasons = []experiment.Reason{{Code: experiment.ReasonPracticalTie}}
		}},
		{"forged primary aggregate bytes", func(t *testing.T, _ *experiment.Definition, _ *[]experiment.Observation, res *experiment.Result) {
			for i := range res.Candidates {
				if res.Candidates[i].ID == "candidate-a" {
					primary := *res.Candidates[i].Primary
					primary.Value = json.Number("1")
					res.Candidates[i].Primary = &primary
				}
			}
		}},
		{"renumbered aggregate literal", func(t *testing.T, _ *experiment.Definition, _ *[]experiment.Observation, res *experiment.Result) {
			// Same VALUE, different literal: ResultDigest binds exact
			// json.Number literals (CO-3), so this is a genuine byte
			// difference and must fail closed too.
			for i := range res.Candidates {
				if res.Candidates[i].ID == "candidate-a" {
					primary := *res.Candidates[i].Primary
					primary.Value = json.Number("19.0")
					res.Candidates[i].Primary = &primary
				}
			}
		}},
		{"forged eligibility", func(t *testing.T, _ *experiment.Definition, _ *[]experiment.Observation, res *experiment.Result) {
			for i := range res.Candidates {
				res.Candidates[i].Bounds = nil
			}
		}},
		{"result bound to a different observation set", func(t *testing.T, _ *experiment.Definition, obs *[]experiment.Observation, _ *experiment.Result) {
			// Swap the evidence under a result the engine really did emit.
			for i := range *obs {
				if (*obs)[i].Candidate == "candidate-a" {
					(*obs)[i].Measurements[0] = measurement("request-latency", 39, "ms", experiment.SourceEvaluatorMeasured)
				}
			}
		}},
		{"unlocked definition", func(t *testing.T, def *experiment.Definition, _ *[]experiment.Observation, _ *experiment.Result) {
			def.Lock = nil
		}},
		{"tampered lock", func(t *testing.T, def *experiment.Definition, _ *[]experiment.Observation, _ *experiment.Result) {
			def.Lock = &experiment.Lock{DefinitionDigest: fixtureDigest("9")}
		}},
		{"incomplete observations", func(t *testing.T, _ *experiment.Definition, obs *[]experiment.Observation, _ *experiment.Result) {
			*obs = (*obs)[:len(*obs)-1]
		}},
		{"invalid result", func(t *testing.T, _ *experiment.Definition, _ *[]experiment.Observation, res *experiment.Result) {
			*res = experiment.Result{}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, obs, res := verifiableRun(t)
			obs = append([]experiment.Observation(nil), obs...)
			tt.mutate(t, &def, &obs, &res)

			err := VerifyResult(def, obs, res)
			if err == nil {
				t.Fatalf("VerifyResult(%s) = nil error, want an operational error", tt.name)
			}
			if !strings.HasPrefix(err.Error(), errPrefix) {
				t.Errorf("VerifyResult() error = %q, want the %q prefix", err.Error(), errPrefix)
			}
		})
	}
}

// copyFixture copies the committed fixture directory testdata/<name> into
// a fresh temporary repo root and returns that root, so a test can forge
// one artifact without ever writing to the committed fixtures.
func copyFixture(t *testing.T, name string) (root string) {
	t.Helper()
	root = t.TempDir()
	src := filepath.Join("testdata", name)

	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(root, name, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying fixture %s: %v", name, err)
	}
	return root
}

// forgeResult rewrites the copied fixture's result.json, replacing old
// with replacement exactly once and failing the test if old is absent —
// so a fixture edit can never turn this into a silently no-op forgery.
func forgeResult(t *testing.T, root, name, old, replacement string) {
	t.Helper()
	path := filepath.Join(root, name, "result.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("committed %s no longer contains %q", path, old)
	}
	forged := strings.Replace(string(data), old, replacement, 1)
	if err := os.WriteFile(path, []byte(forged), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// TestDeriveStateWithRealVerifierOverFixtures is SI-42's integration arm:
// experiment.DeriveState wired to THIS package's real recompute-equality
// verifier, over the committed fixture directories. The unforged copies
// must still reach their registered rungs, and a forged-but-shape-valid
// result.json — one that decodes cleanly, carries the right
// definition_digest and algorithm, and would pass every check that existed
// before SI-42 — must be a hard error, never a downgrade to measured.
func TestDeriveStateWithRealVerifierOverFixtures(t *testing.T) {
	tests := []struct {
		name      string
		fixture   string
		wantState experiment.State
		// forgery, when set, is a shape-valid rewrite of result.json that
		// the closed engine would never have produced.
		forgeOld string
		forgeNew string
	}{
		{
			name:      "unforged proven fixture",
			fixture:   "caching-proven",
			wantState: experiment.StateRecommended,
		},
		{
			name:      "unforged inconclusive fixture",
			fixture:   "caching-inconclusive",
			wantState: experiment.StateInconclusive,
		},
		{
			name:     "forged winner aggregate",
			fixture:  "caching-proven",
			forgeOld: `"eligible":true,"id":"facts-cache","primary":{"aggregation":"p95","rounds":3,"unit":"ms","value":19}`,
			forgeNew: `"eligible":true,"id":"facts-cache","primary":{"aggregation":"p95","rounds":3,"unit":"ms","value":9}`,
		},
		{
			name:     "forged verdict and winner",
			fixture:  "caching-inconclusive",
			forgeOld: `"reasons":[{"candidate":"facts-cache","code":"insufficient-separation","detail":"insufficient separation from candidate edge-cache"}],"run":"run-1","schema":"verdi.experiment-result/v1","verdict":"disclosed-unproven"`,
			forgeNew: `"run":"run-1","schema":"verdi.experiment-result/v1","verdict":"proven-winner","winner":"facts-cache"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := copyFixture(t, tt.fixture)
			forged := tt.forgeOld != ""
			if forged {
				forgeResult(t, root, tt.fixture, tt.forgeOld, tt.forgeNew)
				// The forgery must be a shape-valid document, or this test
				// would be proving DecodeResult's checks rather than SI-42's.
				raw, err := os.ReadFile(filepath.Join(root, tt.fixture, "result.json"))
				if err != nil {
					t.Fatalf("reading forged result: %v", err)
				}
				if _, err := experiment.DecodeResult(raw); err != nil {
					t.Fatalf("forged result.json does not decode cleanly (%v); the test must forge a SHAPE-VALID document", err)
				}
			}

			state, _, err := experiment.DeriveState(root, tt.fixture, VerifyResult)
			if forged {
				if err == nil {
					t.Fatalf("DeriveState() over a forged result.json = (%q, nil error), want an operational error", state)
				}
				if state != "" {
					t.Errorf("DeriveState() = %q alongside an error, want the zero State", state)
				}
				return
			}
			if err != nil {
				t.Fatalf("DeriveState() unexpected error: %v", err)
			}
			if state != tt.wantState {
				t.Errorf("DeriveState() = %q, want %q", state, tt.wantState)
			}
		})
	}
}

// TestVerifyResultIsTheDeriveStateVerifier proves the exported function
// satisfies internal/experiment's injected port type exactly — the wiring
// the import direction forbids doing from the other side (SI-42).
func TestVerifyResultIsTheDeriveStateVerifier(t *testing.T) {
	var verify experiment.ResultVerifier = VerifyResult
	def, obs, res := verifiableRun(t)
	if err := verify(def, obs, res); err != nil {
		t.Fatalf("wired verifier: %v", err)
	}
}
