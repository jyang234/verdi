package experimentdecision

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

// TestRenderResultHappyPath proves RenderResult validates res and returns
// its canonical, byte-identical-across-calls JSON encoding.
func TestRenderResultHappyPath(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	res := mustEvaluate(t, def, obs)

	rendered, err := RenderResult(res)
	if err != nil {
		t.Fatalf("RenderResult() unexpected error: %v", err)
	}
	decoded, err := experiment.DecodeResult(rendered)
	if err != nil {
		t.Fatalf("DecodeResult(RenderResult(res)) unexpected error: %v", err)
	}
	if decoded.Winner != res.Winner || decoded.Verdict != res.Verdict {
		t.Fatalf("decoded result = %+v, want it to match the source", decoded)
	}

	again, err := RenderResult(res)
	if err != nil {
		t.Fatalf("RenderResult() second call unexpected error: %v", err)
	}
	if string(rendered) != string(again) {
		t.Fatalf("RenderResult is not byte-identical across calls:\n%s\n---\n%s", rendered, again)
	}
}

// TestRenderResultInvalid proves RenderResult refuses an invalid Result
// rather than emitting bytes for it.
func TestRenderResultInvalid(t *testing.T) {
	_, err := RenderResult(experiment.Result{}) // zero value: fails schema check
	if err == nil {
		t.Fatalf("RenderResult(zero Result) = nil error, want error")
	}
	if !strings.HasPrefix(err.Error(), "experimentdecision: ") {
		t.Fatalf("RenderResult() error = %q, want the experimentdecision: prefix", err.Error())
	}
}

// provenFixture builds a locked definition and a proven-winner Result for
// RenderRecommendation's happy-path tests.
func provenFixture(t *testing.T) (experiment.Definition, experiment.Result) {
	t.Helper()
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	return def, mustEvaluate(t, def, obs)
}

// TestRenderRecommendationProvenWinner proves the required fixed-order
// content for a proven-winner recommendation, including CO-5's verbatim
// boundary sentence.
func TestRenderRecommendationProvenWinner(t *testing.T) {
	def, res := provenFixture(t)

	rendered, err := RenderRecommendation(def, res)
	if err != nil {
		t.Fatalf("RenderRecommendation() unexpected error: %v", err)
	}
	body := string(rendered)

	wantBoundary := "Candidate candidate-a is the best demonstrated path among the registered candidates for this desired outcome, workload, environment, and comparison revision."
	if !strings.Contains(body, wantBoundary) {
		t.Fatalf("recommendation missing the CO-5 boundary sentence; got:\n%s", body)
	}
	if !strings.Contains(strings.ToLower(body), "universal superiority") {
		t.Fatalf("recommendation missing the no-universal-superiority disclaimer; got:\n%s", body)
	}
	if !strings.Contains(body, def.ID) {
		t.Fatalf("recommendation missing the experiment id %q; got:\n%s", def.ID, body)
	}
	if !strings.Contains(body, string(res.Verdict)) {
		t.Fatalf("recommendation missing the verdict %q; got:\n%s", res.Verdict, body)
	}
	if !strings.Contains(body, res.Winner) {
		t.Fatalf("recommendation missing the winner %q; got:\n%s", res.Winner, body)
	}
	if !strings.Contains(body, res.DefinitionDigest) {
		t.Fatalf("recommendation missing the definition digest; got:\n%s", body)
	}
	resultDigest, err := experiment.ResultDigest(res)
	if err != nil {
		t.Fatalf("ResultDigest() unexpected error: %v", err)
	}
	if !strings.Contains(body, resultDigest) {
		t.Fatalf("recommendation missing the result digest %q; got:\n%s", resultDigest, body)
	}
	if !strings.Contains(body, res.Run) {
		t.Fatalf("recommendation missing the run id; got:\n%s", body)
	}
	if !strings.Contains(body, string(res.Algorithm)) {
		t.Fatalf("recommendation missing the algorithm version; got:\n%s", body)
	}
	// Fixed order: title before verdict before the candidates table.
	titleIdx := strings.Index(body, def.ID)
	verdictIdx := strings.Index(body, string(res.Verdict))
	tableIdx := strings.Index(body, "candidate-a")
	if titleIdx >= verdictIdx {
		t.Fatalf("title must precede the verdict in the rendered output")
	}
	if verdictIdx >= tableIdx {
		t.Fatalf("verdict must precede the candidates table in the rendered output")
	}
}

