package designprovenance

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// OperationKind is the closed ASD draft-operation vocabulary.
type OperationKind string

// Closed operation kinds.
const (
	OpSetProblem       OperationKind = "set-problem"
	OpSetOutcome       OperationKind = "set-outcome"
	OpAddAC            OperationKind = "add-ac"
	OpEditAC           OperationKind = "edit-ac"
	OpRemoveAC         OperationKind = "remove-ac"
	OpReorderAC        OperationKind = "reorder-ac"
	OpSetACEvidence    OperationKind = "set-ac-evidence"
	OpAddConstraint    OperationKind = "add-constraint"
	OpEditConstraint   OperationKind = "edit-constraint"
	OpRemoveConstraint OperationKind = "remove-constraint"
	OpAddDecision      OperationKind = "add-decision"
	OpEditDecision     OperationKind = "edit-decision"
	OpRemoveDecision   OperationKind = "remove-decision"
	OpAddQuestion      OperationKind = "add-question"
	OpEditQuestion     OperationKind = "edit-question"
	OpRemoveQuestion   OperationKind = "remove-question"
	OpAddLink          OperationKind = "add-link"
	OpRemoveLink       OperationKind = "remove-link"
	OpAddStub          OperationKind = "add-stub"
	OpEditStub         OperationKind = "edit-stub"
	OpRemoveStub       OperationKind = "remove-stub"
	OpReorderStub      OperationKind = "reorder-stub"
	OpAddContextRef    OperationKind = "add-context-ref"
	OpRemoveContextRef OperationKind = "remove-context-ref"
)

// Operation is the exact request/provenance union. Fields not owned by an
// operation arm are rejected by Validate and during strict JSON decode.
type Operation struct {
	Op                 OperationKind           `json:"op"`
	Text               string                  `json:"text,omitempty"`
	Anchor             string                  `json:"anchor,omitempty"`
	ID                 string                  `json:"id,omitempty"`
	Evidence           []artifact.EvidenceKind `json:"evidence,omitempty"`
	AfterID            string                  `json:"after_id,omitempty"`
	AfterSlug          string                  `json:"after_slug,omitempty"`
	Source             string                  `json:"source,omitempty"`
	Type               artifact.LinkType       `json:"type,omitempty"`
	Ref                string                  `json:"ref,omitempty"`
	Note               string                  `json:"note,omitempty"`
	Slug               string                  `json:"slug,omitempty"`
	AcceptanceCriteria []string                `json:"acceptance_criteria,omitempty"`
	Spike              *bool                   `json:"spike,omitempty"`
	Resolves           []string                `json:"resolves,omitempty"`
}

var operationFields = map[OperationKind]map[string]bool{
	OpSetProblem:       fieldSet("op", "text", "anchor"),
	OpSetOutcome:       fieldSet("op", "text", "anchor"),
	OpAddAC:            fieldSet("op", "id", "text", "evidence", "anchor"),
	OpEditAC:           fieldSet("op", "id", "text", "evidence", "anchor"),
	OpRemoveAC:         fieldSet("op", "id"),
	OpReorderAC:        fieldSet("op", "id", "after_id"),
	OpSetACEvidence:    fieldSet("op", "id", "evidence"),
	OpAddConstraint:    fieldSet("op", "id", "text", "anchor"),
	OpEditConstraint:   fieldSet("op", "id", "text", "anchor"),
	OpRemoveConstraint: fieldSet("op", "id"),
	OpAddDecision:      fieldSet("op", "id", "text", "anchor"),
	OpEditDecision:     fieldSet("op", "id", "text", "anchor"),
	OpRemoveDecision:   fieldSet("op", "id"),
	OpAddQuestion:      fieldSet("op", "id", "text", "anchor"),
	OpEditQuestion:     fieldSet("op", "id", "text", "anchor"),
	OpRemoveQuestion:   fieldSet("op", "id"),
	OpAddLink:          fieldSet("op", "source", "type", "ref", "note"),
	OpRemoveLink:       fieldSet("op", "source", "type", "ref", "note"),
	OpAddStub:          fieldSet("op", "slug", "acceptance_criteria", "spike", "resolves"),
	OpEditStub:         fieldSet("op", "slug", "acceptance_criteria", "spike", "resolves"),
	OpRemoveStub:       fieldSet("op", "slug"),
	OpReorderStub:      fieldSet("op", "slug", "after_slug"),
	OpAddContextRef:    fieldSet("op", "ref"),
	OpRemoveContextRef: fieldSet("op", "ref"),
}

