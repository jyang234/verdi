// Closure evidence seam tests (Task 10 correction; design §9; SI-150;
// controller pin P2). VerifyAcceptedClosureEvidence is the seam Lane 3
// consumes instead of duplicating any capsule algorithm or constructing
// its own trust fact — every P2 result shape gets its own behavioral
// proof here, built on the existing release fixture machinery.
package experimentapp

import (
	"bytes"
	"context"
	"path"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

// closureCapsuleBindingFixture recomputes the exact deterministic capsule
// binding bytes a genuine ReleaseRatified publication would produce,
// through the SAME extracted resolveCapsuleBinding path
// VerifyAcceptedClosureEvidence itself calls (Task 10 correction:
// extraction, not duplication) — the simplest honest construction for
// planting committed manifest bytes without re-exercising the writer-lock
// publish machinery a second time.
func closureCapsuleBindingFixture(t *testing.T, fixture releaseFixture) (capsuleBindingResult, acceptedSnapshot) {
	t.Helper()
	ctx := context.Background()
	snapshot, err := resolveAccepted(ctx, fixture.service.git, fixture.identity)
	if err != nil {
		t.Fatalf("resolveAccepted() error = %v", err)
	}
	facts, outcome := fixture.service.acceptedRatificationAt(ctx, fixture.identity, snapshot)
	if outcome.Classification != ClassificationClean {
		t.Fatalf("acceptedRatificationAt() outcome = %+v, want clean", outcome)
	}
	definitionDigest, payload, payloadOutcome := fixture.service.resolveEffectivePolicyPayload(ctx, fixture.identity, snapshot, facts)
	if payloadOutcome.Classification != ClassificationClean {
		t.Fatalf("resolveEffectivePolicyPayload() outcome = %+v, want clean", payloadOutcome)
	}
	binding, bindingOutcome := fixture.service.resolveCapsuleBinding(ctx, fixture.identity, snapshot, facts, definitionDigest, payload.Limits.RetainedArtifactBytes)
	if bindingOutcome.Classification != ClassificationClean {
		t.Fatalf("resolveCapsuleBinding() outcome = %+v, want clean", bindingOutcome)
	}
	return binding, snapshot
}

// flipDigestTail mutates the last hex digit of a canonical sha256 digest,
// producing a different but still grammatically valid digest — the
// mismatch fixture needs bytes that decode cleanly but diverge from the
// recomputed binding, never bytes that merely fail to decode (that is a
// separate, distinctly classified case).
func flipDigestTail(t *testing.T, digest string) string {
	t.Helper()
	if digest == "" {
		t.Fatalf("flipDigestTail: empty digest")
	}
	b := []byte(digest)
	replacement := byte('0')
	if b[len(b)-1] == '0' {
		replacement = '1'
	}
	b[len(b)-1] = replacement
	return string(b)
}

// TestVerifyAcceptedClosureEvidenceSelectingCleanManifest is P2 case 1:
// a selecting disposition whose accepted tree carries a committed
// selected/capsule-manifest.json byte-identical to the recomputed binding
// is CLEAN with populated capsule facts. It also proves case 7 (deep-copy):
// mutating the returned Capsule/ArtifactIDs never leaks into a second call.
func TestVerifyAcceptedClosureEvidenceSelectingCleanManifest(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("closure-workload-bytes\n"), experiment.DispositionSelectRecommended, "")
	binding, snapshot := closureCapsuleBindingFixture(t, fixture)
	manifestPath := path.Join(snapshot.experimentPath, capsuleManifestFile)
	plantGitBlob(fixture.git, manifestPath, binding.encoded)

	result := fixture.service.VerifyAcceptedClosureEvidence(context.Background(), fixture.identity)
	if result.Outcome.Classification != ClassificationClean {
		t.Fatalf("outcome = %+v, want clean", result.Outcome)
	}
	if !result.Selecting {
		t.Fatalf("Selecting = false, want true")
	}
	if result.Disposition != experiment.DispositionSelectRecommended || result.ResultDigest != fixture.winnerDigest {
		t.Fatalf("ratification facts = %+v, want disposition/result digest from the accepted ratification", result)
	}
	if result.PrincipalID != fixture.verification.Resolution.PrincipalID {
		t.Fatalf("PrincipalID = %q, want %q", result.PrincipalID, fixture.verification.Resolution.PrincipalID)
	}
	if result.AcceptedHead != snapshot.revision.Head || result.ExperimentPath != snapshot.experimentPath {
		t.Fatalf("accepted coordinates = %+v", result)
	}
	if result.Capsule == nil {
		t.Fatalf("Capsule = nil, want populated capsule facts for a selecting disposition")
	}
	if result.Capsule.Selected != binding.manifest.Selected {
		t.Fatalf("Capsule.Selected = %q, want %q", result.Capsule.Selected, binding.manifest.Selected)
	}
	if result.Capsule.ManifestDigest != rawDigest(binding.encoded) {
		t.Fatalf("Capsule.ManifestDigest = %q, want %q", result.Capsule.ManifestDigest, rawDigest(binding.encoded))
	}
	if len(result.Capsule.ArtifactIDs) == 0 {
		t.Fatalf("Capsule.ArtifactIDs is empty")
	}
	wantIDs := map[string]bool{}
	for _, artifact := range binding.manifest.Artifacts {
		wantIDs[artifact.ID] = true
	}
	for _, id := range result.Capsule.ArtifactIDs {
		if !wantIDs[id] {
			t.Fatalf("Capsule.ArtifactIDs contains unexpected id %q", id)
		}
	}

	// Case 7: deep-copy proof — mutating the first result's Capsule facts
	// (including its backing ArtifactIDs array) must never be visible to a
	// second call; no aliasing of snapshot or kernel state.
	originalSelected, originalDigest, originalFirstID := result.Capsule.Selected, result.Capsule.ManifestDigest, result.Capsule.ArtifactIDs[0]
	result.Capsule.Selected = "tampered"
	result.Capsule.ManifestDigest = "tampered"
	result.Capsule.ArtifactIDs[0] = "tampered"
	result.Disposition = "tampered"

	again := fixture.service.VerifyAcceptedClosureEvidence(context.Background(), fixture.identity)
	if again.Outcome.Classification != ClassificationClean {
		t.Fatalf("second call outcome = %+v, want clean", again.Outcome)
	}
	if again.Disposition != experiment.DispositionSelectRecommended {
		t.Fatalf("second call Disposition = %q, want the untampered accepted disposition", again.Disposition)
	}
	if again.Capsule == nil || again.Capsule.Selected != originalSelected || again.Capsule.ManifestDigest != originalDigest {
		t.Fatalf("second call Capsule = %+v, want the untampered facts (Selected=%q Digest=%q)", again.Capsule, originalSelected, originalDigest)
	}
	if again.Capsule.ArtifactIDs[0] != originalFirstID {
		t.Fatalf("second call ArtifactIDs[0] = %q, want %q — first result's backing array leaked", again.Capsule.ArtifactIDs[0], originalFirstID)
	}
}

