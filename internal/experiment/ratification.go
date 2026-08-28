package experiment

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// RatificationSchema is the v1 ratification.yaml schema identifier. V1 is
// strict decode/state-history compatibility only: it is never emitted and
// can never become fresh release or closure authority (Wave 5 design §7).
const RatificationSchema = "verdi.experiment-ratification/v1"

// RatificationSchemaV2 is decode-only history (Task 10 correction,
// SI-150): its actor block persists the adapter-resolved principal claim
// and kernel-derived principal id, but Task 10's independent closure
// review proved that a persisted claim/id pair plus role-mapping
// membership cannot establish that the named human ever asserted the
// ratification operation — role configuration is not evidence. V2 can
// still be decoded to describe historical ratification posture, but it
// may never again be freshly proposed or authorize capsule publication,
// workspace release, or closure (design §7).
const RatificationSchemaV2 = "verdi.experiment-ratification/v2"

// RatificationSchemaV3 is the only emitted ratification schema (design
// §§7-9, SI-150). It carries the same explicit v2-shaped actor block plus
// a retained `authentication_proof` block: the exact action-bound
// challenge and detached Ed25519 signature a successful
// experimenthuman.Verify authenticated at proposal time. Accepted use
// re-verifies that signature against the historical accepted profile
// instead of trusting the persisted claim/id pair as self-evidencing.
const RatificationSchemaV3 = "verdi.experiment-ratification/v3"

// HumanProofSchema is the single-sourced schema literal for the v3
// authentication_proof block's own `schema` field AND the SI-147
// evidence-digest domain separator (controller pin P3): this package is
// the wire-schema home, and internal/experimenthuman consumes this
// constant rather than defining a second copy.
const HumanProofSchema = "verdi.experiment-human-proof/v1"

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

// AuthenticationProof is the v3 wire `authentication_proof` block (design
// §§7-9, SI-150): the exact retained action-bound Ed25519 evidence for a
// human ratification decision. This package owns only the WIRE grammar
// (controller pin P4) — the proof schema literal, canonical unpadded
// base64.RawURLEncoding round-trip equality for both fields, an
// exactly-64-byte decoded signature, and nonempty decoded challenge
// bytes. Challenge-JSON strict decode and all Ed25519 verification live
// in internal/experimenthuman and internal/experimentapp; this type never
// interprets the challenge bytes it carries.
type AuthenticationProof struct {
	Schema             string
	ChallengeBase64URL string
	SignatureBase64URL string
}

// ChallengeBytes decodes the canonical unpadded base64url challenge
// bytes. Callers strict-decode the challenge JSON semantics themselves.
// It routes through the SAME canonical-decode check Validate uses (F5,
// lane review): a struct-literal AuthenticationProof nobody ran through
// Validate — never produced by DecodeRatification, only hand-built —
// cannot leak bytes recovered from a non-canonical alternate spelling.
func (p AuthenticationProof) ChallengeBytes() ([]byte, error) {
	return decodeCanonicalBase64URL(p.ChallengeBase64URL)
}

// SignatureBytes decodes the canonical unpadded base64url detached
// Ed25519 signature bytes (exactly 64 bytes once Validate has run). It
// routes through the same canonical-decode check as ChallengeBytes (F5).
func (p AuthenticationProof) SignatureBytes() ([]byte, error) {
	return decodeCanonicalBase64URL(p.SignatureBase64URL)
}

// Validate checks the closed v3 wire grammar only: the proof schema
// literal, canonical unpadded base64.RawURLEncoding for both fields
// (decode succeeds AND re-encodes to the exact same string — an
// alternate spelling of the same bytes is refused just like a spelling
// that fails to decode at all), a nonempty decoded challenge, and an
// exactly-64-byte decoded signature. It never inspects the challenge
// bytes' own content.
func (p AuthenticationProof) Validate() error {
	if p.Schema != HumanProofSchema {
		return fmt.Errorf("experiment: authentication_proof.schema %q, want %q", p.Schema, HumanProofSchema)
	}
	challenge, err := decodeCanonicalBase64URL(p.ChallengeBase64URL)
	if err != nil {
		return fmt.Errorf("experiment: authentication_proof.challenge_base64url: %w", err)
	}
	if len(challenge) == 0 {
		return fmt.Errorf("experiment: authentication_proof.challenge_base64url decodes to zero bytes")
	}
	signature, err := decodeCanonicalBase64URL(p.SignatureBase64URL)
	if err != nil {
		return fmt.Errorf("experiment: authentication_proof.signature_base64url: %w", err)
	}
	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("experiment: authentication_proof.signature_base64url decodes to %d bytes, want exactly %d", len(signature), ed25519.SignatureSize)
	}
	return nil
}

