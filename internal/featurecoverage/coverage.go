// Package featurecoverage computes an accepted feature's acceptance-
// criterion coverage as one pure function (spec/index-coverage ac-1,
// dc-1, co-1, co-2): for each declared criterion, which stubs list it and
// which stories implement it. It is assembled from the two projections
// the board already computes — the wall's stub-declaration counts
// (internal/workbench's scoping-canvas projection) and the corpus index's
// implements backlinks (internal/index) — not written anew, and it is the
// one shared package the wall, the index, and the New story dialog all
// import (co-2: no consumer copies this logic).
//
// Coverage is family structure, never evidence-bearing state (co-1): a
// declared stub and an implements edge are both unverified claims of
// relatedness, never proof that anything is implemented or tested. An
// input the caller could not read is disclosed on every criterion it
// might have affected, never silently counted as no coverage (ac-2).
package featurecoverage

// StubDecl is one declared stub as Compute reads it (ac-1, dc-1): its
// slug and the acceptance-criterion ids it declares. A spike stub's
// AcceptanceCriteria is always empty — spike stubs resolve open
// questions, never acceptance criteria; artifact.Stub.Validate enforces
// the two lists as mutually exclusive — so a caller may pass every
// declared stub through unconditionally at no cost.
//
// Unreadable, when non-empty, means the caller could not decode this
// stub's declaration at all. Compute then discloses Unreadable's text on
// EVERY criterion id it was given (ac-2): an undecodable declaration
// could have named any of them, and none is ruled out. AcceptanceCriteria
// is ignored when Unreadable is set.
type StubDecl struct {
	Slug               string
	AcceptanceCriteria []string
	Unreadable         string
}

// StoryLink is one implements-edge backlink onto this feature's
// acceptance criteria, as Compute reads it (dc-1): the id of the
// criterion the edge's fragment names (spec/<feature>#<ac>) and the
// implementing story's ref. This is read directly from the corpus
// index's backlinks, never through a stub's own declared criterion list
// (dc-1) — a story implementing a criterion no stub lists still covers
// it.
//
// Unreadable, when non-empty, means the caller could not resolve this
// backlink into a trustworthy story identity. Compute then discloses
// Unreadable's text on the one named criterion only (ac-2): unlike an
// unreadable stub, a backlink already names exactly which criterion it
// targets. StoryRef is ignored when Unreadable is set.
type StoryLink struct {
	CriterionID string
	StoryRef    string
	Unreadable  string
}

// Coverage is what covers one acceptance criterion (ac-1): the slugs of
// the declared stubs that list it and the refs of the stories whose
// implements edge targets it — either half possibly empty — plus every
// disclosed reason Compute could not read an input that might have
// covered it (ac-2). Coverage never says "implemented" or "evidenced":
// it is family structure only (co-1), read from declarations and edges,
// never from a verdict, a receipt, or a gate result.
type Coverage struct {
	Stubs     []string
	Stories   []string
	Disclosed []string
}

// Uncovered reports dc-2's call-to-action test: no declared stub lists
// this criterion and no story implements it. An unreadable input is
// never counted as no coverage (ac-2), so a criterion carrying only a
// disclosure is not Uncovered either — the caller renders the disclosure
// in place of a bare "uncovered" claim.
func (c Coverage) Uncovered() bool {
	return len(c.Stubs) == 0 && len(c.Stories) == 0 && len(c.Disclosed) == 0
}

// Compute is spec/index-coverage's one pure function (ac-1, dc-1, co-1,
// co-3): for every id in ids (an accepted feature's declared acceptance-
// criterion ids, in declared order), it returns the stub half from stubs
// and the story half from links. It reads nothing else — no I/O, no
// index walk, no git, no clock, no randomness (ac-1: "it reads nothing
// else").
//
// A stub or link naming a criterion id absent from ids is silently
// ignored — it is not this feature's own declared criterion to cover.
// Every id in ids is present in the result with an explicit (possibly
// empty) Coverage, never an absent key, so "no coverage" is always a
// computed fact rather than a missing lookup.
func Compute(ids []string, stubs []StubDecl, links []StoryLink) map[string]Coverage {
	result := make(map[string]Coverage, len(ids))
	for _, id := range ids {
		result[id] = Coverage{}
	}

	for _, sd := range stubs {
		if sd.Unreadable != "" {
			for _, id := range ids {
				c := result[id]
				c.Disclosed = append(c.Disclosed, sd.Unreadable)
				result[id] = c
			}
			continue
		}
		// Per-stub dedup: a stub repeating an id within its own declared
		// list still counts once (artifact.Stub.Validate already refuses
		// this shape on decode; this is defense-in-depth for a value that
		// reached Compute another way — an in-memory literal, or a value
		// decoded before that refusal existed — mirroring the wall
		// projection's own pre-extraction dedup).
		counted := make(map[string]bool, len(sd.AcceptanceCriteria))
		for _, id := range sd.AcceptanceCriteria {
			if counted[id] {
				continue
			}
			counted[id] = true
			c, ok := result[id]
			if !ok {
				continue
			}
			c.Stubs = append(c.Stubs, sd.Slug)
			result[id] = c
		}
	}

	for _, ln := range links {
		c, ok := result[ln.CriterionID]
		if !ok {
			continue
		}
		if ln.Unreadable != "" {
			c.Disclosed = append(c.Disclosed, ln.Unreadable)
		} else {
			c.Stories = append(c.Stories, ln.StoryRef)
		}
		result[ln.CriterionID] = c
	}

	return result
}
