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
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

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
const semanticPrompt = `You are evaluating a closed set of normalized, human-authored authority claims for policy conflict.

Each claim below carries its own id, category, scope, governing authority digest, and normalized text. Some claims are typed constraints whose exact scope relationship to the group could not be proven mechanically and are included here only as unknown-scope witnesses (id/digest/category, no free text); every other claim is prose and its full authored text is included.

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
	Claims        []contextcompile.ProseClaim
	UnknownScopes []ScopeProof
	Exemptions    []policyartifact.SemanticExemptionWitness
	Prompt        []byte
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
	return SemanticInput{Claims: claims, UnknownScopes: unknown, Exemptions: exemptions, Prompt: prompt}, nil
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
		// Every contextcompile prose-claim builder sets ID == LineIdentity
		// (policy-instruction/spec/fragment/adr-decision claims further
		// compose it as SourceRef+"#"+Object; obligation-declaration claims
		// use the bare obligation ref instead, with Object naming the bound
		// AC separately) — ID==LineIdentity is the one invariant every
		// category shares, so that is what this defends, rather than a
		// single ref#object formula that would wrongly reject a legitimate
		// obligation-declaration claim.
		if c.ID != c.LineIdentity {
			return nil, fmt.Errorf("policyconflict: %s.id: %q does not match its own line identity %q", field, c.ID, c.LineIdentity)
		}
		if err := validateProseClaimScope(field+".scope", c.Scope); err != nil {
			return nil, err
		}
		out = append(out, cloneProseClaim(c))
	}
	return out, nil
}

// validateProseClaimScope checks a ProseClaim's own inherited scope. It
// reuses policyartifact.Scope's real phase/environment/path grammar (by
// validating a copy with Refs cleared, since that grammar's own dimensions
// are unaffected by what Refs carries) but does NOT run the shared
// validateScope/Scope.Validate ref-grammar check on Refs itself:
// ProseClaim.Scope.Refs carries the claim's own "ref#object" LINE IDENTITY
// (newProseClaim/buildPolicyInstructionProse/buildObligationProse all set
// it that way), a different vocabulary from a governing policy's own
// declared scope refs — artifact.ParseRef's fragment-object grammar
// requires a hyphenated object id (e.g. "ac-2", "instruction-1") and
// genuinely rejects the spec kernel's own fixed single-word anchors
// "problem"/"outcome"/"decision", which real Task 4 output legitimately
// carries. Refs here is instead required non-nil, sorted-unique, and
// non-blank per entry — structural well-formedness without imposing an
// object-id grammar this field was never that vocabulary.
func validateProseClaimScope(field string, s policyartifact.Scope) error {
	forGrammar := s
	forGrammar.Refs = []string{}
	if err := validateScope(field, forGrammar); err != nil {
		return err
	}
	if s.Refs == nil {
		return fmt.Errorf("policyconflict: %s.refs: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, r := range s.Refs {
		if err := validateNonEmpty(fmt.Sprintf("%s.refs[%d]", field, i), r); err != nil {
			return err
		}
	}
	return requireSortedUniqueStrings(field+".refs", s.Refs)
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

// unknownScopeWitnesses collects the ScopeProof of every evaluation row
// whose top-level scope state is unknown — the "mechanically unknown or
// conservatively unresolved scope witness" §6 names — in the evaluations'
// own (already row-ID-sorted) order, and requires evaluations to already
// arrive sorted-unique by id.
func unknownScopeWitnesses(evaluations []MechanicalEvaluation) ([]ScopeProof, error) {
	if err := requireSortedUnique("evaluations", evaluations, func(e MechanicalEvaluation) string { return e.ID }); err != nil {
		return nil, err
	}
	out := make([]ScopeProof, 0)
	for _, e := range evaluations {
		if err := validateScopeProof(fmt.Sprintf("evaluations[%s].scope", e.ID), e.Scope); err != nil {
			return nil, err
		}
		if e.Scope.State == ScopeUnknown {
			out = append(out, cloneScopeProof(e.Scope))
		}
	}
	return out, nil
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
// (Claims, UnknownScopes, Exemptions) — never Prompt, which is fixed
// repository code shared by every input and carries no per-evaluation
// identity. This value is never decoded by anything (it is hashed, not
// persisted as its own wire artifact), so it needs no json tags beyond
// determinism: canonjson.Marshal sorts object keys and fixes encoding
// regardless of struct field naming.
type semanticInputWitnessDoc struct {
	Claims        []contextcompile.ProseClaim
	UnknownScopes []ScopeProof
	Exemptions    []policyartifact.SemanticExemptionWitness
}

// semanticInputDigest returns in's canonical "sha256:<hex>" content
// address over its identity-bearing fields (authority design §6: "the one
// semantic-input ID from the complete normalized witness identity").
func semanticInputDigest(in SemanticInput) (string, error) {
	digest, err := canonjson.Digest(semanticInputWitnessDoc{
		Claims:        in.Claims,
		UnknownScopes: in.UnknownScopes,
		Exemptions:    in.Exemptions,
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
