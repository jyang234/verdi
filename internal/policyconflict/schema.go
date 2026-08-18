// Package policyconflict defines the Wave-3 policy conflict gate's strict
// wire contracts (docs/superpowers/specs/2026-08-12-policy-conflict-gate-
// authority-design.md, ledger SI-93/SI-96/SI-99/SI-103): the request the
// caller supplies, the judge's inner result and the immutable judgment
// record the CLI process adapter caches, and the canonical report every
// lifecycle consumer evaluates before its first effect. This file declares
// only the shapes; codec.go owns byte<->value conversion and validate.go
// owns every grammar, enum-closure, digest, and ordering rule.
package policyconflict

import (
	"context"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/repositoryfacts"
)

// Schema identifiers for the four wire documents this package owns
// (authority design §§2, 6, 7, 10).
const (
	RequestSchema     = "verdi.policy-conflict-request/v1"
	ReportSchema      = "verdi.policy-conflict-report/v1"
	JudgeResultSchema = "verdi.policy-conflict-judge-result/v1"
	JudgmentSchema    = "verdi.policy-conflict-judgment/v1"
)

// --- closed enums (authority design §§2, 6, 7, 10) -------------------------

// TargetKind selects a Request's or a Report's InputIdentity's exact
// target-identity union arm (authority design §2). Unknown values fail
// closed.
type TargetKind string

const (
	TargetAcceptedContext     TargetKind = "accepted-context"
	TargetAcceptanceCandidate TargetKind = "acceptance-candidate"
)

// Verdict is the report's closed three-state overall outcome (authority
// design §10). Unknown values fail closed.
type Verdict string

const (
	VerdictPass            Verdict = "pass"
	VerdictBlockedViolated Verdict = "blocked-violated"
	VerdictBlockedUnproven Verdict = "blocked-unproven"
)

// ProofState is the closed three-valued proof outcome shared by every row
// and authority-resolution sub-state (authority design §§5-10; mirrors the
// three-valued-honesty vocabulary CLAUDE.md fixes). Unknown values fail
// closed.
type ProofState string

const (
	ProofProven              ProofState = "proven"
	ProofViolatedWithWitness ProofState = "violated-with-witness"
	ProofUnproven            ProofState = "unproven"
)

// Recommendation is the judge's closed three-valued whole-input verdict
// (authority design §6). Unknown values fail closed.
type Recommendation string

const (
	RecommendationConflict     Recommendation = "conflict"
	RecommendationNoConflict   Recommendation = "no-conflict"
	RecommendationInconclusive Recommendation = "inconclusive"
)

// ReasonCode is the closed eighteen-value report-row reason vocabulary
// (authority design §10; ledger SI-103). Unknown values fail closed.
type ReasonCode string

const (
	ReasonMechanicalSatisfiable          ReasonCode = "mechanical-satisfiable"
	ReasonScopeDisjoint                  ReasonCode = "scope-disjoint"
	ReasonMechanicalConflict             ReasonCode = "mechanical-conflict"
	ReasonScopeUnproven                  ReasonCode = "scope-unproven"
	ReasonHigherOrderScopeUnproven       ReasonCode = "higher-order-scope-unproven"
	ReasonPrincipalRelationViolated      ReasonCode = "principal-relation-violated"
	ReasonPrincipalRelationUnproven      ReasonCode = "principal-relation-unproven"
	ReasonExemptionEffective             ReasonCode = "exemption-effective"
	ReasonExemptionIneffective           ReasonCode = "exemption-ineffective"
	ReasonJudgeUnavailable               ReasonCode = "judge-unavailable"
	ReasonJudgeInconclusive              ReasonCode = "judge-inconclusive"
	ReasonChallengerUnavailable          ReasonCode = "challenger-unavailable"
	ReasonJudgmentDisagreement           ReasonCode = "judgment-disagreement"
	ReasonDispositionRequired            ReasonCode = "disposition-required"
	ReasonDispositionEffectiveNoConflict ReasonCode = "disposition-effective-no-conflict"
	ReasonDispositionEffectiveConflict   ReasonCode = "disposition-effective-conflict"
	ReasonDispositionIneffective         ReasonCode = "disposition-ineffective"
	ReasonProfileExperimental            ReasonCode = "profile-experimental"
)

// ScopeState is the closed three-valued scope-comparison outcome (authority
// design §4). Unknown values fail closed.
type ScopeState string

