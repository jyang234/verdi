package experiment

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// RatificationSchema is the v1 ratification.yaml schema identifier. V1 is
// strict decode/state-history compatibility only: it is never emitted and
// can never become fresh release or closure authority (Wave 5 design §7).
const RatificationSchema = "verdi.experiment-ratification/v1"

// RatificationSchemaV2 is the only emitted ratification schema. Its actor
// block persists the adapter-resolved principal claim and the
// kernel-derived principal id — never a serialized resolution seal.
const RatificationSchemaV2 = "verdi.experiment-ratification/v2"

// RatificationActor is the explicit v2 actor block: the persisted
// governance-principal claim (trust_source + stable subject) plus the
// kernel-derived canonical principal id. The keys are explicit
// ratification-v2 schema fields, not a serialized Go structure.
type RatificationActor struct {
	TrustSource string
	Subject     string
	PrincipalID string
}

// Validate checks the claim grammar, the canonical principal-id grammar,
// and that the persisted id IS the kernel derivation of the persisted
// claim — an id that does not derive from its own claim is an internally
// inconsistent record, never a differently-authenticated one.
func (a RatificationActor) Validate() error {
	claim := governanceprincipal.PrincipalClaim{TrustSource: a.TrustSource, Subject: a.Subject}
	if err := claim.Validate(); err != nil {
		return fmt.Errorf("experiment: ratification.actor: %w", err)
	}
	if err := governanceprincipal.PrincipalID(a.PrincipalID).Validate(); err != nil {
		return fmt.Errorf("experiment: ratification.actor.principal_id: %w", err)
	}
	derived, err := governanceprincipal.CanonicalPrincipalID(a.TrustSource, a.Subject)
	if err != nil {
		return fmt.Errorf("experiment: ratification.actor: %w", err)
	}
	if string(derived) != a.PrincipalID {
		return fmt.Errorf("experiment: ratification.actor.principal_id %q is not the kernel derivation %q of its own claim", a.PrincipalID, derived)
	}
	return nil
}

// Ratification is one ratification.yaml record (AC-5, DC-16): a human's
// adapter-authenticated response to one immutable result. Exactly one
// actor arm is set, matching the schema version: v1 carries the legacy
// bare principal id in Actor (decode-only history), v2 carries the
// explicit claim/id block in ActorV2.
type Ratification struct {
	Schema       string
	ResultDigest string
	// Actor is the v1 wire actor scalar: a bare canonical principal id.
	// Decode-only predecessor history — never emitted, never fresh
	// authority.
	Actor string
	// ActorV2 is the v2 wire actor block.
	ActorV2     *RatificationActor
	Disposition Disposition
	Candidate   string
	Reason      string
}

// ratificationActorField is the version-divergent wire form of the
// `actor` key: a v1 scalar principal id or a v2 three-field block. The
// document-level dialect guard (anchors/aliases/tags) has already run by
// the time this unmarshals, and the node shape is judged only through the
// shared internal/artifact seam (RawNode projections) — this file owns
// the closed key set and required keys, never yaml handling itself.
type ratificationActorField struct {
	scalar    string
	hasScalar bool
	block     *RatificationActor
}

func (f *ratificationActorField) UnmarshalYAML(node *artifact.RawNode) error {
	if value, ok := artifact.RawNodeStringScalar(node); ok {
		f.scalar, f.hasScalar = value, true
		return nil
	}
	fields, isMapping, err := artifact.RawNodeStringMapping(node)
	if err != nil {
		return fmt.Errorf("ratification actor: %w", err)
	}
	if !isMapping {
		return fmt.Errorf("ratification actor must be a v1 principal-id string or a v2 actor block")
	}
	allowed := map[string]bool{"trust_source": true, "subject": true, "principal_id": true}
	for key := range fields {
		if !allowed[key] {
			return fmt.Errorf("ratification actor field %q is not known", key)
		}
	}
	for _, required := range []string{"trust_source", "subject", "principal_id"} {
		if _, ok := fields[required]; !ok {
			return fmt.Errorf("ratification actor field %q is required", required)
		}
	}
	f.block = &RatificationActor{TrustSource: fields["trust_source"], Subject: fields["subject"], PrincipalID: fields["principal_id"]}
	return nil
}