func fieldSet(fields ...string) map[string]bool {
	out := make(map[string]bool, len(fields))
	for _, field := range fields {
		out[field] = true
	}
	return out
}

// UnmarshalJSON enforces the arm's exact field set before decoding values.
func (o *Operation) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := decodeStrictJSON(data, &fields); err != nil {
		return err
	}
	var op OperationKind
	opRaw, ok := fields["op"]
	if !ok {
		return fmt.Errorf("designprovenance: operation is missing field %q", "op")
	}
	if err := json.Unmarshal(opRaw, &op); err != nil {
		return fmt.Errorf("designprovenance: operation op: %w", err)
	}
	allowed, ok := operationFields[op]
	if !ok {
		return fmt.Errorf("designprovenance: unknown operation %q", op)
	}
	for field := range fields {
		if !allowed[field] {
			return fmt.Errorf("designprovenance: operation %q does not allow field %q", op, field)
		}
	}
	type plain Operation
	var decoded plain
	if err := decodeStrictJSON(data, &decoded); err != nil {
		return err
	}
	*o = Operation(decoded)
	return o.validateFields(fields)
}

// Validate enforces the operation vocabulary and field/arm invariants for a
// programmatically constructed operation.
func (o Operation) Validate() error {
	fields := map[string]json.RawMessage{"op": nil}
	if o.Text != "" {
		fields["text"] = nil
	}
	if o.Anchor != "" {
		fields["anchor"] = nil
	}
	if o.ID != "" {
		fields["id"] = nil
	}
	if o.Evidence != nil {
		fields["evidence"] = nil
	}
	if o.AfterID != "" {
		fields["after_id"] = nil
	}
	if o.AfterSlug != "" {
		fields["after_slug"] = nil
	}
	if o.Source != "" {
		fields["source"] = nil
	}
	if o.Type != "" {
		fields["type"] = nil
	}
	if o.Ref != "" {
		fields["ref"] = nil
	}
	if o.Note != "" {
		fields["note"] = nil
	}
	if o.Slug != "" {
		fields["slug"] = nil
	}
	if o.AcceptanceCriteria != nil {
		fields["acceptance_criteria"] = nil
	}
	if o.Spike != nil {
		fields["spike"] = nil
	}
	if o.Resolves != nil {
		fields["resolves"] = nil
	}
	return o.validateFields(fields)
}

