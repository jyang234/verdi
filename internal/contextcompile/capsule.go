package contextcompile

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/specstate"
)

// BoundObligation is one exact obligation payload bound to a target
// acceptance-criterion/evidence-kind pair.
type BoundObligation struct {
	Ref           string
	Path          string
	TargetRef     string
	AC            string
	Kind          artifact.EvidenceKind
	ContentDigest string
	Content       []byte
}

// PolicyOperand is one applicable constitution policy, overlay or exemption
// operand included in a phase capsule.
type PolicyOperand struct {
	Kind   string
	ID     string
	Path   string
	Digest string
	Scope  policyartifact.Scope
}

// CapsuleInput contains every already-resolved semantic input the pure phase
// composer may select. It contains no report or receipt placeholder.
type CapsuleInput struct {
	Target                ResolvedSpec
	ParentFragments       []FeatureFragment
	Obligations           []BoundObligation
	EffectivePolicyDigest string
	PolicyOperands        []PolicyOperand
	Grants                execworkspace.GrantSet
	DeclaredContext       []Candidate
	RepositoryCandidates  []Candidate
	Projection            []Candidate
	Opaque                Candidate
}

// Capsule is the exact phase-specific semantic input set. Design/build carry
// explicit empty RequiredInputs and Disclosures; review carries its closed
// five-row proof posture and three fixed disclosures.
type Capsule struct {
	Phase                 Phase
	Target                ResolvedSpec
	ParentFragments       []FeatureFragment
	Obligations           []BoundObligation
	EffectivePolicyDigest string
	PolicyOperands        []PolicyOperand
	Grants                execworkspace.GrantSet
	DeclaredContext       []Candidate
	RepositoryCandidates  []Candidate
	Projection            []Candidate
	Opaque                Candidate
	RequiredInputs        []RequiredInput
	Disclosures           []DisclosureCode
}

// ComposeCapsule validates and copies already-resolved semantic inputs into
// the requested pure design, build or review capsule.
func ComposeCapsule(phase Phase, in CapsuleInput) (Capsule, error) {
	if err := phase.Validate(); err != nil {
		return Capsule{}, err
	}
	if err := validateCapsuleTarget(phase, in.Target); err != nil {
		return Capsule{}, err
	}
	if err := validateParentFragments(in.Target, in.ParentFragments); err != nil {
		return Capsule{}, err
	}
	if err := validateBoundObligations(in.Target, in.Obligations); err != nil {
		return Capsule{}, err
	}
	if err := validatePolicyOperands(in.EffectivePolicyDigest, in.PolicyOperands); err != nil {
		return Capsule{}, err
	}
	if err := in.Grants.Validate(); err != nil {
		return Capsule{}, fmt.Errorf("contextcompile: capsule grants: %w", err)
	}
	if err := validateCapsuleCandidates("declared context", SourceDeclaredContext, in.DeclaredContext); err != nil {
		return Capsule{}, err
	}
	if err := validateCapsuleCandidates("repository", SourceHeadTree, in.RepositoryCandidates); err != nil {
		return Capsule{}, err
	}
	if err := validateCapsuleCandidates("projection", SourceProjection, in.Projection); err != nil {
		return Capsule{}, err
	}
	if err := validateCapsuleCandidate("opaque", SourceOpaque, in.Opaque); err != nil {
		return Capsule{}, err
	}

	capsule := Capsule{
		Phase:                 phase,
		Target:                cloneResolvedSpec(in.Target),
		ParentFragments:       cloneFeatureFragments(in.ParentFragments),
		Obligations:           cloneBoundObligations(in.Obligations),
		EffectivePolicyDigest: in.EffectivePolicyDigest,
		PolicyOperands:        clonePolicyOperands(in.PolicyOperands),
		Grants:                cloneGrantSet(in.Grants),
		DeclaredContext:       append([]Candidate{}, in.DeclaredContext...),
		RepositoryCandidates:  append([]Candidate{}, in.RepositoryCandidates...),
		Projection:            append([]Candidate{}, in.Projection...),
		Opaque:                in.Opaque,
		RequiredInputs:        []RequiredInput{},
		Disclosures:           []DisclosureCode{},
	}
	if phase == PhaseReview {
		capsule.RequiredInputs, capsule.Disclosures = reviewRequiredInputs(in.Target.ContentDigest, in.EffectivePolicyDigest)
	}
	return capsule, nil
}

