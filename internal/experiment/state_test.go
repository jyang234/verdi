package experiment

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// acceptResult and rejectResult are the stub ResultVerifier ports this
// package's own tests inject (SI-42). The real recompute-equality
// verifier lives in internal/experimentdecision — which imports this
// package, so it can never be wired from here; the injected-port shape is
// exactly what keeps that import direction one-way. The integration test
// wiring the REAL verifier over committed fixtures lives on the
// experimentdecision side.
func acceptResult(Definition, []Observation, Result) error { return nil }

func rejectResult(Definition, []Observation, Result) error {
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
	writeFile(t, root, filepath.Join(testExperimentDir, relPath), content)
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
// REQUIRED input, not an optional hardening step (SI-42): without it a
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
// never a silent downgrade to the measured rung (SI-42): the artifact IS
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
	verify := func(def Definition, obs []Observation, res Result) error {
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

// TestDeriveStateIncompleteObservationsIsError proves that an
// observations.jsonl file which is present, decodes, and digest-links
// correctly but does not cover every registered (candidate, round) pair
// is an error, not a silent report of "registered".
func TestDeriveStateIncompleteObservationsIsError(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeExperimentFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	lines := validObservationLines(digest)
	writeExperimentFile(t, dir, "observations.jsonl", strings.Join(lines[:3], "\n")+"\n") // drop facts-cache round 2

	if _, _, err := DeriveState(dir, testExperimentDir, acceptResult); err == nil {
		t.Errorf("DeriveState() with an incomplete observation set = nil error, want error")
	}
}