func (o Operation) validateFields(fields map[string]json.RawMessage) error {
	allowed, ok := operationFields[o.Op]
	if !ok {
		return fmt.Errorf("designprovenance: unknown operation %q", o.Op)
	}
	for field := range fields {
		if !allowed[field] {
			return fmt.Errorf("designprovenance: operation %q does not allow field %q", o.Op, field)
		}
	}
	require := func(field, value string) error {
		if !allowed[field] || value != "" {
			return nil
		}
		return fmt.Errorf("designprovenance: operation %q field %q must be nonempty", o.Op, field)
	}
	for _, pair := range [][2]string{{"text", o.Text}, {"anchor", o.Anchor}, {"id", o.ID}, {"source", o.Source}, {"type", string(o.Type)}, {"ref", o.Ref}, {"slug", o.Slug}} {
		if err := require(pair[0], pair[1]); err != nil {
			return err
		}
	}
	for _, pair := range [][2]string{{"after_id", o.AfterID}, {"after_slug", o.AfterSlug}, {"note", o.Note}} {
		if _, present := fields[pair[0]]; present && pair[1] == "" {
			return fmt.Errorf("designprovenance: operation %q field %q must be nonempty when present", o.Op, pair[0])
		}
	}
	if (o.Op == OpAddLink || o.Op == OpRemoveLink) && !o.Type.Valid() {
		return fmt.Errorf("designprovenance: operation %q has unknown link type %q", o.Op, o.Type)
	}
	if _, required := allowed["evidence"]; required {
		if len(o.Evidence) == 0 {
			return fmt.Errorf("designprovenance: operation %q evidence must be nonempty", o.Op)
		}
		seen := map[artifact.EvidenceKind]bool{}
		for _, kind := range o.Evidence {
			switch kind {
			case artifact.EvidenceStatic, artifact.EvidenceBehavioral, artifact.EvidenceRuntime, artifact.EvidenceAttestation:
			default:
				return fmt.Errorf("designprovenance: operation %q unknown evidence kind %q", o.Op, kind)
			}
			if seen[kind] {
				return fmt.Errorf("designprovenance: operation %q duplicate evidence kind %q", o.Op, kind)
			}
			seen[kind] = true
		}
	}
	if o.Op == OpAddStub || o.Op == OpEditStub {
		spike := o.Spike != nil && *o.Spike
		if o.Spike != nil && !*o.Spike {
			// vocab:identity — exact ASD operation-union field name
			return fmt.Errorf("designprovenance: operation %q spike, when present, must be true", o.Op)
		}
		if spike {
			if len(o.Resolves) == 0 || o.AcceptanceCriteria != nil {
				// vocab:identity — exact ASD operation-union arm and field names
				return fmt.Errorf("designprovenance: operation %q spike arm requires resolves and omits acceptance_criteria", o.Op)
			}
		} else if len(o.AcceptanceCriteria) == 0 || o.Resolves != nil {
			return fmt.Errorf("designprovenance: operation %q plain arm requires acceptance_criteria and omits resolves/spike", o.Op)
		}
		values := o.AcceptanceCriteria
		if spike {
			values = o.Resolves
		}
		seen := map[string]bool{}
		for _, value := range values {
			if value == "" {
				return fmt.Errorf("designprovenance: operation %q stub collection contains an empty value", o.Op)
			}
			if seen[value] {
				return fmt.Errorf("designprovenance: operation %q stub collection contains duplicate %q", o.Op, value)
			}
			seen[value] = true
		}
	}
	return nil
}

// ChangeKind is the closed semantic-change vocabulary.
type ChangeKind string

const (
	ChangeAdded               ChangeKind = "added"
	ChangeReplaced            ChangeKind = "replaced"
	ChangeRemoved             ChangeKind = "removed"
	ChangeReordered           ChangeKind = "reordered"
	ChangeRelationshipAdded   ChangeKind = "relationship-added"
	ChangeRelationshipRemoved ChangeKind = "relationship-removed"
)

// Change records one target's before/after canonical object digests.
type Change struct {
	Target       string     `json:"target"`
	Change       ChangeKind `json:"change"`
	BeforeDigest string     `json:"before_digest,omitempty"`
	AfterDigest  string     `json:"after_digest,omitempty"`
}

// Validate enforces the digest arm for each change kind.
func (c Change) Validate() error {
	if c.Target == "" {
		return fmt.Errorf("designprovenance: change target must be nonempty")
	}
	needBefore, needAfter := false, false
	switch c.Change {
	case ChangeAdded, ChangeRelationshipAdded:
		needAfter = true
	case ChangeRemoved, ChangeRelationshipRemoved:
		needBefore = true
	case ChangeReplaced, ChangeReordered:
		needBefore, needAfter = true, true
	default:
		return fmt.Errorf("designprovenance: unknown change %q", c.Change)
	}
	if needBefore != (c.BeforeDigest != "") || needAfter != (c.AfterDigest != "") {
		return fmt.Errorf("designprovenance: change %q carries the wrong digest arm", c.Change)
	}
	if c.BeforeDigest != "" && !artifact.ValidDigest(c.BeforeDigest) {
		return fmt.Errorf("designprovenance: invalid before digest %q", c.BeforeDigest)
	}
	if c.AfterDigest != "" && !artifact.ValidDigest(c.AfterDigest) {
		return fmt.Errorf("designprovenance: invalid after digest %q", c.AfterDigest)
	}
	return nil
}