const (
	ScopeOverlap  ScopeState = "overlap"
	ScopeDisjoint ScopeState = "disjoint"
	ScopeUnknown  ScopeState = "unknown"
)

// SolverState is the closed three-valued mechanical-satisfiability outcome
// (authority design §5). Unknown values fail closed.
type SolverState string

const (
	SolverSatisfiable   SolverState = "satisfiable"
	SolverUnsatisfiable SolverState = "unsatisfiable"
	SolverUnproven      SolverState = "unproven"
)

// JudgeRole distinguishes a judgment exchange's primary and challenger
// posture (authority design §6). Unknown values fail closed.
type JudgeRole string

const (
	JudgePrimary    JudgeRole = "primary"
	JudgeChallenger JudgeRole = "challenger"
)

// DisclosureCode is a genuine type alias for contextcompile's own closed
// disclosure vocabulary (authority design §10, ledger SI-103: "top-level
// disclosures reuse the fourteen context-compiler codes"): every value and
// every method contextcompile.DisclosureCode carries — including its own
// exported Validate — applies to a policyconflict.DisclosureCode value
// without this package re-declaring a parallel type or duplicating that
// vocabulary.
type DisclosureCode = contextcompile.DisclosureCode

// DisclosureSoloPrincipalCollapse is the one disclosure code this package
// adds beyond contextcompile's fourteen (authority design §10, ledger
// SI-103): "kernel authorization proves and discloses the solo profile's
// permitted author/approver collapse."
const DisclosureSoloPrincipalCollapse contextcompile.DisclosureCode = "solo-principal-collapse"

// --- Request (authority design §2) ------------------------------------------

// AcceptanceCandidate is the Target union's acceptance-candidate arm
// (authority design §2): a proposed revision's identity, evaluated before
// it can be an accepted context. Expected and Scope are mandatory-whole,
// unlike contextcompile.Request's own optional Expected.
type AcceptanceCandidate struct {
	Adapter  contextcompile.AdapterRef
	Expected contextcompile.Expected
	Grants   execworkspace.GrantSet
	Scope    policyartifact.Scope
	Spec     string
}

// Target is the Request's strict accepted-context/acceptance-candidate
// union (authority design §2): Kind selects exactly one arm; the other is
// nil, and DecodeRequest requires the other's wire key be entirely absent
// (not merely null).
type Target struct {
	Kind                TargetKind
	AcceptedContext     *contextcompile.Request
	AcceptanceCandidate *AcceptanceCandidate
}

// Request is the decoded, validated `verdi.policy-conflict-request/v1`
// document (authority design §2).
type Request struct {
	Schema string
	Target Target
}

// Result is one VerdictProvider.Evaluate call's outcome: the decoded report
// value and the exact canonical bytes EncodeReport produced for it (so a
// caller that only needs to persist/emit the report never re-encodes and
// risks drifting from the value that was actually verdicted).
type Result struct {
	Report      Report
	ReportBytes []byte
}

// VerdictProvider is the one service boundary every CLI/lifecycle call
// consumes (authority design §1/§3, ledger SI-93): a pure function from a
// Request to a Result. This package declares the interface; a later task
// implements it.
type VerdictProvider interface {
	Evaluate(ctx context.Context, request Request) (Result, error)
}

// --- Semantic claims and judge protocol (authority design §6) --------------

// DimensionProof is one scope dimension's (phase, environment, path, or
// ref) comparison outcome (authority design §4).
type DimensionProof struct {
	Dimension    string // phase | environment | path | ref
	State        ScopeState
	Left         []string
	Right        []string
	Intersection []string
	Witnesses    []string
}

// ScopeProof is a claim group's complete N-way scope comparison (authority
// design §4).
type ScopeProof struct {
	State      ScopeState
	Dimensions []DimensionProof
}

// SolverProof is one mechanical-satisfiability solver result for a claim
// group's operand domain (authority design §5). Minimum/Maximum are nil
// outside the integer-interval domain.
type SolverProof struct {
	State      SolverState
	Domain     string
	Values     []string
	Required   []string
	Forbidden  []string
	Minimum    *int
	Maximum    *int
	OpenDomain bool
	Witnesses  []string
}

