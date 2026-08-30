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

// humanBasis distinguishes which adapter-controlled constructor produced an
// ActorHuman value. It is compared only inside this package (isExplicitBrowserHuman)
// and has no public accessor: a NewTrustedHuman actor with a violated or
// unproven resolution and an explicit NewUnauthenticatedHuman actor
// deliberately serialize the identical kernel unauthenticated attribution
// (§4.1) — basis is the one place their otherwise-identical shapes are told
// apart, and it never appears in any attribution, provenance entry, or other
// serialized output.
type humanBasis string

const (
	// humanBasisResolved marks a NewTrustedHuman product, whatever its
	// resolution state (authenticated, violated-with-witness, or
	// unproven). Its existing policy-gated authorization matrix is
	// unchanged by this basis split.
	humanBasisResolved humanBasis = "resolved"
	// humanBasisExplicitUnauthenticated marks a NewUnauthenticatedHuman
	// product: the sealed explicit browser-human actor authorized
	// independently of design_assistance mode/adoption (§4.1, SI-176).
	humanBasisExplicitUnauthenticated humanBasis = "explicit-unauthenticated"
)

// Actor can only be minted through adapter-controlled constructors. Its
// fields are private so a request decoder cannot populate or promote it.
type Actor struct {
	kind        ActorKind
	attribution governanceprincipal.Attribution
	harness     string
	session     string
	basis       humanBasis
	seal        string
}

type actorProjection struct {
	Kind        ActorKind                       `json:"kind"`
	Attribution governanceprincipal.Attribution `json:"attribution"`
	Harness     string                          `json:"harness,omitempty"`
	Session     string                          `json:"session,omitempty"`
	Basis       humanBasis                      `json:"basis,omitempty"`
}

func sealActor(actor *Actor) error {
	seal, err := canonjson.Digest(actorProjection{actor.kind, actor.attribution, actor.harness, actor.session, actor.basis})
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
	want, err := canonjson.Digest(actorProjection{a.kind, a.attribution, a.harness, a.session, a.basis})
	if err != nil || want != a.seal {
		return fmt.Errorf("draftmutation: actor was modified after construction")
	}
	if err := a.attribution.Validate(); err != nil {
		return err
	}
	switch a.kind {
	case ActorDelegatedAgent:
		if a.basis != "" {
			return fmt.Errorf("draftmutation: delegated agent must not carry a human basis")
		}
		if !a.attribution.Unauthenticated || a.attribution.PrincipalID != "" || strings.TrimSpace(a.harness) == "" || !utf8.ValidString(a.harness) || a.session != "" && (strings.TrimSpace(a.session) == "" || !utf8.ValidString(a.session)) {
			return fmt.Errorf("draftmutation: delegated agent requires unauthenticated attribution and nonblank harness")
		}
	case ActorHuman:
		if a.harness != "" || a.session != "" {
			return fmt.Errorf("draftmutation: resolved human attribution requires no harness/session")
		}
		switch a.basis {
		case humanBasisResolved, humanBasisExplicitUnauthenticated:
		default:
			return fmt.Errorf("draftmutation: human actor has unknown basis %q", a.basis)
		}
	default:
		return fmt.Errorf("draftmutation: unknown actor kind %q", a.kind)
	}
	return nil
}

// isExplicitBrowserHuman reports whether a was minted by
// NewUnauthenticatedHuman — the ONLY actors AuthorizeBrowserHuman accepts.
// A NewTrustedHuman actor with a violated or unproven resolution carries the
// identical Kind/Attribution/Harness/Session shape but a different basis, so
// it always answers false here and keeps routing through AuthorizePolicy's
// existing, unchanged authorization matrix.
func (a Actor) isExplicitBrowserHuman() bool {
	return a.kind == ActorHuman && a.basis == humanBasisExplicitUnauthenticated
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
	actor := Actor{kind: ActorHuman, attribution: attribution, basis: humanBasisResolved}
	if err := sealActor(&actor); err != nil {
		return Actor{}, err
	}
	return actor, nil
}

