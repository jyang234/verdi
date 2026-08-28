// Selected evidence capsules and disposable-workspace release (Wave 5C
// Task 9; design §§3–5, 9–10; AC-4, DC-8/DC-9, SI-141, SI-146). Only an
// exact accepted v3 ratification triggers this operation (SI-150): a selecting
// disposition first builds, publishes, and re-verifies the immutable
// capsule manifest from accepted bytes under the checkout writer lock,
// then releases every receipt-derived disposable workspace; non-selecting
// dispositions release the same complete set without a capsule. Cleanup
// failure never edits or removes any minimal durable record.
package experimentapp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentpolicy"
)

// capsuleManifestFile is the immutable publication target under the
// experiment directory.
const capsuleManifestFile = "selected/capsule-manifest.json"

// WorkspaceReleaser is the consumer-owned port over the existing
// execworkspace release operation (satisfied by *execworkspace.Releaser).
type WorkspaceReleaser interface {
	Release(workspaceID string) error
}

// ReleaseAuthority is the configured authority for the release operation:
// only the workspace releaser (Task 10 correction, SI-150, controller pin
// P5). Accepted ratification authority is resolved entirely from the
// accepted Git tree itself — there is no caller-supplied governance
// profile or trust-fact reader left to configure.
type ReleaseAuthority struct {
	Releaser WorkspaceReleaser
}

// ReleaseResult is the typed release outcome. Released and Failed name
// the exact workspace targets in deterministic order; cleanup failure is
// operational and retryable, never a rewrite of durable records.
type ReleaseResult struct {
	Outcome          Outcome
	AcceptedHead     string
	ExperimentPath   string
	Disposition      experiment.Disposition
	Selected         string
	CapsulePublished bool
	ManifestDigest   string
	Released         []string
	Failed           []string
}

// CapsulePublicationResult is the typed publication-only outcome. It carries
// no workspace-release projection because publication never invokes the
// release port or writes release markers.
type CapsulePublicationResult struct {
	Outcome          Outcome
	AcceptedHead     string
	ExperimentPath   string
	Disposition      experiment.Disposition
	Selected         string
	CapsulePublished bool
	ManifestDigest   string
}

// PublishRatifiedCapsule publishes and verifies the selected capsule for an
// accepted selecting ratification. A non-selecting ratification is a clean
// no-op. This operation never releases a workspace.
func (s *Service) PublishRatifiedCapsule(ctx context.Context, identity Identity) CapsulePublicationResult {
	if ctx == nil {
		return CapsulePublicationResult{Outcome: operationalOutcome("invalid-request", fmt.Errorf("experimentapp: capsule publication context is nil"))}
	}
	if err := identity.validate(); err != nil {
		return CapsulePublicationResult{Outcome: operationalOutcome("invalid-request", err)}
	}

	lifecycle, outcome := s.resolveAcceptedRatifiedLifecycle(ctx, identity)
	if outcome.Classification != ClassificationClean {
		return CapsulePublicationResult{Outcome: outcome}
	}
	result := CapsulePublicationResult{
		AcceptedHead: lifecycle.snapshot.revision.Head, ExperimentPath: lifecycle.snapshot.experimentPath,
		Disposition: lifecycle.facts.record.Disposition,
	}
	selecting := lifecycle.facts.record.Disposition == experiment.DispositionSelectRecommended ||
		lifecycle.facts.record.Disposition == experiment.DispositionSelectOther
	if !selecting {
		result.Outcome = cleanOutcome()
		return result
	}

	definitionDigest, payload, payloadOutcome := s.resolveEffectivePolicyPayload(ctx, identity, lifecycle.snapshot, lifecycle.facts)
	if payloadOutcome.Classification != ClassificationClean {
		result.Outcome = payloadOutcome
		return result
	}
	manifestDigest, selected, capsuleOutcome := s.publishSelectedCapsule(ctx, identity, lifecycle.snapshot, lifecycle.facts, definitionDigest, payload.Limits.RetainedArtifactBytes)
	if capsuleOutcome.Classification != ClassificationClean {
		result.Outcome = capsuleOutcome
		return result
	}
	result.CapsulePublished = true
	result.ManifestDigest = manifestDigest
	result.Selected = selected
	result.Outcome = cleanOutcome()
	return result
}

