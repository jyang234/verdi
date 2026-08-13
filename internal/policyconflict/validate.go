package policyconflict

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// digestRe matches the shared "sha256:" + 64 lowercase hex digest form
// (mirrors internal/contextcompile's own digestRe and
// internal/policyartifact's own sha256Re — a generic format check, not a
// closed vocabulary, independently owned by every package that carries
// digest-shaped fields).
var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// bareHexRe matches a bare 64-lowercase-hex digest with no "sha256:"
// prefix — the D4 cache path-key form (authority design §7, SI-101).
var bareHexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// fullHexRe matches a full 40-lowercase-hex git object hash (authority
// design §2: "requires ... full 40-hex HEAD").
var fullHexRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

func validateDigest(field, value string) error {
	if !digestRe.MatchString(value) {
		return fmt.Errorf("policyconflict: %s: %q is not a valid sha256:<64 lowercase hex> digest", field, value)
	}
	return nil
}

func validateBareHex(field, value string) error {
	if !bareHexRe.MatchString(value) {
		return fmt.Errorf("policyconflict: %s: %q is not a valid bare 64 lowercase hex digest (no sha256: prefix)", field, value)
	}
	return nil
}

func validateNonEmpty(field, value string) error {
	if value == "" {
		return fmt.Errorf("policyconflict: %s: must be non-empty", field)
	}
	return nil
}

// singleLineNonBlank enforces the shared single-line-prose grammar used for
// ids, categories, and explanations: non-blank, and free of control
// characters (mirrors internal/policyartifact's own identical helper —
// generic string shape, not a closed vocabulary).
func singleLineNonBlank(field, s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("policyconflict: %s: must not be blank", field)
	}
	for _, r := range s {
		if r == '\n' || r == '\r' {
			return fmt.Errorf("policyconflict: %s: must be single-line", field)
		}
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("policyconflict: %s: contains a control character (U+%04X)", field, r)
		}
	}
	return nil
}

func requireSortedUnique[T any](field string, items []T, key func(T) string) error {
	for i := 1; i < len(items); i++ {
		prev, cur := key(items[i-1]), key(items[i])
		switch {
		case cur == prev:
			return fmt.Errorf("policyconflict: %s: duplicate identity %q", field, cur)
		case cur < prev:
			return fmt.Errorf("policyconflict: %s: must be sorted ascending (found %q after %q)", field, cur, prev)
		}
	}
	return nil
}

func requireSortedUniqueStrings(field string, ss []string) error {
	return requireSortedUnique(field, ss, func(s string) string { return s })
}

// --- enum closure checks (policyconflict's own closed vocabularies) --------

func (k TargetKind) Validate() error {
	switch k {
	case TargetAcceptedContext, TargetAcceptanceCandidate:
		return nil
	}
	return fmt.Errorf("policyconflict: unknown target kind %q", string(k))
}

func (v Verdict) Validate() error {
	switch v {
	case VerdictPass, VerdictBlockedViolated, VerdictBlockedUnproven:
		return nil
	}
	return fmt.Errorf("policyconflict: unknown verdict %q", string(v))
}

func (p ProofState) Validate() error {
	switch p {
	case ProofProven, ProofViolatedWithWitness, ProofUnproven:
		return nil
	}
	return fmt.Errorf("policyconflict: unknown proof state %q", string(p))
}

func (r Recommendation) Validate() error {
	switch r {
	case RecommendationConflict, RecommendationNoConflict, RecommendationInconclusive:
		return nil
	}
	return fmt.Errorf("policyconflict: unknown recommendation %q", string(r))
}

var knownReasonCodes = map[ReasonCode]bool{
	ReasonMechanicalSatisfiable: true, ReasonScopeDisjoint: true, ReasonMechanicalConflict: true,
	ReasonScopeUnproven: true, ReasonHigherOrderScopeUnproven: true,
	ReasonPrincipalRelationViolated: true, ReasonPrincipalRelationUnproven: true,
	ReasonExemptionEffective: true, ReasonExemptionIneffective: true,
	ReasonJudgeUnavailable: true, ReasonJudgeInconclusive: true, ReasonChallengerUnavailable: true,
	ReasonJudgmentDisagreement: true, ReasonDispositionRequired: true,
	ReasonDispositionEffectiveNoConflict: true, ReasonDispositionEffectiveConflict: true,
	ReasonDispositionIneffective: true, ReasonProfileExperimental: true,
}

func (r ReasonCode) Validate() error {
	if !knownReasonCodes[r] {
		return fmt.Errorf("policyconflict: unknown reason code %q", string(r))
	}
	return nil
}