// NewUnauthenticatedHuman returns the ONE explicit browser-human actor
// (Wave 6 design §4.1, SI-163/SI-176): the kernel's unauthenticated
// attribution minted directly by an adapter, never derived from a
// governance-principal resolution attempt. It takes no request-derived
// input at all, so no request decoder, CLI flag, or MCP argument can ever
// construct one — the only route is this literal, argument-free call from
// adapter source code (proven structurally by
// TestNewUnauthenticatedHumanHasNoNonTestProductionCaller). Its private
// sealed basis is distinct from a NewTrustedHuman actor with a violated or
// unproven resolution even though both serialize the identical
// unauthenticated attribution; a failed identity proof never acquires this
// constructor's allowance because nothing routes it here.
func NewUnauthenticatedHuman() (Actor, error) {
	actor := Actor{kind: ActorHuman, attribution: governanceprincipal.NewUnauthenticatedAttribution(), basis: humanBasisExplicitUnauthenticated}
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

// PolicyGrant is the resolved design_assistance authority: the typed
// payload's own field values plus the sealed effective-policy digest and
// the policy id that carried it. Layout carries the payload's reserved
// `layout:` posture verbatim (policyartifact.DesignAssistancePayload
// validates it as false in v1) rather than being re-derived — a reader
// reporting the applicable policy content (AC-3/AC-5) must show what the
// policy actually says, never a hardcoded constant that would silently
// stop tracking the payload if the reserved field ever opens.
type PolicyGrant struct {
	Mode     string
	Layout   bool
	Digest   string
	PolicyID string
}

// resolvePolicyIdentity resolves the sealed effective-policy digest alone —
// the ONE shared effective-policy identity resolution path every
// authorization composes, never a second policy interpretation.
// ResolvePolicyGrant layers the design_assistance payload requirement on
// top of this; AuthorizeBrowserHuman consumes it directly, so a browser-
// human mutation can resolve the digest without requiring the payload
// AuthorizePolicy's design_assistance-gated callers still need (Wave 6
// design §4.1, SI-176). notAdopted reports exactly the canonical
// errors.Is(err, policyauthority.ErrNotAdopted) condition, distinct from
// every other resolution failure (nil source, malformed store, unsealed or
// forged resolver output), which every caller must still treat
// operationally rather than as non-adoption.
func resolvePolicyIdentity(ctx context.Context, root string, source PolicySource) (effective *policyauthority.EffectivePolicy, digest string, notAdopted bool, err error) {
	if source == nil {
		return nil, "", false, fmt.Errorf("draftmutation: policy source is nil")
	}
	effective, err = source.ResolveEffectivePolicy(ctx, root)
	if err != nil {
		if errors.Is(err, policyauthority.ErrNotAdopted) {
			return nil, "", true, err
		}
		return nil, "", false, err
	}
	digest, err = effective.Digest()
	if err != nil {
		return nil, "", false, fmt.Errorf("effective policy is not sealed resolver output: %w", err)
	}
	return effective, digest, false, nil
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
	effective, digest, notAdopted, err := resolvePolicyIdentity(ctx, root, source)
	if err != nil {
		if notAdopted {
			return PolicyGrant{}, WrapError(CodePolicyForbidden, identity, "project has not adopted policy authority", err)
		}
		return PolicyGrant{}, WrapError(CodeAuthorityInvalid, identity, "resolving effective policy", err)
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
	return PolicyGrant{Mode: payload.Mode, Layout: payload.Layout, Digest: digest, PolicyID: policyID}, nil
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

// PolicyPosture is the exact provenance-facing policy-identity outcome for
// an explicit browser-human mutation (§4.1, SI-176): Adopted is true only
// when a valid adopted effective policy exists, in which case Digest
// carries its sealed identity; otherwise policy authority is genuinely not
// adopted and Digest is empty. It never fabricates a sentinel or hash of
// absence — designprovenance's v2 policy union projects this pair
// directly onto its two closed arms.
type PolicyPosture struct {
	Adopted bool
	Digest  string
}

// AuthorizeBrowserHuman authorizes the explicit unauthenticated-human
// mutation path independently of design_assistance mode, adoption, or
// payload presence (Wave 6 design §4.1, SI-176): a valid adopted effective
// policy contributes only its sealed digest regardless of mode or whether a
// design_assistance payload exists at all, and the delegated-agent path's
// own missing-payload refusal is untouched because this function is never
// reached for that actor. It shares resolvePolicyIdentity — the one
// effective-policy identity resolution path — with ResolvePolicyGrant
// rather than interpreting policy authority a second time. Only the
// canonical errors.Is(err, policyauthority.ErrNotAdopted) condition becomes
// PolicyPosture{Adopted: false}; a nil source, malformed store, or unsealed
// or forged resolver output all remain an operational CodeAuthorityInvalid
// refusal, never a silent non-adoption. Every actor other than the one
// minted by NewUnauthenticatedHuman is refused with CodeActorForbidden,
// including a violated or unproven NewTrustedHuman actor, which keeps
// routing through AuthorizePolicy's own unchanged matrix instead.
func AuthorizeBrowserHuman(ctx context.Context, root string, identity Identity, actor Actor, source PolicySource) (PolicyPosture, *Error) {
	if err := actor.validate(); err != nil {
		return PolicyPosture{}, WrapError(CodeActorForbidden, identity, "actor is not adapter-controlled", err)
	}
	if !actor.isExplicitBrowserHuman() {
		return PolicyPosture{}, NewError(CodeActorForbidden, identity, "browser-human authorization requires the explicit unauthenticated-human actor")
	}
	_, digest, notAdopted, err := resolvePolicyIdentity(ctx, root, source)
	if err != nil {
		if notAdopted {
			return PolicyPosture{Adopted: false}, nil
		}
		return PolicyPosture{}, WrapError(CodeAuthorityInvalid, identity, "resolving effective policy", err)
	}
	return PolicyPosture{Adopted: true, Digest: digest}, nil
}
