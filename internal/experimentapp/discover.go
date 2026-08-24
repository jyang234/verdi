package experimentapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/experiment"
)

// DiscoverCapabilitiesResult is the read-only projection of one evaluator
// describe response and the proposal identity that selected it.
type DiscoverCapabilitiesResult struct {
	Outcome            Outcome
	AcceptedHead       string
	ExperimentPath     string
	Capabilities       experiment.Capabilities
	CapabilitiesBytes  []byte
	CapabilitiesDigest string
}

// DiscoverCapabilities resolves the expected accepted HEAD, reads the strict
// proposed definition, and invokes the evaluator's describe adapter once. It
// neither resolves policy nor acquires a writer capability.
func (s *Service) DiscoverCapabilities(ctx context.Context, identity Identity) DiscoverCapabilitiesResult {
	if err := identity.validate(); err != nil {
		return DiscoverCapabilitiesResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	revision, err := resolveAcceptedHead(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return DiscoverCapabilitiesResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return DiscoverCapabilitiesResult{Outcome: operationalOutcome("accepted-head-invalid", err)}
	}
	experimentPath, err := proposedExperimentPath(identity)
	if err != nil {
		return DiscoverCapabilitiesResult{Outcome: operationalOutcome("proposal-location-invalid", err)}
	}
	definitionBytes, err := fs.ReadFile(os.DirFS(identity.CheckoutRoot), path.Join(experimentPath, "experiment.yaml"))
	if err != nil {
		return DiscoverCapabilitiesResult{Outcome: operationalOutcome("definition-unreadable", err)}
	}
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		return DiscoverCapabilitiesResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	discovery, err := s.capabilities.DiscoverCapabilities(ctx, CapabilityRequest{
		CheckoutRoot: identity.CheckoutRoot,
		Definition:   cloneDefinition(definition),
	})
	if err != nil {
		return DiscoverCapabilitiesResult{Outcome: operationalOutcome("capability-discovery-failed", err)}
	}
	capabilitiesBytes := append([]byte(nil), discovery.Bytes...)
	capabilities, err := experiment.DecodeCapabilities(capabilitiesBytes)
	if err != nil {
		return DiscoverCapabilitiesResult{Outcome: operationalOutcome("capabilities-invalid", err)}
	}
	canonicalBytes, err := canonjson.Marshal(capabilities)
	if err != nil {
		return DiscoverCapabilitiesResult{Outcome: operationalOutcome("capabilities-invalid", err)}
	}
	if !bytes.Equal(capabilitiesBytes, canonicalBytes) {
		return DiscoverCapabilitiesResult{Outcome: operationalOutcome("capabilities-noncanonical", errors.New("discovered capabilities are not canonically encoded"))}
	}
	capabilitiesDigest := rawDigest(capabilitiesBytes)
	if capabilitiesDigest != definition.Evaluator.CapabilitiesDigest {
		return DiscoverCapabilitiesResult{Outcome: operationalOutcome("capabilities-digest-mismatch", fmt.Errorf("discovered capabilities digest %q does not match definition %q", capabilitiesDigest, definition.Evaluator.CapabilitiesDigest))}
	}
	return DiscoverCapabilitiesResult{
		Outcome: cleanOutcome(), AcceptedHead: revision.Head, ExperimentPath: experimentPath,
		Capabilities: cloneCapabilities(capabilities), CapabilitiesBytes: append([]byte(nil), capabilitiesBytes...),
		CapabilitiesDigest: capabilitiesDigest,
	}
}
