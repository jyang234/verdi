package policyartifact

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// DispositionConclusion is a semantic disposition's closed verdict on the
// exact semantic input it witnesses (authority-design §8: "Conclusion is
// `conflict` or `no-conflict`"). Unknown values fail closed (co-2).
type DispositionConclusion string

const (
	DispositionConflict   DispositionConclusion = "conflict"
	DispositionNoConflict DispositionConclusion = "no-conflict"
)

var knownDispositionConclusions = map[DispositionConclusion]bool{
	DispositionConflict:   true,
	DispositionNoConflict: true,
}

// Validate reports whether c is one of the two closed conclusion values.
// Unknown values, including empty, fail closed. Wave-3's policy-conflict-
// gate authority design embeds DispositionConclusion directly into its own
// report schema (authority design §8/§10, ledger SI-93/SI-96/SI-99/SI-103);
// this is the one exported seam that lets that sibling package validate a
// conclusion without duplicating knownDispositionConclusions.
func (c DispositionConclusion) Validate() error {
	if !knownDispositionConclusions[c] {
		return fmt.Errorf("policyartifact: unknown disposition conclusion %q (known: conflict, no-conflict)", c)
	}
	return nil
}

// DispositionOrigin is the closed provenance of a semantic disposition:
// a validated judge exchange, or a human fallback recorded when no
// current judge result exists, a well-formed result is inconclusive, or
// primary and challenger disagree (§8). Unknown values fail closed.
type DispositionOrigin string

const (
	DispositionJudgeResult   DispositionOrigin = "judge-result"
	DispositionHumanFallback DispositionOrigin = "human-fallback"
)

var knownDispositionOrigins = map[DispositionOrigin]bool{
	DispositionJudgeResult:   true,
	DispositionHumanFallback: true,
}

// knownWitnessCategories is the closed source-category vocabulary §6
// fixes for the prose claims a disposition's witness may name: an
// authored policy instruction, a target spec's problem/outcome or
// AC/open-question/constraint/decision object (the same categories from
// each governing parent feature), an ADR decision, or an obligation
// declaration. This package never interprets a claim's own text — the
// witness records only its identity fields (AC-3) — so the category set
// is owned here purely as closed vocabulary, exactly as Family and
// Operator are.
var knownWitnessCategories = map[string]bool{
	"policy-instruction":     true,
	"spec-problem":           true,
	"spec-outcome":           true,
	"acceptance-criterion":   true,
	"open-question":          true,
	"constraint":             true,
	"decision":               true,
	"adr-decision":           true,
	"obligation-declaration": true,
}

// SemanticClaimWitness names one exact prose claim a disposition's
// semantic input included: its canonical source id and content digest,
// closed source category, governing authority digest, inherited scope,
// and typed values/bound when the claim carries them (§8: "every claim's
// identity fields required by AC-3 — claim ID and digest, category,
// scope, typed values/bounds when present, governing authority digest").
// It never carries the claim's authored text.
type SemanticClaimWitness struct {
	ID              string   `json:"id"`
	Digest          string   `json:"digest"`
	Category        string   `json:"category"`
	AuthorityDigest string   `json:"authority_digest"`
	Scope           Scope    `json:"scope"`
	Values          []string `json:"values"`
	Bound           *int     `json:"bound,omitempty"`
}

// ValidateWitnessCategory reports whether category is a member of the
// closed §6 source-category vocabulary (knownWitnessCategories). Unknown
// values fail closed. Wave-3's policy-conflict-gate authority design
// embeds SemanticClaimWitness directly into its own report schema
// (authority design §6/§8, ledger SI-93/SI-96/SI-99/SI-103); this is the
// one exported seam that lets that sibling package validate a witness's
// category without duplicating the private vocabulary above.
func ValidateWitnessCategory(category string) error {
	if err := validateWitnessCategory(category); err != nil {
		return fmt.Errorf("policyartifact: %w", err)
	}
	return nil
}

// validateWitnessCategory is the same check without the package prefix, for
// in-package callers whose own error already carries one (doubling it would
// read "policyartifact: ...: policyartifact: ..."). ValidateWitnessCategory
// adds the prefix for cross-package callers, which have none of their own.
func validateWitnessCategory(category string) error {
	if !knownWitnessCategories[category] {
		return fmt.Errorf("unknown witness category %q", category)
	}
	return nil
}