// UnclassifiedGap discloses a direct-Markdown discontinuity between the
// previous typed chain tip and this entry's previous spec bytes.
type UnclassifiedGap struct {
	FromDigest string `json:"from_digest"`
	ToDigest   string `json:"to_digest"`
}

// Context is v1's explicit unavailable context identity.
type Context struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

// UnavailableContext returns the only v1 context record.
func UnavailableContext() Context {
	return Context{State: "unavailable", Reason: ContextUnavailableReason}
}

// ExcerptClassification is the closed origin classification.
type ExcerptClassification string

const (
	ClassificationHumanStated   ExcerptClassification = "human-stated"
	ClassificationAISynthesized ExcerptClassification = "ai-synthesized"
	ClassificationAIInferred    ExcerptClassification = "ai-inferred"
	ClassificationUnresolved    ExcerptClassification = "unresolved"
)

// ExcerptRepresentation identifies verbatim text versus paraphrase.
type ExcerptRepresentation string

const (
	RepresentationVerbatim   ExcerptRepresentation = "verbatim"
	RepresentationParaphrase ExcerptRepresentation = "paraphrase"
)

// Excerpt is one bounded, content-addressed provenance excerpt.
type Excerpt struct {
	Target         string                `json:"target"`
	TargetDigest   string                `json:"target_digest"`
	Classification ExcerptClassification `json:"classification"`
	Representation ExcerptRepresentation `json:"representation"`
	Text           string                `json:"text"`
}

func (e Excerpt) validate() error {
	if !ValidExcerptTarget(e.Target) || e.Text == "" || !utf8.ValidString(e.Text) {
		return fmt.Errorf("designprovenance: excerpt target must be problem, outcome, or an object ID, and text must be nonempty valid UTF-8")
	}
	if utf8.RuneCountInString(e.Text) > MaxExcerptScalars {
		return fmt.Errorf("designprovenance: excerpt text exceeds 600 Unicode scalars")
	}
	if !artifact.ValidDigest(e.TargetDigest) {
		return fmt.Errorf("designprovenance: excerpt target_digest %q is invalid", e.TargetDigest)
	}
	switch e.Classification {
	case ClassificationHumanStated, ClassificationAISynthesized, ClassificationAIInferred, ClassificationUnresolved:
	default:
		return fmt.Errorf("designprovenance: unknown excerpt classification %q", e.Classification)
	}
	switch e.Representation {
	case RepresentationVerbatim, RepresentationParaphrase:
	default:
		return fmt.Errorf("designprovenance: unknown excerpt representation %q", e.Representation)
	}
	return nil
}

// ValidExcerptTarget reports whether target is a problem, outcome, or
// syntactically valid declared-object ID. Relationship/group IDs are not
// attachable provenance excerpt targets.
func ValidExcerptTarget(target string) bool {
	if target == "problem" || target == "outcome" {
		return true
	}
	ref, err := artifact.ParseRef("spec/excerpt-target#" + target)
	return err == nil && ref.Fragment()
}

// PolicyState is v2's closed policy-identity union tag (§4.1, SI-176).
type PolicyState string

const (
	// PolicyResolved carries a valid adopted effective policy's sealed
	// digest as provenance only.
	PolicyResolved PolicyState = "resolved"
	// PolicyNotApplicable is the honest declaration that policy authority
	// is genuinely not adopted. It is permitted only for the explicit
	// browser-human actor and forbids a digest — no sentinel or hash of
	// absence may be presented as a policy identity.
	PolicyNotApplicable PolicyState = "not-applicable"
)

// Policy is v2's exact top-level policy-identity union: exactly one closed
// arm, `{"state":"resolved","digest":"sha256:..."}` or
// `{"state":"not-applicable"}`. Its own strict UnmarshalJSON — not just
// Validate's zero-value checks — enforces the cross-arm field prohibition
// even when a forbidden field is present as an explicit null or empty
// value, mirroring Operation's own field-set closure above.
type Policy struct {
	State  PolicyState `json:"state"`
	Digest string      `json:"digest,omitempty"`
}

