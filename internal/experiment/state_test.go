package experiment

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
)

// acceptResult and rejectResult are the stub ResultVerifier ports this
// package's own tests inject (SI-43). The real recompute-equality
// verifier lives in internal/experimentdecision — which imports this
// package, so it can never be wired from here; the injected-port shape is
// exactly what keeps that import direction one-way. The integration test
// wiring the REAL verifier over committed fixtures lives on the
// experimentdecision side.
func acceptResult(Definition, []Observation, *ExecutionReceipt, Result) error { return nil }

func rejectResult(Definition, []Observation, *ExecutionReceipt, Result) error {
	return errors.New("stub verifier: recomputed result differs")
}

// writeFile writes content to path (relative to dir), creating parent
// directories as needed.
func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", full, err)
	}
}

// writeExperimentFile writes content at relPath INSIDE the fixture
// experiment's own repo-relative directory (testExperimentDir) under the
// repo root root — the layout DeriveState's two arguments describe.
func writeExperimentFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	if relPath == observationsFile || relPath == resultFile || relPath == executionFile {
		relPath = filepath.Join(runsDirectory, "run-1", relPath)
	}
	writeFile(t, root, filepath.Join(testExperimentDir, relPath), content)
}

func writeRunFile(t *testing.T, root, run, relPath, content string) {
	t.Helper()
	writeFile(t, root, filepath.Join(testExperimentDir, runsDirectory, run, relPath), content)
}

// lockedDefinitionDoc returns validDefinitionYAML — with rounds reduced to
// 2 so a COMPLETE observation set stays small (completeObservationsJSONLForDigest
// below provides exactly the 2 candidates x 2 rounds this registers) — with
// a correct lock block appended, and the digest it locks against.
func lockedDefinitionDoc(t *testing.T) (doc, digest string) {
	t.Helper()
	unlocked := mutate(t, "rounds: 10", "rounds: 2")
	def := mustDecodeDefinition(t, unlocked)
	digest, err := DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	return unlocked + "lock:\n  definition_digest: " + digest + "\n", digest
}

func capabilitiesAuthorityForState(t *testing.T, guards []string, evaluatorVersion string) ([]byte, string) {
	t.Helper()
	capabilities := Capabilities{
		Schema:           CapabilitiesSchemaV2,
		EvaluatorVersion: evaluatorVersion,
		ProtocolVersions: []string{EvaluatorProtocolSchema, ObservationSchemaV2},
		Metrics: []CapabilityMetric{
			{ID: "request-latency", Type: MetricDuration, Unit: "ms", Direction: DirectionLower},
		},
		Guards:           guards,
		RequiresNetwork:  false,
		RequiresElevated: false,
	}
	if err := capabilities.Validate(); err != nil {
		t.Fatalf("Capabilities.Validate(): %v", err)
	}
	raw, err := canonjson.Marshal(capabilities)
	if err != nil {
		t.Fatalf("canonjson.Marshal(capabilities): %v", err)
	}
	return raw, sha256Digest(raw)
}

func lockedV2DefinitionDoc(t *testing.T, capabilitiesDigest string) (doc, digest string) {
	t.Helper()
	unlocked := mutate(t, "rounds: 10", "rounds: 2")
	unlocked = strings.Replace(unlocked, "capabilities_digest: "+digestOf("4"), "capabilities_digest: "+capabilitiesDigest, 1)
	unlocked = strings.Replace(unlocked, "- id: peak-rss", "- id: "+EvaluatorPeakRSSMetricID, 1)
	def := mustDecodeDefinition(t, unlocked)
	digest, err := DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	return unlocked + "lock:\n  definition_digest: " + digest + "\n", digest
}

// writeCandidatePatches writes the real patch content the shared fixture
// definition registers digests for (baselinePatchContent,
// factsCachePatchContent — definition_test.go), so DeriveState's
// commit-3-strengthened registered rung, which verifies each patch
// against its registered digest, sees genuinely matching bytes.
func writeCandidatePatches(t *testing.T, dir string) {
	t.Helper()
	writeExperimentFile(t, dir, "candidates/baseline.patch", baselinePatchContent)
	writeExperimentFile(t, dir, "candidates/facts-cache.patch", factsCachePatchContent)
}

// completeObservationsJSONLForDigest returns a full, valid observation set
// for the rounds:2 fixture definition lockedDefinitionDoc registers:
// both candidates, both rounds, every required guard passing, and every
// registered measurement present with a decision-eligible source
// (reusing observations_validation_test.go's shared record builders).
func completeObservationsJSONLForDigest(defDigest string) string {
	return strings.Join(validObservationLines(defDigest), "\n") + "\n"
}