func validateReasons(field string, reasons []ReasonCode) error {
	if reasons == nil {
		return fmt.Errorf("policyconflict: %s: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, r := range reasons {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("policyconflict: %s[%d]: %w", field, i, err)
		}
	}
	return requireSortedUnique(field, reasons, func(r ReasonCode) string { return string(r) })
}

func (s ScopeState) Validate() error {
	switch s {
	case ScopeOverlap, ScopeDisjoint, ScopeUnknown:
		return nil
	}
	return fmt.Errorf("policyconflict: unknown scope state %q", string(s))
}

func (s SolverState) Validate() error {
	switch s {
	case SolverSatisfiable, SolverUnsatisfiable, SolverUnproven:
		return nil
	}
	return fmt.Errorf("policyconflict: unknown solver state %q", string(s))
}

func (r JudgeRole) Validate() error {
	switch r {
	case JudgePrimary, JudgeChallenger:
		return nil
	}
	return fmt.Errorf("policyconflict: unknown judge role %q", string(r))
}

// validateDisclosureCode checks d against the closed vocabulary authority
// design §10/ledger SI-103 fixes: the fourteen existing
// contextcompile.DisclosureCode values (checked via that type's own
// exported Validate — DisclosureCode is a genuine alias, so the method is
// already inherited) plus exactly DisclosureSoloPrincipalCollapse.
func validateDisclosureCode(d DisclosureCode) error {
	if d == DisclosureSoloPrincipalCollapse {
		return nil
	}
	if err := d.Validate(); err != nil {
		return fmt.Errorf("policyconflict: unknown disclosure code %q", string(d))
	}
	return nil
}

// --- Request (authority design §2) ------------------------------------------

func (a AcceptanceCandidate) validate() error {
	if err := validateNonEmpty("acceptance_candidate.adapter.id", a.Adapter.ID); err != nil {
		return err
	}
	if err := validateNonEmpty("acceptance_candidate.adapter.version", a.Adapter.Version); err != nil {
		return err
	}
	if err := validateNonEmpty("acceptance_candidate.expected.branch", a.Expected.Branch); err != nil {
		return err
	}
	if !fullHexRe.MatchString(a.Expected.Head) {
		return fmt.Errorf("policyconflict: acceptance_candidate.expected.head: %q must be a full 40-lowercase-hex commit", a.Expected.Head)
	}
	if err := a.Grants.Validate(); err != nil {
		return fmt.Errorf("policyconflict: acceptance_candidate.grants: %w", err)
	}
	if err := validateScope("acceptance_candidate.scope", a.Scope); err != nil {
		return err
	}
	parsed, err := artifact.ParseRef(a.Spec)
	if err != nil {
		return fmt.Errorf("policyconflict: acceptance_candidate.spec: %w", err)
	}
	if parsed.Kind != artifact.KindSpec {
		return fmt.Errorf("policyconflict: acceptance_candidate.spec: %q must be a spec/<name> ref, got kind %q", a.Spec, parsed.Kind)
	}
	if parsed.Pinned() {
		return fmt.Errorf("policyconflict: acceptance_candidate.spec: %q must be unpinned (no @commit)", a.Spec)
	}
	if parsed.Fragment() {
		return fmt.Errorf("policyconflict: acceptance_candidate.spec: %q must name the whole spec (no #object-id)", a.Spec)
	}
	return nil
}

func validateScope(field string, s policyartifact.Scope) error {
	if err := s.Validate(); err != nil {
		return fmt.Errorf("policyconflict: %s: %w", field, err)
	}
	if err := requireSortedUniqueStrings(field+".phases", s.Phases); err != nil {
		return err
	}
	if err := requireSortedUniqueStrings(field+".environments", s.Environments); err != nil {
		return err
	}
	if err := requireSortedUniqueStrings(field+".paths", s.Paths); err != nil {
		return err
	}
	return requireSortedUniqueStrings(field+".refs", s.Refs)
}

// Validate checks r's complete grammar: schema, the strict target union,
// and (for the arm present) full nested validation.
func (r Request) Validate() error {
	if r.Schema != RequestSchema {
		return fmt.Errorf("policyconflict: request: schema %q, want %q", r.Schema, RequestSchema)
	}
	return r.Target.validate()
}