// decodeCanonicalBase64URL decodes s as base64.RawURLEncoding and
// requires the decoded bytes to re-encode to the exact same string — the
// one canonical spelling a two-faced or alternate encoding cannot
// produce.
func decodeCanonicalBase64URL(s string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("not valid base64.RawURLEncoding: %w", err)
	}
	if base64.RawURLEncoding.EncodeToString(decoded) != s {
		return nil, fmt.Errorf("not the canonical unpadded base64.RawURLEncoding spelling of its own bytes")
	}
	return decoded, nil
}

// Ratification is one ratification.yaml record (AC-5, DC-16): a human's
// adapter-authenticated response to one immutable result. Exactly one
// actor arm is set, matching the schema version: v1 carries the legacy
// bare principal id in Actor (decode-only history), v2 and v3 carry the
// explicit claim/id block in ActorV2. Proof is set only on v3, which adds
// the retained authentication_proof block.
type Ratification struct {
	Schema       string
	ResultDigest string
	// Actor is the v1 wire actor scalar: a bare canonical principal id.
	// Decode-only predecessor history — never emitted, never fresh
	// authority.
	Actor string
	// ActorV2 is the v2/v3 wire actor block.
	ActorV2 *RatificationActor
	// Proof is the v3 wire authentication_proof block. Nil on v1/v2,
	// which must not carry it (Validate enforces both directions).
	Proof       *AuthenticationProof
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

// ratificationWire is the strict on-disk shape shared by every schema
// version; actor and authentication_proof are the version-divergent
// nodes. authentication_proof is captured as a raw artifact.RawNode
// rather than through a custom UnmarshalYAML wrapper (unlike the actor
// field) because of a yaml.v3 gap this package's lane review surfaced
// (F1): yaml.v3 never invokes a field's custom UnmarshalYAML for a
// !!null-tagged node — a bare `authentication_proof:` key, `: null`, or
// `: ~` would silently look identical to the key's total ABSENCE to any
// Unmarshaler-based projection. A plain (non-pointer) artifact.RawNode
// field does not have that gap: yaml.v3 populates it for every key the
// document actually carries, null included, and leaves it at its exact
// zero value (Kind == 0) only when the key is truly absent — see
// decodeRatificationProofNode, which reads that distinction explicitly
// so v1/v2 can refuse the key's PRESENCE, null or populated, not merely
// a populated block (controller pin P4).
type ratificationWire struct {
	Schema       string                 `yaml:"schema"`
	ResultDigest string                 `yaml:"result_digest"`
	Actor        ratificationActorField `yaml:"actor"`
	Proof        artifact.RawNode       `yaml:"authentication_proof"`
	Disposition  Disposition            `yaml:"disposition"`
	Candidate    string                 `yaml:"candidate,omitempty"`
	Reason       string                 `yaml:"reason,omitempty"`
}

// ratificationProofNodePresent reports whether the document carried the
// authentication_proof key AT ALL, independent of its value — including
// an explicit or implicit null. yaml.v3 leaves a plain RawNode field at
// its exact zero value (Kind == 0, a Kind no parsed node ever carries)
// only when the mapping never had the key; every other outcome — a
// mapping, a null, a scalar, a sequence — sets Kind to a nonzero value.
func ratificationProofNodePresent(node artifact.RawNode) bool {
	return node.Kind != 0
}

// decodeRatificationProofNode projects a PRESENT authentication_proof
// node into its typed block: the closed three-key set (schema,
// challenge_base64url, signature_base64url), all three required, judged
// only through the shared internal/artifact RawNode projection — this
// file owns the closed key set and required keys, never yaml handling
// itself. Any non-mapping shape (null, scalar, sequence) is refused here
// too: a present authentication_proof key is never valid unless it is
// exactly this mapping, for every schema version.
func decodeRatificationProofNode(node artifact.RawNode) (*AuthenticationProof, error) {
	fields, isMapping, err := artifact.RawNodeStringMapping(&node)
	if err != nil {
		return nil, fmt.Errorf("ratification authentication_proof: %w", err)
	}
	if !isMapping {
		return nil, fmt.Errorf("ratification authentication_proof must be a mapping")
	}
	allowed := map[string]bool{"schema": true, "challenge_base64url": true, "signature_base64url": true}
	for key := range fields {
		if !allowed[key] {
			return nil, fmt.Errorf("ratification authentication_proof field %q is not known", key)
		}
	}
	for _, required := range []string{"schema", "challenge_base64url", "signature_base64url"} {
		if _, ok := fields[required]; !ok {
			return nil, fmt.Errorf("ratification authentication_proof field %q is required", required)
		}
	}
	return &AuthenticationProof{
		Schema:             fields["schema"],
		ChallengeBase64URL: fields["challenge_base64url"],
		SignatureBase64URL: fields["signature_base64url"],
	}, nil
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
	if ratificationProofNodePresent(wire.Proof) {
		proof, err := decodeRatificationProofNode(wire.Proof)
		if err != nil {
			return Ratification{}, fmt.Errorf("experiment: ratification.authentication_proof: %w", err)
		}
		r.Proof = proof
	}
	if err := r.Validate(); err != nil {
		return Ratification{}, err
	}
	return r, nil
}

// EncodeRatification renders the exact deterministic v3 bytes for a valid
// record. V1 and V2 are decode-only history and are refused (design §9:
// "deterministic ratification encoding emits only V3"). The emitted bytes
// are proven to strict-decode back to the same record before they are
// returned, so no encoding shortcut can produce two-faced bytes.
func EncodeRatification(r Ratification) ([]byte, error) {
	if r.Schema != RatificationSchemaV3 {
		return nil, fmt.Errorf("experiment: only ratification %s is emitted; %q is decode-only history", RatificationSchemaV3, r.Schema)
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString("schema: " + RatificationSchemaV3 + "\n")
	b.WriteString("result_digest: " + r.ResultDigest + "\n")
	b.WriteString("actor:\n")
	b.WriteString("  trust_source: " + r.ActorV2.TrustSource + "\n")
	b.WriteString("  subject: " + yamlQuotedScalar(r.ActorV2.Subject) + "\n")
	b.WriteString("  principal_id: " + r.ActorV2.PrincipalID + "\n")
	b.WriteString("authentication_proof:\n")
	b.WriteString("  schema: " + r.Proof.Schema + "\n")
	b.WriteString("  challenge_base64url: " + yamlQuotedScalar(r.Proof.ChallengeBase64URL) + "\n")
	b.WriteString("  signature_base64url: " + yamlQuotedScalar(r.Proof.SignatureBase64URL) + "\n")
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
		decoded.Proof == nil || *decoded.Proof != *r.Proof ||
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
// version-matched actor arm, the v3-only authentication_proof
// presence/grammar, and the candidate/reason conditionals (required only
// for select-other). The v1 actor must resolve as a canonical kernel
// principal (owner adjudication OD-4); the v2/v3 actor block must be an
// internally consistent claim/id pair. v1 and v2 must NOT carry a proof
// block (unknown declarations are absent-by-construction at decode; this
// enforces presence explicitly per version, controller pin P4); v3
// requires it.
func (r Ratification) Validate() error {
	switch r.Schema {
	case RatificationSchema:
		if r.ActorV2 != nil {
			return fmt.Errorf("experiment: ratification v1 carries a bare principal-id actor, not a v2/v3 actor block")
		}
		if r.Proof != nil {
			return fmt.Errorf("experiment: ratification v1 must not carry an authentication_proof block")
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
		if r.Proof != nil {
			return fmt.Errorf("experiment: ratification v2 must not carry an authentication_proof block")
		}
		if err := r.ActorV2.Validate(); err != nil {
			return err
		}
	case RatificationSchemaV3:
		if r.Actor != "" {
			return fmt.Errorf("experiment: ratification v3 carries an actor block, not a bare principal-id actor")
		}
		if r.ActorV2 == nil {
			return fmt.Errorf("experiment: ratification v3 requires the actor block")
		}
		if err := r.ActorV2.Validate(); err != nil {
			return err
		}
		if r.Proof == nil {
			return fmt.Errorf("experiment: ratification v3 requires the authentication_proof block")
		}
		if err := r.Proof.Validate(); err != nil {
			return fmt.Errorf("experiment: ratification.authentication_proof: %w", err)
		}
	default:
		return fmt.Errorf("experiment: unknown ratification schema %q, want %q, %q, or %q", r.Schema, RatificationSchema, RatificationSchemaV2, RatificationSchemaV3)
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

// Clone returns a deep copy sharing no actor-block or proof-block storage.
func (r Ratification) Clone() Ratification {
	out := r
	if r.ActorV2 != nil {
		block := *r.ActorV2
		out.ActorV2 = &block
	}
	if r.Proof != nil {
		proof := *r.Proof
		out.Proof = &proof
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
