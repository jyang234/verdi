// semantic.go builds the AC-3 semantic input a judge exchange evaluates and
// validates a judge's strict inner result against it (authority design
// §§6-7, ledger SI-96): BuildSemanticInput assembles the complete sorted
// prose-claim universe, every mechanically unknown scope witness, and the
// applicable exemption identities Task 6/6A already resolved onto the
// mechanical evaluation rows it is given, plus the one fixed repository
// prompt (never project configuration). ValidateJudgeResult is the witness
// cross-check §6 requires beyond a judge result's own self-contained wire
// grammar (already enforced by JudgeResult.Validate/DecodeJudgeResult): an
// unknown, digest-mismatched, or category-mismatched finding claim witness
// invalidates the result, because Verdi never trusts a model-supplied claim
// identity against anything but the exact witness set it was actually
// shown.
package policyconflict

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// semanticPrompt is the one canonical prompt every primary/challenger
// judge invocation sends (authority design §6: "Prompt bytes are fixed
// repository code, not project configuration"). It asks about overlap,
// simultaneous satisfiability, refinement, explicit exception, authority,
// and the strongest reasonable non-conflict interpretation — the exact six
// topics §6 names. Its bytes are a ratcheted constant: semantic_test.go
// pins them so an accidental edit here is caught as a diff, not silently
// shipped as a changed judge behavior.
// vocab:identity — "closed" names the judge protocol's fixed complete-input mechanism, not a lifecycle/display-state label.
const semanticPrompt = `You are evaluating a closed set of normalized, human-authored authority claims for policy conflict.

Each claim below carries its own id, category, scope, governing authority digest, and normalized text. Some claims are typed constraints whose exact scope relationship to the group could not be proven mechanically and are included as unknown mechanical witnesses with complete policy-bound typed claim records and exact scope proof; they carry no authored prose. Every other claim is prose and its full authored text is included.

Considering ALL claims and unknown-scope witnesses together as one group, and any exemption identities listed as already-authorized departures from named claims, determine:

1. Overlap: do any two or more claims apply to the same subject at the same time?
2. Simultaneous satisfiability: can every overlapping claim be honored at once, or does honoring one require violating another?
3. Refinement: does a narrower claim merely sharpen a broader one (not a conflict) rather than contradict it?
4. Explicit exception: does any listed exemption identity already authorize the specific departure an apparent conflict would otherwise require?
5. Authority: does one claim's governing authority explicitly subordinate or supersede another's for this exact subject?
6. Strongest reasonable non-conflict interpretation: is there a reading under which every claim can be satisfied simultaneously without contradiction?

Report your recommendation as exactly one of "conflict", "no-conflict", or "inconclusive". "conflict" requires at least one finding naming the specific claims that cannot be simultaneously satisfied and why. "no-conflict" requires an explicit empty findings list. "inconclusive" may include findings explaining what could not be resolved. Cite every claim you rely on by its exact given id; never invent an id, and never restate or summarize authored text as evidence beyond a single-line explanation per finding.
`

// SemanticInput is the complete, deterministic AC-3 semantic input one
// primary or challenger judge invocation evaluates (authority design §6):
// the sorted normalized prose-claim universe, every mechanically unknown
// scope witness, the applicable exemption identities/digests, and the
// fixed prompt.
type SemanticInput struct {
	Claims             []contextcompile.ProseClaim
	UnknownMechanicals []UnknownMechanicalWitness
	Exemptions         []policyartifact.SemanticExemptionWitness
	Prompt             []byte
}

// ValidatedExchange is one judge result that has passed ValidateJudgeResult
// (authority design §6): the exchange carrying that cross-checked result,
// and RecordDigest — the one semantic-input ID §6 says Verdi mints "from
// the complete normalized witness identity", never trusted from the judge.
// Exchange's transport-provenance fields (Role, Adapter, Model, and every
// digest but the input identity these two arguments cannot supply) are the
// caller's own responsibility to complete before persisting a Judgment —
// ValidateJudgeResult's narrow two-argument contract only certifies the
// RESULT against the INPUT it was shown.
type ValidatedExchange struct {
	Exchange     JudgmentExchange
	RecordDigest string
}