func validateCapsuleTarget(phase Phase, target ResolvedSpec) error {
	if target.Spec == nil {
		return fmt.Errorf("contextcompile: capsule target has no decoded specification")
	}
	if phase == PhaseBuild && target.Spec.Class == artifact.ClassFeature {
		// vocab:identity — SI-84 build-capsule refusal naming the fixed feature-class identity
		return &DeclaredScopeRefusal{Phase: phase, Ref: target.Ref, Reason: "feature specifications are not authoritative build targets"}
	}
	if target.State != specstate.AcceptedPendingBuild && target.State != specstate.Closed {
		return &AcceptedSpecRefusal{Ref: target.Ref, State: target.State, Relation: specstate.RelationUnproven}
	}
	if target.Ref == "" || target.Spec.ID != target.Ref {
		return fmt.Errorf("contextcompile: capsule target ref does not match decoded specification")
	}
	if err := validateSpecWholeRef("capsule target ref", target.Ref); err != nil {
		return err
	}
	parsed, _ := artifact.ParseRef(target.Ref)
	wantActive := ".verdi/specs/active/" + parsed.Name + "/spec.md"
	wantArchive := ".verdi/specs/archive/" + parsed.Name + "/spec.md"
	if target.Path != wantActive && target.Path != wantArchive {
		return fmt.Errorf("contextcompile: capsule target path %q does not match ref %q", target.Path, target.Ref)
	}
	if (target.State == specstate.AcceptedPendingBuild && target.Path != wantActive) ||
		(target.State == specstate.Closed && target.Path != wantArchive) {
		return fmt.Errorf("contextcompile: capsule target state %s is incompatible with path %s", target.State, target.Path)
	}
	if err := target.Spec.Validate(); err != nil {
		return fmt.Errorf("contextcompile: capsule target %s: %w", target.Ref, err)
	}
	if target.Spec.Class != artifact.ClassFeature && target.Spec.Class != artifact.ClassStory {
		return fmt.Errorf("contextcompile: capsule target %s has unsupported class %q", target.Ref, target.Spec.Class)
	}
	if err := validateGitHash("capsule target blob", target.Blob); err != nil {
		return err
	}
	if err := validateGitHash("capsule target commit", target.Commit); err != nil {
		return err
	}
	if err := validateDigest("capsule target content digest", target.ContentDigest); err != nil {
		return err
	}
	if rawContentDigest(target.Content) != target.ContentDigest {
		return fmt.Errorf("contextcompile: capsule target content digest does not bind exact accepted bytes")
	}
	return nil
}

func validateParentFragments(target ResolvedSpec, fragments []FeatureFragment) error {
	if fragments == nil {
		return fmt.Errorf("contextcompile: capsule parent fragments must be an explicit array")
	}
	if err := requireSortedUnique("capsule parent fragments", fragments, func(fragment FeatureFragment) string {
		return fragment.Feature.Ref
	}); err != nil {
		return err
	}

	expected := make(map[string]bool)
	if target.Spec.Class == artifact.ClassStory {
		typeWant := artifact.LinkImplements
		if target.Spec.Spike {
			typeWant = artifact.LinkResolves
		}
		for _, link := range target.Spec.Links {
			if link.Type == typeWant {
				if expected[link.Ref] {
					return fmt.Errorf("contextcompile: capsule target duplicates governing edge %s", link.Ref)
				}
				expected[link.Ref] = true
			}
		}
	}
	seen := make(map[string]bool, len(expected))
	for i, fragment := range fragments {
		if err := fragment.validate(); err != nil {
			return fmt.Errorf("contextcompile: capsule parent_fragments[%d]: %w", i, err)
		}
		for _, target := range fragment.Targets {
			ref := fragment.Feature.Ref + "#" + target.ID
			if !expected[ref] {
				return fmt.Errorf("contextcompile: capsule parent fragment carries undeclared edge %s", ref)
			}
			if seen[ref] {
				return fmt.Errorf("contextcompile: capsule parent edge %s is duplicated", ref)
			}
			seen[ref] = true
		}
	}
	if len(seen) != len(expected) {
		var missing []string
		for ref := range expected {
			if !seen[ref] {
				missing = append(missing, ref)
			}
		}
		sort.Strings(missing)
		return fmt.Errorf("contextcompile: capsule is missing parent fragments %v", missing)
	}
	return nil
}