// Validate checks w's complete per-claim grammar: category-specific semantic
// id and scope-ref identity, sha256 digest, closed §6 category vocabulary (the
// same check ValidateWitnessCategory exports), sha256 authority digest, strict
// phase/environment/path scope dimensions, and non-empty values entries.
// Policy instructions alone retain ordinary Scope validation because their
// scope is inherited rather than narrowed to the instruction line. Its errors
// are unprefixed field-scoped fragments; every caller supplies its own
// package/field label.
// Bound is unconstrained. This expresses exactly the rule set
// decodeSemanticWitness already applies to each witness claim; it is
// exported for the same reason ValidateWitnessCategory is (see its doc
// comment) — decodeSemanticWitness delegates to it below so the two call
// sites cannot drift.
func (w SemanticClaimWitness) Validate() error {
	if err := singleLineNonBlank(w.ID); err != nil {
		return fmt.Errorf("id: %w", err)
	}
	if !sha256Re.MatchString(w.Digest) {
		return fmt.Errorf("digest %q is not sha256:<64 hex> form", w.Digest)
	}
	if err := validateWitnessCategory(w.Category); err != nil {
		return err
	}
	if !sha256Re.MatchString(w.AuthorityDigest) {
		return fmt.Errorf("authority_digest %q is not sha256:<64 hex> form", w.AuthorityDigest)
	}
	if err := validateSemanticClaimScope(w.ID, w.Category, w.Scope); err != nil {
		return err
	}
	for i, v := range w.Values {
		if v == "" {
			return fmt.Errorf("values[%d]: empty value", i)
		}
	}
	return nil
}

// validateSemanticClaimScope enforces SI-112's closed line-identity grammar
// without widening artifact.ParseRef. Problem, outcome, and ADR-body decision
// anchors are semantic line identities, not general declared-object fragments.
func validateSemanticClaimScope(id, category string, scope Scope) error {
	switch category {
	case "policy-instruction":
		source, object, err := semanticLineIdentityParts(id)
		if err != nil {
			return err
		}
		if _, err := parseKindedID(source, KindPolicy); err != nil {
			return fmt.Errorf("id: policy-instruction source: %w", err)
		}
		if !positiveInstructionObject(object) {
			return fmt.Errorf("id: policy-instruction requires instruction-<positive-n>, got %q", object)
		}
		return scope.Validate()

	case "spec-problem", "spec-outcome", "acceptance-criterion", "open-question", "constraint", "decision":
		source, object, err := semanticLineIdentityParts(id)
		if err != nil {
			return err
		}
		ref, err := artifact.ParseRef(source)
		if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
			return fmt.Errorf("id: %q must use an unpinned whole spec ref", id)
		}
		if err := validateSpecSemanticObject(category, source, object); err != nil {
			return err
		}
		return validateSoleSemanticRef(id, scope)

	case "adr-decision":
		source, object, err := semanticLineIdentityParts(id)
		if err != nil {
			return err
		}
		ref, err := artifact.ParsePinnedRef(source)
		if err != nil || ref.Kind != artifact.KindADR || ref.Fragment() {
			return fmt.Errorf("id: %q must use an exact pinned whole ADR ref", id)
		}
		if object != "decision" {
			return fmt.Errorf("id: adr-decision requires suffix %q, got %q", "decision", object)
		}
		return validateSoleSemanticRef(id, scope)

	case "obligation-declaration":
		ref, err := artifact.ParseRef(id)
		if err != nil || ref.Kind != artifact.KindObligation || ref.Pinned() || ref.Fragment() {
			return fmt.Errorf("id: %q must be an unpinned whole obligation ref", id)
		}
		return validateSoleSemanticRef(id, scope)
	}
	return fmt.Errorf("unknown witness category %q", category)
}

func semanticLineIdentityParts(id string) (string, string, error) {
	source, object, ok := strings.Cut(id, "#")
	if !ok || source == "" || object == "" || strings.Contains(object, "#") {
		return "", "", fmt.Errorf("id: %q must have exact <source-ref>#<object> form", id)
	}
	return source, object, nil
}