// BuildSemanticInput assembles view's prose-claim universe and evaluations'
// mechanically unknown scope/applicable-exemption witnesses into one
// SemanticInput (authority design §6). It defends against a hand-built or
// mutated ConflictView/evaluations slice the same way this package's other
// operand consumers do (authority design §3: "Hand-constructed,
// mutated-after-construction, digest-inconsistent... operands fail
// operationally"): every claim's category must be a legal §6 witness
// category (excluding any non-authoritative repository data that is not
// one of the nine closed categories), every claim's digest is recomputed
// and checked against its own text, every claim's line identity is
// recomputed and checked against its own ref/object, no claim text may
// carry a raw \r (a genuinely normalized claim was already CRLF-converted
// upstream; a \r's presence means the input was never normalized or was
// mutated after normalization), and both the claim and evaluation slices
// must already arrive sorted-unique by id — fail closed rather than
// silently reordering, matching this package's own established idiom
// (validate.go, policyartifact/disposition.go).
func BuildSemanticInput(view contextcompile.ConflictView, evaluations []MechanicalEvaluation) (SemanticInput, error) {
	claims, err := normalizedProseClaims(view.ProseClaims)
	if err != nil {
		return SemanticInput{}, fmt.Errorf("policyconflict: build semantic input: %w", err)
	}
	unknown, err := unknownScopeWitnesses(evaluations)
	if err != nil {
		return SemanticInput{}, fmt.Errorf("policyconflict: build semantic input: %w", err)
	}
	exemptions, err := applicableExemptionWitnesses(evaluations)
	if err != nil {
		return SemanticInput{}, fmt.Errorf("policyconflict: build semantic input: %w", err)
	}
	prompt := make([]byte, len(semanticPrompt))
	copy(prompt, semanticPrompt)
	return SemanticInput{Claims: claims, UnknownMechanicals: unknown, Exemptions: exemptions, Prompt: prompt}, nil
}

// validateSemanticInput defends the cache/launch boundary against a
// hand-built or mutated input. A launch is valid only for the fixed prompt
// and complete, explicitly-present witness sets BuildSemanticInput emits.
func validateSemanticInput(in SemanticInput) error {
	if !bytes.Equal(in.Prompt, []byte(semanticPrompt)) {
		return fmt.Errorf("policyconflict: semantic input prompt does not match the fixed repository prompt")
	}
	if in.Claims == nil {
		return fmt.Errorf("policyconflict: semantic input claims must be non-nil (an explicitly empty set is [])")
	}
	if _, err := normalizedProseClaims(in.Claims); err != nil {
		return fmt.Errorf("policyconflict: semantic input: %w", err)
	}
	if in.UnknownMechanicals == nil {
		return fmt.Errorf("policyconflict: semantic input unknown mechanicals must be non-nil (an explicitly empty set is [])")
	}
	for i, witness := range in.UnknownMechanicals {
		if err := validateUnknownMechanicalWitness(fmt.Sprintf("semantic input unknown_mechanicals[%d]", i), witness); err != nil {
			return err
		}
	}
	if err := requireSortedUnique("semantic input unknown_mechanicals", in.UnknownMechanicals, func(w UnknownMechanicalWitness) string { return w.ID }); err != nil {
		return err
	}
	if in.Exemptions == nil {
		return fmt.Errorf("policyconflict: semantic input exemptions must be non-nil (an explicitly empty set is [])")
	}
	for i, witness := range in.Exemptions {
		if err := validateNonEmpty(fmt.Sprintf("semantic input exemptions[%d].id", i), witness.ID); err != nil {
			return err
		}
		if err := validateDigest(fmt.Sprintf("semantic input exemptions[%d].digest", i), witness.Digest); err != nil {
			return err
		}
	}
	return requireSortedUnique("semantic input exemptions", in.Exemptions, func(w policyartifact.SemanticExemptionWitness) string { return w.ID })
}

