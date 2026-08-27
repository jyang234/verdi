package experiment

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// capsuleBindingFixture is one complete, internally consistent binding
// context: a definition whose workload/contract/fixture/capabilities
// digests are the REAL digests of the returned artifact bytes, a proven
// v2 result, and a v2 ratification bound to that result.
type capsuleBindingFixture struct {
	def          Definition
	defDigest    string
	result       Result
	resultDigest string
	ratification Ratification
	artifacts    []CapsuleRetainedArtifact
}

func capsuleBindingBytes() map[string][]byte {
	return map[string][]byte{
		CapsuleArtifactDefinition:            []byte("definition-bytes\n"),
		CapsuleArtifactCandidatePatch:        []byte(factsCachePatchContent),
		CapsuleArtifactEvaluatorCapabilities: []byte("capabilities-bytes\n"),
		CapsuleArtifactContract:              []byte("contract-bytes\n"),
		CapsuleArtifactWorkload:              []byte("workload-bytes\n"),
		"fixture-request-log":                []byte("fixture-bytes\n"),
		CapsuleArtifactExecutionReceipt:      []byte("receipt-bytes\n"),
		CapsuleArtifactObservations:          []byte("observations-bytes\n"),
		CapsuleArtifactRatification:          []byte("ratification-bytes\n"),
		CapsuleArtifactRecommendation:        []byte("recommendation-bytes\n"),
	}
}