// TestVerifyAcceptedClosureEvidenceNonSelectingHasNoCapsule is P2 case 2:
// a clean non-selecting ratified experiment is CLEAN with the ratification
// facts populated and no capsule facts at all — there is no capsule to
// verify.
func TestVerifyAcceptedClosureEvidenceNonSelectingHasNoCapsule(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("closure-workload-bytes\n"), experiment.DispositionRejectAll, "")
	result := fixture.service.VerifyAcceptedClosureEvidence(context.Background(), fixture.identity)
	if result.Outcome.Classification != ClassificationClean {
		t.Fatalf("outcome = %+v, want clean", result.Outcome)
	}
	if result.Selecting {
		t.Fatalf("Selecting = true, want false for a non-selecting disposition")
	}
	if result.Capsule != nil {
		t.Fatalf("Capsule = %+v, want nil for a non-selecting disposition", result.Capsule)
	}
	if result.Disposition != experiment.DispositionRejectAll || result.ResultDigest != fixture.winnerDigest {
		t.Fatalf("ratification facts = %+v", result)
	}
	if result.PrincipalID != fixture.verification.Resolution.PrincipalID {
		t.Fatalf("PrincipalID = %q, want %q", result.PrincipalID, fixture.verification.Resolution.PrincipalID)
	}
}

