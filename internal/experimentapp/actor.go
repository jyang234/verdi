package experimentapp

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// ActorKind is the closed adapter-controlled caller vocabulary.
type ActorKind string

const (
	ActorDelegatedAgent     ActorKind = "delegated-agent"
	ActorAuthenticatedHuman ActorKind = "authenticated-human"
)

// Actor can only be minted by adapter-facing constructors. Private operands
// and a seal prevent request bytes or post-construction mutation from minting
// authority.
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
	digest, err := canonjson.Digest(actorProjection{actor.kind, actor.attribution, actor.harness, actor.session})
	if err != nil {
		return fmt.Errorf("experimentapp: seal actor: %w", err)
	}
	actor.seal = digest
	return nil
}

func (a Actor) validate() error {
	if a.seal == "" {
		return fmt.Errorf("experimentapp: actor was not produced by an adapter-controlled constructor")
	}
	want, err := canonjson.Digest(actorProjection{a.kind, a.attribution, a.harness, a.session})
	if err != nil || want != a.seal {
		return fmt.Errorf("experimentapp: actor was modified after construction")
	}
	if err := a.attribution.Validate(); err != nil {
		return err
	}
	switch a.kind {
	case ActorDelegatedAgent:
		if !a.attribution.Unauthenticated || a.attribution.PrincipalID != "" || strings.TrimSpace(a.harness) == "" || !utf8.ValidString(a.harness) {
			return fmt.Errorf("experimentapp: delegated agent requires unauthenticated attribution and nonblank harness")
		}
		if a.session != "" && (strings.TrimSpace(a.session) == "" || !utf8.ValidString(a.session)) {
			return fmt.Errorf("experimentapp: delegated-agent session must be nonblank valid UTF-8 when present")
		}
	case ActorAuthenticatedHuman:
		if a.attribution.Unauthenticated || a.attribution.PrincipalID == "" || a.harness != "" || a.session != "" {
			return fmt.Errorf("experimentapp: authenticated human requires principal attribution and no harness/session")
		}
	default:
		return fmt.Errorf("experimentapp: unknown actor kind %q", a.kind)
	}
	return nil
}

// NewDelegatedAgent returns sealed explicit unauthenticated attribution.
func NewDelegatedAgent(harness, session string) (Actor, error) {
	if strings.TrimSpace(harness) == "" || !utf8.ValidString(harness) {
		return Actor{}, fmt.Errorf("experimentapp: delegated-agent harness must be nonblank valid UTF-8")
	}
	if session != "" && (strings.TrimSpace(session) == "" || !utf8.ValidString(session)) {
		return Actor{}, fmt.Errorf("experimentapp: delegated-agent session must be nonblank valid UTF-8 when present")
	}
	actor := Actor{kind: ActorDelegatedAgent, attribution: governanceprincipal.NewUnauthenticatedAttribution(), harness: harness, session: session}
	if err := sealActor(&actor); err != nil {
		return Actor{}, err
	}
	return actor, nil
}

// NewAuthenticatedHuman derives a sealed actor from genuine kernel output.
func NewAuthenticatedHuman(resolution governanceprincipal.PrincipalResolution) (Actor, error) {
	attribution, err := governanceprincipal.AttributionFromResolution(resolution)
	if err != nil {
		return Actor{}, fmt.Errorf("experimentapp: authenticated human: %w", err)
	}
	if attribution.Unauthenticated {
		return Actor{}, fmt.Errorf("experimentapp: authenticated human requires an authenticated principal resolution")
	}
	actor := Actor{kind: ActorAuthenticatedHuman, attribution: attribution}
	if err := sealActor(&actor); err != nil {
		return Actor{}, err
	}
	return actor, nil
}

func (a Actor) Kind() ActorKind                              { return a.kind }
func (a Actor) Attribution() governanceprincipal.Attribution { return a.attribution }
func (a Actor) Harness() string                              { return a.harness }
func (a Actor) Session() string                              { return a.session }