// TypedClaimRecord is one mechanical row's typed claim, bound to the exact
// policy/overlay artifact and claim digest it was read from (authority
// design §5, DC-8's "exact witnesses").
//
// Its identity is the composite (PolicyID, Claim.ID) — never the claim
// digest alone (ledger SI-105): policy identity is part of an exemption
// witness and cannot be discarded merely because two policies declare
// byte-identical claims, so equal bytes from two policies remain two
// records. ClaimDigest must equal the canonical digest of the carried
// Claim; the runtime seams recompute it rather than trusting a carried
// value.
type TypedClaimRecord struct {
	PolicyID     string
	PolicyDigest string
	ClaimDigest  string
	Claim        policyartifact.Claim
}

// MechanicalClaimWitness names one exact typed mechanical claim an
// exemption departed from (authority design §5.5, ledger SI-105): "A
// removed mechanical claim is identified by (policy_id, claim_id,
// claim_digest), not by the semantic prose-witness vocabulary." Its
// identity is the composite (PolicyID, ClaimID); ClaimDigest is the exact
// current row claim's digest, so a later claim change visibly stales the
// exemption instead of silently widening it.
type MechanicalClaimWitness struct {
	PolicyID    string
	ClaimID     string
	ClaimDigest string
}

// ClaimWitness names one exact SEMANTIC claim a judge finding cites: its
// canonical source id, content digest, and closed §6 source category
// (authority design §6). This is policyconflict's own witness-reference
// type — distinct from policyartifact.SemanticClaimWitness, which
// additionally carries the claim's own authority binding and is never
// re-derived here; a ClaimWitness only cites an identity already
// established elsewhere.
//
// A typed mechanical constraint is NOT one of §6's prose categories, so an
// exemption's removed claims use MechanicalClaimWitness instead (SI-105).
type ClaimWitness struct {
	ID       string
	Digest   string
	Category string
}

// JudgeFinding is one named overlap/conflict the judge's result reports
// (authority design §6): the ≥2 distinct claim witnesses it names, their
// sorted-unique categories, and a single-line explanation (evidence, never
// authority).
type JudgeFinding struct {
	Claims      []ClaimWitness
	Categories  []string
	Explanation string
}

// JudgeResult is the judge's strict inner result, canonical schema
// `verdi.policy-conflict-judge-result/v1` (authority design §6).
type JudgeResult struct {
	Schema         string
	Recommendation Recommendation
	Findings       []JudgeFinding
}

// JudgmentExchange is one completed, successfully validated primary or
// challenger exchange (authority design §6/§7): the adapter-declared
// transport/model posture, every bound digest, the raw result bytes and
// their digest, and the parsed result. Its presence alone proves the
// single successful process/validation state — there is deliberately no
// process/validation-state enum or always-empty reasons field alongside
// it.
type JudgmentExchange struct {
	Role          JudgeRole
	Adapter       contextcompile.AdapterRef
	Model         string
	CommandDigest string
	PromptDigest  string
	InputDigest   string
	RawResult     string
	RawDigest     string
	Result        JudgeResult
}

// Judgment is the immutable machine judgment record, canonical schema
// `verdi.policy-conflict-judgment/v1` (authority design §7). TreeHash and
// InputDigest are the D4 cache path-key components — bare 64 lowercase hex,
// no "sha256:" prefix — distinct from JudgmentExchange.InputDigest, which
// retains the full "sha256:<hex>" record form (SI-101).
type Judgment struct {
	Schema          string
	TreeHash        string
	InputDigest     string
	ProfileID       string
	ProfileDigest   string
	AuthorityDigest string
	Exchange        JudgmentExchange
	Digest          string
}

// --- Report (authority design §10) ------------------------------------------

// AuthorityResolution is one embedded authority artifact's (exemption or
// disposition) complete proof substate: identity match, freshness, scope
// coverage, time/review bound, and authorization (authority design §9/§10).
type AuthorityResolution struct {
	Match         ProofState
	Freshness     ProofState
	Scope         ProofState
	Bound         ProofState
	Authorization ProofState
}

// ExemptionResolution is one mechanical row's applicable exemption
// application: the exemption's identity/digest, its authority resolution,
// and the exact claims it removed from the post-exemption solve (authority
// design §5.5).
//
// RemovedClaims is mandatory-present, never absent: it is nonempty (at
// least one exact current row witness) exactly when all five authority
// states are proven, and the explicit empty set for every rejected
// resolution, which removed nothing. Witnesses sort and deduplicate by
// their composite (PolicyID, ClaimID) identity.
type ExemptionResolution struct {
	ID            string
	Digest        string
	Resolution    AuthorityResolution
	RemovedClaims []MechanicalClaimWitness
}

