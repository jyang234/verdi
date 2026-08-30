package designapp

import (
	"context"
	"errors"
	"os"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// GetDesignCapabilitiesRequest names the one spec whose capability posture
// to describe.
type GetDesignCapabilitiesRequest struct {
	Spec string
}

func (r GetDesignCapabilitiesRequest) validate() error {
	ref, err := artifact.ParseRef(r.Spec)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return errors.New("designapp: get_design_capabilities spec must be an unpinned whole spec ref")
	}
	return nil
}

// DirectMarkdownPosture is CO-1's non-configurable default for a raw
// Markdown edit: "preserve the Git-recoverable change and disclose
// unclassified origin." Reason is the fixed field name, always exactly
// this value in v1 — there is no `direct_markdown.origin: allow|block`
// payload field wired yet (internal/policyartifact.DesignAssistancePayload
// carries only Mode and Layout), so this posture is reported as the
// system's own fixed invariant rather than a per-policy configurable
// value the payload cannot yet express.
type DirectMarkdownPosture struct {
	Origin string `json:"origin"`
}

// ProvenancePosture is CO-3/CO-5's non-configurable invariants: provenance
// is non-authoritative, excluded from normal build/design context by the
// fixed reason code, and every excerpt carries one of the four closed
// classifications.
type ProvenancePosture struct {
	Authoritative          bool     `json:"authoritative"`
	ExcludedFromContext    bool     `json:"excluded_from_context"`
	ExclusionReason        string   `json:"exclusion_reason"`
	ExcerptClassifications []string `json:"excerpt_classifications"`
}

// ReviewPosture is AC-6's non-configurable invariant: a semantic review
// packet is always available before acceptance; there is no policy knob
// that disables it.
type ReviewPosture struct {
	SemanticPacketAvailable bool `json:"semantic_packet_available"`
}

// The closed vocabulary of mutability preconditions
// get_design_capabilities can name as unmet. They are exactly the three
// facts the mutation kernel itself enforces before any write, in the same
// order Service.Mutate applies them: draftmutation.AuthorizeState checks
// the design/<spec-name> branch and then the Git-derived proposal state,
// and draftmutation.AuthorizePolicy then checks the design_assistance
// mode. When more than one is unmet, the FIRST one in that order is
// named — the same one a real mutate_draft attempt would refuse on.
const (
	// PreconditionDesignBranch: the checkout is not on this spec's own
	// mutable design/<spec-name> branch.
	PreconditionDesignBranch = "design-branch"
	// PreconditionProposalState: the spec's Git-derived state is not
	// `proposed` (CO-3/AC-3: "only draft specs accept semantic
	// mutations"; CO-1: an accepted or review-mode spec is refused).
	PreconditionProposalState = "proposal-state"
	// PreconditionPolicyMode: the resolved design_assistance mode is not
	// draft-write, so a delegated agent may not write at all.
	PreconditionPolicyMode = "policy-mode"
)

// MutabilityRefusal is the machine-readable reason PermittedOperations is
// empty: which single precondition fails, and the human-readable detail.
// It is present exactly when Mutable is false — CO-1's "refuse semantic
// mutation even if an adapter mistakenly advertises it" is enforced by the
// kernel, and this field is how a capability response avoids being the
// adapter that mistakenly advertises it in the first place.
type MutabilityRefusal struct {
	Precondition string `json:"precondition"`
	Detail       string `json:"detail"`
}

// CapabilitiesResult is get_design_capabilities' exact content (AC-3):
// schema versions, checkout/branch/HEAD/spec/current digest, current spec
// state, permitted operations, provenance/direct-Markdown posture, review
// requirements, and the available ASD operation surfaces.
//
// Schema is this envelope's own version (CO-2). PolicyDigest is AC-3's
// "project and policy digests": v1 exposes exactly one digest
// (internal/policyauthority.EffectivePolicy.Digest(), the same value
// draftmutation.PolicyGrant already carries for mutation authorization)
// because no second, distinct "project" digest exists anywhere in the
// accepted policy-authority schema today — reporting a second, fabricated
// digest here would invent authority this package does not own (disclosed
// interpretation, not a silent narrowing).
type CapabilitiesResult struct {
	Schema              string                        `json:"schema"`
	Identity            draftmutation.Identity        `json:"identity"`
	MutationSchema      string                        `json:"mutation_schema"`
	ResultSchema        string                        `json:"result_schema"`
	CurrentDigest       string                        `json:"current_digest"`
	SpecState           specstate.State               `json:"spec_state"`
	PolicyDigest        string                        `json:"policy_digest"`
	PolicyMode          string                        `json:"policy_mode"`
	Mutable             bool                          `json:"mutable"`
	MutabilityRefusal   *MutabilityRefusal            `json:"mutability_refusal,omitempty"`
	PermittedOperations []draftmutation.OperationKind `json:"permitted_operations"`
	Layout              bool                          `json:"layout"`
	DirectMarkdown      DirectMarkdownPosture         `json:"direct_markdown"`
	Provenance          ProvenancePosture             `json:"provenance"`
	Review              ReviewPosture                 `json:"review"`
	AvailableOperations []string                      `json:"available_operations"`
}