// TestVerifyAcceptedClosureEvidenceSelectingMissingManifestIsVerdict is P2
// case 3: a selecting disposition with NO committed
// selected/capsule-manifest.json at the accepted HEAD is a verdict, never
// an operational failure or a silent clean pass.
func TestVerifyAcceptedClosureEvidenceSelectingMissingManifestIsVerdict(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("closure-workload-bytes\n"), experiment.DispositionSelectRecommended, "")
	result := fixture.service.VerifyAcceptedClosureEvidence(context.Background(), fixture.identity)
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "closure-evidence-unsatisfied-manifest-missing" {
		t.Fatalf("outcome = %+v, want closure-evidence-unsatisfied-manifest-missing verdict", result.Outcome)
	}
	if result.Capsule != nil {
		t.Fatalf("Capsule = %+v, want nil on a non-clean outcome", result.Capsule)
	}
}

// TestVerifyAcceptedClosureEvidenceSelectingManifestMismatchIsVerdict is
// P2 case 4: a selecting disposition whose committed manifest strict-
// decodes cleanly but whose exact canonical bytes differ from the
// recomputed binding is a verdict, not an operational corruption refusal —
// the bytes are well-formed, they merely fail to match.
func TestVerifyAcceptedClosureEvidenceSelectingManifestMismatchIsVerdict(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("closure-workload-bytes\n"), experiment.DispositionSelectRecommended, "")
	binding, snapshot := closureCapsuleBindingFixture(t, fixture)
	decoded, err := experiment.DecodeCapsuleManifest(binding.encoded)
	if err != nil {
		t.Fatalf("DecodeCapsuleManifest(recomputed) error = %v", err)
	}
	mutated := decoded
	mutated.ResultDigest = flipDigestTail(t, mutated.ResultDigest)
	mutatedEncoded, err := experiment.EncodeCapsuleManifest(mutated)
	if err != nil {
		t.Fatalf("EncodeCapsuleManifest(mutated) error = %v", err)
	}
	if bytes.Equal(mutatedEncoded, binding.encoded) {
		t.Fatalf("mutated manifest bytes equal the recomputed bytes; fixture failed to diverge")
	}
	manifestPath := path.Join(snapshot.experimentPath, capsuleManifestFile)
	plantGitBlob(fixture.git, manifestPath, mutatedEncoded)

	result := fixture.service.VerifyAcceptedClosureEvidence(context.Background(), fixture.identity)
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "closure-evidence-unsatisfied-manifest-mismatch" {
		t.Fatalf("outcome = %+v, want closure-evidence-unsatisfied-manifest-mismatch verdict", result.Outcome)
	}
	if result.Capsule != nil {
		t.Fatalf("Capsule = %+v, want nil on a non-clean outcome", result.Capsule)
	}
}

// TestVerifyAcceptedClosureEvidenceSelectingUndecodableManifestIsOperational
// is P2's decode-failure branch: committed manifest bytes that fail
// strict decode are corrupted accepted evidence, an operational refusal,
// never a verdict.
func TestVerifyAcceptedClosureEvidenceSelectingUndecodableManifestIsOperational(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("closure-workload-bytes\n"), experiment.DispositionSelectRecommended, "")
	snapshot, err := resolveAccepted(context.Background(), fixture.service.git, fixture.identity)
	if err != nil {
		t.Fatalf("resolveAccepted() error = %v", err)
	}
	manifestPath := path.Join(snapshot.experimentPath, capsuleManifestFile)
	plantGitBlob(fixture.git, manifestPath, []byte("{not valid json"))

	result := fixture.service.VerifyAcceptedClosureEvidence(context.Background(), fixture.identity)
	if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "closure-capsule-manifest-invalid" {
		t.Fatalf("outcome = %+v, want closure-capsule-manifest-invalid operational", result.Outcome)
	}
}

// TestVerifyAcceptedClosureEvidencePassesThroughAcceptedRatificationVerdict
// is P2 case 6: with no accepted ratification at all, the seam returns
// exactly acceptedRatificationAt's own verdict rather than reimplementing
// a second classification.
func TestVerifyAcceptedClosureEvidencePassesThroughAcceptedRatificationVerdict(t *testing.T) {
	_, service, identity, _, _ := ratifiableService(t)
	result := service.VerifyAcceptedClosureEvidence(context.Background(), identity)
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-not-accepted" {
		t.Fatalf("outcome = %+v, want ratification-not-accepted verdict", result.Outcome)
	}
}