// ReleaseRatified performs the post-ratification lifecycle consequence.
func (s *Service) ReleaseRatified(ctx context.Context, identity Identity, authority ReleaseAuthority) ReleaseResult {
	if ctx == nil {
		return ReleaseResult{Outcome: operationalOutcome("invalid-request", fmt.Errorf("experimentapp: release context is nil"))}
	}
	if err := identity.validate(); err != nil {
		return ReleaseResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	if authority.Releaser == nil {
		return ReleaseResult{Outcome: operationalOutcome("release-authority-invalid", fmt.Errorf("experimentapp: release requires a configured workspace releaser"))}
	}

	lifecycle, outcome := s.resolveAcceptedRatifiedLifecycle(ctx, identity)
	if outcome.Classification != ClassificationClean {
		return ReleaseResult{Outcome: outcome}
	}
	result := ReleaseResult{
		AcceptedHead: lifecycle.snapshot.revision.Head, ExperimentPath: lifecycle.snapshot.experimentPath,
		Disposition: lifecycle.facts.record.Disposition,
	}
	definitionDigest, payload, payloadOutcome := s.resolveEffectivePolicyPayload(ctx, identity, lifecycle.snapshot, lifecycle.facts)
	if payloadOutcome.Classification != ClassificationClean {
		result.Outcome = payloadOutcome
		return result
	}

	targets, targetOutcome := s.releaseTargets(lifecycle.snapshot, lifecycle.facts.definition, definitionDigest, lifecycle.facts.derived)
	if targetOutcome.Classification != ClassificationClean {
		result.Outcome = targetOutcome
		return result
	}

	selecting := lifecycle.facts.record.Disposition == experiment.DispositionSelectRecommended ||
		lifecycle.facts.record.Disposition == experiment.DispositionSelectOther
	if selecting {
		manifestDigest, selected, capsuleOutcome := s.publishSelectedCapsule(ctx, identity, lifecycle.snapshot, lifecycle.facts, definitionDigest, payload.Limits.RetainedArtifactBytes)
		if capsuleOutcome.Classification != ClassificationClean {
			result.Outcome = capsuleOutcome
			return result
		}
		result.CapsulePublished = true
		result.ManifestDigest = manifestDigest
		result.Selected = selected
	}

	// Attempt every known workspace in deterministic order even if one
	// release fails; successful markers remain and retry is idempotent.
	for _, target := range targets {
		if err := authority.Releaser.Release(target); err != nil {
			result.Failed = append(result.Failed, target)
			continue
		}
		result.Released = append(result.Released, target)
	}
	if len(result.Failed) > 0 {
		result.Outcome = operationalOutcome("workspace-release-failed", fmt.Errorf("experimentapp: release failed for workspaces %s", strings.Join(result.Failed, ", ")))
		return result
	}
	result.Outcome = cleanOutcome()
	return result
}

type acceptedRatifiedLifecycle struct {
	snapshot acceptedSnapshot
	facts    acceptedRatificationFacts
}

// resolveAcceptedRatifiedLifecycle resolves the one exact accepted snapshot and its
// authenticated ratification facts shared by publication and release.
func (s *Service) resolveAcceptedRatifiedLifecycle(ctx context.Context, identity Identity) (acceptedRatifiedLifecycle, Outcome) {
	snapshot, err := resolveAccepted(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return acceptedRatifiedLifecycle{}, verdictOutcome("accepted-head-stale", stale.Error())
		}
		return acceptedRatifiedLifecycle{}, operationalOutcome("accepted-tree-invalid", err)
	}
	facts, outcome := s.acceptedRatificationAt(ctx, identity, snapshot)
	if outcome.Classification != ClassificationClean {
		return acceptedRatifiedLifecycle{}, outcome
	}
	return acceptedRatifiedLifecycle{snapshot: snapshot, facts: facts}, cleanOutcome()
}