// draftWriteOperations is AC-1's closed operation vocabulary, reported in
// full as PermittedOperations exactly when this spec is genuinely mutable
// (see resolveMutability) — draftmutation.Apply enforces the same closed
// set independently, so this list can never grant more than the mutation
// core itself would accept.
var draftWriteOperations = []draftmutation.OperationKind{
	draftmutation.OpSetProblem, draftmutation.OpSetOutcome,
	draftmutation.OpAddAC, draftmutation.OpEditAC, draftmutation.OpRemoveAC, draftmutation.OpReorderAC, draftmutation.OpSetACEvidence,
	draftmutation.OpAddConstraint, draftmutation.OpEditConstraint, draftmutation.OpRemoveConstraint,
	draftmutation.OpAddDecision, draftmutation.OpEditDecision, draftmutation.OpRemoveDecision,
	draftmutation.OpAddQuestion, draftmutation.OpEditQuestion, draftmutation.OpRemoveQuestion,
	draftmutation.OpAddLink, draftmutation.OpRemoveLink,
	draftmutation.OpAddStub, draftmutation.OpEditStub, draftmutation.OpRemoveStub, draftmutation.OpReorderStub,
	draftmutation.OpAddContextRef, draftmutation.OpRemoveContextRef,
}

// availableASDOperations is AC-8's exact six-operation surface, reported
// verbatim for a genuinely mutable spec so a caller can discover the whole
// ASD contract from one read.
var availableASDOperations = []string{
	"get_board", "get_design_context", "get_design_capabilities",
	"mutate_draft", "get_design_provenance", "prepare_design_review",
}

// readOnlyASDOperations is the same surface with mutate_draft withheld —
// what a non-mutable spec reports. CO-1's last failure-behavior bullet
// ("accepted or review-mode spec: refuse semantic mutation even if an
// adapter mistakenly advertises it") makes the kernel the backstop for an
// adapter that advertises a write it cannot honor; this list is how
// get_design_capabilities avoids being that adapter, rather than relying
// on the backstop.
var readOnlyASDOperations = []string{
	"get_board", "get_design_context", "get_design_capabilities",
	"get_design_provenance", "prepare_design_review",
}

// resolveMutability applies the same three facts, in the same order, that
// draftmutation.AuthorizeState and draftmutation.AuthorizePolicy apply
// inside Service.Mutate — over the values this operation has ALREADY
// resolved through the very same seams (identity from
// draftmutation.ResolveCanonicalIdentity, state from the shared
// draftmutation.StateProjector port, mode from
// draftmutation.ResolvePolicyGrant). It is a parallel of those checks over
// shared inputs, never a second authorization algorithm: it computes no
// Git or policy fact of its own, and grants nothing — the kernel still
// re-decides every one of them at write time.
//
// The delegated-agent posture is the one reported (SI-163: "MCP actor
// stays delegated-agent"); AuthorizePolicy's authenticated-human bypass is
// a workbench/CLI-authenticated-caller concern outside this read-only
// discovery response, exactly as GetDesignCapabilities' own doc comment
// records.
func resolveMutability(identity draftmutation.Identity, specName string, state specstate.State, mode string) *MutabilityRefusal {
	wantBranch := "design/" + specName
	switch {
	case identity.Branch != wantBranch:
		return &MutabilityRefusal{
			Precondition: PreconditionDesignBranch,
			Detail:       "branch " + identity.Branch + " is not mutable design branch " + wantBranch,
		}
	case state != specstate.Proposed:
		return &MutabilityRefusal{
			Precondition: PreconditionProposalState,
			Detail:       "Git-derived state " + string(state) + " is not mutable proposal state",
		}
	case mode != "draft-write":
		return &MutabilityRefusal{
			Precondition: PreconditionPolicyMode,
			// vocab:identity — "draft-write" is the accepted ASD spec's literal enum value
			Detail: "design_assistance mode " + mode + " forbids delegated-agent writes",
		}
	}
	return nil
}