// MechanicalEvaluation is one mechanical row: the deterministic claim
// group's typed claims, scope proof, operand domain, pre/post-exemption
// solver results, applicable exemption resolutions, proof state, and reason
// codes (authority design §10).
type MechanicalEvaluation struct {
	ID         string
	Family     policyartifact.Family
	Subject    string
	Claims     []TypedClaimRecord
	Scope      ScopeProof
	Domain     string
	Before     SolverProof
	Exemptions []ExemptionResolution
	After      SolverProof
	State      ProofState
	Reasons    []ReasonCode
}

// UnknownMechanicalWitness is the lossless semantic identity of one
// mechanically unresolved row (authority design §§6/10, SI-110). It keeps
// exactly the row ID, its complete composite typed-claim records, and its
// exact scope proof. Solver output and later authority-resolution state are
// deliberately absent so unrelated mechanical recomputation cannot stale a
// semantic disposition.
type UnknownMechanicalWitness struct {
	ID     string
	Claims []TypedClaimRecord
	Scope  ScopeProof
}

// DispositionResolution is one semantic row's applicable disposition:
// identity/digest, its closed conclusion, and its authority resolution
// (authority design §8/§9/§10).
type DispositionResolution struct {
	ID         string
	Digest     string
	Conclusion policyartifact.DispositionConclusion
	Resolution AuthorityResolution
}

// SemanticEvaluation is one semantic row: the normalized claim identities
// it witnesses, any mechanically-unknown scope it explains, the primary
// and (when run) challenger exchanges, applicable disposition resolutions,
// proof state, and reason codes (authority design §10).
type SemanticEvaluation struct {
	ID                 string
	InputID            string
	Claims             []policyartifact.SemanticClaimWitness
	UnknownMechanicals []UnknownMechanicalWitness
	Primary            *JudgmentExchange
	Challenger         *JudgmentExchange
	Dispositions       []DispositionResolution
	State              ProofState
	Reasons            []ReasonCode
}

// AcceptedIdentity is the report's target-identity union's accepted arm:
// the accepted context manifest's own self digest (authority design §10).
type AcceptedIdentity struct {
	ManifestDigest string
}

// CandidateIdentity is the report's target-identity union's candidate arm:
// the proposed revision's exact ref/path/branch/HEAD/blob/content identity,
// scope, adapter, and grant digest (authority design §10).
type CandidateIdentity struct {
	Ref           string
	Path          string
	Branch        string
	Head          string
	Blob          string
	ContentDigest string
	Scope         policyartifact.Scope
	Adapter       contextcompile.AdapterRef
	GrantDigest   string
}

// TargetIdentity is the report's InputIdentity's target union: exactly one
// of Accepted/Candidate is present, matching Kind (authority design §10).
type TargetIdentity struct {
	Kind      TargetKind
	Accepted  *AcceptedIdentity
	Candidate *CandidateIdentity
}

// PolicyEntryIdentity is one row of the report's sorted policy-entry
// ledger: one applicable policy, overlay, or exemption operand's identity
// (authority design §10).
type PolicyEntryIdentity struct {
	Kind   string
	ID     string
	Digest string
}

// ProfileIdentity is the report's governing governance-profile identity
// (authority design §10).
type ProfileIdentity struct {
	ID     string
	Class  string
	Digest string
}

// InputIdentity is the report's single occurrence of every input fact:
// the strict target union, repository snapshot, constitution/effective-
// policy digests, sorted policy-entry identities, profile identity, and
// the injected evaluation date (authority design §10). These facts occur
// exactly once — the report carries no second context_manifest, exemption
// ledger, or disposition ledger.
type InputIdentity struct {
	Target                TargetIdentity
	Repository            repositoryfacts.Facts
	ConstitutionDigest    string
	EffectivePolicyDigest string
	PolicyEntries         []PolicyEntryIdentity
	Profile               ProfileIdentity
	EvaluatedOn           string
}

// Disclosure is one closed-code, sorted-witness top-level report disclosure
// (authority design §10).
type Disclosure struct {
	Code      DisclosureCode
	Witnesses []string
}

// Report is the canonical machine report, schema
// `verdi.policy-conflict-report/v1` (authority design §10): one input
// identity, sorted mechanical and semantic rows, closed disclosures, the
// overall three-state verdict, and a self digest computed over the
// digestless canonical form.
type Report struct {
	Schema      string
	Input       InputIdentity
	Mechanical  []MechanicalEvaluation
	Semantic    []SemanticEvaluation
	Disclosures []Disclosure
	Verdict     Verdict
	Digest      string
}