// releaseTargets derives the complete disposable-workspace set only from
// validated execution receipts: every visible run's receipt, every receipt
// candidate, full identity reconstruction against the receipt's
// materialization, sorted and deduplicated. It never walks the filesystem
// or infers a target from names.
func (s *Service) releaseTargets(snapshot acceptedSnapshot, definition experiment.Definition, definitionDigest string, derived experiment.StateDerivation) ([]string, Outcome) {
	candidatesByID := make(map[string]experiment.Candidate, len(definition.Candidates))
	for _, candidate := range definition.Candidates {
		candidatesByID[candidate.ID] = candidate
	}
	seen := map[string]bool{}
	targets := make([]string, 0, len(derived.Runs)*len(definition.Candidates))
	for _, run := range derived.Runs {
		raw, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "runs", run.Run, "execution.json"))
		if errors.Is(err, fs.ErrNotExist) {
			// A run without a validated receipt (decode-only v1 evidence)
			// contributes no derivable workspace identity.
			continue
		}
		if err != nil {
			return nil, operationalOutcome("receipt-unreadable", err)
		}
		receipt, err := experiment.DecodeExecutionReceipt(raw)
		if err != nil {
			return nil, operationalOutcome("receipt-invalid", err)
		}
		if receipt.ExperimentDigest != definitionDigest || receipt.Run != run.Run {
			return nil, operationalOutcome("receipt-invalid", fmt.Errorf("experimentapp: receipt identity for run %q does not match the locked definition", run.Run))
		}
		// The receipt's COMPLETE candidate authority must match the locked
		// definition — exact membership, cardinality, base commits, and
		// patch digests — before any workspace target derives from it.
		if err := experiment.ValidateReceiptCandidateAuthority(definition, receipt); err != nil {
			return nil, operationalOutcome("receipt-invalid", err)
		}
		for _, candidate := range receipt.Candidates {
			registered, ok := candidatesByID[candidate.ID]
			if !ok {
				return nil, operationalOutcome("receipt-invalid", fmt.Errorf("experimentapp: receipt candidate %q is not registered by the definition", candidate.ID))
			}
			patchBytes, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, registered.Patch))
			if err != nil {
				return nil, operationalOutcome("candidate-invalid", err)
			}
			workspaceRunID, err := experiment.WorkspaceRunID(definitionDigest, run.Run, candidate.ID)
			if err != nil {
				return nil, operationalOutcome("workspace-identity-invalid", err)
			}
			reconstructed, err := execworkspace.NewPatchIdentity(workspaceRunID, candidate.BaseCommit, patchBytes)
			if err != nil {
				return nil, operationalOutcome("workspace-identity-invalid", err)
			}
			materialization := candidate.Materialization
			if materialization.Shape != experiment.WorkspaceBasePlusPatch ||
				reconstructed.Shape != execworkspace.BasePlusPatch ||
				materialization.RunID != reconstructed.RunID ||
				materialization.CommitSHA != reconstructed.CommitSHA ||
				materialization.PatchSHA256 != reconstructed.PatchSHA256 ||
				candidate.WorkspaceRunID != workspaceRunID {
				return nil, operationalOutcome("workspace-identity-mismatch", fmt.Errorf("experimentapp: receipt materialization for run %q candidate %q does not match the reconstructed workspace identity", run.Run, candidate.ID))
			}
			workspaceID, err := reconstructed.WorkspaceID()
			if err != nil {
				return nil, operationalOutcome("workspace-identity-invalid", err)
			}
			if !seen[workspaceID] {
				seen[workspaceID] = true
				targets = append(targets, workspaceID)
			}
		}
	}
	sort.Strings(targets)
	return targets, cleanOutcome()
}

// resolveEffectivePolicyPayload resolves the accepted definition digest and
// the one sealed effective policy payload for an accepted, ratified
// experiment: exactly one PolicyResolver call, with the retention ceiling
// read only through the seal-checked payload projection (SI-141). Both
// ReleaseRatified and VerifyAcceptedClosureEvidence (Task 10 correction)
// share this single resolution path — extracted unchanged from
// ReleaseRatified's prior body.
func (s *Service) resolveEffectivePolicyPayload(ctx context.Context, identity Identity, snapshot acceptedSnapshot, facts acceptedRatificationFacts) (definitionDigest string, payload experimentpolicy.Payload, outcome Outcome) {
	definitionDigest, err := experiment.DefinitionDigest(facts.definition)
	if err != nil {
		return "", experimentpolicy.Payload{}, operationalOutcome("definition-invalid", err)
	}
	candidatePaths, err := experiment.ValidateCandidatePatchesFromSource(snapshot.source, snapshot.experimentPath, facts.definition)
	if err != nil {
		return "", experimentpolicy.Payload{}, operationalOutcome("candidate-invalid", err)
	}
	capabilitiesBytes, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "evaluator-capabilities.json"))
	if err != nil {
		return "", experimentpolicy.Payload{}, operationalOutcome("capabilities-unreadable", err)
	}
	capabilities, err := experiment.DecodeCapabilities(capabilitiesBytes)
	if err != nil {
		return "", experimentpolicy.Payload{}, operationalOutcome("capabilities-invalid", err)
	}
	decision, err := s.policy.ResolvePolicy(ctx, clonePolicyRequest(PolicyRequest{
		CheckoutRoot: identity.CheckoutRoot, ExperimentPath: snapshot.experimentPath,
		Spike: identity.Spike, AcceptedCommit: snapshot.revision.Head,
		Definition: facts.definition, Capabilities: capabilities, CandidatePaths: candidatePaths,
	}))
	if err != nil {
		return "", experimentpolicy.Payload{}, policyResolutionErrorOutcome(err)
	}
	if decision == nil {
		return "", experimentpolicy.Payload{}, operationalOutcome("policy-resolution-invalid", fmt.Errorf("experimentapp: policy resolver returned nil decision"))
	}
	payload, err = decision.Payload()
	if err != nil {
		return "", experimentpolicy.Payload{}, operationalOutcome("policy-resolution-invalid", err)
	}
	return definitionDigest, payload, cleanOutcome()
}