// UnmarshalJSON strict-decodes one policy union value: the arm named by
// `state` fixes exactly which other field may appear, unknown fields fail
// closed, and `digest`'s presence (not merely its decoded value) is what is
// checked, so a `not-applicable` arm carrying an explicit null or empty
// `digest` still fails closed.
func (p *Policy) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := decodeStrictJSON(data, &fields); err != nil {
		return err
	}
	type plain Policy
	var decoded plain
	if err := decodeStrictJSON(data, &decoded); err != nil {
		return fmt.Errorf("designprovenance: decoding policy: %w", err)
	}
	switch decoded.State {
	case PolicyResolved:
		if _, ok := fields["digest"]; !ok {
			return fmt.Errorf("designprovenance: policy state %q requires field %q", PolicyResolved, "digest")
		}
	case PolicyNotApplicable:
		if _, ok := fields["digest"]; ok {
			return fmt.Errorf("designprovenance: policy state %q forbids field %q", PolicyNotApplicable, "digest")
		}
	default:
		return fmt.Errorf("designprovenance: unknown policy state %q", decoded.State)
	}
	*p = Policy(decoded)
	return p.validate()
}

func (p Policy) validate() error {
	switch p.State {
	case PolicyResolved:
		if !artifact.ValidDigest(p.Digest) {
			return fmt.Errorf("designprovenance: resolved policy digest %q is invalid", p.Digest)
		}
	case PolicyNotApplicable:
		if p.Digest != "" {
			return fmt.Errorf("designprovenance: not-applicable policy must not carry a digest")
		}
	default:
		return fmt.Errorf("designprovenance: unknown policy state %q", p.State)
	}
	return nil
}

// Entry is one canonical JSONL sidecar record. Schema selects between two
// mutually exclusive policy-identity representations: v1's required
// `PolicyDigest` scalar (decode-only history) or v2's required, non-null
// `Policy` union (every current writer) — never both, never neither.
type Entry struct {
	Schema          string                          `json:"schema"`
	Spec            string                          `json:"spec"`
	PreviousDigest  string                          `json:"previous_digest"`
	ResultDigest    string                          `json:"result_digest"`
	UnclassifiedGap *UnclassifiedGap                `json:"unclassified_gap,omitempty"`
	Attribution     governanceprincipal.Attribution `json:"attribution"`
	Harness         string                          `json:"harness,omitempty"`
	Session         string                          `json:"session,omitempty"`
	PolicyDigest    string                          `json:"policy_digest,omitempty"`
	Policy          *Policy                         `json:"policy,omitempty"`
	Context         Context                         `json:"context"`
	Operations      []Operation                     `json:"operations"`
	Changes         []Change                        `json:"changes"`
	Excerpts        []Excerpt                       `json:"excerpts"`
	Digest          string                          `json:"digest"`
}

// UnmarshalJSON strict-decodes one entry and enforces the schema-version-
// conditioned exclusive policy representation: v1 requires `policy_digest`
// and forbids `policy`; v2 forbids `policy_digest` and requires a non-null
// `policy`. Field PRESENCE is checked directly against the raw object (not
// only the decoded zero value), so an explicitly null `policy` under v2, or
// a `policy_digest` present under v2 even as an empty string, fails closed
// exactly like a missing field would. Every other field's unknown/duplicate/
// trailing-data rejection and canonical round-trip already come from the
// `plain`-typed strict decode below and DecodeEntry's own canonical
// re-encode comparison — this method adds no second general-purpose
// decode path.
func (e *Entry) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := decodeStrictJSON(data, &fields); err != nil {
		return err
	}
	type plain Entry
	var decoded plain
	if err := decodeStrictJSON(data, &decoded); err != nil {
		// Returned unwrapped: DecodeEntry already prefixes
		// "designprovenance: decoding entry", so wrapping here doubled
		// that prefix in every operator-visible strict-decode diagnostic.
		// Strictness is unchanged — the same error, same rejections.
		return err
	}
	*e = Entry(decoded)

	_, hasPolicyDigest := fields["policy_digest"]
	policyRaw, hasPolicy := fields["policy"]
	switch e.Schema {
	case Schema:
		if hasPolicy {
			return fmt.Errorf("designprovenance: v1 entry must not carry field %q", "policy")
		}
		if !hasPolicyDigest {
			return fmt.Errorf("designprovenance: v1 entry missing required field %q", "policy_digest")
		}
	case SchemaV2:
		if hasPolicyDigest {
			return fmt.Errorf("designprovenance: v2 entry must not carry field %q", "policy_digest")
		}
		if !hasPolicy {
			return fmt.Errorf("designprovenance: v2 entry missing required field %q", "policy")
		}
		if string(policyRaw) == "null" {
			return fmt.Errorf("designprovenance: v2 entry field %q must not be null", "policy")
		}
	default:
		return fmt.Errorf("designprovenance: unknown schema %q", e.Schema)
	}
	return nil
}

