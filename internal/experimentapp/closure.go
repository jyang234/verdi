// Closure evidence (Wave 5C Task 10 correction; design §9; SI-150). The
// application core exposes one read-only, parameterless
// Service.VerifyAcceptedClosureEvidence operation: it resolves one
// accepted snapshot, reuses acceptedRatificationAt's retained-proof
// re-verification, and — for a selecting disposition — reuses the
// release path's own retained-input gathering and
// experiment.BindCapsuleManifest authority to require the committed
// selected/capsule-manifest.json's exact canonical bytes to equal the
// recomputed binding. The existing spike-close service (Lane 3) consumes
// this operation instead of duplicating any capsule algorithm or
// constructing its own trust fact.
package experimentapp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// ClosureCapsuleEvidence is the byte-verified capsule facts a selecting
// disposition's closure evidence carries: present only when the committed
// manifest strict-decodes to its own deterministic encoding AND its exact
// canonical bytes equal the recomputed experiment.BindCapsuleManifest
// encoding (controller pin P2).
type ClosureCapsuleEvidence struct {
	Selected       string
	ManifestDigest string
	ArtifactIDs    []string
}

// ClosureEvidenceResult is VerifyAcceptedClosureEvidence's typed result
// (design §9, controller pin P2): a CLEAN outcome always carries the exact
// accepted ratification facts (disposition, result digest, principal,
// experiment path, accepted head); a selecting disposition additionally
// carries the byte-verified capsule facts. Every field is a deep copy — no
// aliasing of accepted-snapshot or kernel state.
type ClosureEvidenceResult struct {
	Outcome        Outcome
	AcceptedHead   string
	ExperimentPath string
	Disposition    experiment.Disposition
	ResultDigest   string
	PrincipalID    governanceprincipal.PrincipalID
	Selecting      bool
	Capsule        *ClosureCapsuleEvidence
}

// VerifyAcceptedClosureEvidence resolves one current accepted snapshot,
// verifies the retained V3 ratification proof through acceptedRatificationAt,
// and — for a selecting disposition — re-verifies the committed capsule
// manifest byte-for-byte against the recomputed binding (controller pins
// P2, P5: no authority or trust-fact parameter). A non-selecting ratified
// experiment is CLEAN with no capsule facts: there is no capsule to verify.
func (s *Service) VerifyAcceptedClosureEvidence(ctx context.Context, identity Identity) ClosureEvidenceResult {
	if ctx == nil {
		return ClosureEvidenceResult{Outcome: operationalOutcome("invalid-request", fmt.Errorf("experimentapp: closure evidence context is nil"))}
	}
	if err := identity.validate(); err != nil {
		return ClosureEvidenceResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	snapshot, err := resolveAccepted(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return ClosureEvidenceResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return ClosureEvidenceResult{Outcome: operationalOutcome("accepted-tree-invalid", err)}
	}
	facts, outcome := s.acceptedRatificationAt(ctx, identity, snapshot)
	if outcome.Classification != ClassificationClean {
		return ClosureEvidenceResult{Outcome: outcome}
	}

	result := ClosureEvidenceResult{
		Outcome: cleanOutcome(), AcceptedHead: snapshot.revision.Head, ExperimentPath: snapshot.experimentPath,
		Disposition: facts.record.Disposition, ResultDigest: facts.record.ResultDigest, PrincipalID: facts.principal,
	}
	selecting := facts.record.Disposition == experiment.DispositionSelectRecommended ||
		facts.record.Disposition == experiment.DispositionSelectOther
	result.Selecting = selecting
	if !selecting {
		return result
	}

	definitionDigest, payload, payloadOutcome := s.resolveEffectivePolicyPayload(ctx, identity, snapshot, facts)
	if payloadOutcome.Classification != ClassificationClean {
		result.Outcome = payloadOutcome
		return result
	}
	binding, bindingOutcome := s.resolveCapsuleBinding(ctx, identity, snapshot, facts, definitionDigest, payload.Limits.RetainedArtifactBytes)
	if bindingOutcome.Classification != ClassificationClean {
		result.Outcome = bindingOutcome
		return result
	}

	committedRaw, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, capsuleManifestFile))
	if errors.Is(err, fs.ErrNotExist) {
		result.Outcome = verdictOutcome("closure-evidence-unsatisfied-manifest-missing", "no committed selected/capsule-manifest.json is present at the accepted HEAD")
		return result
	}
	if err != nil {
		result.Outcome = operationalOutcome("closure-capsule-manifest-unreadable", err)
		return result
	}
	committedManifest, err := experiment.DecodeCapsuleManifest(committedRaw)
	if err != nil {
		result.Outcome = operationalOutcome("closure-capsule-manifest-invalid", fmt.Errorf("committed capsule manifest does not strict-decode: %w", err))
		return result
	}
	reencodedCommitted, err := experiment.EncodeCapsuleManifest(committedManifest)
	if err != nil || !slicesEqualBytes(reencodedCommitted, committedRaw) {
		result.Outcome = operationalOutcome("closure-capsule-manifest-invalid", fmt.Errorf("committed capsule manifest bytes are not its own deterministic encoding"))
		return result
	}
	if !slicesEqualBytes(committedRaw, binding.encoded) {
		result.Outcome = verdictOutcome("closure-evidence-unsatisfied-manifest-mismatch", "committed capsule manifest bytes do not equal the recomputed binding")
		return result
	}

	artifactIDs := make([]string, 0, len(binding.manifest.Artifacts))
	for _, artifact := range binding.manifest.Artifacts {
		artifactIDs = append(artifactIDs, artifact.ID)
	}
	result.Capsule = &ClosureCapsuleEvidence{
		Selected: binding.manifest.Selected, ManifestDigest: rawDigest(binding.encoded), ArtifactIDs: artifactIDs,
	}
	return result
}