func validResultJSONForDigest(defDigest string, verdict Verdict) string {
	verdictBlock := `"verdict": "proven-winner",
  "winner": "facts-cache",`
	candidatesBlock := `[
    {"id": "baseline", "baseline": true, "eligible": true},
    {"id": "facts-cache", "baseline": false, "eligible": true}
  ]`
	if verdict != VerdictProvenWinner {
		verdictBlock = `"verdict": "` + string(verdict) + `",
  "reasons": [{"code": "practical-tie"}],`
	}
	return `{
  "schema": "verdi.experiment-result/v1",
  "experiment": "cache-placement-v1",
  "definition_digest": "` + defDigest + `",
  "run": "run-1",
  "algorithm": "verdi.experiment-recommendation/v1",
  ` + verdictBlock + `
  "candidates": ` + candidatesBlock + `,
  "observations_digest": "` + digestOf("b") + `"
}`
}

// TestDeriveStateRejectsInvalidExperimentDir proves the experiment
// directory is a REPO-RELATIVE coordinate and never a filesystem one: an
// absolute (or otherwise non-canonical) experimentDir is refused outright,
// because such a value could never be compared to the repo-relative paths
// a candidate patch names and would silently disarm the self-check.
func TestDeriveStateRejectsInvalidExperimentDir(t *testing.T) {
	dirs := []string{"", "/experiments/cache-placement-v1", "experiments/../experiments/cache-placement-v1", "./experiments/cache-placement-v1"}
	for _, experimentDir := range dirs {
		t.Run("experimentDir "+experimentDir, func(t *testing.T) {
			root := t.TempDir()
			doc, _ := lockedDefinitionDoc(t)
			writeFile(t, root, filepath.Join("experiments/cache-placement-v1", "experiment.yaml"), doc)
			writeCandidatePatches(t, root)
			if _, _, err := DeriveState(root, experimentDir, acceptResult); err == nil {
				t.Errorf("DeriveState(root, %q) = nil error, want error", experimentDir)
			}
		})
	}
}

