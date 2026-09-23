package policyartifact

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// Exemption is one policy-exemption artifact — the single exemption
// artifact kind and home (DC-24; SI-4): a bounded, governed departure
// from named policy claims, carrying exact witnesses, ownership and
// approval facts, compensating controls, and an expiry or review
// condition (DC-8) — or, as its one other witness family, a departure
// from the review phase's three sealed-provenance inputs carried by a
// RequiredInputWitness, whose expiry is mandatory (unsealed-provenance
// exemption design §4; SI-241, SI-255). Lifecycle-wide accountability
// and escalation around this artifact belong to Guided Lifecycle and
// Governance; later-wave conflict evaluation decides when an exemption
// actually excuses a proven conflict. This package owns only the
// artifact.
type Exemption struct {
	Schema    string    `json:"schema"`
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Owners    []string  `json:"owners"`
	Scope     Scope     `json:"scope"`
	Witnesses []Witness `json:"witnesses"`
	// RequiredInput is the required-input witness family (unsealed-
	// provenance exemption design §4; SI-241, SI-255). An exemption
	// carries exactly one family: claim Witnesses, or this one witness —
	// in which case Witnesses is the empty non-nil slice. omitempty keeps
	// every claim-family exemption's canonical encoding, and therefore
	// its digest, byte-identical to the encoding before this field.
	RequiredInput        *RequiredInputWitness `json:"required_input,omitempty"`
	CompensatingControls []string              `json:"compensating_controls"`
	Approvals            []Approval            `json:"approvals"`
	Expiry               string                `json:"expiry,omitempty"`
	ReviewCondition      string                `json:"review_condition,omitempty"`
	Template             *TemplateRecord       `json:"template,omitempty"`
	Rationale            string                `json:"rationale"`

	seal string
}

// Witness names one exact departed-from claim: the governing policy id,
// the claim id, and the claim's canonical content digest (ClaimDigest),
// so a later claim change visibly stales the exemption instead of
// silently widening it.
type Witness struct {
	Policy      string `yaml:"policy" json:"policy"`
	Claim       string `yaml:"claim" json:"claim"`
	ClaimDigest string `yaml:"claim_digest" json:"claim_digest"`
}

// Approval records one approval fact: the approving role and the
// approver's canonical kernel principal identity (DC-17: a process-local
// username or agent assertion is never authoritative identity, so the
// stored fact carries the kernel's canonical principal ID grammar).
// Whether the recorded principal actually satisfies the governing
// profile's separation-of-duties mode is kernel authorization work
// consumed by later gates, never re-interpreted here (DC-22).
type Approval struct {
	Role      string `yaml:"role" json:"role"`
	Principal string `yaml:"principal" json:"principal"`
}

// The required-input witness family's closed vocabulary (unsealed-
// provenance exemption design §4, §9; SI-255). Exported so the evaluator
// and approval lanes never retype them.
const (
	// RequiredInputPhase is the one phase a required-input witness may
	// name: the review phase, whose three sealed-provenance inputs are
	// the departure target.
	RequiredInputPhase = PhaseReview

	// The three sealed-provenance review inputs (compiler design §6), in
	// their sorted order. The same wire strings are contextcompile's
	// RequiredInput* constants; this package cannot import that one (it
	// imports this), so exemption_required_input_external_test.go pins
	// the two homes equal.
	InputBuilderReceipt = "builder-receipt"
	InputEvidenceBundle = "evidence-bundle"
	InputResultDiff     = "result-diff"

	// EscalationRole is the approval role an escalation record's
	// distinct signed approval fills (design §9, W10).
	EscalationRole = "unsealed-exemption-escalation"
)

// RequiredInputNames returns the exact input set a required-input witness
// must name, in its required sorted order, as a fresh slice.
func RequiredInputNames() []string {
	return []string{InputBuilderReceipt, InputEvidenceBundle, InputResultDiff}
}

// RequiredInputWitness is the required-input witness (design §4; SI-255's
// wire form): a departure from the review phase's three sealed-provenance
// inputs for one exact completed implementation of one inventoried spec.
//
// Story is a syntactically valid, unpinned, unfragmented spec ref. This
// decoder does not decide whether it names a story rather than a feature
// — that needs the spec store, and the conflict evaluator decides it
// (design §4 W2, §5 Match).
type RequiredInputWitness struct {
	Phase              string                 `json:"phase"`
	Inputs             []string               `json:"inputs"`
	Story              string                 `json:"story"`
	AcceptedSpecDigest string                 `json:"accepted_spec_digest"`
	Implementation     ImplementationSnapshot `json:"implementation"`
	InventoryEntry     string                 `json:"inventory_entry"`
	LandedSnapshot     string                 `json:"landed_snapshot"`
	Escalation         *EscalationRecord      `json:"escalation,omitempty"`
}