func buildCapsuleBindingFixture(t *testing.T, disposition Disposition, candidate string) capsuleBindingFixture {
	t.Helper()
	bytesByID := capsuleBindingBytes()
	doc := validDefinitionYAML()
	doc = strings.Replace(doc, "schema: verdi.experiment/v1\n",
		"schema: verdi.experiment/v2\nclass: cache-placement-performance\n", 1)
	doc = strings.Replace(doc, "workload:\n  id: representative-request-mix\n  digest: "+digestOf("5"),
		"workload:\n  id: representative-request-mix\n  digest: "+sha256Digest(bytesByID[CapsuleArtifactWorkload]), 1)
	doc = strings.Replace(doc, "fixtures:\n  - id: request-log\n    digest: "+digestOf("6"),
		"fixtures:\n  - id: request-log\n    digest: "+sha256Digest(bytesByID["fixture-request-log"]), 1)
	doc = strings.Replace(doc, "contract:\n  id: behavioral-equivalence-contract\n  digest: "+digestOf("7"),
		"contract:\n  id: behavioral-equivalence-contract\n  digest: "+sha256Digest(bytesByID[CapsuleArtifactContract]), 1)
	doc = strings.Replace(doc, "capabilities_digest: "+digestOf("4"),
		"capabilities_digest: "+sha256Digest(bytesByID[CapsuleArtifactEvaluatorCapabilities]), 1)
	unlocked := mustDecodeDefinition(t, doc)
	defDigest, err := DefinitionDigest(unlocked)
	if err != nil {
		t.Fatal(err)
	}
	doc = doc + "lock:\n  definition_digest: " + defDigest + "\n"
	def := mustDecodeDefinition(t, doc)
	if locked, err := Locked(def); err != nil || !locked {
		t.Fatalf("binding fixture definition is not locked: %v/%v", locked, err)
	}
	// The retained definition artifact is the exact locked accepted bytes.
	bytesByID[CapsuleArtifactDefinition] = []byte(doc)

	decision := ResultDecision{
		Experiment: def.ID, DefinitionDigest: defDigest, Run: "run-1",
		Algorithm: def.Algorithm, Verdict: VerdictProvenWinner, Winner: "facts-cache",
		Candidates: []DecisionCandidate{
			{ID: "baseline", Baseline: true, Eligible: true, ExecutionFailures: []CandidateExecutionFailure{}},
			{ID: "facts-cache", Eligible: true, ExecutionFailures: []CandidateExecutionFailure{}},
		},
		ObservationsDigest: sha256Digest([]byte("observations")),
	}
	result, err := NewResultV2(decision, ResultExecution{
		ExecutionDigest: sha256Digest([]byte("receipt")),
		Isolation: ResultIsolation{
			Network:     ReceiptNetwork{Mode: NetworkDeny, Configured: true, Reason: "test default deny"},
			Disclosures: []IsolationDisclosure{},
		},
		WarmupDiagnostics: WarmupDiagnostics{
			Authority: WarmupAuthorityNonDecisionDiagnostic,
			Scope:     WarmupScopeFinalInvocation, Failures: []WarmupFailure{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resultBytes, err := EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	resultDigest, err := ResultDigest(result)
	if err != nil {
		t.Fatal(err)
	}
	bytesByID[CapsuleArtifactResult] = resultBytes

	ratification := Ratification{
		Schema: RatificationSchemaV2, ResultDigest: resultDigest,
		ActorV2:     &RatificationActor{TrustSource: "github", Subject: "user-123", PrincipalID: validActor},
		Disposition: disposition,
	}
	if candidate != "" {
		ratification.Candidate = candidate
		ratification.Reason = "explicit selection for the binding fixture"
	}
	if ratification.Disposition == DispositionSelectRecommended || ratification.Disposition == DispositionSelectOther {
		ratificationBytes, err := EncodeRatification(ratification)
		if err != nil {
			t.Fatal(err)
		}
		bytesByID[CapsuleArtifactRatification] = ratificationBytes
	}

	ids := make([]string, 0, len(bytesByID))
	for id := range bytesByID {
		ids = append(ids, id)
	}
	artifacts := make([]CapsuleRetainedArtifact, 0, len(ids))
	for _, id := range ids {
		artifacts = append(artifacts, CapsuleRetainedArtifact{ID: id, Bytes: bytesByID[id]})
	}
	return capsuleBindingFixture{
		def: def, defDigest: defDigest, result: result, resultDigest: resultDigest,
		ratification: ratification, artifacts: artifacts,
	}
}

func (f capsuleBindingFixture) input(cap int64) CapsuleBindingInput {
	artifacts := make([]CapsuleRetainedArtifact, len(f.artifacts))
	copy(artifacts, f.artifacts)
	return CapsuleBindingInput{
		Definition: f.def, DefinitionDigest: f.defDigest,
		Ratification: f.ratification, Result: f.result,
		Artifacts: artifacts, RetainedArtifactBytes: cap,
	}
}

func (f capsuleBindingFixture) withoutArtifact(id string) capsuleBindingFixture {
	kept := make([]CapsuleRetainedArtifact, 0, len(f.artifacts))
	for _, artifact := range f.artifacts {
		if artifact.ID != id {
			kept = append(kept, artifact)
		}
	}
	f.artifacts = kept
	return f
}

func TestCapsuleSelectedCandidateDerivation(t *testing.T) {
	recommended := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
	selected, err := SelectedCapsuleCandidate(recommended.def, recommended.result, recommended.ratification)
	if err != nil || selected != "facts-cache" {
		t.Fatalf("SelectedCapsuleCandidate(select-recommended) = %q/%v, want the bound result's winner", selected, err)
	}

	other := buildCapsuleBindingFixture(t, DispositionSelectOther, "baseline")
	selected, err = SelectedCapsuleCandidate(other.def, other.result, other.ratification)
	if err != nil || selected != "baseline" {
		t.Fatalf("SelectedCapsuleCandidate(select-other) = %q/%v, want the explicit registered candidate", selected, err)
	}

	for _, disposition := range []Disposition{DispositionRejectAll, DispositionMisframed, DispositionRequestNewRevision} {
		fixture := buildCapsuleBindingFixture(t, disposition, "")
		if _, err := SelectedCapsuleCandidate(fixture.def, fixture.result, fixture.ratification); err == nil {
			t.Errorf("SelectedCapsuleCandidate(%s) = nil error, want non-selecting refusal", disposition)
		}
	}
}

func TestCapsuleFixtureArtifactID(t *testing.T) {
	id, err := CapsuleFixtureArtifactID("request-log")
	if err != nil || id != "fixture-request-log" {
		t.Fatalf("CapsuleFixtureArtifactID(request-log) = %q/%v, want fixture-request-log", id, err)
	}
	if _, err := CapsuleFixtureArtifactID("Bad:ID"); err == nil {
		t.Fatalf("CapsuleFixtureArtifactID(Bad:ID) = nil error, want id-grammar refusal")
	}
}

func TestBindCapsuleManifestClosedInventory(t *testing.T) {
	fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
	manifest, err := BindCapsuleManifest(fixture.input(1 << 20))
	if err != nil {
		t.Fatalf("BindCapsuleManifest() error: %v", err)
	}
	if manifest.Schema != CapsuleManifestSchema || manifest.Experiment != fixture.def.ID ||
		manifest.DefinitionDigest != fixture.defDigest || manifest.ResultDigest != fixture.resultDigest ||
		manifest.Selected != "facts-cache" {
		t.Fatalf("manifest header = %+v", manifest)
	}
	wantIDs := []string{
		CapsuleArtifactCandidatePatch, CapsuleArtifactContract, CapsuleArtifactDefinition,
		CapsuleArtifactEvaluatorCapabilities, CapsuleArtifactExecutionReceipt,
		"fixture-request-log", CapsuleArtifactObservations, CapsuleArtifactRatification,
		CapsuleArtifactRecommendation, CapsuleArtifactResult, CapsuleArtifactWorkload,
	}
	if len(manifest.Artifacts) != len(wantIDs) {
		t.Fatalf("manifest has %d artifacts, want %d: %+v", len(manifest.Artifacts), len(wantIDs), manifest.Artifacts)
	}
	byID := map[string]string{}
	for i, artifact := range manifest.Artifacts {
		byID[artifact.ID] = artifact.Digest
		if i > 0 && manifest.Artifacts[i-1].ID >= artifact.ID {
			t.Fatalf("manifest artifacts are not in deterministic sorted order: %+v", manifest.Artifacts)
		}
	}
	for _, id := range wantIDs {
		if byID[id] == "" {
			t.Errorf("manifest omits closed inventory member %q", id)
		}
	}
	// Digests are recomputed from exact raw bytes.
	if byID[CapsuleArtifactWorkload] != fixture.def.Workload.Digest {
		t.Errorf("workload digest %q does not match the locked reference", byID[CapsuleArtifactWorkload])
	}
	if byID[CapsuleArtifactResult] != fixture.resultDigest {
		t.Errorf("result digest %q does not match the ratification-bound result", byID[CapsuleArtifactResult])
	}

	// Encoding is deterministic and round-trips through the strict decoder.
	first, err := EncodeCapsuleManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncodeCapsuleManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("EncodeCapsuleManifest is not deterministic")
	}
	if _, err := DecodeCapsuleManifest(first); err != nil {
		t.Fatalf("encoded manifest does not strict-decode: %v", err)
	}

	// The recommendation is optional: absent, the manifest simply omits it.
	withoutRecommendation, err := BindCapsuleManifest(fixture.withoutArtifact(CapsuleArtifactRecommendation).input(1 << 20))
	if err != nil {
		t.Fatalf("BindCapsuleManifest(no recommendation) error: %v", err)
	}
	for _, artifact := range withoutRecommendation.Artifacts {
		if artifact.ID == CapsuleArtifactRecommendation {
			t.Fatalf("manifest names an absent recommendation")
		}
	}
}

func TestBindCapsuleManifestRejectsBrokenInventories(t *testing.T) {
	base := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")

	t.Run("missing required member", func(t *testing.T) {
		if _, err := BindCapsuleManifest(base.withoutArtifact(CapsuleArtifactWorkload).input(1 << 20)); err == nil {
			t.Fatalf("missing workload accepted")
		}
	})
	t.Run("missing fixture", func(t *testing.T) {
		if _, err := BindCapsuleManifest(base.withoutArtifact("fixture-request-log").input(1 << 20)); err == nil {
			t.Fatalf("missing registered fixture accepted")
		}
	})
	t.Run("extra member", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
		fixture.artifacts = append(fixture.artifacts, CapsuleRetainedArtifact{ID: "extra-thing", Bytes: []byte("x")})
		if _, err := BindCapsuleManifest(fixture.input(1 << 20)); err == nil {
			t.Fatalf("extra inventory member accepted")
		}
	})
	t.Run("duplicate member", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
		fixture.artifacts = append(fixture.artifacts, CapsuleRetainedArtifact{ID: CapsuleArtifactWorkload, Bytes: []byte("workload-bytes\n")})
		if _, err := BindCapsuleManifest(fixture.input(1 << 20)); err == nil {
			t.Fatalf("duplicate inventory member accepted")
		}
	})
	t.Run("digest mismatch", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
		for i := range fixture.artifacts {
			if fixture.artifacts[i].ID == CapsuleArtifactContract {
				fixture.artifacts[i].Bytes = []byte("tampered contract\n")
			}
		}
		if _, err := BindCapsuleManifest(fixture.input(1 << 20)); err == nil {
			t.Fatalf("digest-mismatched contract accepted")
		}
	})
	t.Run("result bytes must match the ratification binding", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
		for i := range fixture.artifacts {
			if fixture.artifacts[i].ID == CapsuleArtifactResult {
				fixture.artifacts[i].Bytes = []byte("not the bound result\n")
			}
		}
		if _, err := BindCapsuleManifest(fixture.input(1 << 20)); err == nil {
			t.Fatalf("result bytes diverging from the ratified digest accepted")
		}
	})
}

