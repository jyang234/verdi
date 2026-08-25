package experimentapp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentpolicy"
)

// InspectResult is the immutable read-only accepted experiment projection.
type InspectResult struct {
	Outcome        Outcome
	AcceptedHead   string
	ExperimentPath string
	Definition     experiment.Definition
	State          experiment.StateDerivation
	Reproduction   experiment.ReproductionStatus
	PolicyDigest   string
	PolicyLimits   experimentpolicy.Limits
}

// Inspect resolves all authority bytes from one exact accepted commit and
// runs the source-backed experiment state owner. It never reads the worktree.
func (s *Service) Inspect(ctx context.Context, identity Identity) InspectResult {
	if err := identity.validate(); err != nil {
		return InspectResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	snapshot, err := resolveAccepted(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return InspectResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return InspectResult{Outcome: operationalOutcome("accepted-tree-invalid", err)}
	}
	definitionBytes, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "experiment.yaml"))
	if err != nil {
		return InspectResult{Outcome: operationalOutcome("definition-unreadable", err)}
	}
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		return InspectResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	candidatePaths, err := experiment.ValidateCandidatePatchesFromSource(snapshot.source, snapshot.experimentPath, definition)
	if err != nil {
		return InspectResult{Outcome: operationalOutcome("candidate-invalid", err)}
	}
	capabilities := experiment.Capabilities{}
	if definition.Schema == experiment.DefinitionSchemaV2 {
		capabilitiesBytes, readErr := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "evaluator-capabilities.json"))
		if readErr != nil {
			return InspectResult{Outcome: operationalOutcome("capabilities-unreadable", readErr)}
		}
		capabilities, err = experiment.DecodeCapabilities(capabilitiesBytes)
		if err != nil {
			return InspectResult{Outcome: operationalOutcome("capabilities-invalid", err)}
		}
		capabilitiesDigest := rawDigest(capabilitiesBytes)
		if capabilitiesDigest != definition.Evaluator.CapabilitiesDigest {
			return InspectResult{Outcome: operationalOutcome("capabilities-digest-mismatch", fmt.Errorf("accepted capabilities digest %q does not match definition %q", capabilitiesDigest, definition.Evaluator.CapabilitiesDigest))}
		}
	}
	decision, err := s.policy.ResolvePolicy(ctx, clonePolicyRequest(PolicyRequest{
		CheckoutRoot: identity.CheckoutRoot, ExperimentPath: snapshot.experimentPath,
		Spike: identity.Spike, AcceptedCommit: snapshot.revision.Head,
		Definition: definition, Capabilities: capabilities,
		CandidatePaths: candidatePaths,
	}))
	if err != nil {
		return InspectResult{Outcome: policyResolutionErrorOutcome(err)}
	}
	if decision == nil {
		return InspectResult{Outcome: operationalOutcome("policy-resolution-invalid", fmt.Errorf("experimentapp: policy resolver returned nil decision"))}
	}
	policyDigest, err := decision.EffectivePolicyDigest()
	if err != nil {
		return InspectResult{Outcome: operationalOutcome("policy-resolution-invalid", err)}
	}
	policyPayload, err := decision.Payload()
	if err != nil {
		return InspectResult{Outcome: operationalOutcome("policy-resolution-invalid", err)}
	}
	if definition.Schema == experiment.DefinitionSchemaV2 {
		if _, err := experimentpolicy.Authorize(decision, experimentpolicy.AuthorizationInput{
			Definition: definition, Capabilities: capabilities, ExperimentPath: snapshot.experimentPath,
			CandidatePaths: candidatePaths,
		}); err != nil {
			return InspectResult{Outcome: verdictOutcome("policy-refused", err.Error())}
		}
	}
	derived, err := experiment.DeriveStateDetailsFromSource(snapshot.source, snapshot.experimentPath, s.results.VerifyResult)
	if err != nil {
		return InspectResult{Outcome: operationalOutcome("state-invalid", err)}
	}
	return InspectResult{
		Outcome: cleanOutcome(), AcceptedHead: snapshot.revision.Head,
		ExperimentPath: snapshot.experimentPath, Definition: cloneDefinition(definition),
		State: derived, Reproduction: derived.Reproduction, PolicyDigest: policyDigest,
		PolicyLimits: policyPayload.Limits,
	}
}
