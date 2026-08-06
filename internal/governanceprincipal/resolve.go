package governanceprincipal

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
)

// ResolutionState is the closed three-valued principal-resolution state
// (GLG DC-18): never a silent pass.
type ResolutionState string

// The three resolution states. Unknown states fail closed.
const (
	ResolutionAuthenticated ResolutionState = "authenticated"
	ResolutionViolated      ResolutionState = "violated-with-witness"
	ResolutionUnproven      ResolutionState = "unproven"
)

// Validate fails closed on any state outside the vocabulary.
func (s ResolutionState) Validate() error {
	switch s {
	case ResolutionAuthenticated, ResolutionViolated, ResolutionUnproven:
		return nil
	}
	return fmt.Errorf("governanceprincipal: unknown resolution state %q", string(s))
}

// PrincipalClaim is one actor claim to resolve: a profile trust-source
// reference and the stable subject the adapter would authenticate.
type PrincipalClaim struct {
	TrustSource string `json:"trust_source"`
	Subject     string `json:"subject"`
}

// Validate rejects malformed claims: the trust-source reference must
// satisfy the local ID grammar and the subject must be nonempty valid
// UTF-8.
func (c PrincipalClaim) Validate() error {
	if err := ValidateID(c.TrustSource); err != nil {
		return fmt.Errorf("governanceprincipal: claim trust source: %w", err)
	}
	if c.Subject == "" {
		return fmt.Errorf("governanceprincipal: claim subject must be nonempty")
	}
	if !utf8.ValidString(c.Subject) {
		return fmt.Errorf("governanceprincipal: claim subject is not valid UTF-8")
	}
	return nil
}

// TrustFact is the normalized data an adapter reports about one claim
// against one trust source. It is evidence, never a verdict: the port
// never returns authenticated, authorized, roles, or a principal ID —
// only the kernel turns facts into results.
type TrustFact struct {
	// SourceID and SourceKind must echo the trust source the kernel asked
	// about.
	SourceID   string
	SourceKind TrustSourceKind
	// Subjects are the stable subjects the source's evidence attests.
	// Empty when the fact is unavailable.
	Subjects []string
	// EvidenceDigest is the canonical sha256 digest of the evidence the
	// adapter observed; required exactly when Available is true.
	EvidenceDigest string
	// Available=false is expected missing or unreachable evidence — not a
	// Go error; Reason is then required and Subjects must be empty.
	Available bool
	// Valid=false reports explicit contradictory or invalid evidence.
	// Valid=true means the evidence was structurally and cryptographically
	// valid where applicable; the kernel still compares the claimed
	// subject itself.
	Valid bool
	// Reason is the stable explanation required for unavailable or
	// invalid facts.
	Reason string
}

// evidenceDigestRe is the canonical evidence-digest form: "sha256:" and
// 64 lowercase hex digits.
var evidenceDigestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// validEvidenceDigest reports whether s is a canonical sha256 digest.
func validEvidenceDigest(s string) bool { return evidenceDigestRe.MatchString(s) }