// normalizedProseClaims clones and defensively re-validates view's prose
// claims (see BuildSemanticInput's doc comment for the exact checks), and
// requires the input already sorted-unique by id (authority design §6:
// "deterministic order").
func normalizedProseClaims(in []contextcompile.ProseClaim) ([]contextcompile.ProseClaim, error) {
	if err := requireSortedUnique("claims", in, func(c contextcompile.ProseClaim) string { return c.ID }); err != nil {
		return nil, err
	}
	out := make([]contextcompile.ProseClaim, 0, len(in))
	for i, c := range in {
		field := fmt.Sprintf("claims[%d]", i)
		if err := validateWitnessCategory(field+".category", c.Category); err != nil {
			return nil, err
		}
		if !utf8.ValidString(c.Text) {
			return nil, fmt.Errorf("policyconflict: %s.text: not valid UTF-8", field)
		}
		if c.Text == "" {
			return nil, fmt.Errorf("policyconflict: %s.text: must be non-empty", field)
		}
		if strings.ContainsRune(c.Text, '\r') {
			return nil, fmt.Errorf("policyconflict: %s.text: carries a raw CR — normalized authority prose is always CRLF-converted to LF before this package sees it", field)
		}
		if err := validateDigest(field+".text_digest", c.TextDigest); err != nil {
			return nil, err
		}
		if want := rawContentDigest([]byte(c.Text)); want != c.TextDigest {
			return nil, fmt.Errorf("policyconflict: %s.text_digest: %q does not match the exact bytes carried in text (want %q)", field, c.TextDigest, want)
		}
		if err := validateNonEmpty(field+".source_ref", c.SourceRef); err != nil {
			return nil, err
		}
		if err := validateNonEmpty(field+".source_path", c.SourcePath); err != nil {
			return nil, err
		}
		if err := validateDigest(field+".source_digest", c.SourceDigest); err != nil {
			return nil, err
		}
		if err := validateDigest(field+".authority_digest", c.AuthorityDigest); err != nil {
			return nil, err
		}
		if err := validateNonEmpty(field+".object", c.Object); err != nil {
			return nil, err
		}
		if err := validateProseClaimIdentity(field, c); err != nil {
			return nil, err
		}
		out = append(out, cloneProseClaim(c))
	}
	return out, nil
}

var (
	policyProseSourceRe = regexp.MustCompile(`^policy/[a-z0-9]+(?:-[a-z0-9]+)*$`)
	instructionObjectRe = regexp.MustCompile(`^instruction-[1-9][0-9]*$`)
)