func validateBoundObligations(target ResolvedSpec, obligations []BoundObligation) error {
	if obligations == nil {
		return fmt.Errorf("contextcompile: capsule obligations must be an explicit array")
	}
	if err := requireSortedUnique("capsule obligations", obligations, func(obligation BoundObligation) string {
		return obligation.Ref
	}); err != nil {
		return err
	}
	declaredPairs := make(map[string]bool)
	targetRef, _ := artifact.ParseRef(target.Ref)
	for _, criterion := range target.Spec.AcceptanceCriteria {
		for _, kind := range criterion.Evidence {
			declaredPairs[criterion.ID+"\x00"+string(kind)] = true
		}
	}
	seenPairs := make(map[string]bool, len(obligations))
	for i, obligation := range obligations {
		if err := validateObligationRef(fmt.Sprintf("capsule obligations[%d].ref", i), obligation.Ref); err != nil {
			return err
		}
		if obligation.Path == "" || obligation.TargetRef != target.Ref {
			return fmt.Errorf("contextcompile: capsule obligations[%d] is not bound to target %s", i, target.Ref)
		}
		if err := validateACID(fmt.Sprintf("capsule obligations[%d].ac", i), obligation.AC); err != nil {
			return err
		}
		if err := validateEvidenceKind(fmt.Sprintf("capsule obligations[%d].kind", i), obligation.Kind); err != nil {
			return err
		}
		wantRef := fmt.Sprintf("obligation/%s--%s--%s", targetRef.Name, obligation.AC, obligation.Kind)
		if obligation.Ref != wantRef {
			return fmt.Errorf("contextcompile: capsule obligations[%d].ref %q does not bind exact target pair (want %q)", i, obligation.Ref, wantRef)
		}
		pair := obligation.AC + "\x00" + string(obligation.Kind)
		if !declaredPairs[pair] {
			return fmt.Errorf("contextcompile: capsule obligations[%d] names undeclared target pair %s/%s", i, obligation.AC, obligation.Kind)
		}
		if seenPairs[pair] {
			return fmt.Errorf("contextcompile: capsule obligation pair %s/%s is duplicated", obligation.AC, obligation.Kind)
		}
		seenPairs[pair] = true
		if err := validateDigest(fmt.Sprintf("capsule obligations[%d].content_digest", i), obligation.ContentDigest); err != nil {
			return err
		}
		if rawContentDigest(obligation.Content) != obligation.ContentDigest {
			return fmt.Errorf("contextcompile: capsule obligations[%d] digest does not bind exact bytes", i)
		}
	}
	return nil
}

func validatePolicyOperands(effectiveDigest string, operands []PolicyOperand) error {
	if err := validateDigest("capsule effective policy digest", effectiveDigest); err != nil {
		return err
	}
	if operands == nil {
		return fmt.Errorf("contextcompile: capsule policy operands must be an explicit array")
	}
	if err := requireSortedUnique("capsule policy operands", operands, func(operand PolicyOperand) string {
		return operand.Kind + "\x00" + operand.ID
	}); err != nil {
		return err
	}
	for i, operand := range operands {
		switch operand.Kind {
		case PolicyEntryPolicy, PolicyEntryOverlay, PolicyEntryExemption:
		default:
			return fmt.Errorf("contextcompile: capsule policy_operands[%d] has unknown kind %q", i, operand.Kind)
		}
		if operand.ID == "" || operand.Path == "" {
			return fmt.Errorf("contextcompile: capsule policy_operands[%d] requires id and path", i)
		}
		if err := validateDigest(fmt.Sprintf("capsule policy_operands[%d].digest", i), operand.Digest); err != nil {
			return err
		}
		if err := validateScope(fmt.Sprintf("capsule policy_operands[%d].scope", i), operand.Scope); err != nil {
			return err
		}
	}
	return nil
}

func validateCapsuleCandidates(field string, source Source, candidates []Candidate) error {
	if candidates == nil {
		return fmt.Errorf("contextcompile: capsule %s candidates must be an explicit array", field)
	}
	if err := requireSortedUnique("capsule "+field, candidates, func(candidate Candidate) string { return candidate.ID }); err != nil {
		return err
	}
	for i, candidate := range candidates {
		if err := validateCapsuleCandidate(fmt.Sprintf("%s[%d]", field, i), source, candidate); err != nil {
			return err
		}
	}
	return nil
}