// ratificationWire is the strict on-disk shape shared by both schema
// versions; the actor field is the one version-divergent node.
type ratificationWire struct {
	Schema       string                 `yaml:"schema"`
	ResultDigest string                 `yaml:"result_digest"`
	Actor        ratificationActorField `yaml:"actor"`
	Disposition  Disposition            `yaml:"disposition"`
	Candidate    string                 `yaml:"candidate,omitempty"`
	Reason       string                 `yaml:"reason,omitempty"`
}

// DecodeRatification strict-decodes raw as a ratification.yaml document
// and fully validates it (decodeStrictYAML: the shared strict seam plus
// this package's trailing-document guard).
func DecodeRatification(raw []byte) (Ratification, error) {
	var wire ratificationWire
	if err := decodeStrictYAML(raw, &wire); err != nil {
		return Ratification{}, fmt.Errorf("experiment: decoding ratification: %w", err)
	}
	r := Ratification{
		Schema: wire.Schema, ResultDigest: wire.ResultDigest,
		Disposition: wire.Disposition, Candidate: wire.Candidate, Reason: wire.Reason,
	}
	if wire.Actor.hasScalar {
		r.Actor = wire.Actor.scalar
	}
	if wire.Actor.block != nil {
		block := *wire.Actor.block
		r.ActorV2 = &block
	}
	if err := r.Validate(); err != nil {
		return Ratification{}, err
	}
	return r, nil
}

