package experimentapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentpolicy"
)

// ValidateDraftResult proves proposal schema, patches, evaluator discovery,
// effective policy, and the current derived worktree posture without writing.
type ValidateDraftResult struct {
	Outcome            Outcome
	AcceptedHead       string
	ExperimentPath     string
	Definition         experiment.Definition
	DefinitionDigest   string
	Capabilities       experiment.Capabilities
	CapabilitiesDigest string
	PolicyDigest       string
	PolicyLimits       experimentpolicy.Limits
	State              experiment.StateDerivation
}

// ValidateDraft validates caller-selected filesystem proposal bytes against
// one accepted HEAD fact and exactly one effective-policy resolution.
func (s *Service) ValidateDraft(ctx context.Context, identity Identity) ValidateDraftResult {
	if err := identity.validate(); err != nil {
		return ValidateDraftResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	revision, err := resolveAcceptedHead(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return ValidateDraftResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return ValidateDraftResult{Outcome: operationalOutcome("accepted-head-invalid", err)}
	}
	return s.validateDraftAtRevision(ctx, identity, revision)
}

func (s *Service) validateDraftAtRevision(ctx context.Context, identity Identity, revision DefaultBranch) ValidateDraftResult {
	experimentPath, err := proposedExperimentPath(identity)
	if err != nil {
		return ValidateDraftResult{Outcome: operationalOutcome("proposal-location-invalid", err)}
	}
	source := os.DirFS(identity.CheckoutRoot)
	definitionBytes, err := fs.ReadFile(source, path.Join(experimentPath, "experiment.yaml"))
	if err != nil {
		return ValidateDraftResult{Outcome: operationalOutcome("definition-unreadable", err)}
	}
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		return ValidateDraftResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	definitionDigest, err := experiment.DefinitionDigest(definition)
	if err != nil {
		return ValidateDraftResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	candidatePaths, err := experiment.ValidateCandidatePatchesFromSource(source, experimentPath, definition)
	if err != nil {
		return ValidateDraftResult{Outcome: operationalOutcome("candidate-invalid", err)}
	}
	decision, err := s.policy.ResolvePolicy(ctx, clonePolicyRequest(PolicyRequest{
		CheckoutRoot: identity.CheckoutRoot, ExperimentPath: experimentPath,
		Spike: identity.Spike, Definition: definition, CandidatePaths: candidatePaths,
	}))
	if err != nil {
		return ValidateDraftResult{Outcome: operationalOutcome("policy-resolution-failed", err)}
	}
	if decision == nil {
		return ValidateDraftResult{Outcome: operationalOutcome("policy-resolution-invalid", fmt.Errorf("experimentapp: policy resolver returned nil decision"))}
	}
	policyDigest, err := decision.Digest()
	if err != nil {
		return ValidateDraftResult{Outcome: operationalOutcome("policy-resolution-invalid", err)}
	}
	policyPayload, err := decision.Payload()
	if err != nil {
		return ValidateDraftResult{Outcome: operationalOutcome("policy-resolution-invalid", err)}
	}

	capabilities := experiment.Capabilities{}
	capabilitiesDigest := ""
	if definition.Schema == experiment.DefinitionSchemaV2 {
		discovery, discoverErr := s.capabilities.DiscoverCapabilities(ctx, CapabilityRequest{
			CheckoutRoot: identity.CheckoutRoot, Definition: cloneDefinition(definition),
		})
		if discoverErr != nil {
			return ValidateDraftResult{Outcome: operationalOutcome("capability-discovery-failed", discoverErr)}
		}
		capabilities, err = experiment.DecodeCapabilities(append([]byte(nil), discovery.Bytes...))
		if err != nil {
			return ValidateDraftResult{Outcome: operationalOutcome("capabilities-invalid", err)}
		}
		capabilitiesDigest = rawDigest(discovery.Bytes)
		if capabilitiesDigest != definition.Evaluator.CapabilitiesDigest {
			return ValidateDraftResult{Outcome: operationalOutcome("capabilities-digest-mismatch", fmt.Errorf("discovered capabilities digest %q does not match definition %q", capabilitiesDigest, definition.Evaluator.CapabilitiesDigest))}
		}
	}

	base := ValidateDraftResult{
		AcceptedHead: revision.Head, ExperimentPath: experimentPath,
		Definition: cloneDefinition(definition), DefinitionDigest: definitionDigest,
		Capabilities: cloneCapabilities(capabilities), CapabilitiesDigest: capabilitiesDigest,
		PolicyDigest: policyDigest, PolicyLimits: policyPayload.Limits,
	}
	if definition.Schema != experiment.DefinitionSchemaV2 {
		base.Outcome = verdictOutcome("definition-v1-not-registrable", "definition v1 is strict decode-only compatibility and cannot be registered")
		return base
	}
	if _, err := experimentpolicy.Authorize(decision, experimentpolicy.AuthorizationInput{
		Definition: definition, Capabilities: capabilities, ExperimentPath: experimentPath,
		CandidatePaths: candidatePaths,
	}); err != nil {
		base.Outcome = verdictOutcome("policy-refused", err.Error())
		return base
	}
	derived, err := experiment.DeriveStateDetailsFromSource(source, experimentPath, s.results.VerifyResult)
	if err != nil {
		base.Outcome = operationalOutcome("state-invalid", err)
		return base
	}
	base.State = derived
	base.Outcome = cleanOutcome()
	return base
}

func proposedExperimentPath(identity Identity) (string, error) {
	spikeID := strings.TrimPrefix(identity.Spike, "spec/")
	candidates := []string{
		path.Join(".verdi/specs/active", spikeID, "experiments", identity.ExperimentID),
		path.Join(".verdi/specs/archive", spikeID, "experiments", identity.ExperimentID),
	}
	present := make([]string, 0, 2)
	for _, candidate := range candidates {
		info, err := os.Lstat(filepath.Join(identity.CheckoutRoot, filepath.FromSlash(candidate), "experiment.yaml"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("experimentapp: inspect proposal %s: %w", candidate, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("experimentapp: proposal %s/experiment.yaml is not a regular file", candidate)
		}
		present = append(present, candidate)
	}
	if len(present) != 1 {
		return "", fmt.Errorf("experimentapp: proposed experiment resolves in %d active/archive locations, want exactly one", len(present))
	}
	return present[0], nil
}

func rawDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