func (e Entry) digestProjection() any {
	type projection Entry
	p := projection(e)
	p.Digest = ""
	return struct {
		Schema          string                          `json:"schema"`
		Spec            string                          `json:"spec"`
		PreviousDigest  string                          `json:"previous_digest"`
		ResultDigest    string                          `json:"result_digest"`
		UnclassifiedGap *UnclassifiedGap                `json:"unclassified_gap,omitempty"`
		Attribution     governanceprincipal.Attribution `json:"attribution"`
		Harness         string                          `json:"harness,omitempty"`
		Session         string                          `json:"session,omitempty"`
		PolicyDigest    string                          `json:"policy_digest,omitempty"`
		Policy          *Policy                         `json:"policy,omitempty"`
		Context         Context                         `json:"context"`
		Operations      []Operation                     `json:"operations"`
		Changes         []Change                        `json:"changes"`
		Excerpts        []Excerpt                       `json:"excerpts"`
	}{p.Schema, p.Spec, p.PreviousDigest, p.ResultDigest, p.UnclassifiedGap, p.Attribution, p.Harness, p.Session, p.PolicyDigest, p.Policy, p.Context, p.Operations, p.Changes, p.Excerpts}
}

// Seal validates the entry and sets its own canonical projection digest.
func (e *Entry) Seal() error {
	e.Digest = ""
	if err := e.validate(false); err != nil {
		return err
	}
	digest, err := canonjson.Digest(e.digestProjection())
	if err != nil {
		return fmt.Errorf("designprovenance: digesting entry: %w", err)
	}
	e.Digest = digest
	return nil
}

// Validate checks all entry fields and verifies the own digest.
func (e Entry) Validate() error { return e.validate(true) }

