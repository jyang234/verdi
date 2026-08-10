package experimentdecision

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

// goldenExperiment reads and decodes one committed end-to-end fixture
// directory under testdata/<name>: its locked definition and complete
// observation set. It never regenerates anything — every byte it reads
// was committed by hand once, via the throwaway generator this test
// file's sibling comment history describes, and stays static from then on.
func goldenExperiment(t *testing.T, name string) (def experiment.Definition, obs []experiment.Observation) {
	t.Helper()
	dir := filepath.Join("testdata", name)

	raw, err := os.ReadFile(filepath.Join(dir, "experiment.yaml"))
	if err != nil {
		t.Fatalf("reading %s/experiment.yaml: %v", name, err)
	}
	def, err = experiment.DecodeDefinition(raw)
	if err != nil {
		t.Fatalf("DecodeDefinition(%s): %v", name, err)
	}
	locked, err := experiment.Locked(def)
	if err != nil {
		t.Fatalf("Locked(%s): %v", name, err)
	}
	if !locked {
		t.Fatalf("%s: fixture definition is not locked", name)
	}

	rawObs, err := os.ReadFile(filepath.Join(dir, "observations.jsonl"))
	if err != nil {
		t.Fatalf("reading %s/observations.jsonl: %v", name, err)
	}
	obs, err = experiment.DecodeObservations(rawObs)
	if err != nil {
		t.Fatalf("DecodeObservations(%s): %v", name, err)
	}
	return def, obs
}

// goldenBytes reads a committed golden file's exact bytes, failing the
// test if it is absent.
func goldenBytes(t *testing.T, name, file string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name, file)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v", path, err)
	}
	return data
}

// TestCachingProvenFixture is CO-7's required committed deterministic
// caching fixture: a faster incorrect candidate (final-cache) loses to a
// slower correct candidate (facts-cache), end to end from the committed
// experiment directory through DeriveState, Evaluate, RenderResult, and
// RenderRecommendation.
func TestCachingProvenFixture(t *testing.T) {
	state, err := experiment.DeriveState("testdata", "caching-proven", VerifyResult)
	if err != nil {
		t.Fatalf("DeriveState(caching-proven): %v", err)
	}
	if state != experiment.StateRecommended {
		t.Fatalf("DeriveState(caching-proven) = %q, want %q", state, experiment.StateRecommended)
	}

	def, obs := goldenExperiment(t, "caching-proven")
	res, err := Evaluate(def, obs, attestation(def))
	if err != nil {
		t.Fatalf("Evaluate(caching-proven): %v", err)
	}
	if res.Verdict != experiment.VerdictProvenWinner {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictProvenWinner)
	}
	if res.Winner != "facts-cache" {
		t.Fatalf("Winner = %q, want %q", res.Winner, "facts-cache")
	}

	loser := candidateResult(t, res, "final-cache")
	if loser.Eligible {
		t.Fatalf("final-cache Eligible = true, want false (it fails behavioral-equivalence)")
	}
	if len(loser.Violations) != 1 {
		t.Fatalf("final-cache Violations = %+v, want exactly one preserved staleness witness", loser.Violations)
	}
	const wantWitness = "stale response detected after policy update in round 3"
	if loser.Violations[0].Witness != wantWitness {
		t.Fatalf("final-cache violation witness = %q, want %q", loser.Violations[0].Witness, wantWitness)
	}
	// final-cache is genuinely faster than facts-cache — the fixture is only
	// meaningful if the losing candidate's own primary aggregate would have
	// beaten the winner's had correctness not ruled it out.
	winner := candidateResult(t, res, "facts-cache")
	loserVal, err := loser.Primary.Value.Float64()
	if err != nil {
		t.Fatalf("final-cache primary value: %v", err)
	}
	winnerVal, err := winner.Primary.Value.Float64()
	if err != nil {
		t.Fatalf("facts-cache primary value: %v", err)
	}
	if loserVal >= winnerVal {
		t.Fatalf("final-cache primary %v is not faster than the winner's %v; the fixture no longer demonstrates faster-incorrect-loses", loserVal, winnerVal)
	}

	rendered, err := RenderResult(res)
	if err != nil {
		t.Fatalf("RenderResult(caching-proven): %v", err)
	}
	wantResult := goldenBytes(t, "caching-proven", "result.json")
	if string(rendered) != string(wantResult) {
		t.Fatalf("RenderResult(caching-proven) does not match the committed golden.\ngot:\n%s\nwant:\n%s", rendered, wantResult)
	}

	recommendation, err := RenderRecommendation(def, res)
	if err != nil {
		t.Fatalf("RenderRecommendation(caching-proven): %v", err)
	}
	wantRecommendation := goldenBytes(t, "caching-proven", "recommendation.md")
	if string(recommendation) != string(wantRecommendation) {
		t.Fatalf("RenderRecommendation(caching-proven) does not match the committed golden.\ngot:\n%s\nwant:\n%s", recommendation, wantRecommendation)
	}
}