func TestBindCapsuleManifestRetainedArtifactCeiling(t *testing.T) {
	fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
	largest := int64(0)
	largestID := ""
	for _, artifact := range fixture.artifacts {
		if int64(len(artifact.Bytes)) > largest {
			largest = int64(len(artifact.Bytes))
			largestID = artifact.ID
		}
	}

	if _, err := BindCapsuleManifest(fixture.input(largest)); err != nil {
		t.Fatalf("cap equal to the largest artifact must pass: %v", err)
	}

	_, err := BindCapsuleManifest(fixture.input(largest - 1))
	if err == nil {
		t.Fatalf("one-byte-under cap accepted")
	}
	if !IsCapsuleArtifactOversized(err) {
		t.Fatalf("cap violation error %v is not the typed oversized refusal", err)
	}
	if !strings.Contains(err.Error(), largestID) || !strings.Contains(err.Error(), "bytes") {
		t.Fatalf("oversized witness %v does not name the exact artifact and observed size", err)
	}

	// The ceiling applies independently to every member, including each
	// fixture: shrink the cap below the fixture's size and the fixture is
	// the named witness.
	fixtureSize := int64(len([]byte("fixture-bytes\n")))
	_, err = BindCapsuleManifest(fixture.input(fixtureSize - 1))
	if err == nil || !IsCapsuleArtifactOversized(err) {
		t.Fatalf("fixture over cap accepted or untyped: %v", err)
	}

}