func validateCapsuleCandidate(field string, source Source, candidate Candidate) error {
	if candidate.Source != source {
		return fmt.Errorf("contextcompile: capsule %s source %q, want %q", field, candidate.Source, source)
	}
	if candidate.ID == "" {
		return fmt.Errorf("contextcompile: capsule %s has empty id", field)
	}
	switch source {
	case SourceDeclaredContext:
		if candidate.Ref == "" || candidate.ID != "ref:"+candidate.Ref || candidate.Path != "" || candidate.Object != "" || candidate.Mode != "" || candidate.Type != "" {
			return fmt.Errorf("contextcompile: capsule %s is not a canonical declared-context candidate", field)
		}
		if _, err := artifact.ParseRef(candidate.Ref); err != nil {
			return fmt.Errorf("contextcompile: capsule %s ref: %w", field, err)
		}
	case SourceHeadTree:
		if candidate.Path == "" || candidate.ID != "path:"+candidate.Path || candidate.Ref != "" || candidate.Object == "" || candidate.Mode == "" || candidate.Type == "" {
			return fmt.Errorf("contextcompile: capsule %s is not a canonical HEAD-tree candidate", field)
		}
		if err := validateCandidatePath(candidate.Path); err != nil {
			return err
		}
	case SourceProjection:
		if candidate.Path == "" || candidate.ID != "path:"+candidate.Path || candidate.Ref != "" || candidate.Object != "" || candidate.Mode != "" || candidate.Type != "" {
			return fmt.Errorf("contextcompile: capsule %s is not a canonical projection candidate", field)
		}
		if err := validateCandidatePath(candidate.Path); err != nil {
			return err
		}
	case SourceOpaque:
		if !strings.HasPrefix(candidate.ID, "opaque:"+OpaqueKindHarnessVendorBase+"/") || candidate.Path != "" || candidate.Ref != "" || candidate.Object != "" || candidate.Mode != "" || candidate.Type != "" {
			return fmt.Errorf("contextcompile: capsule %s is not the fixed opaque base candidate", field)
		}
	default:
		return fmt.Errorf("contextcompile: capsule %s unsupported source %q", field, source)
	}
	return nil
}

func reviewRequiredInputs(acceptedDigest, policyDigest string) ([]RequiredInput, []DisclosureCode) {
	accepted := acceptedDigest
	policy := policyDigest
	inputs := []RequiredInput{
		{Kind: RequiredInputAcceptedSpec, Resolution: ResolutionProven, Digest: &accepted, Witnesses: []string{}},
		{Kind: RequiredInputBuilderReceipt, Resolution: ResolutionUnproven, Witnesses: []string{string(DisclosureReviewBuilderReceiptUnproven)}},
		{Kind: RequiredInputEvidenceBundle, Resolution: ResolutionUnproven, Witnesses: []string{string(DisclosureReviewEvidenceBundleUnproven)}},
		{Kind: RequiredInputResultDiff, Resolution: ResolutionUnproven, Witnesses: []string{string(DisclosureReviewResultDiffUnproven)}},
		{Kind: RequiredInputReviewPolicy, Resolution: ResolutionProven, Digest: &policy, Witnesses: []string{}},
	}
	disclosures := []DisclosureCode{
		DisclosureReviewBuilderReceiptUnproven,
		DisclosureReviewEvidenceBundleUnproven,
		DisclosureReviewResultDiffUnproven,
	}
	return inputs, disclosures
}

func cloneResolvedSpec(in ResolvedSpec) ResolvedSpec {
	out := in
	out.Content = append([]byte(nil), in.Content...)
	return out
}

func cloneFeatureFragments(in []FeatureFragment) []FeatureFragment {
	out := make([]FeatureFragment, len(in))
	for i, fragment := range in {
		out[i] = fragment
		out[i].Targets = make([]FragmentTarget, len(fragment.Targets))
		for j, target := range fragment.Targets {
			out[i].Targets[j] = target
			out[i].Targets[j].Evidence = append([]artifact.EvidenceKind(nil), target.Evidence...)
		}
		out[i].Constraints = cloneConstraints(fragment.Constraints)
		out[i].Decisions = cloneDecisions(fragment.Decisions)
	}
	return out
}

func cloneBoundObligations(in []BoundObligation) []BoundObligation {
	out := make([]BoundObligation, len(in))
	for i, obligation := range in {
		out[i] = obligation
		out[i].Content = append([]byte(nil), obligation.Content...)
	}
	return out
}

func clonePolicyOperands(in []PolicyOperand) []PolicyOperand {
	out := make([]PolicyOperand, len(in))
	for i, operand := range in {
		out[i] = operand
		out[i].Scope = policyartifact.Scope{
			Phases: cloneStrings(operand.Scope.Phases), Environments: cloneStrings(operand.Scope.Environments),
			Paths: cloneStrings(operand.Scope.Paths), Refs: cloneStrings(operand.Scope.Refs),
		}
	}
	return out
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string{}, in...)
}

func cloneGrantSet(in execworkspace.GrantSet) execworkspace.GrantSet {
	if in.Grants == nil {
		return execworkspace.GrantSet{}
	}
	out := execworkspace.GrantSet{Grants: make([]execworkspace.Grant, len(in.Grants))}
	for i, grant := range in.Grants {
		out.Grants[i] = grant
		out.Grants[i].Paths = append([]string(nil), grant.Paths...)
		out.Grants[i].Argv0s = append([]string(nil), grant.Argv0s...)
		if grant.Ceilings != nil {
			out.Grants[i].Ceilings = make(map[string]int, len(grant.Ceilings))
			for name, ceiling := range grant.Ceilings {
				out.Grants[i].Ceilings[name] = ceiling
			}
		}
	}
	return out
}