// GetDesignCapabilities declares the active schema, checkout/branch/HEAD/
// spec identity, policy digest, permitted operations, and fixed
// provenance/review/direct-Markdown posture (AC-3). It reports the
// PermittedOperations set a delegated (MCP) agent would receive — the
// posture MCP's mutate_draft actually enforces (SI-163: "MCP actor stays
// delegated-agent"); an authenticated human actor's own broader posture
// (AuthorizePolicy's human bypass) is a workbench/CLI-authenticated-caller
// concern outside this read-only discovery response.
//
// The write vocabulary is reported only when all three of the kernel's own
// mutability preconditions hold — the design/<spec-name> branch, the
// Git-derived `proposed` state, and design_assistance mode draft-write
// (resolveMutability). Otherwise PermittedOperations is empty,
// AvailableOperations withholds mutate_draft, and MutabilityRefusal names
// the exact failing precondition: AC-3's "only draft specs accept semantic
// mutations" is a fact this response must REFLECT, not a claim it may
// overstate and leave the kernel to walk back (CO-1).
func (s Service) GetDesignCapabilities(ctx context.Context, start string, req GetDesignCapabilitiesRequest) (*CapabilitiesResult, *Error) {
	if err := req.validate(); err != nil {
		return nil, inputInvalid("input-invalid", err.Error())
	}
	identity, typed := s.resolveIdentity(ctx, start, req.Spec)
	if typed != nil {
		return nil, typed
	}
	ref, err := artifact.ParseRef(identity.Spec)
	if err != nil {
		return nil, operational("authority-invalid", "parsing canonical spec identity", err)
	}
	specPath := store.SpecPath(identity.Checkout, store.ZoneActive, ref.Name)
	current, err := os.ReadFile(specPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, notFound("spec-not-found", "no such active spec: "+identity.Spec)
	}
	if err != nil {
		return nil, operational("io-failure", "reading current spec", err)
	}

	if s.State == nil {
		return nil, operational("state-projector-unavailable", "state projector is not configured", nil)
	}
	candidate := specstate.Candidate{Path: store.SpecRelPath(store.ZoneActive, ref.Name), Content: current}
	state, err := s.State.ResolveState(ctx, identity.Checkout, candidate)
	if err != nil {
		return nil, operational("authority-invalid", "projecting Git-derived spec state", err)
	}

	if s.Policy == nil {
		return nil, operational("policy-source-unavailable", "policy source is not configured", nil)
	}
	grant, policyErr := draftmutation.ResolvePolicyGrant(ctx, identity.Checkout, identity, s.Policy)
	if policyErr != nil {
		return nil, translateDraftmutationError(policyErr)
	}

	refusal := resolveMutability(identity, ref.Name, state.State, grant.Mode)
	permitted := []draftmutation.OperationKind{}
	available := readOnlyASDOperations
	if refusal == nil {
		permitted = append(permitted, draftWriteOperations...)
		available = availableASDOperations
	}

	return &CapabilitiesResult{
		Schema:              CapabilitiesResultSchema,
		Identity:            identity,
		MutationSchema:      draftmutation.RequestSchema,
		ResultSchema:        draftmutation.ResultSchema,
		CurrentDigest:       draftmutation.DigestBytes(current),
		SpecState:           state.State,
		PolicyDigest:        grant.Digest,
		PolicyMode:          grant.Mode,
		Mutable:             refusal == nil,
		MutabilityRefusal:   refusal,
		PermittedOperations: permitted,
		Layout:              grant.Layout,
		DirectMarkdown:      DirectMarkdownPosture{Origin: "disclose"},
		Provenance: ProvenancePosture{
			Authoritative:       false,
			ExcludedFromContext: true,
			ExclusionReason:     "design-provenance-sidecar",
			ExcerptClassifications: []string{
				string(designprovenance.ClassificationHumanStated),
				string(designprovenance.ClassificationAISynthesized),
				string(designprovenance.ClassificationAIInferred),
				string(designprovenance.ClassificationUnresolved),
			},
		},
		Review:              ReviewPosture{SemanticPacketAvailable: true},
		AvailableOperations: append([]string(nil), available...),
	}, nil
}