// TestBindCapsuleManifestExcludesManifestFromCeiling constructs a
// many-fixture inventory whose encoded manifest is strictly larger than
// every retained member, then binds with the cap set to the largest
// member: success proves the manifest bytes are never measured against
// the retained-artifact ceiling.
func TestBindCapsuleManifestExcludesManifestFromCeiling(t *testing.T) {
	fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
	lockedDoc := ""
	for _, artifact := range fixture.artifacts {
		if artifact.ID == CapsuleArtifactDefinition {
			lockedDoc = string(artifact.Bytes)
		}
	}
	unlockedDoc := strings.Split(lockedDoc, "lock:\n")[0]
	extraFixtures := ""
	extraArtifacts := []CapsuleRetainedArtifact{}
	for i := 0; i < 150; i++ {
		id := fmt.Sprintf("fx-%03d", i)
		content := []byte("fixture-" + id + "\n")
		extraFixtures += "  - id: " + id + "\n    digest: " + sha256Digest(content) + "\n"
		extraArtifacts = append(extraArtifacts, CapsuleRetainedArtifact{ID: "fixture-" + id, Bytes: content})
	}
	unlockedDoc = strings.Replace(unlockedDoc, "fixtures:\n", "fixtures:\n"+extraFixtures, 1)
	unlocked := mustDecodeDefinition(t, unlockedDoc)
	digest, err := DefinitionDigest(unlocked)
	if err != nil {
		t.Fatal(err)
	}
	relocked := unlockedDoc + "lock:\n  definition_digest: " + digest + "\n"

	// Rebuild the dependent evidence chain over the widened definition.
	def := mustDecodeDefinition(t, relocked)
	decision := *fixture.result.Decision
	decision.DefinitionDigest = digest
	result, err := NewResultV2(decision, *fixture.result.Execution)
	if err != nil {
		t.Fatal(err)
	}
	resultBytes, err := EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	resultDigest, err := ResultDigest(result)
	if err != nil {
		t.Fatal(err)
	}
	ratification := fixture.ratification
	ratification.ResultDigest = resultDigest
	ratificationBytes, err := EncodeRatification(ratification)
	if err != nil {
		t.Fatal(err)
	}

	artifacts := []CapsuleRetainedArtifact{}
	for _, artifact := range fixture.artifacts {
		switch artifact.ID {
		case CapsuleArtifactDefinition:
			artifact.Bytes = []byte(relocked)
		case CapsuleArtifactResult:
			artifact.Bytes = resultBytes
		case CapsuleArtifactRatification:
			artifact.Bytes = ratificationBytes
		}
		artifacts = append(artifacts, artifact)
	}
	artifacts = append(artifacts, extraArtifacts...)

	largest := int64(0)
	for _, artifact := range artifacts {
		if int64(len(artifact.Bytes)) > largest {
			largest = int64(len(artifact.Bytes))
		}
	}
	input := CapsuleBindingInput{
		Definition: def, DefinitionDigest: digest,
		Ratification: ratification, Result: result,
		Artifacts: artifacts, RetainedArtifactBytes: largest,
	}
	manifest, err := BindCapsuleManifest(input)
	if err != nil {
		t.Fatalf("BindCapsuleManifest(cap = largest member) error: %v", err)
	}
	encoded, err := EncodeCapsuleManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(encoded)) <= largest {
		t.Fatalf("fixture cannot witness the manifest exclusion: manifest %d bytes, cap %d", len(encoded), largest)
	}
}

