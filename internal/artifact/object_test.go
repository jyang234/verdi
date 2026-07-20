package artifact

import (
	"strings"
	"testing"
)

func TestAttribute_Validate_Happy(t *testing.T) {
	a := Attribute{Text: "borrowers cannot self-serve", Anchor: "#problem"}
	if err := a.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestAttribute_Validate_Negative(t *testing.T) {
	cases := []Attribute{
		{Text: "", Anchor: "#problem"},
		{Text: "borrowers cannot self-serve", Anchor: ""},
		{},
	}
	for i, a := range cases {
		if err := a.Validate(); err == nil {
			t.Fatalf("case %d Validate(%+v): want error, got nil", i, a)
		}
	}
}

func TestConstraint_Validate_Happy(t *testing.T) {
	c := Constraint{ID: "co-1", Text: "must not touch legacy schema", Anchor: "#co-1"}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestConstraint_Validate_Negative(t *testing.T) {
	cases := []Constraint{
		{ID: "bad-id", Text: "t", Anchor: "#a"},
		{ID: "co-1", Text: "", Anchor: "#a"},
		{ID: "co-1", Text: "t", Anchor: ""},
		{ID: "ac-1", Text: "t", Anchor: "#a"}, // wrong prefix
	}
	for i, c := range cases {
		if err := c.Validate(); err == nil {
			t.Fatalf("case %d Validate(%+v): want error, got nil", i, c)
		}
	}
}

func TestDecision_Validate_Happy(t *testing.T) {
	cases := []Decision{
		{ID: "dc-1", Text: "use outbox pattern", Anchor: "#dc-1"},
		{ID: "dc-2", Text: "excuse from ADR-12", Anchor: "#dc-2",
			Links: []Link{{Type: LinkExempts, Ref: "adr/0012-outbox-loansvc-events", Note: "legacy schema constraint"}}},
	}
	for i, d := range cases {
		if err := d.Validate(); err != nil {
			t.Fatalf("case %d Validate(%+v): %v", i, d, err)
		}
	}
}

func TestDecision_Validate_Negative(t *testing.T) {
	cases := []Decision{
		{ID: "bad-id", Text: "t", Anchor: "#a"},
		{ID: "dc-1", Text: "", Anchor: "#a"},
		{ID: "dc-1", Text: "t", Anchor: ""},
		{ID: "dc-1", Text: "t", Anchor: "#a", Links: []Link{{Type: "bogus", Ref: "adr/0001"}}},
	}
	for i, d := range cases {
		if err := d.Validate(); err == nil {
			t.Fatalf("case %d Validate(%+v): want error, got nil", i, d)
		}
	}
}

func TestOpenQuestion_Validate_Happy(t *testing.T) {
	q := OpenQuestion{ID: "oq-1", Text: "should this route be PUT or PATCH?", Anchor: "#oq-1"}
	if err := q.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestOpenQuestion_Validate_Negative(t *testing.T) {
	cases := []OpenQuestion{
		{ID: "bad-id", Text: "t", Anchor: "#a"},
		{ID: "oq-1", Text: "", Anchor: "#a"},
		{ID: "oq-1", Text: "t", Anchor: ""},
		{ID: "co-1", Text: "t", Anchor: "#a"}, // wrong prefix
	}
	for i, q := range cases {
		if err := q.Validate(); err == nil {
			t.Fatalf("case %d Validate(%+v): want error, got nil", i, q)
		}
	}
}

// guide-claim: 6.2-stubs
func TestStub_Validate_Happy(t *testing.T) {
	s := Stub{Slug: "borrower-update-api", AcceptanceCriteria: []string{"ac-1"}}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestStub_Validate_Negative(t *testing.T) {
	cases := []Stub{
		{Slug: "Not-Kebab", AcceptanceCriteria: []string{"ac-1"}},
		{Slug: "borrower-update-api", AcceptanceCriteria: nil},
		{Slug: "borrower-update-api", AcceptanceCriteria: []string{"bad-id"}},
	}
	for i, s := range cases {
		if err := s.Validate(); err == nil {
			t.Fatalf("case %d Validate(%+v): want error, got nil", i, s)
		}
	}
}

// TestSpikeStub_Validate_Happy proves the round-5.4 spike-stub shape (02
// §Kind registry amendment, DC-4): spike: true plus a non-empty resolves
// list of oq-<slug> ids, and no acceptance_criteria.
func TestSpikeStub_Validate_Happy(t *testing.T) {
	s := Stub{Slug: "retry-strategy-spike", Spike: true, Resolves: []string{"oq-1", "oq-2"}}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// TestSpikeStub_Validate_Negative proves the DC-4 grammar fails closed:
// resolves requires spike: true; a spike stub declares resolves and no
// acceptance_criteria; a plain stub the reverse.
func TestSpikeStub_Validate_Negative(t *testing.T) {
	cases := map[string]Stub{
		"resolves without spike: true":       {Slug: "retry-strategy-spike", Resolves: []string{"oq-1"}},
		"spike with no resolves":             {Slug: "retry-strategy-spike", Spike: true},
		"spike with acceptance_criteria":     {Slug: "retry-strategy-spike", Spike: true, Resolves: []string{"oq-1"}, AcceptanceCriteria: []string{"ac-1"}},
		"spike resolves entry not oq-shaped": {Slug: "retry-strategy-spike", Spike: true, Resolves: []string{"bad-id"}},
		"spike resolves entry is an ac id":   {Slug: "retry-strategy-spike", Spike: true, Resolves: []string{"ac-1"}},
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			if err := s.Validate(); err == nil {
				t.Fatalf("Validate(%+v): want error, got nil", s)
			}
		})
	}
}

func TestObjectContentHash_Deterministic(t *testing.T) {
	h1, err := ObjectContentHash(ObjectKindAcceptanceCriterion, "ac-2", "the update API has no PUT route")
	if err != nil {
		t.Fatalf("ObjectContentHash: %v", err)
	}
	h2, err := ObjectContentHash(ObjectKindAcceptanceCriterion, "ac-2", "the update API has no PUT route")
	if err != nil {
		t.Fatalf("ObjectContentHash: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("same (kind, id, text) produced different hashes: %q vs %q", h1, h2)
	}
	if !sha256Re.MatchString(h1) {
		t.Fatalf("ObjectContentHash = %q, want sha256:<64 hex> form", h1)
	}
}

// TestObjectContentHash_RoundTripsThroughDecode proves an object decoded
// from raw frontmatter bytes hashes identically to the same (kind, id,
// text) tuple computed directly — the "object IDs round-trip through the
// content hash" exit criterion: decoding never perturbs the bytes the
// identity hash is computed over.
func TestObjectContentHash_RoundTripsThroughDecode(t *testing.T) {
	const y = `
id: spec/hash-roundtrip
kind: spec
class: feature
title: "Hash round-trip fixture"
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-2, text: "the update API has no PUT route for a submitted application", evidence: [static], anchor: "#ac-2" }
`
	fm, err := DecodeSpec([]byte(y))
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	ac := fm.AcceptanceCriteria[0]

	decoded, err := ObjectContentHash(ObjectKindAcceptanceCriterion, ac.ID, ac.Text)
	if err != nil {
		t.Fatalf("ObjectContentHash(decoded): %v", err)
	}
	direct, err := ObjectContentHash(ObjectKindAcceptanceCriterion, "ac-2", "the update API has no PUT route for a submitted application")
	if err != nil {
		t.Fatalf("ObjectContentHash(direct): %v", err)
	}
	if decoded != direct {
		t.Fatalf("hash computed from decoded object %q != hash computed directly %q", decoded, direct)
	}
}

func TestObjectContentHash_ChangedTextChangesHash(t *testing.T) {
	h1, err := ObjectContentHash(ObjectKindAcceptanceCriterion, "ac-2", "original text")
	if err != nil {
		t.Fatalf("ObjectContentHash: %v", err)
	}
	h2, err := ObjectContentHash(ObjectKindAcceptanceCriterion, "ac-2", "amended text")
	if err != nil {
		t.Fatalf("ObjectContentHash: %v", err)
	}
	if h1 == h2 {
		t.Fatalf("changed text produced the same hash %q — carried/amended classification would be indistinguishable", h1)
	}
}

func TestObjectContentHash_ChangedKindChangesHash(t *testing.T) {
	h1, err := ObjectContentHash(ObjectKindAcceptanceCriterion, "x-1", "same text")
	if err != nil {
		t.Fatalf("ObjectContentHash: %v", err)
	}
	h2, err := ObjectContentHash(ObjectKindConstraint, "x-1", "same text")
	if err != nil {
		t.Fatalf("ObjectContentHash: %v", err)
	}
	if h1 == h2 {
		t.Fatalf("changed object kind produced the same hash %q for the same (id, text)", h1)
	}
}

func TestHeadingAnchors_And_ResolveAnchor(t *testing.T) {
	body := []byte("# Title\n\n## Problem\n\ntext\n\n## AC-2\n\nmore text\n")
	anchors := HeadingAnchors(body)
	for _, want := range []string{"title", "problem", "ac-2"} {
		if !anchors[want] {
			t.Fatalf("HeadingAnchors(%q) missing %q, got %v", body, want, anchors)
		}
	}
	if !ResolveAnchor(anchors, "#problem") {
		t.Fatal("ResolveAnchor(#problem) = false, want true")
	}
	if !ResolveAnchor(anchors, "ac-2") {
		t.Fatal("ResolveAnchor(ac-2) (no leading #) = false, want true")
	}
	if ResolveAnchor(anchors, "#nonexistent") {
		t.Fatal("ResolveAnchor(#nonexistent) = true, want false")
	}
}

func TestResolveObjectAnchors_Happy(t *testing.T) {
	const y = `
id: spec/anchor-happy
kind: spec
class: feature
title: "Anchor happy fixture"
status: draft
owners: [platform-team]
problem: { text: "borrowers cannot self-serve", anchor: "#problem" }
outcome: { text: "a borrower can update their application", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can update their application", evidence: [static], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "must not touch legacy schema", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "use outbox pattern", anchor: "#dc-1" }
`
	fm, err := DecodeSpec([]byte(y))
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	body := []byte("# Anchor happy fixture\n\n## Problem\n\n## Outcome\n\n## AC-1\n\n## CO-1\n\n## DC-1\n")
	if err := fm.ResolveObjectAnchors(body); err != nil {
		t.Fatalf("ResolveObjectAnchors: %v", err)
	}
}

// TestResolveObjectAnchors_MismatchedAnchorFails is the "mismatched-anchor
// twin fails naming the anchor rule" exit criterion: an object's anchor
// pointing at a heading that does not exist in the body must fail, and the
// error must name the anchor-resolution rule.
func TestResolveObjectAnchors_MismatchedAnchorFails(t *testing.T) {
	const y = `
id: spec/anchor-mismatch
kind: spec
class: feature
title: "Anchor mismatch fixture"
status: draft
owners: [platform-team]
problem: { text: "borrowers cannot self-serve", anchor: "#problem" }
outcome: { text: "a borrower can update their application", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can update their application", evidence: [static], anchor: "#nonexistent-heading" }
`
	fm, err := DecodeSpec([]byte(y))
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	body := []byte("# Anchor mismatch fixture\n\n## Problem\n\n## Outcome\n")
	err = fm.ResolveObjectAnchors(body)
	if err == nil {
		t.Fatal("ResolveObjectAnchors: want error for mismatched anchor, got nil")
	}
	if !strings.Contains(err.Error(), "anchor") || !strings.Contains(err.Error(), "ac-1") {
		t.Fatalf("ResolveObjectAnchors error = %q, want it to name the anchor rule and the offending object", err)
	}
}
