package artifact

import "fmt"

// SupersessionNote is one `{id, note}` entry in a feature spec's
// `supersession:` block — used by the amended/amended_advisory/removed
// buckets, each of which requires a reason (03 §The amendment ladder).
type SupersessionNote struct {
	ID   string `yaml:"id"`
	Note string `yaml:"note"`
}

// Supersession is a superseding feature spec revision's structured object
// manifest (02 §Kind registry, R4-I-4; 03 §The amendment ladder rung 4):
// classifies every predecessor object exactly once across
// carried/amended/amended_advisory/removed, plus added for wholly new
// objects. This package validates entry shape only — completeness
// (every predecessor object classified exactly once) and carried-content
// byte-identity are VL-015's job (V1-P2), not decoded/checked here per this
// phase's "pure types" posture.
type Supersession struct {
	Carried         []string           `yaml:"carried,omitempty"`
	Amended         []SupersessionNote `yaml:"amended,omitempty"`
	AmendedAdvisory []SupersessionNote `yaml:"amended_advisory,omitempty"`
	Removed         []SupersessionNote `yaml:"removed,omitempty"`
	Added           []string           `yaml:"added,omitempty"`
}

// Validate checks every id (in every bucket) looks like a real object id
// and every note-carrying entry (amended/amended_advisory/removed) carries
// a non-empty note.
func (s Supersession) Validate() error {
	for i, id := range s.Carried {
		if !objectIDRe.MatchString(id) {
			return fmt.Errorf("carried[%d]: %q is not a valid object id", i, id)
		}
	}
	for i, n := range s.Amended {
		if err := n.validate(); err != nil {
			return fmt.Errorf("amended[%d]: %w", i, err)
		}
	}
	for i, n := range s.AmendedAdvisory {
		if err := n.validate(); err != nil {
			return fmt.Errorf("amended_advisory[%d]: %w", i, err)
		}
	}
	for i, n := range s.Removed {
		if err := n.validate(); err != nil {
			return fmt.Errorf("removed[%d]: %w", i, err)
		}
	}
	for i, id := range s.Added {
		if !objectIDRe.MatchString(id) {
			return fmt.Errorf("added[%d]: %q is not a valid object id", i, id)
		}
	}
	return nil
}

// WholeSpecSupersedesRefs returns every `links: {type: supersedes}` entry
// in links whose ref names a WHOLE spec-kind artifact, normalized to its
// unpinned kind/name form, in link order.
//
// Two shapes are deliberately excluded, because neither is a claim to
// replace a whole spec document:
//
//   - an object-FRAGMENT supersedes edge (spec/x#dc-1) is a decision-level
//     override (02 §Object model: decision objects "may carry their own
//     links: ... for supersedes/exempts edges against ADRs or other
//     decisions"; 03 §Decision-conflict gate), scoped to that one object;
//   - a non-spec-kind (adr/..., or an unparseable external svc/... ref)
//     target, which names no spec predecessor at all.
//
// It is the single shared definition of "whole-spec predecessor" that
// internal/artifact's own validation, internal/lint's VL-015, and
// internal/specstate's successor-corpus scan all resolve the question
// through — never three drifting copies (CLAUDE.md: anything used by two or
// more packages lives in one shared internal/ package).
func WholeSpecSupersedesRefs(links []Link) []Ref {
	var out []Ref
	for _, l := range links {
		if l.Type != LinkSupersedes {
			continue
		}
		ref, err := ParseRef(l.Ref)
		if err != nil || ref.Kind != KindSpec || ref.Fragment() {
			continue
		}
		out = append(out, Ref{Kind: ref.Kind, Name: ref.Name})
	}
	return out
}

// validateSupersessionPredecessor enforces I-47 (PLAN.md §7): a spec
// carrying a `supersession:` block must name EXACTLY ONE whole-spec,
// spec-kind predecessor via `links: {type: supersedes}`.
//
// The block is a manifest ABOUT one named predecessor's object set — 02
// §Lint rules states VL-015 as "every object in THE predecessor spec (at
// ITS frozen.commit) is classified exactly once", a check that is simply
// uncomputable against zero predecessors and ambiguous against two or more
// (one manifest cannot honestly classify two different object sets). The
// specs never write the cardinality as a rule, so this is the smallest
// reversible reading recorded in the invention ledger; fragment override
// edges alongside the single whole-spec one stay legal, and superseding a
// non-spec artifact (an ADR) is likewise untouched.
//
// Enforced here, at the decode seam, rather than only in lint: internal/
// specstate's successor-corpus scan credits a validated supersession: block
// to the predecessor it names, and must fail CLOSED on an ambiguous shape
// (the predecessor projects disclosed-unproven via corpus.failures) without
// depending on lint having run at all.
func (fm SpecFrontmatter) validateSupersessionPredecessor() error {
	refs := WholeSpecSupersedesRefs(fm.Links)
	if len(refs) == 1 {
		return nil
	}
	names := make([]string, 0, len(refs))
	for _, r := range refs {
		names = append(names, r.String())
	}
	// vocab:identity — strict-decode/schema diagnostic speaking class/field ids
	return fmt.Errorf(
		"artifact: a spec carrying a supersession: block must name exactly one whole-spec predecessor via links: {type: supersedes}, got %d %v (a fragment supersedes edge is a decision-level override, not a whole-spec predecessor; I-47)",
		len(refs), names,
	)
}

func (n SupersessionNote) validate() error {
	if !objectIDRe.MatchString(n.ID) {
		return fmt.Errorf("%q is not a valid object id", n.ID)
	}
	if n.Note == "" {
		return fmt.Errorf("entry %q has no note", n.ID)
	}
	return nil
}