func TestBindCapsuleManifestReproductionIsStatusOnly(t *testing.T) {
	// Reproduction posture never selects: an identical fixture with a
	// reproduction rule present in the definition derives the same
	// selected candidate from the same disposition rules.
	fixture := buildCapsuleBindingFixture(t, DispositionSelectOther, "baseline")
	withRule := fixture
	reproduction := &ReproductionRule{MinimumValidRuns: 2}
	withRule.def.Reproduction = reproduction
	selected, err := SelectedCapsuleCandidate(withRule.def, withRule.result, withRule.ratification)
	if err != nil || selected != "baseline" {
		t.Fatalf("reproduction rule changed selection: %q/%v", selected, err)
	}
}

func TestBindCapsuleManifestNonSelectingRefused(t *testing.T) {
	fixture := buildCapsuleBindingFixture(t, DispositionRejectAll, "")
	if _, err := BindCapsuleManifest(fixture.input(1 << 20)); err == nil {
		t.Fatalf("non-selecting disposition bound a capsule")
	}
}

// mustJSON is a tiny sanity helper keeping the deterministic-order check
// honest against raw encoding rather than struct order alone.
func TestEncodeCapsuleManifestCanonicalBytes(t *testing.T) {
	fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
	manifest, err := BindCapsuleManifest(fixture.input(1 << 20))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeCapsuleManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("encoded manifest is not JSON: %v", err)
	}
	if _, ok := decoded["artifacts"]; !ok {
		t.Fatalf("encoded manifest omits artifacts: %s", encoded)
	}
}