// validateTrustFact enforces the port contract. A violation means the
// port is broken, which is an operational error, never a verdict.
func validateTrustFact(fact TrustFact, want TrustSource) error {
	if fact.SourceID != want.ID {
		return fmt.Errorf("governanceprincipal: trust fact source id %q does not match requested source %q", fact.SourceID, want.ID)
	}
	if fact.SourceKind != want.Kind {
		return fmt.Errorf("governanceprincipal: trust fact source kind %q does not match requested kind %q", fact.SourceKind, want.Kind)
	}
	seen := make(map[string]bool, len(fact.Subjects))
	for _, s := range fact.Subjects {
		if s == "" {
			return fmt.Errorf("governanceprincipal: trust fact subject must be nonempty")
		}
		if !utf8.ValidString(s) {
			return fmt.Errorf("governanceprincipal: trust fact subject is not valid UTF-8")
		}
		if seen[s] {
			return fmt.Errorf("governanceprincipal: trust fact carries duplicate subject %q", s)
		}
		seen[s] = true
	}
	if !fact.Available {
		if fact.Reason == "" {
			return fmt.Errorf("governanceprincipal: unavailable trust fact must carry a reason")
		}
		if len(fact.Subjects) != 0 {
			return fmt.Errorf("governanceprincipal: unavailable trust fact must carry no subjects")
		}
		if fact.Valid {
			return fmt.Errorf("governanceprincipal: unavailable trust fact cannot be valid")
		}
		if fact.EvidenceDigest != "" {
			return fmt.Errorf("governanceprincipal: unavailable trust fact must carry no evidence digest")
		}
		return nil
	}
	if !validEvidenceDigest(fact.EvidenceDigest) {
		return fmt.Errorf("governanceprincipal: available trust fact must carry a canonical sha256 evidence digest, got %q", fact.EvidenceDigest)
	}
	if !fact.Valid && fact.Reason == "" {
		return fmt.Errorf("governanceprincipal: invalid trust fact must carry a reason")
	}
	return nil
}

// TrustFactReader is the consumer-owned port through which adapters
// supply trust facts. The kernel never reads ambient identity, the
// network, or repository configuration itself. A returned Go error means
// the port violated its contract or hit a malformed operational condition
// it cannot represent honestly as an unavailable fact.
type TrustFactReader interface {
	ReadTrustFact(ctx context.Context, source TrustSource, claim PrincipalClaim) (TrustFact, error)
}

// Witness is one stable trust witness in a resolution or decision: a
// closed reason code, the trust source involved, the evidence digest when
// evidence exists, and optional human detail.
type Witness struct {
	Code           string `json:"code"`
	SourceID       string `json:"source_id"`
	EvidenceDigest string `json:"evidence_digest,omitempty"`
	Detail         string `json:"detail,omitempty"`
}