// capsuleBindingResult is the deterministic recomputed capsule manifest and
// its canonical bytes for one accepted, ratified, selecting experiment.
type capsuleBindingResult struct {
	manifest experiment.CapsuleManifest
	encoded  []byte
}

// resolveCapsuleBinding recomputes the deterministic capsule manifest for
// one accepted ratification fact set: the complete retained-input
// gathering and experiment.BindCapsuleManifest. It is the single owner
// both ReleaseRatified's publication path and
// VerifyAcceptedClosureEvidence's byte-verification path call (Task 10
// correction: extraction, not duplication, from publishSelectedCapsule's
// prior body — ReleaseRatified's behavior is unchanged).
func (s *Service) resolveCapsuleBinding(ctx context.Context, identity Identity, snapshot acceptedSnapshot, facts acceptedRatificationFacts, definitionDigest string, retainedArtifactBytes int64) (capsuleBindingResult, Outcome) {
	inputs, inputsOutcome := s.retainedInputs(ctx, identity, snapshot, facts)
	if inputsOutcome.Classification != ClassificationClean {
		return capsuleBindingResult{}, inputsOutcome
	}
	manifest, err := experiment.BindCapsuleManifest(experiment.CapsuleBindingInput{
		Definition: facts.definition, DefinitionDigest: definitionDigest,
		Ratification: facts.record, Result: facts.result,
		Artifacts: inputs, RetainedArtifactBytes: retainedArtifactBytes,
	})
	if err != nil {
		if experiment.IsCapsuleArtifactOversized(err) {
			return capsuleBindingResult{}, verdictOutcome("capsule-retention-refused", err.Error())
		}
		return capsuleBindingResult{}, operationalOutcome("capsule-binding-invalid", err)
	}
	encoded, err := experiment.EncodeCapsuleManifest(manifest)
	if err != nil {
		return capsuleBindingResult{}, operationalOutcome("capsule-binding-invalid", err)
	}
	return capsuleBindingResult{manifest: manifest, encoded: encoded}, cleanOutcome()
}