// TestDeriveStateAbsoluteRepoRootReadsThroughExperimentDir proves the two
// arguments do different jobs: an absolute filesystem root still reaches
// the artifacts, while the repo-relative experimentDir is what the
// protected-input self-check compares against.
func TestDeriveStateAbsoluteRepoRootReadsThroughExperimentDir(t *testing.T) {
	root := t.TempDir()
	if !filepath.IsAbs(root) {
		t.Fatalf("t.TempDir() = %q, want an absolute path", root)
	}
	doc, _ := lockedDefinitionDoc(t)
	writeExperimentFile(t, root, "experiment.yaml", doc)
	writeCandidatePatches(t, root)

	state, _, err := DeriveState(root, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if state != StateRegistered {
		t.Errorf("DeriveState() = %q, want %q", state, StateRegistered)
	}
}

func TestDeriveStateDetailsFromSourceMatchesFilesystem(t *testing.T) {
	root := t.TempDir()
	doc, _ := lockedDefinitionDoc(t)
	writeExperimentFile(t, root, definitionFile, doc)
	writeCandidatePatches(t, root)

	filesystem, err := DeriveStateDetails(root, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveStateDetails() error = %v", err)
	}
	fromSource, err := DeriveStateDetailsFromSource(os.DirFS(root), testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveStateDetailsFromSource() error = %v", err)
	}
	if !reflect.DeepEqual(fromSource, filesystem) {
		t.Fatalf("source derivation = %#v, filesystem derivation = %#v", fromSource, filesystem)
	}
}

// TestDeriveStateSelfTouchingPatchIsError proves the experiment-directory
// self-check is genuinely wired through DeriveState: a registered patch
// that edits the experiment's own directory is an error, never a silently
// skipped protected input.
func TestDeriveStateSelfTouchingPatchIsError(t *testing.T) {
	root := t.TempDir()
	selfPatch := "diff --git a/" + testExperimentDir + "/notes.md b/" + testExperimentDir + "/notes.md\n"
	doc := validDefinitionYAML()
	doc = strings.Replace(doc, "digest: "+baselinePatchDigest, "digest: "+sha256Digest([]byte(selfPatch)), 1)
	doc = strings.Replace(doc, "rounds: 10", "rounds: 2", 1)
	def := mustDecodeDefinition(t, doc)
	digest, err := DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	writeExperimentFile(t, root, "experiment.yaml", doc+"lock:\n  definition_digest: "+digest+"\n")
	writeExperimentFile(t, root, "candidates/baseline.patch", selfPatch)
	writeExperimentFile(t, root, "candidates/facts-cache.patch", factsCachePatchContent)

	if _, _, err := DeriveState(root, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with a patch touching the experiment's own directory = nil error, want error")
	}
}

func TestDeriveStateExploratoryNoDefinition(t *testing.T) {
	dir := t.TempDir()
	state, _, err := DeriveState(dir, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if state != StateExploratory {
		t.Errorf("DeriveState() = %q, want %q", state, StateExploratory)
	}
}

func TestDeriveStateExploratoryUnlockedDefinition(t *testing.T) {
	dir := t.TempDir()
	writeExperimentFile(t, dir, "experiment.yaml", validDefinitionYAML())
	state, _, err := DeriveState(dir, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if state != StateExploratory {
		t.Errorf("DeriveState() = %q, want %q", state, StateExploratory)
	}
}

func TestDeriveStateUndecodableDefinitionIsError(t *testing.T) {
	dir := t.TempDir()
	writeExperimentFile(t, dir, "experiment.yaml", "not: valid: yaml: at all:\n")
	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with corrupt experiment.yaml = nil error, want error")
	}
}

func TestDeriveStateTamperedLockIsError(t *testing.T) {
	dir := t.TempDir()
	doc := validDefinitionYAML() + "lock:\n  definition_digest: " + digestOf("9") + "\n"
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with a tampered lock = nil error, want error")
	}
}

func TestDeriveStateRegistered(t *testing.T) {
	dir := t.TempDir()
	doc, _ := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)

	state, _, err := DeriveState(dir, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if state != StateRegistered {
		t.Errorf("DeriveState() = %q, want %q", state, StateRegistered)
	}
}

func TestDeriveStateRegisteredMissingPatchIsError(t *testing.T) {
	dir := t.TempDir()
	doc, _ := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeExperimentFile(t, dir, "candidates/baseline.patch", baselinePatchContent)
	// facts-cache.patch deliberately absent: a locked definition's
	// registered candidate patches must exist; absence here is a store
	// inconsistency, not a legitimate lower rung (see DeriveState's own
	// doc comment).

	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with a missing registered candidate patch = nil error, want error")
	}
}

func TestDeriveStateMeasured(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))

	state, _, err := DeriveState(dir, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if state != StateMeasured {
		t.Errorf("DeriveState() = %q, want %q", state, StateMeasured)
	}
}

func TestDeriveStateMeasuredBadEnvelopeDigestIsError(t *testing.T) {
	dir := t.TempDir()
	doc, _ := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digestOf("0")))

	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with a mismatched observation experiment_digest = nil error, want error")
	}
}

func TestDeriveStateMeasuredUndecodableObservationsIsError(t *testing.T) {
	dir := t.TempDir()
	doc, _ := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", "not json\n")

	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with undecodable observations.jsonl = nil error, want error")
	}
}

func TestDeriveStateRecommended(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeExperimentFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictProvenWinner))

	state, _, err := DeriveState(dir, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if state != StateRecommended {
		t.Errorf("DeriveState() = %q, want %q", state, StateRecommended)
	}
}

func TestDeriveStateInconclusive(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeExperimentFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictDisclosedUnproven))

	state, _, err := DeriveState(dir, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if state != StateInconclusive {
		t.Errorf("DeriveState() = %q, want %q", state, StateInconclusive)
	}
}

func TestDeriveStateResultDigestMismatchIsError(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeExperimentFile(t, dir, "result.json", validResultJSONForDigest(digestOf("0"), VerdictProvenWinner))

	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with result.definition_digest mismatched = nil error, want error")
	}
}