// TestBindCapsuleManifestDerivesAuthorityFromRetainedBytes is the
// correction matrix for the trusted-caller finding: every authority-
// bearing definition/result/ratification fact must be derived or verified
// from the exact retained bytes, never accepted from the caller's typed
// projections alone.
func TestCapsuleBindingDerivesAuthorityFromRetainedBytes(t *testing.T) {
	setArtifact := func(f capsuleBindingFixture, id string, data []byte) capsuleBindingFixture {
		artifacts := make([]CapsuleRetainedArtifact, len(f.artifacts))
		copy(artifacts, f.artifacts)
		for i := range artifacts {
			if artifacts[i].ID == id {
				artifacts[i].Bytes = data
			}
		}
		f.artifacts = artifacts
		return f
	}

	t.Run("arbitrary valid but incorrect DefinitionDigest is refused", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
		input := fixture.input(1 << 20)
		input.DefinitionDigest = digestOf("9")
		if _, err := BindCapsuleManifest(input); err == nil {
			t.Fatalf("caller-supplied wrong definition digest accepted")
		}
	})

	t.Run("unlocked definition bytes are refused", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
		lockedDoc := ""
		for _, artifact := range fixture.artifacts {
			if artifact.ID == CapsuleArtifactDefinition {
				lockedDoc = string(artifact.Bytes)
			}
		}
		unlockedDoc := strings.Split(lockedDoc, "lock:\n")[0]
		unlocked := setArtifact(fixture, CapsuleArtifactDefinition, []byte(unlockedDoc))
		unlockedDef := mustDecodeDefinition(t, unlockedDoc)
		input := unlocked.input(1 << 20)
		input.Definition = unlockedDef
		if _, err := BindCapsuleManifest(input); err == nil {
			t.Fatalf("unlocked definition bytes accepted as capsule authority")
		}
	})

	t.Run("non-v2 definition bytes are refused", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
		v1 := mustDecodeDefinition(t, validDefinitionYAML())
		v1Digest, err := DefinitionDigest(v1)
		if err != nil {
			t.Fatal(err)
		}
		v1Doc := validDefinitionYAML() + "lock:\n  definition_digest: " + v1Digest + "\n"
		v1Locked := mustDecodeDefinition(t, v1Doc)
		swapped := setArtifact(fixture, CapsuleArtifactDefinition, []byte(v1Doc))
		input := swapped.input(1 << 20)
		input.Definition = v1Locked
		input.DefinitionDigest = v1Digest
		if _, err := BindCapsuleManifest(input); err == nil {
			t.Fatalf("locked v1 definition accepted as fresh capsule authority")
		}
	})

	t.Run("typed definition diverging from retained bytes is refused", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
		input := fixture.input(1 << 20)
		input.Definition.ID = "some-other-experiment"
		if _, err := BindCapsuleManifest(input); err == nil {
			t.Fatalf("typed definition projection diverging from retained bytes accepted")
		}
	})

	t.Run("typed result winner diverging from retained result bytes is refused", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
		input := fixture.input(1 << 20)
		tamperedDecision := *input.Result.Decision
		tamperedDecision.Winner = "baseline"
		tamperedDecision.Candidates = append([]DecisionCandidate(nil), input.Result.Decision.Candidates...)
		input.Result.Decision = &tamperedDecision
		manifest, err := BindCapsuleManifest(input)
		if err == nil && manifest.Selected != "facts-cache" {
			t.Fatalf("caller-modified result selected %q instead of the retained result's winner", manifest.Selected)
		}
		if err == nil {
			t.Fatalf("typed result diverging from retained bytes accepted")
		}
	})

	t.Run("typed ratification diverging from retained bytes is refused", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectOther, "baseline")
		input := fixture.input(1 << 20)
		input.Ratification.Candidate = "facts-cache"
		input.Ratification.Reason = "tampered selection"
		if _, err := BindCapsuleManifest(input); err == nil {
			t.Fatalf("typed ratification diverging from retained bytes accepted")
		}
	})

	t.Run("retained ratification bound to a different result is refused", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
		foreign := Ratification{
			Schema: RatificationSchemaV2, ResultDigest: digestOf("8"),
			ActorV2:     &RatificationActor{TrustSource: "github", Subject: "user-123", PrincipalID: validActor},
			Disposition: DispositionSelectRecommended,
		}
		foreignBytes, err := EncodeRatification(foreign)
		if err != nil {
			t.Fatal(err)
		}
		swapped := setArtifact(fixture, CapsuleArtifactRatification, foreignBytes)
		input := swapped.input(1 << 20)
		input.Ratification = foreign
		if _, err := BindCapsuleManifest(input); err == nil {
			t.Fatalf("retained ratification bound to a foreign result accepted")
		}
	})

	t.Run("malformed retained definition bytes are refused", func(t *testing.T) {
		fixture := buildCapsuleBindingFixture(t, DispositionSelectRecommended, "")
		broken := setArtifact(fixture, CapsuleArtifactDefinition, []byte("not: [valid\n"))
		if _, err := BindCapsuleManifest(broken.input(1 << 20)); err == nil {
			t.Fatalf("malformed retained definition bytes accepted")
		}
	})
}