// validateProseClaimIdentity enforces SI-112's closed, category-specific
// line-identity grammar against the exact contextcompile producers. These
// line identities are deliberately not fed to artifact.ParseRef where the
// kernel/body anchors problem, outcome, and decision are not general
// declared-object fragments.
func validateProseClaimIdentity(field string, c contextcompile.ProseClaim) error {
	switch c.Category {
	case "policy-instruction":
		if !policyProseSourceRe.MatchString(c.SourceRef) {
			return fmt.Errorf("policyconflict: %s.source_ref: %q is not a canonical policy/<name> identity", field, c.SourceRef)
		}
		if !instructionObjectRe.MatchString(c.Object) {
			return fmt.Errorf("policyconflict: %s.object: %q is not instruction-<positive-n> form", field, c.Object)
		}
		if err := requireProseLineIdentity(field, c, c.SourceRef+"#"+c.Object); err != nil {
			return err
		}
		// Policy instructions retain the policy claim's declared scope; it
		// is not replaced by a one-line ref scope.
		return validateScope(field+".scope", c.Scope)

	case "spec-problem", "spec-outcome", "acceptance-criterion", "open-question", "constraint", "decision":
		ref, err := artifact.ParseRef(c.SourceRef)
		if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
			return fmt.Errorf("policyconflict: %s.source_ref: %q must be an unpinned whole spec ref", field, c.SourceRef)
		}
		if err := validateSpecProseObject(field+".object", c.Category, c.SourceRef, c.Object); err != nil {
			return err
		}
		line := c.SourceRef + "#" + c.Object
		if err := requireProseLineIdentity(field, c, line); err != nil {
			return err
		}
		return validateSoleLineScope(field+".scope", c.Scope, line)

	case "adr-decision":
		ref, err := artifact.ParsePinnedRef(c.SourceRef)
		if err != nil || ref.Kind != artifact.KindADR || ref.Fragment() {
			return fmt.Errorf("policyconflict: %s.source_ref: %q must be an exact pinned whole ADR ref", field, c.SourceRef)
		}
		if c.Object != "decision" {
			return fmt.Errorf("policyconflict: %s.object: adr-decision requires %q, got %q", field, "decision", c.Object)
		}
		line := c.SourceRef + "#decision"
		if err := requireProseLineIdentity(field, c, line); err != nil {
			return err
		}
		return validateSoleLineScope(field+".scope", c.Scope, line)

	case "obligation-declaration":
		ref, err := artifact.ParseRef(c.SourceRef)
		if err != nil || ref.Kind != artifact.KindObligation || ref.Pinned() || ref.Fragment() {
			return fmt.Errorf("policyconflict: %s.source_ref: %q must be an unpinned whole obligation ref", field, c.SourceRef)
		}
		_, acID, _, ok := artifact.SplitObligationName(ref.Name)
		if !ok || c.Object != acID {
			return fmt.Errorf("policyconflict: %s.object: %q does not match obligation ref %q's acceptance criterion %q", field, c.Object, c.SourceRef, acID)
		}
		if err := requireProseLineIdentity(field, c, c.SourceRef); err != nil {
			return err
		}
		return validateSoleLineScope(field+".scope", c.Scope, c.SourceRef)
	}
	return fmt.Errorf("policyconflict: %s.category: unknown semantic prose category %q", field, c.Category)
}

func validateSpecProseObject(field, category, sourceRef, object string) error {
	switch category {
	case "spec-problem":
		if object != "problem" {
			return fmt.Errorf("policyconflict: %s: spec-problem requires %q, got %q", field, "problem", object)
		}
		return nil
	case "spec-outcome":
		if object != "outcome" {
			return fmt.Errorf("policyconflict: %s: spec-outcome requires %q, got %q", field, "outcome", object)
		}
		return nil
	}
	prefix := map[string]string{
		"acceptance-criterion": "ac-",
		"open-question":        "oq-",
		"constraint":           "co-",
		"decision":             "dc-",
	}[category]
	if !strings.HasPrefix(object, prefix) {
		return fmt.Errorf("policyconflict: %s: category %q requires an %s* object, got %q", field, category, prefix, object)
	}
	if _, err := artifact.ParseRef(sourceRef + "#" + object); err != nil {
		return fmt.Errorf("policyconflict: %s: %q is not a canonical declared-object identity: %w", field, object, err)
	}
	return nil
}

func requireProseLineIdentity(field string, c contextcompile.ProseClaim, want string) error {
	if c.ID != want || c.LineIdentity != want {
		return fmt.Errorf("policyconflict: %s: source_ref/object require id and line_identity %q, got id=%q line_identity=%q", field, want, c.ID, c.LineIdentity)
	}
	return nil
}

func validateSoleLineScope(field string, scope policyartifact.Scope, line string) error {
	grammar := scope
	grammar.Refs = []string{}
	if err := validateScope(field, grammar); err != nil {
		return err
	}
	if len(scope.Refs) != 1 || scope.Refs[0] != line {
		return fmt.Errorf("policyconflict: %s.refs: must be exactly [%q], got %v", field, line, scope.Refs)
	}
	return nil
}

func cloneProseClaim(c contextcompile.ProseClaim) contextcompile.ProseClaim {
	c.Scope = policyartifact.Scope{
		Phases:       cloneStringSlice(c.Scope.Phases),
		Environments: cloneStringSlice(c.Scope.Environments),
		Paths:        cloneStringSlice(c.Scope.Paths),
		Refs:         cloneStringSlice(c.Scope.Refs),
	}
	return c
}

func cloneStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

// unknownScopeWitnesses collects the lossless witness of every row whose
// aggregate Scope.State is unknown, plus every conservatively unresolved
// higher-order-scope-unproven row even when its aggregate scope is disjoint
// (SI-110/SI-112). Only ID, complete cloned typed claims, and exact scope
// enter the witness; solver and authority-resolution state stay outside its
// identity.
func unknownScopeWitnesses(evaluations []MechanicalEvaluation) ([]UnknownMechanicalWitness, error) {
	if err := requireSortedUnique("evaluations", evaluations, func(e MechanicalEvaluation) string { return e.ID }); err != nil {
		return nil, err
	}
	out := make([]UnknownMechanicalWitness, 0)
	for i, e := range evaluations {
		unresolved := e.Scope.State == ScopeUnknown
		for _, reason := range e.Reasons {
			if reason == ReasonHigherOrderScopeUnproven {
				unresolved = true
				break
			}
		}
		if !unresolved {
			continue
		}
		w := UnknownMechanicalWitness{ID: e.ID, Claims: cloneTypedClaimRecords(e.Claims), Scope: cloneScopeProof(e.Scope)}
		if err := validateUnknownMechanicalWitness(fmt.Sprintf("evaluations[%d].unknown_mechanical", i), w); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

func cloneTypedClaimRecords(in []TypedClaimRecord) []TypedClaimRecord {
	if in == nil {
		return nil
	}
	out := make([]TypedClaimRecord, len(in))
	for i, record := range in {
		claim := record.Claim
		claim.Values = cloneStringSlice(claim.Values)
		if claim.Bound != nil {
			bound := *claim.Bound
			claim.Bound = &bound
		}
		claim.Scope = policyartifact.Scope{
			Phases:       cloneStringSlice(claim.Scope.Phases),
			Environments: cloneStringSlice(claim.Scope.Environments),
			Paths:        cloneStringSlice(claim.Scope.Paths),
			Refs:         cloneStringSlice(claim.Scope.Refs),
		}
		record.Claim = claim
		out[i] = record
	}
	return out
}

func cloneScopeProof(p ScopeProof) ScopeProof {
	dims := make([]DimensionProof, len(p.Dimensions))
	for i, d := range p.Dimensions {
		dims[i] = DimensionProof{
			Dimension:    d.Dimension,
			State:        d.State,
			Left:         cloneStringSlice(d.Left),
			Right:        cloneStringSlice(d.Right),
			Intersection: cloneStringSlice(d.Intersection),
			Witnesses:    cloneStringSlice(d.Witnesses),
		}
	}
	return ScopeProof{State: p.State, Dimensions: dims}
}

// applicableExemptionWitnesses unions every ExemptionResolution attached to
// evaluations (already resolved as applicable to those rows by Task 6/6A)
// into the sorted-unique-by-id policyartifact.SemanticExemptionWitness set
// §6's "applicable exemption identities/digests" names. The same exemption
// id carrying two different digests across rows is contradictory authority
// (a mutated or hand-built operand), not a duplicate to silently collapse.
func applicableExemptionWitnesses(evaluations []MechanicalEvaluation) ([]policyartifact.SemanticExemptionWitness, error) {
	byID := make(map[string]policyartifact.SemanticExemptionWitness)
	for _, e := range evaluations {
		for _, res := range e.Exemptions {
			if err := validateNonEmpty(fmt.Sprintf("evaluations[%s].exemptions[?].id", e.ID), res.ID); err != nil {
				return nil, err
			}
			if err := validateDigest(fmt.Sprintf("evaluations[%s].exemptions[%s].digest", e.ID, res.ID), res.Digest); err != nil {
				return nil, err
			}
			w := policyartifact.SemanticExemptionWitness{ID: res.ID, Digest: res.Digest}
			if prev, ok := byID[w.ID]; ok {
				if prev != w {
					return nil, fmt.Errorf("policyconflict: exemption %q carries two different digests across evaluations (%q and %q)", w.ID, prev.Digest, w.Digest)
				}
				continue
			}
			byID[w.ID] = w
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]policyartifact.SemanticExemptionWitness, 0, len(ids))
	for _, id := range ids {
		out = append(out, byID[id])
	}
	return out, nil
}

// semanticInputWitnessDoc is the exact deterministic shape
// semanticInputDigest hashes: the input's identity-bearing content
// (Claims, UnknownMechanicals, Exemptions) — never Prompt, which is fixed
// repository code shared by every input and carries no per-evaluation
// identity. This value is never decoded by anything (it is hashed, not
// persisted as its own wire artifact), so it needs no json tags beyond
// determinism: canonjson.Marshal sorts object keys and fixes encoding
// regardless of struct field naming.
type semanticInputWitnessDoc struct {
	Claims             []contextcompile.ProseClaim
	UnknownMechanicals []UnknownMechanicalWitness
	Exemptions         []policyartifact.SemanticExemptionWitness
}

// semanticInputDigest returns in's canonical "sha256:<hex>" content
// address over its identity-bearing fields (authority design §6: "the one
// semantic-input ID from the complete normalized witness identity").
func semanticInputDigest(in SemanticInput) (string, error) {
	digest, err := canonjson.Digest(semanticInputWitnessDoc{
		Claims:             in.Claims,
		UnknownMechanicals: in.UnknownMechanicals,
		Exemptions:         in.Exemptions,
	})
	if err != nil {
		return "", fmt.Errorf("policyconflict: digesting semantic input: %w", err)
	}
	return digest, nil
}

// ValidateJudgeResult cross-checks result's finding claim witnesses against
// input's actual claim universe (authority design §6: "An unknown,
// missing, duplicate, or digest-mismatched witness invalidates the
// result") after confirming result's own self-contained wire grammar
// (schema, recommendation/findings cardinality, per-finding shape) via
// JudgeResult.Validate. Duplicate witnesses WITHIN one finding are already
// refused by JudgeFinding's own grammar; this function additionally proves
// every cited witness names a REAL claim this exact input actually
// carried, with the exact digest and category input assigned it — a judge
// can never mint, rename, or reclassify a claim identity by asserting one.
func ValidateJudgeResult(input SemanticInput, result JudgeResult) (ValidatedExchange, error) {
	if err := result.Validate(); err != nil {
		return ValidatedExchange{}, fmt.Errorf("policyconflict: validate judge result: %w", err)
	}
	known := make(map[string]ClaimWitness, len(input.Claims))
	for _, c := range input.Claims {
		known[c.ID] = ClaimWitness{ID: c.ID, Digest: c.TextDigest, Category: c.Category}
	}
	for i, f := range result.Findings {
		for j, c := range f.Claims {
			kw, ok := known[c.ID]
			if !ok {
				return ValidatedExchange{}, fmt.Errorf("policyconflict: validate judge result: findings[%d].claims[%d]: claim witness %q is not present in the semantic input it was shown", i, j, c.ID)
			}
			if kw.Digest != c.Digest {
				return ValidatedExchange{}, fmt.Errorf("policyconflict: validate judge result: findings[%d].claims[%d]: claim witness %q digest %q does not match the semantic input's digest %q", i, j, c.ID, c.Digest, kw.Digest)
			}
			if kw.Category != c.Category {
				return ValidatedExchange{}, fmt.Errorf("policyconflict: validate judge result: findings[%d].claims[%d]: claim witness %q category %q does not match the semantic input's category %q", i, j, c.ID, c.Category, kw.Category)
			}
		}
	}
	digest, err := semanticInputDigest(input)
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("policyconflict: validate judge result: %w", err)
	}
	return ValidatedExchange{Exchange: JudgmentExchange{Result: result}, RecordDigest: digest}, nil
}