// ImplementationSnapshot binds the exempted implementation by content
// identity (design §3; SI-240): the eligible implementation commit H_e
// and its tree id, both full object ids.
type ImplementationSnapshot struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
}

// EscalationRecord is the escalation content a repeated use carries
// (design §9, W10; SI-255): the digest of the use history it reviewed, a
// corrective action, and why sealed execution was unavailable. It lives
// inside the exemption, so the escalation approval row signs it with the
// rest of the artifact.
type EscalationRecord struct {
	UseHistoryDigest                 string `json:"use_history_digest"`
	CorrectiveAction                 string `json:"corrective_action"`
	SealedExecutionUnavailableReason string `json:"sealed_execution_unavailable_reason"`
}

type witnessDoc struct {
	Policy      *string `yaml:"policy"`
	Claim       *string `yaml:"claim"`
	ClaimDigest *string `yaml:"claim_digest"`
}

type requiredInputDoc struct {
	Phase              *string            `yaml:"phase"`
	Inputs             *[]string          `yaml:"inputs"`
	Story              *string            `yaml:"story"`
	AcceptedSpecDigest *string            `yaml:"accepted_spec_digest"`
	Implementation     *implementationDoc `yaml:"implementation"`
	InventoryEntry     *string            `yaml:"inventory_entry"`
	LandedSnapshot     *string            `yaml:"landed_snapshot"`
	Escalation         *escalationDoc     `yaml:"escalation"`
}

type implementationDoc struct {
	Commit *string `yaml:"commit"`
	Tree   *string `yaml:"tree"`
}

type escalationDoc struct {
	UseHistoryDigest                 *string `yaml:"use_history_digest"`
	CorrectiveAction                 *string `yaml:"corrective_action"`
	SealedExecutionUnavailableReason *string `yaml:"sealed_execution_unavailable_reason"`
}

type approvalDoc struct {
	Role      *string `yaml:"role"`
	Principal *string `yaml:"principal"`
}

type exemptionDoc struct {
	kernelDoc            `yaml:",inline"`
	Scope                *scopeDoc         `yaml:"scope"`
	Witnesses            *[]witnessDoc     `yaml:"witnesses"`
	RequiredInput        *requiredInputDoc `yaml:"required_input"`
	CompensatingControls *[]string         `yaml:"compensating_controls"`
	Approvals            *[]approvalDoc    `yaml:"approvals"`
	Expiry               *string           `yaml:"expiry"`
	ReviewCondition      *string           `yaml:"review_condition"`
}