func positiveInstructionObject(object string) bool {
	n := strings.TrimPrefix(object, "instruction-")
	if n == object || n == "" || n[0] == '0' {
		return false
	}
	for _, r := range n {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validateSpecSemanticObject(category, source, object string) error {
	switch category {
	case "spec-problem":
		if object != "problem" {
			return fmt.Errorf("id: spec-problem requires suffix %q, got %q", "problem", object)
		}
		return nil
	case "spec-outcome":
		if object != "outcome" {
			return fmt.Errorf("id: spec-outcome requires suffix %q, got %q", "outcome", object)
		}
		return nil
	}
	var prefix string
	switch category {
	case "acceptance-criterion":
		prefix = "ac-"
	case "open-question":
		prefix = "oq-"
	case "constraint":
		prefix = "co-"
	case "decision":
		prefix = "dc-"
	default:
		return fmt.Errorf("unknown witness category %q", category)
	}
	if !strings.HasPrefix(object, prefix) {
		return fmt.Errorf("id: category %q requires an %s* suffix, got %q", category, prefix, object)
	}
	if _, err := artifact.ParseRef(source + "#" + object); err != nil {
		return fmt.Errorf("id: %q is not a canonical declared-object identity: %w", object, err)
	}
	return nil
}

func validateSoleSemanticRef(id string, scope Scope) error {
	grammar := scope
	grammar.Refs = []string{}
	if err := grammar.Validate(); err != nil {
		return err
	}
	if len(scope.Refs) != 1 || scope.Refs[0] != id {
		return fmt.Errorf("scope.refs: must be exactly [%q], got %v", id, scope.Refs)
	}
	return nil
}

// SemanticExemptionWitness names one exemption id/digest a disposition's
// semantic input applied (§8: "every applicable exemption ID/digest").
type SemanticExemptionWitness struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// SemanticWitness is the complete semantic-input identity a disposition
// binds to: the target/candidate identity digest, and the exact sorted
// claim and exemption witness sets. InputID is the Task 7 canonical
// runtime digest over the complete normalized prose claims,
// unknown-mechanical witnesses, and applicable exemption identities (§8)
// — a digest over strictly more than this witness's own smaller
// claim/exemption/target projection can express. DecodeDisposition
// validates only InputID's digest form; it can no longer prove, and must
// not fabricate, agreement with the witness content it accompanies
// (SI-114). internal/policyconflict is the sole seam that compares
// InputID to the current runtime semantic input.
type SemanticWitness struct {
	InputID      string                     `json:"input_id"`
	TargetDigest string                     `json:"target_digest"`
	Claims       []SemanticClaimWitness     `json:"claims"`
	Exemptions   []SemanticExemptionWitness `json:"exemptions"`
}

// JudgmentProvenance cites the immutable primary and (when present)
// challenger judgment-record digests that informed a judge-result
// disposition, as provenance only — these citations never redefine
// freshness or make the human ruling depend on a repeatable model
// response (§8).
type JudgmentProvenance struct {
	PrimaryDigest    string `json:"primary_digest"`
	ChallengerDigest string `json:"challenger_digest,omitempty"`
}

// Disposition is one policy-disposition artifact (§8): a human ruling on
// an exact, current semantic input, bound to that witness, carrying its
// conclusion, provenance, and (for a human-fallback ruling) the same
// bounded-departure grammar an exemption uses.
type Disposition struct {
	Schema               string                `json:"schema"`
	ID                   string                `json:"id"`
	Kind                 string                `json:"kind"`
	Title                string                `json:"title"`
	Owners               []string              `json:"owners"`
	Scope                Scope                 `json:"scope"`
	Witness              SemanticWitness       `json:"witness"`
	Conclusion           DispositionConclusion `json:"conclusion"`
	Origin               DispositionOrigin     `json:"origin"`
	Judgment             *JudgmentProvenance   `json:"judgment,omitempty"`
	CompensatingControls []string              `json:"compensating_controls"`
	Approvals            []Approval            `json:"approvals"`
	Expiry               string                `json:"expiry,omitempty"`
	ReviewCondition      string                `json:"review_condition,omitempty"`
	Template             *TemplateRecord       `json:"template,omitempty"`
	Rationale            string                `json:"rationale"`

	seal string
}

// semanticClaimWitnessDoc is SemanticClaimWitness's strict decode target;
// bound is the only optional field.
type semanticClaimWitnessDoc struct {
	ID              *string   `yaml:"id"`
	Digest          *string   `yaml:"digest"`
	Category        *string   `yaml:"category"`
	AuthorityDigest *string   `yaml:"authority_digest"`
	Scope           *scopeDoc `yaml:"scope"`
	Values          *[]string `yaml:"values"`
	Bound           *int      `yaml:"bound"`
}

// semanticExemptionWitnessDoc is SemanticExemptionWitness's strict decode
// target.
type semanticExemptionWitnessDoc struct {
	ID     *string `yaml:"id"`
	Digest *string `yaml:"digest"`
}

// semanticWitnessDoc is SemanticWitness's strict decode target: claims
// and exemptions are both mandatory keys, but exemptions' explicit empty
// list is legal — a semantic input always carries claims but may name no
// applicable exemption.
type semanticWitnessDoc struct {
	InputID      *string                        `yaml:"input_id"`
	TargetDigest *string                        `yaml:"target_digest"`
	Claims       *[]semanticClaimWitnessDoc     `yaml:"claims"`
	Exemptions   *[]semanticExemptionWitnessDoc `yaml:"exemptions"`
}

// judgmentProvenanceDoc is JudgmentProvenance's strict decode target;
// challenger_digest is the only optional field.
type judgmentProvenanceDoc struct {
	PrimaryDigest    *string `yaml:"primary_digest"`
	ChallengerDigest *string `yaml:"challenger_digest"`
}

// dispositionDoc is Disposition's strict decode target. judgment, expiry,
// review_condition, and compensating_controls are the only optional
// keys — a judge-result ruling needs no fallback ceremony (§8), so their
// absence is legal there; origin-specific requirements are enforced
// after decode, in DecodeDisposition.
type dispositionDoc struct {
	kernelDoc            `yaml:",inline"`
	Scope                *scopeDoc              `yaml:"scope"`
	Witness              *semanticWitnessDoc    `yaml:"witness"`
	Conclusion           *string                `yaml:"conclusion"`
	Origin               *string                `yaml:"origin"`
	Judgment             *judgmentProvenanceDoc `yaml:"judgment"`
	CompensatingControls *[]string              `yaml:"compensating_controls"`
	Approvals            *[]approvalDoc         `yaml:"approvals"`
	Expiry               *string                `yaml:"expiry"`
	ReviewCondition      *string                `yaml:"review_condition"`
}

// DecodeDisposition strictly decodes data as a verdi.policy-disposition/v1
// artifact, validates its semantic-witness and origin-specific grammar,
// normalizes it, and seals the result. It validates witness.input_id's
// digest form but does not self-derive or compare it against the
// artifact's own (smaller) claim/exemption/target projection — see
// SemanticWitness's doc comment (SI-114, §8).
func DecodeDisposition(data []byte) (*Disposition, error) {
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("policyartifact: %w", err)
	}
	var doc dispositionDoc
	if err := artifact.DecodeStrict(fm, &doc); err != nil {
		return nil, err
	}
	k, err := doc.toKernel(SchemaDisposition, KindDisposition)
	if err != nil {
		return nil, err
	}
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: disposition field %s is missing: every disposition field except judgment, expiry, review_condition, and compensating_controls is mandatory", field)
	}
	if doc.Scope == nil {
		return nil, missing("scope")
	}
	if doc.Witness == nil {
		return nil, missing("witness")
	}
	if doc.Conclusion == nil {
		return nil, missing("conclusion")
	}
	if doc.Origin == nil {
		return nil, missing("origin")
	}
	if doc.Approvals == nil {
		return nil, missing("approvals")
	}

	scope, err := doc.Scope.toScope("disposition.scope")
	if err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}

	witness, err := decodeSemanticWitness(*doc.Witness)
	if err != nil {
		return nil, err
	}

	conclusion := DispositionConclusion(*doc.Conclusion)
	if err := conclusion.Validate(); err != nil {
		return nil, err
	}

	origin := DispositionOrigin(*doc.Origin)
	if !knownDispositionOrigins[origin] {
		return nil, fmt.Errorf("policyartifact: unknown disposition origin %q (known: judge-result, human-fallback)", origin)
	}

	var judgment *JudgmentProvenance
	if doc.Judgment != nil {
		if origin != DispositionJudgeResult {
			return nil, fmt.Errorf("policyartifact: disposition judgment is only legal when origin is judge-result, got origin %q (a human-fallback disposition never carries judge provenance)", origin)
		}
		jd := doc.Judgment
		if jd.PrimaryDigest == nil {
			return nil, fmt.Errorf("policyartifact: disposition judgment.primary_digest is missing")
		}
		if !sha256Re.MatchString(*jd.PrimaryDigest) {
			return nil, fmt.Errorf("policyartifact: disposition judgment.primary_digest %q is not sha256:<64 hex> form", *jd.PrimaryDigest)
		}
		// challenger_digest is optional by ABSENCE only (§8: the challenger
		// citation is made "when present"). A key that is present must name
		// a real judgment record: an empty or malformed value would be a
		// citation wearing the shape of provenance, so it fails closed
		// rather than normalizing to "no challenger".
		challenger := ""
		if jd.ChallengerDigest != nil {
			challenger = *jd.ChallengerDigest
			if !sha256Re.MatchString(challenger) {
				return nil, fmt.Errorf("policyartifact: disposition judgment.challenger_digest %q is not sha256:<64 hex> form (omit the key entirely when no challenger judgment informed the ruling)", challenger)
			}
		}
		judgment = &JudgmentProvenance{PrimaryDigest: *jd.PrimaryDigest, ChallengerDigest: challenger}
	}

	// Compensating controls are optional by ABSENCE only (§8: "Compensating
	// controls, when present, remain nonempty author-ordered single-line
	// prose") — a judge-result ruling that names none omits the key rather
	// than declaring an empty list, so the artifact never states a control
	// set it does not have.
	var controls []string
	if doc.CompensatingControls != nil {
		if len(*doc.CompensatingControls) == 0 {
			return nil, fmt.Errorf("policyartifact: disposition compensating_controls is present but empty: controls, when present, are nonempty (omit the key entirely when the ruling names none)")
		}
		controls = *doc.CompensatingControls
	}
	controls = emptyIfNil(controls)
	// Compensating controls are authored ORDERED content, exactly like an
	// exemption's (exemption.go): never sorted or deduplicated.
	for i, c := range controls {
		if strings.TrimSpace(c) == "" {
			return nil, fmt.Errorf("policyartifact: disposition compensating_controls[%d]: empty control", i)
		}
		if strings.ContainsAny(c, "\n\r") {
			return nil, fmt.Errorf("policyartifact: disposition compensating_controls[%d]: a control must be a single line", i)
		}
	}

	expiry := ""
	if doc.Expiry != nil {
		expiry = *doc.Expiry
		if _, err := time.Parse("2006-01-02", expiry); err != nil {
			return nil, fmt.Errorf("policyartifact: disposition expiry %q is not a real YYYY-MM-DD calendar date", expiry)
		}
	}
	review := ""
	if doc.ReviewCondition != nil {
		review = *doc.ReviewCondition
		if strings.TrimSpace(review) == "" {
			return nil, fmt.Errorf("policyartifact: disposition review_condition must carry a named condition, not blank text")
		}
	}

	switch origin {
	case DispositionHumanFallback:
		// §8: "A `human-fallback` must carry a real calendar-date expiry
		// or nonblank review condition and at least one compensating
		// control."
		if len(controls) == 0 {
			return nil, fmt.Errorf("policyartifact: human-fallback disposition must name at least one compensating control")
		}
		if expiry == "" && review == "" {
			return nil, fmt.Errorf("policyartifact: human-fallback disposition must carry a real expiry or a review condition")
		}
	case DispositionJudgeResult:
		// §8: "A judge-result disposition needs no fallback-only control
		// or time bound" — controls/expiry/review_condition remain legal
		// but optional, and already validated above when present.
	}

	if len(*doc.Approvals) == 0 {
		return nil, fmt.Errorf("policyartifact: disposition must record at least one approval fact")
	}
	approvals := make([]Approval, 0, len(*doc.Approvals))
	seenApproval := make(map[string]bool, len(*doc.Approvals))
	for i, ad := range *doc.Approvals {
		if ad.Role == nil || ad.Principal == nil {
			return nil, fmt.Errorf("policyartifact: disposition approvals[%d]: role and principal are both required", i)
		}
		if !kebabRe.MatchString(*ad.Role) {
			return nil, fmt.Errorf("policyartifact: disposition approvals[%d]: role %q must be kebab-case", i, *ad.Role)
		}
		if err := governanceprincipal.PrincipalID(*ad.Principal).Validate(); err != nil {
			return nil, fmt.Errorf("policyartifact: disposition approvals[%d]: principal: %w", i, err)
		}
		key := *ad.Role + "\x00" + *ad.Principal
		if seenApproval[key] {
			return nil, fmt.Errorf("policyartifact: disposition approvals: duplicate approval (%s, %s)", *ad.Role, *ad.Principal)
		}
		seenApproval[key] = true
		approvals = append(approvals, Approval{Role: *ad.Role, Principal: *ad.Principal})
	}
	sort.Slice(approvals, func(i, j int) bool {
		if approvals[i].Role != approvals[j].Role {
			return approvals[i].Role < approvals[j].Role
		}
		return approvals[i].Principal < approvals[j].Principal
	})

	rationale, err := requireRationale(KindDisposition, body)
	if err != nil {
		return nil, err
	}

	d := &Disposition{
		Schema:               k.Schema,
		ID:                   k.ID,
		Kind:                 k.Kind,
		Title:                k.Title,
		Owners:               k.Owners,
		Scope:                scope,
		Witness:              witness,
		Conclusion:           conclusion,
		Origin:               origin,
		Judgment:             judgment,
		CompensatingControls: controls,
		Approvals:            approvals,
		Expiry:               expiry,
		ReviewCondition:      review,
		Template:             k.Template,
		Rationale:            rationale,
	}
	normalizeScope(&d.Scope)
	seal, err := canonjson.Digest(d)
	if err != nil {
		return nil, err
	}
	d.seal = seal
	return d, nil
}