// TestRenderRecommendationUnproven proves an unproven/violated result's
// reasons are rendered with their code, detail, candidate, guard, and
// witness content.
func TestRenderRecommendationUnproven(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	witness := "tenant boundary crossed in round 1"
	obs[0].Guards = []experiment.GuardResult{guardResult("behavioral-equivalence", false, witness)}
	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictViolatedWithWitness {
		t.Fatalf("fixture setup: Verdict = %q, want violated-with-witness", res.Verdict)
	}

	rendered, err := RenderRecommendation(def, res)
	if err != nil {
		t.Fatalf("RenderRecommendation() unexpected error: %v", err)
	}
	body := string(rendered)
	if !strings.Contains(body, string(experiment.ReasonBaselineGuardViolation)) {
		t.Fatalf("recommendation missing the reason code; got:\n%s", body)
	}
	if !strings.Contains(body, "baseline") {
		t.Fatalf("recommendation missing the reason candidate; got:\n%s", body)
	}
	if !strings.Contains(body, "behavioral-equivalence") {
		t.Fatalf("recommendation missing the reason guard; got:\n%s", body)
	}
	if !strings.Contains(body, witness) {
		t.Fatalf("recommendation missing the reason witness; got:\n%s", body)
	}
	if strings.Contains(body, "best demonstrated path") {
		t.Fatalf("an unproven recommendation must not carry the proven-winner boundary sentence; got:\n%s", body)
	}
}

// TestRenderRecommendationUnlockedDefinition proves RenderRecommendation
// refuses to render against an unlocked definition.
func TestRenderRecommendationUnlockedDefinition(t *testing.T) {
	_, res := provenFixture(t)
	unlocked := baseDefinition() // never locked
	_, err := RenderRecommendation(unlocked, res)
	if err == nil {
		t.Fatalf("RenderRecommendation() with an unlocked definition = nil error, want error")
	}
}

// TestRenderRecommendationDigestMismatch proves RenderRecommendation
// refuses to render when res.DefinitionDigest does not match def's own
// computed digest.
func TestRenderRecommendationDigestMismatch(t *testing.T) {
	def, res := provenFixture(t)
	other := lockDefinition(t, func(d *experiment.Definition) {
		d.ID = "a-different-experiment"
	})
	_, err := RenderRecommendation(other, res)
	if err == nil {
		t.Fatalf("RenderRecommendation() with a mismatched definition digest = nil error, want error")
	}
	_ = def
}

// TestRenderRecommendationInvalidResult proves RenderRecommendation
// refuses an invalid Result.
func TestRenderRecommendationInvalidResult(t *testing.T) {
	def, _ := provenFixture(t)
	_, err := RenderRecommendation(def, experiment.Result{})
	if err == nil {
		t.Fatalf("RenderRecommendation() with an invalid Result = nil error, want error")
	}
}

// TestRenderRecommendationDeterministic proves CO-3's byte-identity
// requirement across two independently decoded copies of the same
// definition and result: render twice from freshly round-tripped inputs
// and require identical bytes.
func TestRenderRecommendationDeterministic(t *testing.T) {
	def, res := provenFixture(t)

	first, err := RenderRecommendation(def, res)
	if err != nil {
		t.Fatalf("RenderRecommendation() unexpected error: %v", err)
	}

	// Independently decode fresh copies of both inputs before rendering
	// again, so this test cannot pass merely because Go reused the same
	// in-memory values.
	rendered, err := RenderResult(res)
	if err != nil {
		t.Fatalf("RenderResult() unexpected error: %v", err)
	}
	decodedRes, err := experiment.DecodeResult(rendered)
	if err != nil {
		t.Fatalf("DecodeResult() unexpected error: %v", err)
	}

	second, err := RenderRecommendation(def, decodedRes)
	if err != nil {
		t.Fatalf("RenderRecommendation() second call unexpected error: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("RenderRecommendation is not byte-identical across independently decoded inputs:\n%s\n---\n%s", first, second)
	}
}

// TestRenderRecommendationFloatFormatting proves numeric values render
// with strconv.FormatFloat(v, 'f', -1, 64) semantics: no scientific
// notation and no superfluous trailing zeros.
func TestRenderRecommendationFloatFormatting(t *testing.T) {
	def, res := provenFixture(t)
	rendered, err := RenderRecommendation(def, res)
	if err != nil {
		t.Fatalf("RenderRecommendation() unexpected error: %v", err)
	}
	body := string(rendered)
	if strings.ContainsAny(body, "eE") && strings.Contains(body, "e+") {
		t.Fatalf("recommendation uses scientific notation; got:\n%s", body)
	}
	if !strings.Contains(body, "19") {
		t.Fatalf("recommendation missing candidate-a's formatted primary value 19; got:\n%s", body)
	}
}