// TestCachingInconclusiveFixture is the second committed end-to-end
// fixture: a genuinely improving candidate lands within the registered
// candidate_separation margin of another eligible candidate, producing a
// disclosed-unproven/insufficient-separation result rather than a forced
// winner.
func TestCachingInconclusiveFixture(t *testing.T) {
	state, err := experiment.DeriveState("testdata", "caching-inconclusive", VerifyResult)
	if err != nil {
		t.Fatalf("DeriveState(caching-inconclusive): %v", err)
	}
	if state != experiment.StateInconclusive {
		t.Fatalf("DeriveState(caching-inconclusive) = %q, want %q", state, experiment.StateInconclusive)
	}

	def, obs := goldenExperiment(t, "caching-inconclusive")
	res, err := Evaluate(def, obs, attestation(def))
	if err != nil {
		t.Fatalf("Evaluate(caching-inconclusive): %v", err)
	}
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonInsufficientSeparation {
		t.Fatalf("Reasons = %+v, want exactly one insufficient-separation", res.Reasons)
	}

	rendered, err := RenderResult(res)
	if err != nil {
		t.Fatalf("RenderResult(caching-inconclusive): %v", err)
	}
	wantResult := goldenBytes(t, "caching-inconclusive", "result.json")
	if string(rendered) != string(wantResult) {
		t.Fatalf("RenderResult(caching-inconclusive) does not match the committed golden.\ngot:\n%s\nwant:\n%s", rendered, wantResult)
	}

	recommendation, err := RenderRecommendation(def, res)
	if err != nil {
		t.Fatalf("RenderRecommendation(caching-inconclusive): %v", err)
	}
	wantRecommendation := goldenBytes(t, "caching-inconclusive", "recommendation.md")
	if string(recommendation) != string(wantRecommendation) {
		t.Fatalf("RenderRecommendation(caching-inconclusive) does not match the committed golden.\ngot:\n%s\nwant:\n%s", recommendation, wantRecommendation)
	}
}

// TestGoldenFixturesDeterministicAcrossFreshDecodes is CO-3's end-to-end
// determinism proof over the committed fixtures: decoding the SAME
// committed bytes twice, independently, and evaluating each decode
// produces byte-identical RenderResult output and an equal ResultDigest.
func TestGoldenFixturesDeterministicAcrossFreshDecodes(t *testing.T) {
	for _, name := range []string{"caching-proven", "caching-inconclusive"} {
		t.Run(name, func(t *testing.T) {
			def1, obs1 := goldenExperiment(t, name)
			def2, obs2 := goldenExperiment(t, name)

			res1, err := Evaluate(def1, obs1, attestation(def1))
			if err != nil {
				t.Fatalf("Evaluate() first decode: %v", err)
			}
			res2, err := Evaluate(def2, obs2, attestation(def2))
			if err != nil {
				t.Fatalf("Evaluate() second decode: %v", err)
			}

			rendered1, err := RenderResult(res1)
			if err != nil {
				t.Fatalf("RenderResult() first decode: %v", err)
			}
			rendered2, err := RenderResult(res2)
			if err != nil {
				t.Fatalf("RenderResult() second decode: %v", err)
			}
			if string(rendered1) != string(rendered2) {
				t.Fatalf("RenderResult differs across independent decodes of the same committed fixture")
			}

			d1, err := experiment.ResultDigest(res1)
			if err != nil {
				t.Fatalf("ResultDigest() first decode: %v", err)
			}
			d2, err := experiment.ResultDigest(res2)
			if err != nil {
				t.Fatalf("ResultDigest() second decode: %v", err)
			}
			if d1 != d2 {
				t.Fatalf("ResultDigest differs across independent decodes: %q vs %q", d1, d2)
			}
		})
	}
}