// decodeSemanticWitness validates and normalizes wd into a SemanticWitness.
// input_id is validated for mandatory presence and sha256:<64 hex> digest
// shape only; this package no longer self-derives a second semantic-input
// identity from the witness's own smaller claim/exemption/target
// projection to compare against it (SI-114, §8) — internal/policyconflict
// alone proves agreement with the current runtime semantic input. Claims
// and exemptions must both arrive strictly sorted by id with no
// duplicates — an unsorted or duplicate witness set fails closed rather
// than being silently reordered, so a witness's on-disk byte order is
// never load-bearing evidence of anything this package re-derives.
func decodeSemanticWitness(wd semanticWitnessDoc) (SemanticWitness, error) {
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: disposition witness field %s is missing: every witness field is mandatory (exemptions' explicit empty set is [])", field)
	}
	if wd.InputID == nil {
		return SemanticWitness{}, missing("input_id")
	}
	if wd.TargetDigest == nil {
		return SemanticWitness{}, missing("target_digest")
	}
	if wd.Claims == nil {
		return SemanticWitness{}, missing("claims")
	}
	if wd.Exemptions == nil {
		return SemanticWitness{}, missing("exemptions")
	}
	if !sha256Re.MatchString(*wd.InputID) {
		return SemanticWitness{}, fmt.Errorf("policyartifact: disposition witness input_id %q is not sha256:<64 hex> form", *wd.InputID)
	}
	if !sha256Re.MatchString(*wd.TargetDigest) {
		return SemanticWitness{}, fmt.Errorf("policyartifact: disposition witness target_digest %q is not sha256:<64 hex> form", *wd.TargetDigest)
	}

	if len(*wd.Claims) == 0 {
		return SemanticWitness{}, fmt.Errorf("policyartifact: disposition witness must name at least one claim (a semantic input always witnesses claims)")
	}
	claims := make([]SemanticClaimWitness, 0, len(*wd.Claims))
	for i, cd := range *wd.Claims {
		if cd.ID == nil || cd.Digest == nil || cd.Category == nil || cd.AuthorityDigest == nil || cd.Scope == nil || cd.Values == nil {
			return SemanticWitness{}, fmt.Errorf("policyartifact: disposition witness claims[%d]: id, digest, category, authority_digest, scope, and values are all required (bound is optional)", i)
		}
		cScope, err := cd.Scope.toScope(fmt.Sprintf("disposition.witness.claims[%d].scope", i))
		if err != nil {
			return SemanticWitness{}, err
		}
		// witness's per-claim grammar (id shape, digest forms, closed
		// category vocabulary, scope validity, value-entry shape) is
		// delegated to SemanticClaimWitness.Validate — the exact rule set
		// this loop applied inline before that method was exported for
		// internal/policyconflict's sake, so the two call sites cannot
		// drift (see Validate's doc comment).
		witness := SemanticClaimWitness{
			ID:              *cd.ID,
			Digest:          *cd.Digest,
			Category:        *cd.Category,
			AuthorityDigest: *cd.AuthorityDigest,
			Scope:           cScope,
			Values:          emptyIfNil(*cd.Values),
			Bound:           cd.Bound,
		}
		if err := witness.Validate(); err != nil {
			return SemanticWitness{}, fmt.Errorf("policyartifact: disposition witness claims[%d]: %w", i, err)
		}
		normalizeScope(&witness.Scope)
		claims = append(claims, witness)
	}
	for i := 1; i < len(claims); i++ {
		if claims[i-1].ID >= claims[i].ID {
			return SemanticWitness{}, fmt.Errorf("policyartifact: disposition witness claims must arrive sorted by id with no duplicates (fail closed rather than silently reordering): %q then %q", claims[i-1].ID, claims[i].ID)
		}
	}

	exemptions := make([]SemanticExemptionWitness, 0, len(*wd.Exemptions))
	for i, ed := range *wd.Exemptions {
		if ed.ID == nil || ed.Digest == nil {
			return SemanticWitness{}, fmt.Errorf("policyartifact: disposition witness exemptions[%d]: id and digest are both required", i)
		}
		if _, err := parseKindedID(*ed.ID, KindExemption); err != nil {
			return SemanticWitness{}, fmt.Errorf("policyartifact: disposition witness exemptions[%d]: id: %w", i, err)
		}
		if !sha256Re.MatchString(*ed.Digest) {
			return SemanticWitness{}, fmt.Errorf("policyartifact: disposition witness exemptions[%d]: digest %q is not sha256:<64 hex> form", i, *ed.Digest)
		}
		exemptions = append(exemptions, SemanticExemptionWitness{ID: *ed.ID, Digest: *ed.Digest})
	}
	for i := 1; i < len(exemptions); i++ {
		if exemptions[i-1].ID >= exemptions[i].ID {
			return SemanticWitness{}, fmt.Errorf("policyartifact: disposition witness exemptions must arrive sorted by id with no duplicates (fail closed rather than silently reordering): %q then %q", exemptions[i-1].ID, exemptions[i].ID)
		}
	}

	w := SemanticWitness{
		InputID:      *wd.InputID,
		TargetDigest: *wd.TargetDigest,
		Claims:       claims,
		Exemptions:   exemptions,
	}
	return w, nil
}

// singleLineNonBlank enforces the shared single-line-prose identifier
// grammar a witness's claim id needs: non-blank, and free of control
// characters (kernel.go's title rule, reused rather than re-invented — a
// claim id may originate outside this store's own kebab-case convention,
// e.g. a spec's acceptance-criterion id, so it is checked for shape
// safety, not kebab-case).
func singleLineNonBlank(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("must not be blank")
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("contains a control character (U+%04X); a claim id is single-line prose", r)
		}
	}
	return nil
}

// Name returns the disposition id's name half.
func (d *Disposition) Name() string { return nameOf(d.ID) }

// Digest returns the disposition's canonical content address after
// proving the value is unmodified DecodeDisposition output.
func (d *Disposition) Digest() (string, error) {
	if err := d.checkSeal(); err != nil {
		return "", err
	}
	return d.seal, nil
}

func (d *Disposition) checkSeal() error {
	return checkSealed("disposition", d.ID, d.seal, d)
}