// sortWitnesses orders witnesses deterministically by complete field
// content.
func sortWitnesses(ws []Witness) {
	sort.Slice(ws, func(i, j int) bool {
		a, b := ws[i], ws[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		if a.EvidenceDigest != b.EvidenceDigest {
			return a.EvidenceDigest < b.EvidenceDigest
		}
		return a.Detail < b.Detail
	})
}

// PrincipalResolution is the single canonical actor representation a
// governed decision may consume (GLG DC-18): the original claim, the
// derived canonical principal ID only when authenticated, the
// three-valued state, and deterministically sorted trust witnesses.
type PrincipalResolution struct {
	Claim       PrincipalClaim  `json:"claim"`
	PrincipalID PrincipalID     `json:"principal_id,omitempty"`
	State       ResolutionState `json:"state"`
	Witnesses   []Witness       `json:"witnesses"`

	// seal is the unexported integrity seal Resolver.Resolve mints: the
	// canonical content digest of every exported field. External packages
	// cannot set it, and any mutation of the exported fields — including
	// witness replacement or removal — makes it stale, so Authorize and
	// AttributionFromResolution can prove a resolution is unmodified
	// resolver output (checkSeal). Only Resolver.Resolve mints a
	// consumable resolution.
	seal string
}

// checkSeal proves the resolution was produced by Resolver.Resolve and
// has not been modified since. Both failures are operational errors.
func (r PrincipalResolution) checkSeal() error {
	if r.seal == "" {
		return fmt.Errorf("governanceprincipal: resolution for claim %q/%q was not produced by Resolver.Resolve", r.Claim.TrustSource, r.Claim.Subject)
	}
	d, err := canonjson.Digest(r)
	if err != nil {
		return err
	}
	if d != r.seal {
		return fmt.Errorf("governanceprincipal: resolution for claim %q/%q was modified after Resolver.Resolve", r.Claim.TrustSource, r.Claim.Subject)
	}
	return nil
}

// sealResolution mints the integrity seal on a freshly resolved record.
func sealResolution(res *PrincipalResolution) error {
	d, err := canonjson.Digest(*res)
	if err != nil {
		return err
	}
	res.seal = d
	return nil
}

// Resolver turns principal claims into resolutions through an injected
// TrustFactReader. Construct with NewResolver; the zero value fails
// closed with an operational error.
type Resolver struct {
	facts TrustFactReader
}

// NewResolver returns a Resolver reading facts through facts.
func NewResolver(facts TrustFactReader) Resolver {
	return Resolver{facts: facts}
}

// Resolve resolves one claim against one profile:
//
//  1. malformed claims are operational validation errors;
//  2. a trust source the profile does not permit is violated-with-witness;
//  3. the fact is read through the port — a port error is a wrapped
//     operational error, and a fact that breaks the port contract is too;
//  4. an unavailable fact is unproven with a stable witness;
//  5. an invalid fact, or valid evidence that contradicts or omits the
//     claimed subject, is violated-with-witness;
//  6. valid evidence containing the subject is authenticated with the
//     derived canonical principal ID.
func (r Resolver) Resolve(ctx context.Context, profile Profile, claim PrincipalClaim) (PrincipalResolution, error) {
	if err := profile.checkSeal(); err != nil {
		return PrincipalResolution{}, err
	}
	if err := claim.Validate(); err != nil {
		return PrincipalResolution{}, err
	}
	if r.facts == nil {
		return PrincipalResolution{}, fmt.Errorf("governanceprincipal: resolver has no trust-fact reader: construct with NewResolver")
	}

	res := PrincipalResolution{Claim: claim}

	source, ok := profile.trustSource(claim.TrustSource)
	if !ok {
		res.State = ResolutionViolated
		res.Witnesses = []Witness{{
			Code:     ReasonTrustSourceForbidden,
			SourceID: claim.TrustSource,
			Detail:   fmt.Sprintf("trust source %q is not permitted by profile %q", claim.TrustSource, profile.ID),
		}}
		sortWitnesses(res.Witnesses)
		if err := sealResolution(&res); err != nil {
			return PrincipalResolution{}, err
		}
		return res, nil
	}

	fact, err := r.facts.ReadTrustFact(ctx, source, claim)
	if err != nil {
		return PrincipalResolution{}, fmt.Errorf("governanceprincipal: reading trust fact from source %q: %w", source.ID, err)
	}
	if err := validateTrustFact(fact, source); err != nil {
		return PrincipalResolution{}, err
	}

	switch {
	case !fact.Available:
		res.State = ResolutionUnproven
		res.Witnesses = []Witness{{
			Code:     ReasonTrustEvidenceUnavailable,
			SourceID: source.ID,
			Detail:   fact.Reason,
		}}
	case !fact.Valid:
		res.State = ResolutionViolated
		res.Witnesses = []Witness{{
			Code:           ReasonTrustEvidenceInvalid,
			SourceID:       source.ID,
			EvidenceDigest: fact.EvidenceDigest,
			Detail:         fact.Reason,
		}}
	case !contains(fact.Subjects, claim.Subject):
		res.State = ResolutionViolated
		res.Witnesses = []Witness{{
			Code:           ReasonTrustSubjectMismatch,
			SourceID:       source.ID,
			EvidenceDigest: fact.EvidenceDigest,
			Detail:         fmt.Sprintf("valid evidence from %q does not attest claimed subject", source.ID),
		}}
	default:
		id, err := CanonicalPrincipalID(claim.TrustSource, claim.Subject)
		if err != nil {
			// Unreachable after claim.Validate, kept as a fail-closed guard.
			return PrincipalResolution{}, err
		}
		res.State = ResolutionAuthenticated
		res.PrincipalID = id
		res.Witnesses = []Witness{{
			Code:           ReasonTrustSubjectVerified,
			SourceID:       source.ID,
			EvidenceDigest: fact.EvidenceDigest,
			Detail:         fmt.Sprintf("claimed subject observed in valid evidence from %q", source.ID),
		}}
	}
	sortWitnesses(res.Witnesses)
	if err := sealResolution(&res); err != nil {
		return PrincipalResolution{}, err
	}
	return res, nil
}