// publishSelectedCapsule recomputes the deterministic capsule binding,
// publishes it immutably under the checkout writer lock, and strict-decodes
// the winning bytes before any release may run.
func (s *Service) publishSelectedCapsule(ctx context.Context, identity Identity, snapshot acceptedSnapshot, facts acceptedRatificationFacts, definitionDigest string, retainedArtifactBytes int64) (manifestDigest, selected string, outcome Outcome) {
	binding, bindingOutcome := s.resolveCapsuleBinding(ctx, identity, snapshot, facts, definitionDigest, retainedArtifactBytes)
	if bindingOutcome.Classification != ClassificationClean {
		return "", "", bindingOutcome
	}
	manifest, encoded := binding.manifest, binding.encoded

	manifestPath := path.Join(snapshot.experimentPath, capsuleManifestFile)
	if err := experiment.ValidateRepoRelativePath(manifestPath); err != nil {
		return "", "", operationalOutcome("capsule-publish-failed", err)
	}
	absolute, err := proposalAbsolutePath(identity.CheckoutRoot, manifestPath)
	if err != nil {
		return "", "", operationalOutcome("capsule-publish-failed", err)
	}
	publishErr := draftmutation.WithWriterLock(ctx, identity.CheckoutRoot, draftmutation.Coordinator{}, func(_ *draftmutation.LockedWriter) error {
		// The existing proposal path-safety seam refuses symlinked,
		// missing-as-non-directory, and escaping parents component by
		// component before any byte lands, and the final component must be
		// absent or an existing regular file — never a symlink or
		// directory the immutable write could follow or collide with.
		if err := ensureProposalDirectory(identity.CheckoutRoot, filepath.Dir(absolute)); err != nil {
			return err
		}
		if info, statErr := os.Lstat(absolute); statErr == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("capsule manifest path %q is not a regular non-symlink file", manifestPath)
			}
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return statErr
		}
		created, existing, err := atomicfile.CreateImmutable(absolute, encoded, 0o600)
		if err != nil {
			return fmt.Errorf("publish capsule manifest: %w", err)
		}
		if !created && !slicesEqualBytes(existing, encoded) {
			return fmt.Errorf("existing capsule manifest %q differs from the recomputed manifest", manifestPath)
		}
		return nil
	})
	if publishErr != nil {
		return "", "", operationalOutcome("capsule-publish-failed", publishErr)
	}

	// Capsule verification: re-read and strict-decode the winning bytes
	// before the first release call.
	winning, err := readCapsuleManifestFile(absolute)
	if err != nil {
		return "", "", operationalOutcome("capsule-publish-failed", err)
	}
	decoded, err := experiment.DecodeCapsuleManifest(winning)
	if err != nil {
		return "", "", operationalOutcome("capsule-publish-failed", fmt.Errorf("published capsule manifest does not strict-decode: %w", err))
	}
	reencoded, err := experiment.EncodeCapsuleManifest(decoded)
	if err != nil || !slicesEqualBytes(reencoded, encoded) {
		return "", "", operationalOutcome("capsule-publish-failed", fmt.Errorf("published capsule manifest does not match the recomputed manifest"))
	}
	return rawDigest(encoded), manifest.Selected, cleanOutcome()
}

// retainedInputs resolves the closed retained artifact set from the exact
// accepted tree: experiment-directory members from the snapshot, and the
// receipt-resolved protected inputs (workload, contract, fixtures) from
// the same pinned commit through Git plumbing. It never follows a symlink
// and never walks a filesystem.
func (s *Service) retainedInputs(ctx context.Context, identity Identity, snapshot acceptedSnapshot, facts acceptedRatificationFacts) ([]experiment.CapsuleRetainedArtifact, Outcome) {
	selectedCandidate, err := experiment.SelectedCapsuleCandidate(facts.definition, facts.result, facts.record)
	if err != nil {
		return nil, operationalOutcome("capsule-binding-invalid", err)
	}
	var patchPath string
	for _, candidate := range facts.definition.Candidates {
		if candidate.ID == selectedCandidate {
			patchPath = candidate.Patch
		}
	}
	runDir := path.Join("runs", facts.run)
	experimentMembers := map[string]string{
		experiment.CapsuleArtifactDefinition:            "experiment.yaml",
		experiment.CapsuleArtifactCandidatePatch:        patchPath,
		experiment.CapsuleArtifactEvaluatorCapabilities: "evaluator-capabilities.json",
		experiment.CapsuleArtifactExecutionReceipt:      path.Join(runDir, "execution.json"),
		experiment.CapsuleArtifactObservations:          path.Join(runDir, "observations.jsonl"),
		experiment.CapsuleArtifactResult:                path.Join(runDir, "result.json"),
		experiment.CapsuleArtifactRatification:          ratificationFileName,
	}
	inputs := make([]experiment.CapsuleRetainedArtifact, 0, len(experimentMembers)+3+len(facts.definition.Fixtures))
	for id, relative := range experimentMembers {
		data, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, relative))
		if err != nil {
			return nil, operationalOutcome("capsule-input-unreadable", fmt.Errorf("experimentapp: retained artifact %q: %w", id, err))
		}
		inputs = append(inputs, experiment.CapsuleRetainedArtifact{ID: id, Bytes: data})
	}
	if data, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "recommendation.md")); err == nil {
		inputs = append(inputs, experiment.CapsuleRetainedArtifact{ID: experiment.CapsuleArtifactRecommendation, Bytes: data})
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, operationalOutcome("capsule-input-unreadable", err)
	}

	// The selected run's receipt owns the resolved protected-input paths
	// (SI-148). Their bytes are read from the SAME pinned accepted commit
	// through Git plumbing; the digest match against the locked definition
	// reference is the authority, the receipt path only transports it.
	receiptInput, outcome := s.retainedProtectedInputs(ctx, identity, snapshot, facts)
	if outcome.Classification != ClassificationClean {
		return nil, outcome
	}
	inputs = append(inputs, receiptInput...)
	return inputs, cleanOutcome()
}