func TestDeriveStateRatified(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	resultDoc := validResultJSONForDigest(digest, VerdictProvenWinner)
	writeExperimentFile(t, dir, "result.json", resultDoc)

	res, err := DecodeResult([]byte(resultDoc))
	if err != nil {
		t.Fatalf("DecodeResult() unexpected error: %v", err)
	}
	resultDigest, err := ResultDigest(res)
	if err != nil {
		t.Fatalf("ResultDigest() unexpected error: %v", err)
	}
	ratificationDoc := "schema: verdi.experiment-ratification/v1\n" +
		"result_digest: " + resultDigest + "\n" +
		"actor: " + validActor + "\n" +
		"disposition: select-recommended\n"
	writeExperimentFile(t, dir, "ratification.yaml", ratificationDoc)

	state, _, err := DeriveState(dir, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if state != StateRatified {
		t.Errorf("DeriveState() = %q, want %q", state, StateRatified)
	}
}

func TestDeriveStateRatificationDigestMismatchIsError(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeExperimentFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictProvenWinner))

	ratificationDoc := "schema: verdi.experiment-ratification/v1\n" +
		"result_digest: " + digestOf("0") + "\n" +
		"actor: " + validActor + "\n" +
		"disposition: select-recommended\n"
	writeExperimentFile(t, dir, "ratification.yaml", ratificationDoc)

	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with ratification.result_digest mismatched = nil error, want error")
	}
}

func TestDeriveStateUndecodableRatificationIsError(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeExperimentFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictProvenWinner))
	writeExperimentFile(t, dir, "ratification.yaml", "not: valid: yaml: at all:\n")

	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with undecodable ratification.yaml = nil error, want error")
	}
}

// TestDeriveStateNilVerifierIsError proves the result verifier is a
// REQUIRED input, not an optional hardening step (SI-43): without it a
// present result.json could only be trusted on its shape, which is exactly
// the forgeable state the port exists to close. The check fires before any
// artifact is read, so a nil verifier never yields a lower rung either.
func TestDeriveStateNilVerifierIsError(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeExperimentFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictProvenWinner))

	if _, _, err := DeriveState(dir, testExperimentDir, nil); err == nil {
		t.Errorf("DeriveState() with a nil verifier = nil error, want error")
	}
	// Not even the rungs below result.json may be reported without it.
	empty := t.TempDir()
	if _, _, err := DeriveState(empty, testExperimentDir, nil); err == nil {
		t.Errorf("DeriveState() with a nil verifier and no artifacts = nil error, want error")
	}
}

// TestDeriveStateFailedResultVerificationIsError proves a present
// result.json that the verifier rejects is a hard operational error and
// never a silent downgrade to the measured rung (SI-43): the artifact IS
// present, and its disagreement with the closed engine's recomputation is
// itself the fact worth reporting.
func TestDeriveStateFailedResultVerificationIsError(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeExperimentFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictProvenWinner))

	state, _, err := DeriveState(dir, testExperimentDir, rejectResult)
	if err == nil {
		t.Fatalf("DeriveState() with a rejected result = (%q, nil error), want error", state)
	}
	if state != "" {
		t.Errorf("DeriveState() = %q alongside an error, want the zero State", state)
	}
}

// TestDeriveStateVerifierUnusedWithoutResult proves the verifier gates a
// PRESENT result.json only: an experiment that has not been evaluated yet
// still reports measured, even under a verifier that rejects everything.
func TestDeriveStateVerifierUnusedWithoutResult(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))

	state, _, err := DeriveState(dir, testExperimentDir, rejectResult)
	if err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if state != StateMeasured {
		t.Errorf("DeriveState() = %q, want %q", state, StateMeasured)
	}
}

// TestDeriveStateVerifierReceivesDecodedArtifacts proves the port is
// handed the three decoded artifacts a recompute needs — the locked
// definition, the complete observation set, and the decoded result — and
// not, say, raw bytes or a partially validated definition.
func TestDeriveStateVerifierReceivesDecodedArtifacts(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeExperimentFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictProvenWinner))

	calls := 0
	var gotDef Definition
	var gotObs []Observation
	var gotRes Result
	verify := func(def Definition, obs []Observation, _ *ExecutionReceipt, res Result) error {
		calls++
		gotDef, gotObs, gotRes = def, obs, res
		return nil
	}

	if _, _, err := DeriveState(dir, testExperimentDir, verify); err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("verifier called %d times, want exactly 1", calls)
	}
	if locked, err := Locked(gotDef); err != nil || !locked {
		t.Errorf("verifier received def with Locked() = (%v, %v), want (true, nil)", locked, err)
	}
	if len(gotObs) != 4 {
		t.Errorf("verifier received %d observations, want the complete set of 4", len(gotObs))
	}
	if gotRes.Verdict != VerdictProvenWinner {
		t.Errorf("verifier received result verdict %q, want %q", gotRes.Verdict, VerdictProvenWinner)
	}
}