func (t Target) validate() error {
	if err := t.Kind.Validate(); err != nil {
		return fmt.Errorf("policyconflict: target.kind: %w", err)
	}
	switch t.Kind {
	case TargetAcceptedContext:
		if t.AcceptedContext == nil {
			return fmt.Errorf("policyconflict: target: kind is accepted-context but accepted_context is nil")
		}
		if t.AcceptanceCandidate != nil {
			return fmt.Errorf("policyconflict: target: kind is accepted-context but acceptance_candidate is also present")
		}
		if err := t.AcceptedContext.Validate(); err != nil {
			return fmt.Errorf("policyconflict: target.accepted_context: %w", err)
		}
	case TargetAcceptanceCandidate:
		if t.AcceptanceCandidate == nil {
			return fmt.Errorf("policyconflict: target: kind is acceptance-candidate but acceptance_candidate is nil")
		}
		if t.AcceptedContext != nil {
			return fmt.Errorf("policyconflict: target: kind is acceptance-candidate but accepted_context is also present")
		}
		if err := t.AcceptanceCandidate.validate(); err != nil {
			return err
		}
	}
	return nil
}

// --- JudgeResult (authority design §6) --------------------------------------

func (w ClaimWitness) validate(field string) error {
	if err := singleLineNonBlank(field+".id", w.ID); err != nil {
		return err
	}
	if err := validateDigest(field+".digest", w.Digest); err != nil {
		return err
	}
	// Category is closed vocabulary, not free prose: authority design §6
	// fixes the source-category list ("The closed source categories are
	// ...") and rules that "An unknown, missing, duplicate, or
	// digest-mismatched witness invalidates the result". A judge naming a
	// category outside that list has classified a claim the semantic
	// universe cannot contain, so the witness fails closed here — at the
	// judge-result boundary and on every report path that carries a
	// ClaimWitness. The vocabulary itself is reached through
	// policyartifact.ValidateWitnessCategory, the exported seam authorized
	// for exactly this embedding; policyconflict never duplicates the
	// private knownWitnessCategories map.
	return validateWitnessCategory(field+".category", w.Category)
}

// validateWitnessCategory closed-vocabulary-checks category via
// policyartifact.ValidateWitnessCategory (authority design §6, ledger
// SI-103) — the single call site through which this package reaches that
// vocabulary.
func validateWitnessCategory(field, category string) error {
	if err := policyartifact.ValidateWitnessCategory(category); err != nil {
		return fmt.Errorf("policyconflict: %s: %w", field, err)
	}
	return nil
}

