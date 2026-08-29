package draftmutation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

type ActorKind string

const (
	ActorDelegatedAgent ActorKind = "delegated-agent"
	ActorHuman          ActorKind = "human"
)

// Actor can only be minted through adapter-controlled constructors. Its
// fields are private so a request decoder cannot populate or promote it.
type Actor struct {
	kind        ActorKind
	attribution governanceprincipal.Attribution
	harness     string
	session     string
	seal        string
}

type actorProjection struct {
	Kind        ActorKind                       `json:"kind"`
	Attribution governanceprincipal.Attribution `json:"attribution"`
	Harness     string                          `json:"harness,omitempty"`
	Session     string                          `json:"session,omitempty"`
}

func sealActor(actor *Actor) error {
	seal, err := canonjson.Digest(actorProjection{actor.kind, actor.attribution, actor.harness, actor.session})
	if err != nil {
		return err
	}
	actor.seal = seal
	return nil
}

func (a Actor) validate() error {
	if a.seal == "" {
		return fmt.Errorf("draftmutation: actor was not produced by an adapter-controlled constructor")
	}
	want, err := canonjson.Digest(actorProjection{a.kind, a.attribution, a.harness, a.session})
	if err != nil || want != a.seal {
		return fmt.Errorf("draftmutation: actor was modified after construction")
	}
	if err := a.attribution.Validate(); err != nil {
		return err
	}
	switch a.kind {
	case ActorDelegatedAgent:
		if !a.attribution.Unauthenticated || a.attribution.PrincipalID != "" || strings.TrimSpace(a.harness) == "" || !utf8.ValidString(a.harness) || a.session != "" && (strings.TrimSpace(a.session) == "" || !utf8.ValidString(a.session)) {
			return fmt.Errorf("draftmutation: delegated agent requires unauthenticated attribution and nonblank harness")
		}
	case ActorHuman:
		if a.harness != "" || a.session != "" {
			return fmt.Errorf("draftmutation: resolved human attribution requires no harness/session")
		}
	default:
		return fmt.Errorf("draftmutation: unknown actor kind %q", a.kind)
	}
	return nil
}

func NewDelegatedAgent(harness, session string) (Actor, error) {
	if strings.TrimSpace(harness) == "" {
		return Actor{}, fmt.Errorf("draftmutation: delegated-agent harness must be nonblank")
	}
	if !utf8.ValidString(harness) || session != "" && (strings.TrimSpace(session) == "" || !utf8.ValidString(session)) {
		return Actor{}, fmt.Errorf("draftmutation: delegated-agent harness/session must be valid UTF-8 and nonblank when present")
	}
	actor := Actor{kind: ActorDelegatedAgent, attribution: governanceprincipal.NewUnauthenticatedAttribution(), harness: harness, session: session}
	if err := sealActor(&actor); err != nil {
		return Actor{}, err
	}
	return actor, nil
}

func NewTrustedHuman(resolution governanceprincipal.PrincipalResolution) (Actor, error) {
	attribution, err := governanceprincipal.AttributionFromResolution(resolution)
	if err != nil {
		return Actor{}, err
	}
	actor := Actor{kind: ActorHuman, attribution: attribution}
	if err := sealActor(&actor); err != nil {
		return Actor{}, err
	}
	return actor, nil
}

func (a Actor) Kind() ActorKind                              { return a.kind }
func (a Actor) Attribution() governanceprincipal.Attribution { return a.attribution }
func (a Actor) Harness() string                              { return a.harness }
func (a Actor) Session() string                              { return a.session }

// PolicySource is the sole policy-authority port. Production resolves the
// existing constitution store; tests can supply sealed effective policies.
type PolicySource interface {
	ResolveEffectivePolicy(ctx context.Context, root string) (*policyauthority.EffectivePolicy, error)
}

type ConstitutionPolicySource struct{}