// EncodeRatification renders the exact deterministic v2 bytes for a valid
// record. V1 is decode-only history and is refused. The emitted bytes are
// proven to strict-decode back to the same record before they are
// returned, so no encoding shortcut can produce two-faced bytes.
func EncodeRatification(r Ratification) ([]byte, error) {
	if r.Schema != RatificationSchemaV2 {
		return nil, fmt.Errorf("experiment: only ratification %s is emitted; %q is decode-only history", RatificationSchemaV2, r.Schema)
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString("schema: " + RatificationSchemaV2 + "\n")
	b.WriteString("result_digest: " + r.ResultDigest + "\n")
	b.WriteString("actor:\n")
	b.WriteString("  trust_source: " + r.ActorV2.TrustSource + "\n")
	b.WriteString("  subject: " + yamlQuotedScalar(r.ActorV2.Subject) + "\n")
	b.WriteString("  principal_id: " + r.ActorV2.PrincipalID + "\n")
	b.WriteString("disposition: " + string(r.Disposition) + "\n")
	if r.Candidate != "" {
		b.WriteString("candidate: " + r.Candidate + "\n")
	}
	if r.Reason != "" {
		b.WriteString("reason: " + yamlQuotedScalar(r.Reason) + "\n")
	}
	data := b.Bytes()
	decoded, err := DecodeRatification(data)
	if err != nil {
		return nil, fmt.Errorf("experiment: emitted ratification bytes do not round-trip: %w", err)
	}
	if decoded.Schema != r.Schema || decoded.ResultDigest != r.ResultDigest ||
		decoded.ActorV2 == nil || *decoded.ActorV2 != *r.ActorV2 ||
		decoded.Disposition != r.Disposition || decoded.Candidate != r.Candidate || decoded.Reason != r.Reason {
		return nil, fmt.Errorf("experiment: emitted ratification bytes decode to a different record")
	}
	return data, nil
}

// yamlQuotedScalar renders an arbitrary UTF-8 string as a double-quoted
// YAML scalar via the JSON string grammar (a strict subset of YAML's
// double-quoted style), keeping the emission deterministic for any
// subject or reason text.
func yamlQuotedScalar(s string) string {
	quoted, err := json.Marshal(s)
	if err != nil {
		// A Go string always marshals; fail closed to an empty quoted
		// scalar rather than emitting unescaped bytes.
		return `""`
	}
	return string(quoted)
}

// Validate checks the schema, result digest, disposition, the
// version-matched actor arm, and the candidate/reason conditionals
// (required only for select-other). The v1 actor must resolve as a
// canonical kernel principal (owner adjudication OD-4); the v2 actor
// block must be an internally consistent claim/id pair.
func (r Ratification) Validate() error {
	switch r.Schema {
	case RatificationSchema:
		if r.ActorV2 != nil {
			return fmt.Errorf("experiment: ratification v1 carries a bare principal-id actor, not a v2 actor block")
		}
		if err := governanceprincipal.PrincipalID(r.Actor).Validate(); err != nil {
			return fmt.Errorf("experiment: ratification.actor: %w", err)
		}
	case RatificationSchemaV2:
		if r.Actor != "" {
			return fmt.Errorf("experiment: ratification v2 carries an actor block, not a bare principal-id actor")
		}
		if r.ActorV2 == nil {
			return fmt.Errorf("experiment: ratification v2 requires the actor block")
		}
		if err := r.ActorV2.Validate(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("experiment: unknown ratification schema %q, want %q or %q", r.Schema, RatificationSchema, RatificationSchemaV2)
	}
	if err := ValidateDigest(r.ResultDigest); err != nil {
		return fmt.Errorf("experiment: ratification.result_digest: %w", err)
	}
	if err := r.Disposition.Validate(); err != nil {
		return fmt.Errorf("experiment: ratification.disposition: %w", err)
	}

	selectOther := r.Disposition == DispositionSelectOther
	if selectOther {
		if r.Candidate == "" {
			return fmt.Errorf("experiment: ratification.candidate is required when disposition is %q", DispositionSelectOther)
		}
		if r.Reason == "" {
			return fmt.Errorf("experiment: ratification.reason is required when disposition is %q", DispositionSelectOther)
		}
	} else if r.Candidate != "" {
		return fmt.Errorf("experiment: ratification.candidate must be absent when disposition is %q", r.Disposition)
	}
	if r.Candidate != "" {
		if err := ValidateID(r.Candidate); err != nil {
			return fmt.Errorf("experiment: ratification.candidate: %w", err)
		}
	}
	return nil
}

// Clone returns a deep copy sharing no actor-block storage.
func (r Ratification) Clone() Ratification {
	out := r
	if r.ActorV2 != nil {
		block := *r.ActorV2
		out.ActorV2 = &block
	}
	return out
}

// ValidateRatificationBinding checks the preconditions AC-5's disposition
// list IMPLIES but its grammar cannot express (invention ledger SI-45),
// for one ratification bound to the definition and result it responds to:
//
//   - select-recommended ("select the recommended candidate") requires the
//     bound result to actually recommend one, i.e. a proven-winner
//     verdict. Against a disclosed-unproven or violated-with-witness
//     result there is no recommendation to select, and accepting the
//     record would record a human decision that names nothing.
//   - select-other ("select a different candidate with an explicit
//     reason") requires the named candidate to be one the definition
//     REGISTERED — a candidate outside the locked contract was never
//     compared — and, when the result does name a winner, to be a
//     different one, because "other" is meaningless otherwise.
//
// Every other disposition (reject-all, misframed, request-new-revision)
// responds to the result as a whole and binds no candidate, so this check
// imposes nothing on it.
//
// It LAYERS over the record's own grammar validation the way
// ValidateComplete layers over ValidateObservations: r.Validate runs
// first, and stays free of any definition or result knowledge. def and res
// are the caller's already-validated context — DeriveState has decoded
// both and pinned res to def's digest before it gets here.
//
// Callers bind a ratification to its context in more than one place over
// time (state derivation now, adapter surfaces later); every one of them
// runs this check, because a disposition's meaning does not change with
// the surface that records it.
func ValidateRatificationBinding(def Definition, res Result, r Ratification) error {
	if err := r.Validate(); err != nil {
		return err
	}

	decision := res.decisionDocument()
	switch r.Disposition {
	case DispositionSelectRecommended:
		if decision.Verdict != VerdictProvenWinner {
			return fmt.Errorf("experiment: ratification disposition %q requires a %q result, but the bound result's verdict is %q",
				DispositionSelectRecommended, VerdictProvenWinner, decision.Verdict)
		}
	case DispositionSelectOther:
		registered := false
		for _, c := range def.Candidates {
			if c.ID == r.Candidate {
				registered = true
				break
			}
		}
		if !registered {
			return fmt.Errorf("experiment: ratification candidate %q does not name a registered candidate of definition %q",
				r.Candidate, def.ID)
		}
		if decision.Winner != "" && r.Candidate == decision.Winner {
			return fmt.Errorf("experiment: ratification disposition %q names candidate %q, which IS the result's recommended winner",
				DispositionSelectOther, r.Candidate)
		}
	}
	return nil
}