// DecodeExemption strictly decodes data as a verdi.policy-exemption/v1
// artifact, validates its bounded-departure grammar, normalizes it, and
// seals the result.
func DecodeExemption(data []byte) (*Exemption, error) {
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("policyartifact: %w", err)
	}
	var doc exemptionDoc
	if err := artifact.DecodeStrict(fm, &doc); err != nil {
		return nil, err
	}
	k, err := doc.toKernel(SchemaExemption, KindExemption)
	if err != nil {
		return nil, err
	}
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: exemption field %s is missing: every exemption field except expiry/review_condition is mandatory", field)
	}
	if doc.Scope == nil {
		return nil, missing("scope")
	}
	// Exactly one witness family (SI-241, SI-255). A required-input
	// exemption carries NO witnesses key: present even as [], it is the
	// mixed-family case, never an empty claim family beside the witness.
	// A key whose value is YAML null decodes, through the one strict
	// seam, to a nil pointer — the same null-is-absent reading every
	// optional field of this artifact already has; it carries no witness
	// of either family, so it cannot mix them.
	switch {
	case doc.Witnesses != nil && doc.RequiredInput != nil:
		return nil, fmt.Errorf("policyartifact: exemption carries both witness families (witnesses and required_input); an exemption names exactly one (SI-241)")
	case doc.Witnesses == nil && doc.RequiredInput == nil:
		return nil, fmt.Errorf("policyartifact: exemption field witnesses is missing: an exemption carries exactly one witness family, claim witnesses or one required_input witness (SI-241)")
	}
	if doc.CompensatingControls == nil {
		return nil, missing("compensating_controls")
	}
	if doc.Approvals == nil {
		return nil, missing("approvals")
	}

	scope, err := doc.Scope.toScope("exemption.scope")
	if err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}

	var (
		witnesses     []Witness
		requiredInput *RequiredInputWitness
	)
	if doc.RequiredInput != nil {
		// The required-input family's claim witness set is the EMPTY
		// NON-NIL slice: internal/contextcompile's cloneExemption rebuilds
		// Witnesses as append([]Witness{}, in.Witnesses...), which turns a
		// nil slice into [] — a nil here would break the seal on every
		// clone.
		witnesses = []Witness{}
		requiredInput, err = doc.RequiredInput.toWitness()
		if err != nil {
			return nil, err
		}
	} else {
		witnesses, err = decodeClaimWitnesses(*doc.Witnesses)
		if err != nil {
			return nil, err
		}
	}

	// Compensating controls are authored ORDERED content, like a policy's
	// instructions: their order is the author's and is digest-bound; they
	// are never sorted or deduplicated. Each entry must carry real text.
	if len(*doc.CompensatingControls) == 0 {
		return nil, fmt.Errorf("policyartifact: exemption must name at least one compensating control")
	}
	for i, c := range *doc.CompensatingControls {
		if strings.TrimSpace(c) == "" {
			return nil, fmt.Errorf("policyartifact: exemption compensating_controls[%d]: empty control", i)
		}
		if strings.ContainsAny(c, "\n\r") {
			return nil, fmt.Errorf("policyartifact: exemption compensating_controls[%d]: a control must be a single line", i)
		}
	}

	if len(*doc.Approvals) == 0 {
		return nil, fmt.Errorf("policyartifact: exemption must record at least one approval fact")
	}
	approvals := make([]Approval, 0, len(*doc.Approvals))
	seenApproval := make(map[string]bool, len(*doc.Approvals))
	for i, ad := range *doc.Approvals {
		if ad.Role == nil || ad.Principal == nil {
			return nil, fmt.Errorf("policyartifact: exemption approvals[%d]: role and principal are both required", i)
		}
		if !kebabRe.MatchString(*ad.Role) {
			return nil, fmt.Errorf("policyartifact: exemption approvals[%d]: role %q must be kebab-case", i, *ad.Role)
		}
		if err := governanceprincipal.PrincipalID(*ad.Principal).Validate(); err != nil {
			return nil, fmt.Errorf("policyartifact: exemption approvals[%d]: principal: %w", i, err)
		}
		key := *ad.Role + "\x00" + *ad.Principal
		if seenApproval[key] {
			return nil, fmt.Errorf("policyartifact: exemption approvals: duplicate approval (%s, %s)", *ad.Role, *ad.Principal)
		}
		seenApproval[key] = true
		approvals = append(approvals, Approval{Role: *ad.Role, Principal: *ad.Principal})
	}

	expiry := ""
	if doc.Expiry != nil {
		expiry = *doc.Expiry
		// A real calendar date, not just the shape of one: a departure
		// stamped 2026-02-31 or 9999-99-99 would be a permanently
		// unbounded exemption wearing a bound (DC-8; CO-2 fails closed).
		if _, err := time.Parse("2006-01-02", expiry); err != nil {
			return nil, fmt.Errorf("policyartifact: exemption expiry %q is not a real YYYY-MM-DD calendar date", expiry)
		}
	}
	review := ""
	if doc.ReviewCondition != nil {
		review = *doc.ReviewCondition
		if strings.TrimSpace(review) == "" {
			return nil, fmt.Errorf("policyartifact: exemption review_condition must carry a named condition, not blank text")
		}
	}
	// A required-input exemption's expiry is mandatory (design §4; SI-255):
	// a review condition may accompany it but never replaces it. The claim
	// family keeps DC-8's expiry-or-review-condition bound.
	if requiredInput != nil && expiry == "" {
		return nil, fmt.Errorf("policyartifact: a required-input exemption must carry an expiry; a review_condition may accompany it but never replaces it (SI-255)")
	}
	if expiry == "" && review == "" {
		return nil, fmt.Errorf("policyartifact: exemption must carry an expiry or review condition (DC-8: every departure is bounded)")
	}

	rationale, err := requireRationale(KindExemption, body)
	if err != nil {
		return nil, err
	}

	e := &Exemption{
		Schema:               k.Schema,
		ID:                   k.ID,
		Kind:                 k.Kind,
		Title:                k.Title,
		Owners:               k.Owners,
		Scope:                scope,
		Witnesses:            witnesses,
		RequiredInput:        requiredInput,
		CompensatingControls: *doc.CompensatingControls,
		Approvals:            approvals,
		Expiry:               expiry,
		ReviewCondition:      review,
		Template:             k.Template,
		Rationale:            rationale,
	}
	normalizeScope(&e.Scope)
	sort.Slice(e.Witnesses, func(i, j int) bool {
		if e.Witnesses[i].Policy != e.Witnesses[j].Policy {
			return e.Witnesses[i].Policy < e.Witnesses[j].Policy
		}
		return e.Witnesses[i].Claim < e.Witnesses[j].Claim
	})
	sort.Slice(e.Approvals, func(i, j int) bool {
		if e.Approvals[i].Role != e.Approvals[j].Role {
			return e.Approvals[i].Role < e.Approvals[j].Role
		}
		return e.Approvals[i].Principal < e.Approvals[j].Principal
	})
	seal, err := canonjson.Digest(e)
	if err != nil {
		return nil, err
	}
	e.seal = seal
	return e, nil
}