func (ConstitutionPolicySource) ResolveEffectivePolicy(_ context.Context, root string) (*policyauthority.EffectivePolicy, error) {
	store, err := policyauthority.Load(root)
	if err != nil {
		return nil, err
	}
	return policyauthority.Resolve(store)
}

type PolicyGrant struct {
	Mode     string
	Digest   string
	PolicyID string
}

// ResolvePolicyGrant resolves the sealed effective-policy digest and the
// one typed design_assistance payload, with no actor-conditional
// authorization decision at all — the shared prologue AuthorizePolicy
// itself now composes. Wave 6 Task 1 exports this narrow slice (a
// behavior-preserving split of AuthorizePolicy's own former body, proven
// necessary because get_design_capabilities (AC-3) must honestly report
// the design_assistance mode/digest for actor kinds that AuthorizePolicy
// would otherwise refuse before ever returning it, e.g. mode "off"/
// "proposal-only" for a delegated agent) so a capability-discovery reader
// never re-derives this exact policy lookup a second time.
func ResolvePolicyGrant(ctx context.Context, root string, identity Identity, source PolicySource) (PolicyGrant, *Error) {
	if source == nil {
		return PolicyGrant{}, NewError(CodeAuthorityInvalid, identity, "policy source is nil")
	}
	effective, err := source.ResolveEffectivePolicy(ctx, root)
	if err != nil {
		if errors.Is(err, policyauthority.ErrNotAdopted) {
			return PolicyGrant{}, WrapError(CodePolicyForbidden, identity, "project has not adopted policy authority", err)
		}
		return PolicyGrant{}, WrapError(CodeAuthorityInvalid, identity, "resolving effective policy", err)
	}
	digest, err := effective.Digest()
	if err != nil {
		return PolicyGrant{}, WrapError(CodeAuthorityInvalid, identity, "effective policy is not sealed resolver output", err)
	}
	var payload *policyartifact.DesignAssistancePayload
	var policyID string
	for _, entry := range effective.Policies {
		raw, ok := entry.Payloads[policyartifact.DesignAssistancePayloadKind]
		if !ok {
			continue
		}
		if payload != nil {
			return PolicyGrant{}, NewError(CodeAuthorityInvalid, identity, "effective policy contains duplicate design_assistance payloads")
		}
		var typed bool
		payload, typed = raw.(*policyartifact.DesignAssistancePayload)
		if !typed || payload == nil {
			return PolicyGrant{}, NewError(CodeAuthorityInvalid, identity, "design_assistance payload is not the registered typed payload")
		}
		policyID = entry.PolicyID
	}
	if payload == nil {
		return PolicyGrant{}, NewError(CodePolicyForbidden, identity, "effective policy has no design_assistance authority")
	}
	if err := payload.Validate(); err != nil {
		return PolicyGrant{}, WrapError(CodeAuthorityInvalid, identity, "invalid design_assistance payload", err)
	}
	return PolicyGrant{Mode: payload.Mode, Digest: digest, PolicyID: policyID}, nil
}

// AuthorizePolicy consumes only the sealed effective-policy digest and the
// one typed design_assistance payload. It never interprets a second
// hierarchy.
func AuthorizePolicy(ctx context.Context, root string, identity Identity, actor Actor, source PolicySource) (PolicyGrant, *Error) {
	if err := actor.validate(); err != nil {
		return PolicyGrant{}, WrapError(CodeActorForbidden, identity, "actor is not adapter-controlled", err)
	}
	grant, typed := ResolvePolicyGrant(ctx, root, identity, source)
	if typed != nil {
		return PolicyGrant{}, typed
	}
	if actor.kind == ActorHuman && actor.attribution.PrincipalID != "" {
		return grant, nil
	}
	if grant.Mode != "draft-write" {
		return PolicyGrant{}, NewError(CodePolicyForbidden, identity, fmt.Sprintf("policy %s design_assistance mode %q forbids delegated-agent writes", grant.PolicyID, grant.Mode))
	}
	return grant, nil
}