func (e Entry) validate(checkDigest bool) error {
	switch e.Schema {
	case Schema, SchemaV2:
	default:
		return fmt.Errorf("designprovenance: unknown schema %q", e.Schema)
	}
	ref, err := artifact.ParseRef(e.Spec)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return fmt.Errorf("designprovenance: spec %q must be an unpinned whole spec ref", e.Spec)
	}
	if !artifact.ValidDigest(e.PreviousDigest) || !artifact.ValidDigest(e.ResultDigest) {
		return fmt.Errorf("designprovenance: previous and result digests must be canonical sha256 values")
	}
	// Schema selects one exclusive policy-identity representation. v1 is
	// strict decode-only history (its required PolicyDigest scalar);
	// every current writer emits v2's required, non-null Policy union
	// instead — the two representations never coexist on one entry.
	switch e.Schema {
	case Schema:
		if e.Policy != nil {
			return fmt.Errorf("designprovenance: v1 entry must not carry a policy field")
		}
		if !artifact.ValidDigest(e.PolicyDigest) {
			return fmt.Errorf("designprovenance: v1 policy digest must be a canonical sha256 value")
		}
	case SchemaV2:
		if e.PolicyDigest != "" {
			return fmt.Errorf("designprovenance: v2 entry must not carry policy_digest")
		}
		if e.Policy == nil {
			return fmt.Errorf("designprovenance: v2 entry requires a policy field")
		}
		if err := e.Policy.validate(); err != nil {
			return fmt.Errorf("designprovenance: policy: %w", err)
		}
		// A nonblank Harness is only ever produced by a delegated-agent
		// actor (draftmutation's own actor-shape invariant); such actors
		// are never routed through the explicit browser-human's
		// not-applicable posture, so a harness-bearing v2 entry recording
		// anything but a resolved policy is structurally impossible from
		// any conforming writer and fails closed here as a defense-in-
		// depth witness of that invariant. The general shape rule below
		// subsumes this case; it is kept ahead of that rule solely for its
		// more specific operator diagnostic.
		if e.Harness != "" && e.Policy.State != PolicyResolved {
			return fmt.Errorf("designprovenance: delegated-agent entries must record a resolved policy")
		}
		// PolicyNotApplicable is reachable from exactly ONE writer shape:
		// the explicit browser-human actor (draftmutation's
		// NewUnauthenticatedHuman), which carries the kernel's
		// unauthenticated attribution with no principal id and no
		// harness/session at all. Every other conforming writer — a
		// delegated agent (nonblank harness) or a resolved principal
		// human (nonempty PrincipalID) — records a resolved policy, so
		// binding the not-applicable arm to that exact shape makes an
		// entry claiming non-adoption from a shape that could never have
		// produced it fail closed rather than pass as honest history.
		if e.Policy.State == PolicyNotApplicable &&
			(!e.Attribution.Unauthenticated || e.Attribution.PrincipalID != "" || e.Harness != "" || e.Session != "") {
			return fmt.Errorf("designprovenance: not-applicable policy requires the explicit unauthenticated-human shape with no principal, harness, or session")
		}
	}
	if err := e.Attribution.Validate(); err != nil {
		return err
	}
	if e.Attribution.Unauthenticated {
		if e.Harness != "" && (strings.TrimSpace(e.Harness) == "" || !utf8.ValidString(e.Harness)) {
			return fmt.Errorf("designprovenance: unauthenticated harness must be nonblank valid UTF-8 when present")
		}
		if e.Session != "" && e.Harness == "" {
			return fmt.Errorf("designprovenance: unauthenticated session requires harness attribution")
		}
		if e.Session != "" && (strings.TrimSpace(e.Session) == "" || !utf8.ValidString(e.Session)) {
			return fmt.Errorf("designprovenance: session must be nonblank valid UTF-8 when present")
		}
	} else if e.Harness != "" || e.Session != "" {
		return fmt.Errorf("designprovenance: principal attribution must omit harness and session")
	}
	if e.Context != UnavailableContext() {
		return fmt.Errorf("designprovenance: context must use fixed unavailable reason %q", ContextUnavailableReason)
	}
	if len(e.Operations) == 0 {
		return fmt.Errorf("designprovenance: operations must be nonempty")
	}
	if e.Changes == nil || e.Excerpts == nil {
		return fmt.Errorf("designprovenance: changes and excerpts must encode as arrays, not null")
	}
	for i, op := range e.Operations {
		if err := op.Validate(); err != nil {
			return fmt.Errorf("designprovenance: operations[%d]: %w", i, err)
		}
	}
	for i, change := range e.Changes {
		if err := change.Validate(); err != nil {
			return fmt.Errorf("designprovenance: changes[%d]: %w", i, err)
		}
	}
	counts := map[string]int{}
	for i, excerpt := range e.Excerpts {
		if err := excerpt.validate(); err != nil {
			return fmt.Errorf("designprovenance: excerpts[%d]: %w", i, err)
		}
		counts[excerpt.Target]++
		if counts[excerpt.Target] > MaxExcerptsPerTarget {
			return fmt.Errorf("designprovenance: target %q has more than three excerpts", excerpt.Target)
		}
	}
	if e.UnclassifiedGap != nil {
		if !artifact.ValidDigest(e.UnclassifiedGap.FromDigest) || !artifact.ValidDigest(e.UnclassifiedGap.ToDigest) || e.UnclassifiedGap.ToDigest != e.PreviousDigest {
			return fmt.Errorf("designprovenance: invalid unclassified gap")
		}
	}
	if checkDigest {
		want, err := canonjson.Digest(e.digestProjection())
		if err != nil {
			return err
		}
		if e.Digest != want {
			return fmt.Errorf("designprovenance: own digest %q does not match %q", e.Digest, want)
		}
	}
	return nil
}
