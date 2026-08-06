package experiment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	writeFile(t, dir, "candidates/baseline.patch", baselinePatchContent)
	writeFile(t, dir, "candidates/facts-cache.patch", factsCachePatchContent)
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

func TestDeriveStateExploratoryNoDefinition(t *testing.T) {
	dir := t.TempDir()
	state, err := DeriveState(dir)
	if err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if state != StateExploratory {
		t.Errorf("DeriveState() = %q, want %q", state, StateExploratory)
	}
}

func TestDeriveStateExploratoryUnlockedDefinition(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "experiment.yaml", validDefinitionYAML())
	state, err := DeriveState(dir)
	if err != nil {
		t.Fatalf("DeriveState() unexpected error: %v", err)
	}
	if state != StateExploratory {
		t.Errorf("DeriveState() = %q, want %q", state, StateExploratory)
	}
}

func TestDeriveStateUndecodableDefinitionIsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "experiment.yaml", "not: valid: yaml: at all:\n")
	if _, err := DeriveState(dir); err == nil {
		t.Errorf("DeriveState() with corrupt experiment.yaml = nil error, want error")
	}
}

func TestDeriveStateTamperedLockIsError(t *testing.T) {
	dir := t.TempDir()
	doc := validDefinitionYAML() + "lock:\n  definition_digest: " + digestOf("9") + "\n"
	writeFile(t, dir, "experiment.yaml", doc)
	if _, err := DeriveState(dir); err == nil {
		t.Errorf("DeriveState() with a tampered lock = nil error, want error")
	}
}

func TestDeriveStateRegistered(t *testing.T) {
	dir := t.TempDir()
	doc, _ := lockedDefinitionDoc(t)
	writeFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)

	state, err := DeriveState(dir)
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
	writeFile(t, dir, "experiment.yaml", doc)
	writeFile(t, dir, "candidates/baseline.patch", baselinePatchContent)
	// facts-cache.patch deliberately absent: a locked definition's
	// registered candidate patches must exist; absence here is a store
	// inconsistency, not a legitimate lower rung (see DeriveState's own
	// doc comment).

	if _, err := DeriveState(dir); err == nil {
		t.Errorf("DeriveState() with a missing registered candidate patch = nil error, want error")
	}
}

func TestDeriveStateMeasured(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))

	state, err := DeriveState(dir)
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
	writeFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digestOf("0")))

	if _, err := DeriveState(dir); err == nil {
		t.Errorf("DeriveState() with a mismatched observation experiment_digest = nil error, want error")
	}
}

func TestDeriveStateMeasuredUndecodableObservationsIsError(t *testing.T) {
	dir := t.TempDir()
	doc, _ := lockedDefinitionDoc(t)
	writeFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeFile(t, dir, "observations.jsonl", "not json\n")

	if _, err := DeriveState(dir); err == nil {
		t.Errorf("DeriveState() with undecodable observations.jsonl = nil error, want error")
	}
}

func TestDeriveStateRecommended(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictProvenWinner))

	state, err := DeriveState(dir)
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
	writeFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictDisclosedUnproven))

	state, err := DeriveState(dir)
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
	writeFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeFile(t, dir, "result.json", validResultJSONForDigest(digestOf("0"), VerdictProvenWinner))

	if _, err := DeriveState(dir); err == nil {
		t.Errorf("DeriveState() with result.definition_digest mismatched = nil error, want error")
	}
}

func TestDeriveStateRatified(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	resultDoc := validResultJSONForDigest(digest, VerdictProvenWinner)
	writeFile(t, dir, "result.json", resultDoc)

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
	writeFile(t, dir, "ratification.yaml", ratificationDoc)

	state, err := DeriveState(dir)
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
	writeFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictProvenWinner))

	ratificationDoc := "schema: verdi.experiment-ratification/v1\n" +
		"result_digest: " + digestOf("0") + "\n" +
		"actor: " + validActor + "\n" +
		"disposition: select-recommended\n"
	writeFile(t, dir, "ratification.yaml", ratificationDoc)

	if _, err := DeriveState(dir); err == nil {
		t.Errorf("DeriveState() with ratification.result_digest mismatched = nil error, want error")
	}
}

func TestDeriveStateUndecodableRatificationIsError(t *testing.T) {
	dir := t.TempDir()
	doc, digest := lockedDefinitionDoc(t)
	writeFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	writeFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
	writeFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictProvenWinner))
	writeFile(t, dir, "ratification.yaml", "not: valid: yaml: at all:\n")

	if _, err := DeriveState(dir); err == nil {
		t.Errorf("DeriveState() with undecodable ratification.yaml = nil error, want error")
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
	writeFile(t, dir, "experiment.yaml", doc)
	writeFile(t, dir, "candidates/baseline.patch", "diff --git a/tampered b/tampered\n")
	writeFile(t, dir, "candidates/facts-cache.patch", factsCachePatchContent)

	if _, err := DeriveState(dir); err == nil {
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
	writeFile(t, dir, "experiment.yaml", doc+"lock:\n  definition_digest: "+digest+"\n")
	writeFile(t, dir, "candidates/baseline.patch", badPatch)
	writeFile(t, dir, "candidates/facts-cache.patch", factsCachePatchContent)

	if _, err := DeriveState(dir); err == nil {
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
	writeFile(t, dir, "experiment.yaml", doc)
	writeCandidatePatches(t, dir)
	lines := validObservationLines(digest)
	writeFile(t, dir, "observations.jsonl", strings.Join(lines[:3], "\n")+"\n") // drop facts-cache round 2

	if _, err := DeriveState(dir); err == nil {
		t.Errorf("DeriveState() with an incomplete observation set = nil error, want error")
	}
}