// TestDeriveStateRatificationBindingIsEnforced proves SI-45's
// preconditions are wired into the ratified rung and not merely available
// as a helper: a ratification whose disposition cannot be true of the
// definition and result it is bound to is a hard error, exactly like every
// other present-but-invalid artifact.
func TestDeriveStateRatificationBindingIsEnforced(t *testing.T) {
	// ratificationFor writes result.json for verdict and a ratification
	// bound to that result's own digest, then returns the derived state.
	ratificationFor := func(t *testing.T, verdict Verdict, dispositionBlock string) (State, error) {
		t.Helper()
		dir := t.TempDir()
		doc, digest := lockedDefinitionDoc(t)
		writeExperimentFile(t, dir, "experiment.yaml", doc)
		writeCandidatePatches(t, dir)
		writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
		resultDoc := validResultJSONForDigest(digest, verdict)
		writeExperimentFile(t, dir, "result.json", resultDoc)

		res, err := DecodeResult([]byte(resultDoc))
		if err != nil {
			t.Fatalf("DecodeResult() unexpected error: %v", err)
		}
		resultDigest, err := ResultDigest(res)
		if err != nil {
			t.Fatalf("ResultDigest() unexpected error: %v", err)
		}
		writeExperimentFile(t, dir, "ratification.yaml", "schema: verdi.experiment-ratification/v1\n"+
			"result_digest: "+resultDigest+"\n"+
			"actor: "+validActor+"\n"+
			dispositionBlock)

		state, _, err := DeriveState(dir, testExperimentDir, acceptResult)
		return state, err
	}

	tests := []struct {
		name             string
		verdict          Verdict
		dispositionBlock string
		wantErr          bool
	}{
		{
			name:             "select-recommended over a proven winner",
			verdict:          VerdictProvenWinner,
			dispositionBlock: "disposition: select-recommended\n",
		},
		{
			name:             "select-recommended over an inconclusive result",
			verdict:          VerdictDisclosedUnproven,
			dispositionBlock: "disposition: select-recommended\n",
			wantErr:          true,
		},
		{
			name:             "select-other naming the other registered candidate",
			verdict:          VerdictProvenWinner,
			dispositionBlock: "disposition: select-other\ncandidate: baseline\nreason: lower operational risk\n",
		},
		{
			name:             "select-other naming the recommended winner",
			verdict:          VerdictProvenWinner,
			dispositionBlock: "disposition: select-other\ncandidate: facts-cache\nreason: lower operational risk\n",
			wantErr:          true,
		},
		{
			name:             "select-other naming an unregistered candidate",
			verdict:          VerdictProvenWinner,
			dispositionBlock: "disposition: select-other\ncandidate: nonexistent\nreason: lower operational risk\n",
			wantErr:          true,
		},
		{
			name:             "reject-all over an inconclusive result",
			verdict:          VerdictDisclosedUnproven,
			dispositionBlock: "disposition: reject-all\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := ratificationFor(t, tt.verdict, tt.dispositionBlock)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DeriveState() = (%q, nil error), want an operational error", state)
				}
				if state != "" {
					t.Errorf("DeriveState() = %q alongside an error, want the zero State", state)
				}
				return
			}
			if err != nil {
				t.Fatalf("DeriveState() unexpected error: %v", err)
			}
			if state != StateRatified {
				t.Errorf("DeriveState() = %q, want %q", state, StateRatified)
			}
		})
	}
}

// TestDeriveStateTamperedCandidatePatchIsError proves the commit-3
// demotion->error behavior: a locked definition whose candidate patch
// bytes on disk no longer match the registered digest is a hard error,
// never a silent report of "registered" (the file IS present) or
// "exploratory" (the definition IS locked).
func TestDeriveStateTamperedCandidatePatchIsError(t *testing.T) {
	dir := t.TempDir()
	doc, _ := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeExperimentFile(t, dir, "candidates/baseline.patch", "diff --git a/tampered b/tampered\n")
	writeExperimentFile(t, dir, "candidates/facts-cache.patch", factsCachePatchContent)

	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with a tampered candidate patch = nil error, want error")
	}
}

