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

// Entry is one canonical JSONL sidecar record.
type Entry struct {
	Schema          string                          `json:"schema"`
	Spec            string                          `json:"spec"`
	PreviousDigest  string                          `json:"previous_digest"`
	ResultDigest    string                          `json:"result_digest"`
	UnclassifiedGap *UnclassifiedGap                `json:"unclassified_gap,omitempty"`
	Attribution     governanceprincipal.Attribution `json:"attribution"`
	Harness         string                          `json:"harness,omitempty"`
	Session         string                          `json:"session,omitempty"`
	PolicyDigest    string                          `json:"policy_digest"`
	Context         Context                         `json:"context"`
	Operations      []Operation                     `json:"operations"`
	Changes         []Change                        `json:"changes"`
	Excerpts        []Excerpt                       `json:"excerpts"`
	Digest          string                          `json:"digest"`
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
		PolicyDigest    string                          `json:"policy_digest"`
		Context         Context                         `json:"context"`
		Operations      []Operation                     `json:"operations"`
		Changes         []Change                        `json:"changes"`
		Excerpts        []Excerpt                       `json:"excerpts"`
	}{p.Schema, p.Spec, p.PreviousDigest, p.ResultDigest, p.UnclassifiedGap, p.Attribution, p.Harness, p.Session, p.PolicyDigest, p.Context, p.Operations, p.Changes, p.Excerpts}
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
	if e.Schema != Schema {
		return fmt.Errorf("designprovenance: unknown schema %q", e.Schema)
	}
	ref, err := artifact.ParseRef(e.Spec)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return fmt.Errorf("designprovenance: spec %q must be an unpinned whole spec ref", e.Spec)
	}
	if !artifact.ValidDigest(e.PreviousDigest) || !artifact.ValidDigest(e.ResultDigest) || !artifact.ValidDigest(e.PolicyDigest) {
		return fmt.Errorf("designprovenance: previous, result, and policy digests must be canonical sha256 values")
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