func (f JudgeFinding) validate(field string) error {
	if len(f.Claims) < 2 {
		return fmt.Errorf("policyconflict: %s.claims: must name at least two distinct claim witnesses, got %d", field, len(f.Claims))
	}
	for i, c := range f.Claims {
		if err := c.validate(fmt.Sprintf("%s.claims[%d]", field, i)); err != nil {
			return err
		}
	}
	if err := requireSortedUnique(field+".claims", f.Claims, func(c ClaimWitness) string { return c.ID }); err != nil {
		return err
	}
	if f.Categories == nil {
		return fmt.Errorf("policyconflict: %s.categories: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, c := range f.Categories {
		if err := validateWitnessCategory(fmt.Sprintf("%s.categories[%d]", field, i), c); err != nil {
			return err
		}
	}
	if err := requireSortedUniqueStrings(field+".categories", f.Categories); err != nil {
		return err
	}
	return singleLineNonBlank(field+".explanation", f.Explanation)
}

// Validate checks r's complete grammar: schema, recommendation closure,
// recommendation/findings cardinality (authority design §6: conflict
// requires >=1 finding, no-conflict requires explicit empty findings,
// inconclusive permits any count), and every finding's own grammar.
func (r JudgeResult) Validate() error {
	if r.Schema != JudgeResultSchema {
		return fmt.Errorf("policyconflict: judge result: schema %q, want %q", r.Schema, JudgeResultSchema)
	}
	if err := r.Recommendation.Validate(); err != nil {
		return fmt.Errorf("policyconflict: judge result.recommendation: %w", err)
	}
	if r.Findings == nil {
		return fmt.Errorf("policyconflict: judge result.findings: must be non-nil (an explicitly empty set is [])")
	}
	switch r.Recommendation {
	case RecommendationConflict:
		if len(r.Findings) < 1 {
			return fmt.Errorf("policyconflict: judge result: recommendation conflict requires at least one finding")
		}
	case RecommendationNoConflict:
		if len(r.Findings) != 0 {
			return fmt.Errorf("policyconflict: judge result: recommendation no-conflict requires explicit empty findings, got %d", len(r.Findings))
		}
	case RecommendationInconclusive:
		// Any count, including zero, is legal.
	}
	for i, f := range r.Findings {
		if err := f.validate(fmt.Sprintf("judge result.findings[%d]", i)); err != nil {
			return err
		}
	}
	return nil
}

// --- Judgment (authority design §7) -----------------------------------------

func (e JudgmentExchange) validate() error {
	if err := e.Role.Validate(); err != nil {
		return fmt.Errorf("policyconflict: judgment.exchange.role: %w", err)
	}
	if err := validateNonEmpty("judgment.exchange.adapter.id", e.Adapter.ID); err != nil {
		return err
	}
	if err := validateNonEmpty("judgment.exchange.adapter.version", e.Adapter.Version); err != nil {
		return err
	}
	if err := validateNonEmpty("judgment.exchange.model", e.Model); err != nil {
		return err
	}
	if err := validateDigest("judgment.exchange.command_digest", e.CommandDigest); err != nil {
		return err
	}
	if err := validateDigest("judgment.exchange.prompt_digest", e.PromptDigest); err != nil {
		return err
	}
	if err := validateDigest("judgment.exchange.input_digest", e.InputDigest); err != nil {
		return err
	}
	if e.RawResult == "" {
		return fmt.Errorf("policyconflict: judgment.exchange.raw_result: must be non-empty")
	}
	if err := validateDigest("judgment.exchange.raw_digest", e.RawDigest); err != nil {
		return err
	}
	if want := rawContentDigest([]byte(e.RawResult)); want != e.RawDigest {
		return fmt.Errorf("policyconflict: judgment.exchange.raw_digest: %q does not match the exact bytes carried in raw_result (want %q)", e.RawDigest, want)
	}
	return e.Result.Validate()
}

// Validate checks j's complete grammar: schema, bare-hex path-key fields,
// and the embedded exchange. Digest's own grammar and self-consistency are
// checked by EncodeJudgment/DecodeJudgment, not here (mirrors
// internal/contextcompile.Manifest.Validate's own digest treatment: only
// checked when non-empty, since the encode path always discards and
// recomputes it).
func (j Judgment) Validate() error {
	if j.Schema != JudgmentSchema {
		return fmt.Errorf("policyconflict: judgment: schema %q, want %q", j.Schema, JudgmentSchema)
	}
	if err := validateBareHex("judgment.tree_hash", j.TreeHash); err != nil {
		return err
	}
	if err := validateBareHex("judgment.input_digest", j.InputDigest); err != nil {
		return err
	}
	if err := j.Exchange.validate(); err != nil {
		return err
	}
	if j.Digest != "" {
		if err := validateDigest("judgment.digest", j.Digest); err != nil {
			return err
		}
	}
	return nil
}

// --- Report (authority design §10) ------------------------------------------
//
// validate.go implements every rule this package's own closed vocabularies
// and every embedded type's own PUBLIC validation seam make reachable.
// SemanticEvaluation.Claims ([]policyartifact.SemanticClaimWitness) and
// DispositionResolution.Conclusion (policyartifact.DispositionConclusion)
// were originally blocked here: policyartifact exported no Validate for
// either type, so this package could not closed-vocabulary-check their
// embedded content without duplicating policyartifact's private
// knownWitnessCategories/knownDispositionConclusions vocabularies (see the
// Task 3 BLOCKED-SEAM handoff report). Controller adjudication authorized
// and internal/policyartifact now exports ValidateWitnessCategory,
// SemanticClaimWitness.Validate, and DispositionConclusion.Validate for
// exactly this embedding; validateSemanticClaims and
// validateDispositionConclusion below delegate to them and no longer
// duplicate or approximate that vocabulary.

var knownPolicyEntryKinds = map[string]bool{"policy": true, "overlay": true, "exemption": true}

func validatePolicyEntryIdentity(field string, e PolicyEntryIdentity) error {
	if !knownPolicyEntryKinds[e.Kind] {
		return fmt.Errorf("policyconflict: %s.kind: unknown value %q", field, e.Kind)
	}
	if err := validateNonEmpty(field+".id", e.ID); err != nil {
		return err
	}
	return validateDigest(field+".digest", e.Digest)
}

var evaluatedOnRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func validateEvaluatedOn(field, value string) error {
	if !evaluatedOnRe.MatchString(value) {
		return fmt.Errorf("policyconflict: %s: %q is not YYYY-MM-DD form", field, value)
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return fmt.Errorf("policyconflict: %s: %q is not a real calendar date", field, value)
	}
	return nil
}

func (i InputIdentity) validate() error {
	if err := i.Target.validate(); err != nil {
		return err
	}
	if err := i.Repository.Validate(); err != nil {
		return fmt.Errorf("policyconflict: input.repository: %w", err)
	}
	if err := validateDigest("input.constitution_digest", i.ConstitutionDigest); err != nil {
		return err
	}
	if err := validateDigest("input.effective_policy_digest", i.EffectivePolicyDigest); err != nil {
		return err
	}
	if i.PolicyEntries == nil {
		return fmt.Errorf("policyconflict: input.policy_entries: must be non-nil (an explicitly empty set is [])")
	}
	for idx, e := range i.PolicyEntries {
		if err := validatePolicyEntryIdentity(fmt.Sprintf("input.policy_entries[%d]", idx), e); err != nil {
			return err
		}
	}
	if err := requireSortedUnique("input.policy_entries", i.PolicyEntries, func(e PolicyEntryIdentity) string { return e.Kind + "\x00" + e.ID }); err != nil {
		return err
	}
	if err := validateNonEmpty("input.profile.id", i.Profile.ID); err != nil {
		return err
	}
	if err := validateNonEmpty("input.profile.class", i.Profile.Class); err != nil {
		return err
	}
	if err := validateDigest("input.profile.digest", i.Profile.Digest); err != nil {
		return err
	}
	return validateEvaluatedOn("input.evaluated_on", i.EvaluatedOn)
}

func (t TargetIdentity) validate() error {
	if err := t.Kind.Validate(); err != nil {
		return fmt.Errorf("policyconflict: input.target.kind: %w", err)
	}
	switch t.Kind {
	case TargetAcceptedContext:
		if t.Accepted == nil || t.Candidate != nil {
			return fmt.Errorf("policyconflict: input.target: kind accepted-context requires exactly accepted present")
		}
		return validateDigest("input.target.accepted.manifest_digest", t.Accepted.ManifestDigest)
	case TargetAcceptanceCandidate:
		if t.Candidate == nil || t.Accepted != nil {
			return fmt.Errorf("policyconflict: input.target: kind acceptance-candidate requires exactly candidate present")
		}
		c := t.Candidate
		if err := validateNonEmpty("input.target.candidate.ref", c.Ref); err != nil {
			return err
		}
		if err := validateNonEmpty("input.target.candidate.path", c.Path); err != nil {
			return err
		}
		if err := validateNonEmpty("input.target.candidate.branch", c.Branch); err != nil {
			return err
		}
		if !fullHexRe.MatchString(c.Head) {
			return fmt.Errorf("policyconflict: input.target.candidate.head: %q must be full 40-lowercase-hex", c.Head)
		}
		if !fullHexRe.MatchString(c.Blob) {
			return fmt.Errorf("policyconflict: input.target.candidate.blob: %q must be full 40-lowercase-hex", c.Blob)
		}
		if err := validateDigest("input.target.candidate.content_digest", c.ContentDigest); err != nil {
			return err
		}
		if err := validateScope("input.target.candidate.scope", c.Scope); err != nil {
			return err
		}
		if err := validateNonEmpty("input.target.candidate.adapter.id", c.Adapter.ID); err != nil {
			return err
		}
		if err := validateNonEmpty("input.target.candidate.adapter.version", c.Adapter.Version); err != nil {
			return err
		}
		return validateDigest("input.target.candidate.grant_digest", c.GrantDigest)
	}
	return nil
}

// scopeDimensionOrder is the fixed sort order authority design §4.4 fixes
// for a scope proof's per-dimension rows ("Sorting is phase, environment,
// path, then ref"). A proof may omit dimensions it did not compare, so a
// legal row set is any SUBSEQUENCE of this order — never a permutation of
// it, and never with a dimension repeated.
var scopeDimensionOrder = []string{"phase", "environment", "path", "ref"}

func scopeDimensionRank(name string) (int, bool) {
	for i, d := range scopeDimensionOrder {
		if d == name {
			return i, true
		}
	}
	return 0, false
}

func validateDimensionProof(field string, d DimensionProof) error {
	if err := singleLineNonBlank(field+".dimension", d.Dimension); err != nil {
		return err
	}
	if _, ok := scopeDimensionRank(d.Dimension); !ok {
		return fmt.Errorf("policyconflict: %s.dimension: unknown value %q", field, d.Dimension)
	}
	if err := d.State.Validate(); err != nil {
		return err
	}
	// Each recorded set is "the exact set, path, or graph-edge witness"
	// (§4.4), and "values inside each dimension are canonical lexical
	// order" — so a repeated or out-of-order value is a malformed witness,
	// never something to silently deduplicate or re-sort.
	if err := requireSortedUniqueStrings(field+".left", d.Left); err != nil {
		return err
	}
	if err := requireSortedUniqueStrings(field+".right", d.Right); err != nil {
		return err
	}
	if err := requireSortedUniqueStrings(field+".intersection", d.Intersection); err != nil {
		return err
	}
	return requireSortedUniqueStrings(field+".witnesses", d.Witnesses)
}

func validateScopeProof(field string, s ScopeProof) error {
	if err := s.State.Validate(); err != nil {
		return fmt.Errorf("policyconflict: %s.state: %w", field, err)
	}
	if s.Dimensions == nil {
		return fmt.Errorf("policyconflict: %s.dimensions: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, d := range s.Dimensions {
		if err := validateDimensionProof(fmt.Sprintf("%s.dimensions[%d]", field, i), d); err != nil {
			return err
		}
	}
	prev := -1
	for i, d := range s.Dimensions {
		rank, _ := scopeDimensionRank(d.Dimension) // known: checked above
		switch {
		case rank == prev:
			return fmt.Errorf("policyconflict: %s.dimensions: duplicate dimension %q", field, d.Dimension)
		case rank < prev:
			return fmt.Errorf("policyconflict: %s.dimensions: must be in phase, environment, path, ref order (found %q after %q)", field, d.Dimension, s.Dimensions[i-1].Dimension)
		}
		prev = rank
	}
	return nil
}

func validateSolverProof(field string, s SolverProof) error {
	if err := s.State.Validate(); err != nil {
		return fmt.Errorf("policyconflict: %s.state: %w", field, err)
	}
	if err := validateNonEmpty(field+".domain", s.Domain); err != nil {
		return err
	}
	// Same canonical-lexical-order rule as a dimension proof's witness sets
	// (§4.4): a solver proof's domain sets are evidence, so a duplicate or
	// out-of-order entry fails closed rather than being normalized away.
	if err := requireSortedUniqueStrings(field+".values", s.Values); err != nil {
		return err
	}
	if err := requireSortedUniqueStrings(field+".required", s.Required); err != nil {
		return err
	}
	if err := requireSortedUniqueStrings(field+".forbidden", s.Forbidden); err != nil {
		return err
	}
	return requireSortedUniqueStrings(field+".witnesses", s.Witnesses)
}

// authorityResolutionMembers is the fixed report order of an authority
// resolution's five states (authority design §10: "match, freshness, scope,
// bound, and authorization states"). Iterating a map here would make the
// reported member nondeterministic whenever two are simultaneously invalid.
func authorityResolutionMembers(r AuthorityResolution) [5]struct {
	name  string
	state ProofState
} {
	return [5]struct {
		name  string
		state ProofState
	}{
		{"match", r.Match},
		{"freshness", r.Freshness},
		{"scope", r.Scope},
		{"bound", r.Bound},
		{"authorization", r.Authorization},
	}
}

func validateAuthorityResolution(field string, r AuthorityResolution) error {
	for _, m := range authorityResolutionMembers(r) {
		if err := m.state.Validate(); err != nil {
			return fmt.Errorf("policyconflict: %s.%s: %w", field, m.name, err)
		}
	}
	return nil
}

func validateTypedClaimRecord(field string, r TypedClaimRecord) error {
	if err := validateNonEmpty(field+".policy_id", r.PolicyID); err != nil {
		return err
	}
	if err := validateDigest(field+".policy_digest", r.PolicyDigest); err != nil {
		return err
	}
	if err := validateDigest(field+".claim_digest", r.ClaimDigest); err != nil {
		return err
	}
	return r.Claim.Validate()
}

func validateExemptionResolution(field string, e ExemptionResolution) error {
	if err := validateNonEmpty(field+".id", e.ID); err != nil {
		return err
	}
	if err := validateDigest(field+".digest", e.Digest); err != nil {
		return err
	}
	if err := validateAuthorityResolution(field+".resolution", e.Resolution); err != nil {
		return err
	}
	if len(e.RemovedClaims) < 1 {
		return fmt.Errorf("policyconflict: %s.removed_claims: must name at least one removed claim", field)
	}
	for i, c := range e.RemovedClaims {
		if err := c.validate(fmt.Sprintf("%s.removed_claims[%d]", field, i)); err != nil {
			return err
		}
	}
	return requireSortedUnique(field+".removed_claims", e.RemovedClaims, func(c ClaimWitness) string { return c.ID })
}

func validateMechanicalEvaluation(field string, m MechanicalEvaluation) error {
	if err := validateNonEmpty(field+".id", m.ID); err != nil {
		return err
	}
	if err := m.Family.Validate(); err != nil {
		return fmt.Errorf("policyconflict: %s.family: %w", field, err)
	}
	if err := validateNonEmpty(field+".subject", m.Subject); err != nil {
		return err
	}
	if len(m.Claims) < 1 {
		return fmt.Errorf("policyconflict: %s.claims: must name at least one claim", field)
	}
	for i, c := range m.Claims {
		if err := validateTypedClaimRecord(fmt.Sprintf("%s.claims[%d]", field, i), c); err != nil {
			return err
		}
	}
	if err := requireSortedUnique(field+".claims", m.Claims, func(c TypedClaimRecord) string { return c.ClaimDigest }); err != nil {
		return err
	}
	if err := validateScopeProof(field+".scope", m.Scope); err != nil {
		return err
	}
	if err := validateNonEmpty(field+".domain", m.Domain); err != nil {
		return err
	}
	if err := validateSolverProof(field+".before", m.Before); err != nil {
		return err
	}
	if m.Exemptions == nil {
		return fmt.Errorf("policyconflict: %s.exemptions: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, e := range m.Exemptions {
		if err := validateExemptionResolution(fmt.Sprintf("%s.exemptions[%d]", field, i), e); err != nil {
			return err
		}
	}
	if err := requireSortedUnique(field+".exemptions", m.Exemptions, func(e ExemptionResolution) string { return e.ID }); err != nil {
		return err
	}
	if err := validateSolverProof(field+".after", m.After); err != nil {
		return err
	}
	if err := m.State.Validate(); err != nil {
		return fmt.Errorf("policyconflict: %s.state: %w", field, err)
	}
	return validateReasons(field+".reasons", m.Reasons)
}

func validateDispositionResolution(field string, d DispositionResolution) error {
	if err := validateNonEmpty(field+".id", d.ID); err != nil {
		return err
	}
	if err := validateDigest(field+".digest", d.Digest); err != nil {
		return err
	}
	if err := validateDispositionConclusion(field+".conclusion", d.Conclusion); err != nil {
		return err
	}
	return validateAuthorityResolution(field+".resolution", d.Resolution)
}

// validateDispositionConclusion closed-vocabulary-checks c via
// policyartifact.DispositionConclusion.Validate — the exported seam
// authorized by controller adjudication specifically for this embedding
// (Task 3 plan / authority design §8/§10; ledger SI-93/SI-96/SI-99/SI-103).
// policyconflict never duplicates policyartifact's private
// knownDispositionConclusions map; this is the one call site that reaches
// it.
func validateDispositionConclusion(field string, c policyartifact.DispositionConclusion) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("policyconflict: %s: %w", field, err)
	}
	return nil
}

// validateSemanticClaims validates every embedded
// policyartifact.SemanticClaimWitness via that type's own exported
// Validate — including its closed §6 source-category vocabulary — plus
// this package's own ordering rules: sorted-unique claim IDs (authority
// design §10: "sorted-unique witness-ID order") and, for each witness's
// inherited scope, the same sorted-unique member rule validateScope applies
// to every other scope in a report. Ordering is policyconflict's own
// report-row concern: policyartifact's Scope.Validate proves membership and
// uniqueness but not canonical lexical order, so an unsorted witness scope
// would otherwise slip into a canonical report. The exported
// SemanticClaimWitness.Validate seam is the same controller-authorized
// addition as validateDispositionConclusion above; policyconflict never
// duplicates policyartifact's private knownWitnessCategories map.
func validateSemanticClaims(field string, claims []policyartifact.SemanticClaimWitness) error {
	if len(claims) < 1 {
		return fmt.Errorf("policyconflict: %s: must name at least one claim", field)
	}
	for i, c := range claims {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("policyconflict: %s[%d]: %w", field, i, err)
		}
		if err := validateScope(fmt.Sprintf("%s[%d].scope", field, i), c.Scope); err != nil {
			return err
		}
	}
	return requireSortedUnique(field, claims, func(c policyartifact.SemanticClaimWitness) string { return c.ID })
}