// decodeClaimWitnesses validates the claim witness family (policy-
// conflict design §5.5): at least one exact (policy, claim, claim_digest)
// witness, no duplicate (policy, claim) pair.
func decodeClaimWitnesses(docs []witnessDoc) ([]Witness, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("policyartifact: exemption must name at least one exact witness")
	}
	witnesses := make([]Witness, 0, len(docs))
	seenWitness := make(map[string]bool, len(docs))
	for i, wd := range docs {
		if wd.Policy == nil || wd.Claim == nil || wd.ClaimDigest == nil {
			return nil, fmt.Errorf("policyartifact: exemption witnesses[%d]: policy, claim, and claim_digest are all required", i)
		}
		if _, err := parseKindedID(*wd.Policy, KindPolicy); err != nil {
			return nil, fmt.Errorf("policyartifact: exemption witnesses[%d]: policy %q must be a policy/<name> id: %w", i, *wd.Policy, err)
		}
		if !kebabRe.MatchString(*wd.Claim) {
			return nil, fmt.Errorf("policyartifact: exemption witnesses[%d]: claim %q must be kebab-case", i, *wd.Claim)
		}
		if !sha256Re.MatchString(*wd.ClaimDigest) {
			return nil, fmt.Errorf("policyartifact: exemption witnesses[%d]: claim_digest %q is not sha256:<64 hex> form", i, *wd.ClaimDigest)
		}
		key := *wd.Policy + "#" + *wd.Claim
		if seenWitness[key] {
			return nil, fmt.Errorf("policyartifact: exemption witnesses: duplicate witness %s", key)
		}
		seenWitness[key] = true
		witnesses = append(witnesses, Witness{Policy: *wd.Policy, Claim: *wd.Claim, ClaimDigest: *wd.ClaimDigest})
	}
	return witnesses, nil
}