// TestDeriveStateTamperedPatchProtectedPathIsError proves a registered
// patch that touches a protected path is rejected even when its bytes
// match the registered digest (i.e. the protected-path violation was
// present at registration time already) — DeriveState surfaces it as an
// error rather than reporting "registered".
func TestDeriveStateTamperedPatchProtectedPathIsError(t *testing.T) {
	dir := t.TempDir()
	badPatch := "diff --git a/internal/cache/store.go b/internal/cache/store.go\n"
	doc := validDefinitionYAML()
	doc = strings.Replace(doc, "digest: "+baselinePatchDigest, "digest: "+sha256Digest([]byte(badPatch)), 1)
	doc = strings.Replace(doc, "rounds: 10", "rounds: 2", 1)
	def := mustDecodeDefinition(t, doc)
	digest, err := DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	writeExperimentFile(t, dir, "experiment.yaml", doc+"lock:\n  definition_digest: "+digest+"\n")
	writeExperimentFile(t, dir, "candidates/baseline.patch", badPatch)
	writeExperimentFile(t, dir, "candidates/facts-cache.patch", factsCachePatchContent)

	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with a registered patch touching a protected path = nil error, want error")
	}
}

// TestDeriveStateIncompleteRunRemainsRegistered proves that an
// observations.jsonl file which is present, decodes, and digest-links
// correctly but does not cover every registered (candidate, round) pair
// is an enumerable registered run and does not invent completed evidence.
func TestDeriveStateIncompleteRunRemainsRegistered(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	lines := validObservationLines(digest)
	writeExperimentFile(t, dir, "observations.jsonl", strings.Join(lines[:3], "\n")+"\n") // drop facts-cache round 2

	derived, err := DeriveStateDetails(dir, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveStateDetails(): %v", err)
	}
	if derived.State != StateRegistered || len(derived.Runs) != 1 || derived.Runs[0].State != StateRegistered {
		t.Fatalf("derived state = %+v", derived)
	}
}

func observationsJSONLForRun(defDigest, run string, complete bool) string {
	lines := validObservationLines(defDigest)
	if !complete {
		lines = lines[:1]
	}
	return strings.ReplaceAll(strings.Join(lines, "\n")+"\n", `"run": "run-1"`, `"run": "`+run+`"`)
}

func resultJSONForRun(defDigest, run string, verdict Verdict) string {
	return strings.Replace(validResultJSONForDigest(defDigest, verdict), `"run": "run-1"`, `"run": "`+run+`"`, 1)
}

func TestDeriveStateDetailsEnumeratesRunsWithoutSelectingTheFavorableOne(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, definitionFile, doc)
	writeCandidatePatches(t, dir)
	writeRunFile(t, dir, "run-2", observationsFile, observationsJSONLForRun(digest, "run-2", false))
	writeRunFile(t, dir, "run-1", observationsFile, observationsJSONLForRun(digest, "run-1", true))
	writeRunFile(t, dir, "run-1", resultFile, resultJSONForRun(digest, "run-1", VerdictProvenWinner))

	derived, err := DeriveStateDetails(dir, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveStateDetails(): %v", err)
	}
	if derived.State != StateRecommended || len(derived.Runs) != 2 {
		t.Fatalf("derived = %+v", derived)
	}
	if derived.Runs[0].Run != "run-1" || derived.Runs[0].State != StateRecommended || derived.Runs[1].Run != "run-2" || derived.Runs[1].State != StateRegistered {
		t.Fatalf("run enumeration = %+v", derived.Runs)
	}

	writeRunFile(t, dir, "run-2", observationsFile, observationsJSONLForRun(digest, "run-2", true))
	writeRunFile(t, dir, "run-2", resultFile, resultJSONForRun(digest, "run-2", VerdictDisclosedUnproven))
	derived, err = DeriveStateDetails(dir, testExperimentDir, acceptResult)
	if err != nil {
		t.Fatalf("DeriveStateDetails(second result): %v", err)
	}
	if derived.State != StateInconclusive || derived.Runs[1].State != StateInconclusive {
		t.Fatalf("mixed result aggregate = %+v", derived)
	}
}

func TestDeriveStateRejectsMalformedRunDirectoryEntry(t *testing.T) {
	dir := t.TempDir()
	doc, _ := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, definitionFile, doc)
	writeCandidatePatches(t, dir)
	writeRunFile(t, dir, "run-1", "latest", "not an authority artifact")
	if _, err := DeriveStateDetails(dir, testExperimentDir, acceptResult); err == nil {
		t.Fatalf("DeriveStateDetails(run with unknown artifact) = nil error")
	}
}