func (s *Service) retainedProtectedInputs(ctx context.Context, identity Identity, snapshot acceptedSnapshot, facts acceptedRatificationFacts) ([]experiment.CapsuleRetainedArtifact, Outcome) {
	raw, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "runs", facts.run, "execution.json"))
	if err != nil {
		return nil, operationalOutcome("receipt-unreadable", err)
	}
	receipt, err := experiment.DecodeExecutionReceipt(raw)
	if err != nil {
		return nil, operationalOutcome("receipt-invalid", err)
	}

	// The snapshot retains the ORIGINAL complete tree enumeration, so no
	// second ListTree runs for the same accepted commit (design §7).
	entriesByPath := make(map[string]GitTreeEntry, len(snapshot.entries))
	for _, entry := range snapshot.entries {
		entriesByPath[entry.Path] = entry
	}

	type role struct {
		id     string
		digest string
	}
	roles := []role{
		{id: experiment.CapsuleArtifactWorkload, digest: facts.definition.Workload.Digest},
		{id: experiment.CapsuleArtifactContract, digest: facts.definition.Contract.Digest},
	}
	for _, fixture := range facts.definition.Fixtures {
		id, err := experiment.CapsuleFixtureArtifactID(fixture.ID)
		if err != nil {
			return nil, operationalOutcome("capsule-binding-invalid", err)
		}
		roles = append(roles, role{id: id, digest: fixture.Digest})
	}

	inputs := make([]experiment.CapsuleRetainedArtifact, 0, len(roles))
	for _, want := range roles {
		wantHex := strings.TrimPrefix(want.digest, "sha256:")
		candidates := make([]string, 0, 1)
		for receiptPath, digest := range receipt.Fingerprint.InputDigests {
			if strings.HasPrefix(receiptPath, "evaluator:") {
				continue
			}
			if digest == wantHex {
				candidates = append(candidates, receiptPath)
			}
		}
		if len(candidates) == 0 {
			return nil, operationalOutcome("capsule-input-unreadable", fmt.Errorf("experimentapp: the selected run's receipt resolves no path for retained artifact %q", want.id))
		}
		sort.Strings(candidates)
		resolvedPath := candidates[0]
		entry, ok := entriesByPath[resolvedPath]
		if !ok {
			return nil, operationalOutcome("capsule-input-unreadable", fmt.Errorf("experimentapp: retained artifact %q path %q is absent from the accepted tree", want.id, resolvedPath))
		}
		if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return nil, operationalOutcome("capsule-input-invalid", fmt.Errorf("experimentapp: retained artifact %q path %q is not a regular non-symlink blob in the accepted tree", want.id, resolvedPath))
		}
		data, err := s.git.ReadBlob(ctx, identity.CheckoutRoot, snapshot.revision.Head, entry.Object, resolvedPath)
		if err != nil {
			return nil, operationalOutcome("capsule-input-unreadable", err)
		}
		if rawDigest(data) != want.digest {
			return nil, operationalOutcome("capsule-input-invalid", fmt.Errorf("experimentapp: retained artifact %q bytes at %q do not recompute to the locked digest %s", want.id, resolvedPath, want.digest))
		}
		inputs = append(inputs, experiment.CapsuleRetainedArtifact{ID: want.id, Bytes: data})
	}
	return inputs, cleanOutcome()
}

func readCapsuleManifestFile(absolute string) ([]byte, error) {
	info, err := os.Lstat(absolute)
	if err != nil {
		return nil, fmt.Errorf("experimentapp: reading published capsule manifest: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("experimentapp: published capsule manifest %q is not a regular non-symlink file", absolute)
	}
	data, err := os.ReadFile(absolute)
	if err != nil {
		return nil, fmt.Errorf("experimentapp: reading published capsule manifest: %w", err)
	}
	return data, nil
}

func slicesEqualBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
