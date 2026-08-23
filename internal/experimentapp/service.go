// Package experimentapp composes the closed CSE schemas, policy decision,
// accepted Git facts, capability discovery, and result verification ports.
package experimentapp

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentpolicy"
)

// Classification is the exact application-level 0/1/2 outcome vocabulary.
type Classification string

const (
	ClassificationClean       Classification = "clean"
	ClassificationVerdict     Classification = "verdict"
	ClassificationOperational Classification = "operational"
)

// Outcome is the shared typed operation result envelope.
type Outcome struct {
	Classification Classification `json:"classification"`
	Code           string         `json:"code"`
	Detail         string         `json:"detail"`
}

// ExitCode returns the fixed CLI projection. Unknown internal values fail
// closed as operational rather than becoming a clean exit.
func (o Outcome) ExitCode() int {
	switch o.Classification {
	case ClassificationClean:
		return 0
	case ClassificationVerdict:
		return 1
	default:
		return 2
	}
}

func cleanOutcome() Outcome {
	return Outcome{Classification: ClassificationClean, Code: "clean", Detail: "operation completed"}
}

func verdictOutcome(code, detail string) Outcome {
	return Outcome{Classification: ClassificationVerdict, Code: code, Detail: detail}
}

func operationalOutcome(code string, err error) Outcome {
	return Outcome{Classification: ClassificationOperational, Code: code, Detail: err.Error()}
}

// Identity is the shared operation envelope. Actor and accepted HEAD are
// adapter-controlled operands, never decoded authority strings.
type Identity struct {
	CheckoutRoot         string
	Spike                string
	ExperimentID         string
	ExpectedAcceptedHEAD string
	Actor                Actor
}

var spikePattern = regexp.MustCompile(`^spec/[a-z0-9]+(-[a-z0-9]+)*$`)

func (i Identity) validate() error {
	if strings.TrimSpace(i.CheckoutRoot) == "" {
		return fmt.Errorf("experimentapp: checkout root must be nonblank")
	}
	if !spikePattern.MatchString(i.Spike) {
		return fmt.Errorf("experimentapp: spike %q does not match ^spec/<id>$", i.Spike)
	}
	if err := experiment.ValidateID(i.ExperimentID); err != nil {
		return fmt.Errorf("experimentapp: experiment id: %w", err)
	}
	if err := experiment.ValidateCommit(i.ExpectedAcceptedHEAD); err != nil {
		return fmt.Errorf("experimentapp: expected accepted HEAD: %w", err)
	}
	if err := i.Actor.validate(); err != nil {
		return fmt.Errorf("experimentapp: actor: %w", err)
	}
	return nil
}

// PolicyRequest gives the policy port every already-validated operand. The
// service passes a deep copy so a port cannot mutate application custody.
type PolicyRequest struct {
	CheckoutRoot   string
	ExperimentPath string
	Spike          string
	Definition     experiment.Definition
	Capabilities   experiment.Capabilities
	CandidatePaths []string
}

// PolicyResolver obtains the one sealed CSE decision for one operation.
type PolicyResolver interface {
	ResolvePolicy(context.Context, PolicyRequest) (*experimentpolicy.Decision, error)
}

// CapabilityRequest binds one strict describe call to draft identity.
type CapabilityRequest struct {
	CheckoutRoot string
	Definition   experiment.Definition
}

// CapabilityDiscovery carries only the exact response bytes. The application
// strict-decodes them rather than trusting a second parsed representation.
type CapabilityDiscovery struct {
	Bytes []byte
}

// CapabilityDiscoverer is the consumer-owned external describe port.
type CapabilityDiscoverer interface {
	DiscoverCapabilities(context.Context, CapabilityRequest) (CapabilityDiscovery, error)
}

// ResultVerifier is the consumer-owned recompute-equality port used by the
// experiment state owner.
type ResultVerifier interface {
	VerifyResult(experiment.Definition, []experiment.Observation, *experiment.ExecutionReceipt, experiment.Result) error
}

// AcceptedGit is the exact default-branch facts and object-reading port.
type AcceptedGit interface {
	ResolveDefaultBranch(context.Context, string) (DefaultBranch, error)
	ListTree(context.Context, string, string) ([]GitTreeEntry, error)
	ReadBlob(context.Context, string, string, string, string) ([]byte, error)
}

// Service owns read-only application composition. It has no writer port.
type Service struct {
	policy       PolicyResolver
	git          AcceptedGit
	capabilities CapabilityDiscoverer
	results      ResultVerifier
}

// NewService requires all four consumer-owned ports.
func NewService(policy PolicyResolver, git AcceptedGit, capabilities CapabilityDiscoverer, results ResultVerifier) (*Service, error) {
	if policy == nil || git == nil || capabilities == nil || results == nil {
		return nil, fmt.Errorf("experimentapp: policy, accepted Git, capability discovery, and result verification ports are required")
	}
	return &Service{policy: policy, git: git, capabilities: capabilities, results: results}, nil
}

func clonePolicyRequest(in PolicyRequest) PolicyRequest {
	return PolicyRequest{
		CheckoutRoot: in.CheckoutRoot, ExperimentPath: in.ExperimentPath, Spike: in.Spike,
		Definition: cloneDefinition(in.Definition), Capabilities: cloneCapabilities(in.Capabilities),
		CandidatePaths: slices.Clone(in.CandidatePaths),
	}
}

func cloneDefinition(in experiment.Definition) experiment.Definition {
	out := in
	out.Candidates = slices.Clone(in.Candidates)
	out.Evaluator.Argv = slices.Clone(in.Evaluator.Argv)
	out.Fixtures = slices.Clone(in.Fixtures)
	out.ProtectedPaths = slices.Clone(in.ProtectedPaths)
	out.Decision.Guards = slices.Clone(in.Decision.Guards)
	for index := range out.Decision.Guards {
		if in.Decision.Guards[index].MaximumRelativeToBaseline != nil {
			value := *in.Decision.Guards[index].MaximumRelativeToBaseline
			out.Decision.Guards[index].MaximumRelativeToBaseline = &value
		}
	}
	out.Decision.BaselineImprovement = cloneThreshold(in.Decision.BaselineImprovement)
	out.Decision.CandidateSeparation = cloneThreshold(in.Decision.CandidateSeparation)
	if in.Decision.Variability != nil {
		value := *in.Decision.Variability
		out.Decision.Variability = &value
	}
	if in.Reproduction != nil {
		value := *in.Reproduction
		out.Reproduction = &value
	}
	if in.Policy != nil {
		value := *in.Policy
		if in.Policy.Digest != nil {
			digest := *in.Policy.Digest
			value.Digest = &digest
		}
		out.Policy = &value
	}
	if in.Lock != nil {
		value := *in.Lock
		out.Lock = &value
	}
	return out
}

func cloneThreshold(in experiment.Threshold) experiment.Threshold {
	out := in
	if in.Relative != nil {
		value := *in.Relative
		out.Relative = &value
	}
	if in.Absolute != nil {
		value := *in.Absolute
		out.Absolute = &value
	}
	return out
}

func cloneCapabilities(in experiment.Capabilities) experiment.Capabilities {
	out := in
	out.ProtocolVersions = slices.Clone(in.ProtocolVersions)
	out.Metrics = slices.Clone(in.Metrics)
	out.Guards = slices.Clone(in.Guards)
	out.Observers = slices.Clone(in.Observers)
	out.WorkloadInputs = slices.Clone(in.WorkloadInputs)
	out.Environment = slices.Clone(in.Environment)
	return out
}