// objectIDRe is a full git object id: lowercase hex of the SHA-1 (40) or
// SHA-256 (64) repository hash algorithm — never an abbreviation.
var objectIDRe = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// toWitness validates the decoded required_input mapping (design §4;
// SI-255's wire form) and returns its value. Every field is mandatory
// except escalation, whose fields are all mandatory when it is present.
// Nothing is normalized: inputs must already be the exact sorted three,
// so the signed text and the canonical value never disagree.
func (d requiredInputDoc) toWitness() (*RequiredInputWitness, error) {
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: exemption required_input.%s is missing: every required_input field except escalation is mandatory (SI-255)", field)
	}
	switch {
	case d.Phase == nil:
		return nil, missing("phase")
	case d.Inputs == nil:
		return nil, missing("inputs")
	case d.Story == nil:
		return nil, missing("story")
	case d.AcceptedSpecDigest == nil:
		return nil, missing("accepted_spec_digest")
	case d.Implementation == nil:
		return nil, missing("implementation")
	case d.Implementation.Commit == nil:
		return nil, missing("implementation.commit")
	case d.Implementation.Tree == nil:
		return nil, missing("implementation.tree")
	case d.InventoryEntry == nil:
		return nil, missing("inventory_entry")
	case d.LandedSnapshot == nil:
		return nil, missing("landed_snapshot")
	}

	w := &RequiredInputWitness{
		Phase:              *d.Phase,
		Inputs:             append([]string{}, (*d.Inputs)...),
		Story:              *d.Story,
		AcceptedSpecDigest: *d.AcceptedSpecDigest,
		Implementation:     ImplementationSnapshot{Commit: *d.Implementation.Commit, Tree: *d.Implementation.Tree},
		InventoryEntry:     *d.InventoryEntry,
		LandedSnapshot:     *d.LandedSnapshot,
	}

	if w.Phase != RequiredInputPhase {
		return nil, fmt.Errorf("policyartifact: exemption required_input.phase %q must be %q", w.Phase, RequiredInputPhase)
	}
	if want := RequiredInputNames(); !slices.Equal(w.Inputs, want) {
		return nil, fmt.Errorf("policyartifact: exemption required_input.inputs %q must be exactly %q, in that sorted order with no duplicates (another order or set is refused, never re-sorted)", w.Inputs, want)
	}
	ref, err := artifact.ParseRef(w.Story)
	if err != nil {
		return nil, fmt.Errorf("policyartifact: exemption required_input.story %q must be a spec ref: %w", w.Story, err)
	}
	if ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return nil, fmt.Errorf("policyartifact: exemption required_input.story %q must be an unpinned, unfragmented spec/<name> ref", w.Story)
	}
	if !sha256Re.MatchString(w.AcceptedSpecDigest) {
		return nil, fmt.Errorf("policyartifact: exemption required_input.accepted_spec_digest %q is not sha256:<64 hex> form", w.AcceptedSpecDigest)
	}
	for _, id := range []struct{ field, value string }{
		{"implementation.commit", w.Implementation.Commit},
		{"implementation.tree", w.Implementation.Tree},
		{"landed_snapshot", w.LandedSnapshot},
	} {
		if !objectIDRe.MatchString(id.value) {
			return nil, fmt.Errorf("policyartifact: exemption required_input.%s %q must be a full lowercase hex object id (40 or 64 characters)", id.field, id.value)
		}
	}
	if len(w.Implementation.Tree) != len(w.Implementation.Commit) || len(w.LandedSnapshot) != len(w.Implementation.Commit) {
		return nil, fmt.Errorf("policyartifact: exemption required_input implementation.commit, implementation.tree, and landed_snapshot must share one length: they name objects of one repository hash algorithm")
	}
	if !kebabRe.MatchString(w.InventoryEntry) {
		return nil, fmt.Errorf("policyartifact: exemption required_input.inventory_entry %q must be a kebab-case inventory entry id", w.InventoryEntry)
	}

	if d.Escalation != nil {
		esc, err := d.Escalation.toRecord()
		if err != nil {
			return nil, err
		}
		w.Escalation = esc
	}
	return w, nil
}

// toRecord validates the decoded escalation mapping (design §9, W10;
// SI-255): all three fields are mandatory, the use-history digest is a
// content digest, and the two text fields are single non-blank lines.
func (d escalationDoc) toRecord() (*EscalationRecord, error) {
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: exemption required_input.escalation.%s is missing: every escalation field is mandatory when the escalation block is present (SI-255)", field)
	}
	switch {
	case d.UseHistoryDigest == nil:
		return nil, missing("use_history_digest")
	case d.CorrectiveAction == nil:
		return nil, missing("corrective_action")
	case d.SealedExecutionUnavailableReason == nil:
		return nil, missing("sealed_execution_unavailable_reason")
	}
	r := &EscalationRecord{
		UseHistoryDigest:                 *d.UseHistoryDigest,
		CorrectiveAction:                 *d.CorrectiveAction,
		SealedExecutionUnavailableReason: *d.SealedExecutionUnavailableReason,
	}
	if !sha256Re.MatchString(r.UseHistoryDigest) {
		return nil, fmt.Errorf("policyartifact: exemption required_input.escalation.use_history_digest %q is not sha256:<64 hex> form", r.UseHistoryDigest)
	}
	for _, text := range []struct{ field, value string }{
		{"corrective_action", r.CorrectiveAction},
		{"sealed_execution_unavailable_reason", r.SealedExecutionUnavailableReason},
	} {
		if strings.TrimSpace(text.value) == "" {
			return nil, fmt.Errorf("policyartifact: exemption required_input.escalation.%s must carry text, not blank text", text.field)
		}
		if strings.ContainsAny(text.value, "\n\r") {
			return nil, fmt.Errorf("policyartifact: exemption required_input.escalation.%s must be a single line", text.field)
		}
	}
	return r, nil
}

// Name returns the exemption id's name half.
func (e *Exemption) Name() string { return nameOf(e.ID) }

// Digest returns the exemption's canonical content address after proving
// the value is unmodified DecodeExemption output.
func (e *Exemption) Digest() (string, error) {
	if err := e.checkSeal(); err != nil {
		return "", err
	}
	return e.seal, nil
}

func (e *Exemption) checkSeal() error {
	return checkSealed("exemption", e.ID, e.seal, e)
}