func validateSemanticEvaluation(field string, s SemanticEvaluation) error {
	if err := validateNonEmpty(field+".id", s.ID); err != nil {
		return err
	}
	if err := validateDigest(field+".input_id", s.InputID); err != nil {
		return err
	}
	if err := validateSemanticClaims(field+".claims", s.Claims); err != nil {
		return err
	}
	if s.UnknownScopes == nil {
		return fmt.Errorf("policyconflict: %s.unknown_scopes: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, u := range s.UnknownScopes {
		if err := validateScopeProof(fmt.Sprintf("%s.unknown_scopes[%d]", field, i), u); err != nil {
			return err
		}
	}
	if s.Primary != nil {
		if s.Primary.Role != JudgePrimary {
			return fmt.Errorf("policyconflict: %s.primary.role: must be %q", field, JudgePrimary)
		}
		if err := s.Primary.validate(); err != nil {
			return fmt.Errorf("policyconflict: %s.primary: %w", field, err)
		}
	}
	if s.Challenger != nil {
		if s.Challenger.Role != JudgeChallenger {
			return fmt.Errorf("policyconflict: %s.challenger.role: must be %q", field, JudgeChallenger)
		}
		if err := s.Challenger.validate(); err != nil {
			return fmt.Errorf("policyconflict: %s.challenger: %w", field, err)
		}
	}
	if s.Dispositions == nil {
		return fmt.Errorf("policyconflict: %s.dispositions: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, d := range s.Dispositions {
		if err := validateDispositionResolution(fmt.Sprintf("%s.dispositions[%d]", field, i), d); err != nil {
			return err
		}
	}
	if err := requireSortedUnique(field+".dispositions", s.Dispositions, func(d DispositionResolution) string { return d.ID }); err != nil {
		return err
	}
	if err := s.State.Validate(); err != nil {
		return fmt.Errorf("policyconflict: %s.state: %w", field, err)
	}
	return validateReasons(field+".reasons", s.Reasons)
}

func validateDisclosure(field string, d Disclosure) error {
	if err := validateDisclosureCode(d.Code); err != nil {
		return fmt.Errorf("policyconflict: %s.code: %w", field, err)
	}
	if d.Witnesses == nil {
		return fmt.Errorf("policyconflict: %s.witnesses: must be non-nil (an explicitly empty set is [])", field)
	}
	return requireSortedUniqueStrings(field+".witnesses", d.Witnesses)
}

// Validate checks r's complete grammar. Digest is only format-checked when
// non-empty; EncodeReport/DecodeReport own the actual self-digest
// recomputation and verification.
func (r Report) Validate() error {
	if r.Schema != ReportSchema {
		return fmt.Errorf("policyconflict: report: schema %q, want %q", r.Schema, ReportSchema)
	}
	if err := r.Input.validate(); err != nil {
		return err
	}
	if r.Mechanical == nil {
		return fmt.Errorf("policyconflict: report.mechanical: must be non-nil (an explicitly empty set is [])")
	}
	for i, m := range r.Mechanical {
		if err := validateMechanicalEvaluation(fmt.Sprintf("report.mechanical[%d]", i), m); err != nil {
			return err
		}
	}
	if err := requireSortedUnique("report.mechanical", r.Mechanical, func(m MechanicalEvaluation) string { return m.ID }); err != nil {
		return err
	}
	if r.Semantic == nil {
		return fmt.Errorf("policyconflict: report.semantic: must be non-nil (an explicitly empty set is [])")
	}
	for i, s := range r.Semantic {
		if err := validateSemanticEvaluation(fmt.Sprintf("report.semantic[%d]", i), s); err != nil {
			return err
		}
	}
	if err := requireSortedUnique("report.semantic", r.Semantic, func(s SemanticEvaluation) string { return s.ID }); err != nil {
		return err
	}
	if r.Disclosures == nil {
		return fmt.Errorf("policyconflict: report.disclosures: must be non-nil (an explicitly empty set is [])")
	}
	for i, d := range r.Disclosures {
		if err := validateDisclosure(fmt.Sprintf("report.disclosures[%d]", i), d); err != nil {
			return err
		}
	}
	if err := requireSortedUnique("report.disclosures", r.Disclosures, func(d Disclosure) string { return string(d.Code) }); err != nil {
		return err
	}
	if err := r.Verdict.Validate(); err != nil {
		return fmt.Errorf("policyconflict: report.verdict: %w", err)
	}
	if r.Digest != "" {
		if err := validateDigest("report.digest", r.Digest); err != nil {
			return err
		}
	}
	return nil
}

// rawContentDigest mirrors internal/contextcompile's own identical helper:
// the raw_digest convention addresses exact bytes directly, a distinct
// notion from canonjson.Digest's canonical-value digest.
func rawContentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