func completeObservationsV2JSONL(t *testing.T, defDigest, run string) ([]Observation, string) {
	t.Helper()
	v1, err := DecodeObservations([]byte(observationsJSONLForRun(defDigest, run, true)))
	if err != nil {
		t.Fatal(err)
	}
	var encoded strings.Builder
	for i := range v1 {
		v1[i].Schema = ObservationSchemaV2
		v1[i].Outcome = &CandidateOutcome{Kind: OutcomeCompleted}
		for j := range v1[i].Measurements {
			if v1[i].Measurements[j].Source == SourceHarnessMeasured {
				v1[i].Measurements[j].ID = EvaluatorPeakRSSMetricID
				v1[i].Measurements[j].Unit = "bytes"
			}
		}
		line, err := EncodeObservation(v1[i])
		if err != nil {
			t.Fatal(err)
		}
		encoded.Write(line)
	}
	return v1, encoded.String()
}

func executionReceiptForState(t *testing.T, def Definition, run string) ExecutionReceipt {
	t.Helper()
	digest, err := DefinitionDigest(def)
	if err != nil {
		t.Fatal(err)
	}
	candidates := make([]ReceiptCandidate, 0, len(def.Candidates))
	for _, candidate := range def.Candidates {
		workspaceID, err := WorkspaceRunID(digest, run, candidate.ID)
		if err != nil {
			t.Fatal(err)
		}
		candidates = append(candidates, ReceiptCandidate{
			ID: candidate.ID, BaseCommit: candidate.Base, PatchDigest: candidate.Digest, WorkspaceRunID: workspaceID,
			Materialization: WorkspaceIdentity{Shape: WorkspaceBasePlusPatch, RunID: workspaceID, CommitSHA: candidate.Base, PatchSHA256: strings.TrimPrefix(candidate.Digest, "sha256:")},
		})
	}
	inputDigests := map[string]string{
		"contracts/equivalence.json":         strings.TrimPrefix(def.Contract.Digest, "sha256:"),
		"evaluator:" + def.Evaluator.Argv[0]: strings.TrimPrefix(def.Evaluator.Digest, "sha256:"),
		"inputs/workload.json":               strings.TrimPrefix(def.Workload.Digest, "sha256:"),
	}
	for _, fixture := range def.Fixtures {
		inputDigests["fixtures/"+fixture.ID+".json"] = strings.TrimPrefix(fixture.Digest, "sha256:")
	}
	fixtureInputs := make([]ResolvedArtifact, len(def.Fixtures))
	for index, fixture := range def.Fixtures {
		fixtureInputs[index] = ResolvedArtifact{ID: fixture.ID, Path: "fixtures/" + fixture.ID + ".json", Digest: fixture.Digest}
	}
	return ExecutionReceipt{
		Schema: ExecutionReceiptSchema, ExperimentDigest: digest, Run: run, EnvironmentPolicy: def.Execution.EnvironmentPolicy,
		AuthorityDigest: digestOf("1"), CapabilitiesDigest: def.Evaluator.CapabilitiesDigest, ScheduleDigest: digestOf("2"), GrantsDigest: digestOf("3"),
		Fingerprint: ExecutionFingerprint{OS: "linux", Arch: "amd64", ToolVersions: map[string]string{"evaluator": "2.1.0", "verdi": "0.1.0"}, Env: map[string]*string{}, InputDigests: inputDigests},
		Inputs: ReceiptInputs{
			Workload: ResolvedArtifact{ID: def.Workload.ID, Path: "inputs/workload.json", Digest: def.Workload.Digest},
			Fixtures: fixtureInputs,
			Contract: ResolvedArtifact{ID: def.Contract.ID, Path: "contracts/equivalence.json", Digest: def.Contract.Digest},
		},
		Enforcement: []ReceiptEnforcement{{Kind: "process-execution", Applied: true, Reason: "allowlist applied"}, {Kind: "timeouts", Applied: true, Reason: "deadline applied"}},
		Network:     ReceiptNetwork{Mode: NetworkDeny, Configured: true, Reason: "network namespace configured"}, Candidates: candidates,
		Versions:    ReceiptVersions{Verdi: "0.1.0", RecommendationEngine: string(AlgorithmV1)},
		Disclosures: []ReceiptDisclosure{DisclosureCPUAllocationUnproven, DisclosureMemoryAllocationUnproven},
	}
}

func TestDeriveStateV2RequiresDigestPinnedCapabilitiesAuthority(t *testing.T) {
	allGuards := []string{"behavioral-equivalence", "tenant-isolation"}
	_, validDigest := capabilitiesAuthorityForState(t, allGuards, "fixture-evaluator/2.1.0")
	mismatchedCapabilities, _ := capabilitiesAuthorityForState(t, allGuards, "fixture-evaluator/2.2.0")
	undeclaredCapabilities, undeclaredDigest := capabilitiesAuthorityForState(t, []string{"behavioral-equivalence"}, "fixture-evaluator/2.1.0")

	tests := []struct {
		name              string
		definitionDigest  string
		capabilities      []byte
		writeCapabilities bool
	}{
		{name: "missing", definitionDigest: validDigest},
		{name: "digest mismatch", definitionDigest: validDigest, capabilities: mismatchedCapabilities, writeCapabilities: true},
		{name: "undeclared required guard", definitionDigest: undeclaredDigest, capabilities: undeclaredCapabilities, writeCapabilities: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			doc, digest := lockedV2DefinitionDoc(t, tt.definitionDigest)
			writeExperimentFile(t, dir, definitionFile, doc)
			writeCandidatePatches(t, dir)
			if tt.writeCapabilities {
				writeExperimentFile(t, dir, "evaluator-capabilities.json", string(tt.capabilities))
			}
			_, encoded := completeObservationsV2JSONL(t, digest, "run-1")
			writeRunFile(t, dir, "run-1", observationsFile, encoded)

			if derived, err := DeriveStateDetails(dir, testExperimentDir, acceptResult); err == nil {
				t.Fatalf("DeriveStateDetails() = %+v, nil error; V2 evidence without exact declared capability authority must not become state-bearing", derived)
			}
		})
	}
}

func TestDeriveStateV2RequiresAndPassesReceiptWithoutV1Disclosure(t *testing.T) {
	dir := t.TempDir()
	capabilities, capabilitiesDigest := capabilitiesAuthorityForState(t, []string{"behavioral-equivalence", "tenant-isolation"}, "fixture-evaluator/2.1.0")
	doc, digest := lockedV2DefinitionDoc(t, capabilitiesDigest)
	def := mustDecodeDefinition(t, doc)
	writeExperimentFile(t, dir, definitionFile, doc)
	writeExperimentFile(t, dir, capabilitiesFile, string(capabilities))
	writeCandidatePatches(t, dir)
	obs, encodedObs := completeObservationsV2JSONL(t, digest, "run-1")
	writeRunFile(t, dir, "run-1", observationsFile, encodedObs)

	legacy, err := DecodeResult([]byte(validResultJSONForDigest(digest, VerdictProvenWinner)))
	if err != nil {
		t.Fatal(err)
	}
	decision, err := DecisionFromResult(legacy, obs)
	if err != nil {
		t.Fatal(err)
	}
	receipt := executionReceiptForState(t, def, "run-1")
	receiptDigest, err := ExecutionReceiptDigest(receipt)
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewResultV2(decision, ResultExecution{
		ExecutionDigest:   receiptDigest,
		Isolation:         ResultIsolation{Network: receipt.Network, Disclosures: []IsolationDisclosure{}},
		WarmupDiagnostics: WarmupDiagnostics{Authority: WarmupAuthorityNonDecisionDiagnostic, Scope: WarmupScopeFinalInvocation, Failures: []WarmupFailure{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	resultBytes, err := EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	writeRunFile(t, dir, "run-1", resultFile, string(resultBytes))
	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Fatalf("DeriveState(v2 without receipt) = nil error")
	}
	receiptBytes, err := EncodeExecutionReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	writeRunFile(t, dir, "run-1", executionFile, string(receiptBytes))
	capabilitiesPath := filepath.Join(dir, testExperimentDir, capabilitiesFile)
	if err := os.Remove(capabilitiesPath); err != nil {
		t.Fatal(err)
	}
	if state, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Fatalf("DeriveState(v2 result without capability authority) = %q, nil error", state)
	}
	writeExperimentFile(t, dir, capabilitiesFile, string(capabilities))
	receivedReceipt := false
	verify := func(_ Definition, _ []Observation, got *ExecutionReceipt, _ Result) error {
		receivedReceipt = got != nil
		return nil
	}
	state, disclosures, err := DeriveState(dir, testExperimentDir, verify)
	if err != nil {
		t.Fatalf("DeriveState(v2): %v", err)
	}
	if state != StateRecommended || !receivedReceipt {
		t.Fatalf("state/receipt = %q/%v", state, receivedReceipt)
	}
	if len(disclosures) != 1 || disclosures[0].Code != DisclosureRegistrationLockWitness {
		t.Fatalf("V2 disclosures = %+v", disclosures)
	}
}
