package designapp

import "github.com/jyang234/verdi/internal/artifact"

// This file holds designapp's OWN wire projections of the corpus spec
// objects AC-5 and AC-6 name. They exist for two reasons, both CO-2
// ("versioned strict schemas ... for capability responses and every typed
// operation"):
//
//  1. Grammar. The authoring types in internal/artifact carry only `yaml`
//     tags (an authoring-corpus convention). Marshaling them straight into
//     a JSON response leaks Go field names — "ID", "AcceptanceCriteria",
//     "OpenQuestions" — into a public response grammar whose every other
//     field is snake_case. The projection is re-tagged here rather than by
//     adding `json` tags to the shared artifact types, because those types
//     are the corpus's own decode surface: giving them a wire grammar would
//     make every future response in the repository silently inherit it.
//     Where a shared type ALREADY carries json tags (artifact.Link,
//     artifact.Attribute), it is reused verbatim rather than re-projected.
//
//  2. Boundedness. AC-5's and AC-6's content lists are exhaustive. Fields
//     that appear in neither list — Schema, Status, Frozen, Provenance,
//     Spike, Impacts, Declares, Supersession, Dispositions, and the
//     free-form Custom extension namespace — are internal authoring or
//     lifecycle state and are deliberately NOT projected. Status in
//     particular is never authority (specstate derives lifecycle state from
//     Git), so echoing it into an agent-facing response would advertise a
//     field an agent must not read. The one Git-derived state that IS
//     authority is reported explicitly and separately
//     (CapabilitiesResult.SpecState, DesignContextResult's ratified-decision
//     posture).
//
// Every conversion below copies each slice, so a caller can mutate a
// result without reaching back into the decoded corpus artifact
// (deep-copy safety, Task 1 contract).

// AcceptanceCriterion is AC-6's "acceptance criteria and declared
// evidence kinds" (and part of AC-5's current draft).
type AcceptanceCriterion struct {
	ID       string                  `json:"id"`
	Text     string                  `json:"text"`
	Evidence []artifact.EvidenceKind `json:"evidence"`
	Anchor   string                  `json:"anchor,omitempty"`
}

// Constraint is one `constraints:` entry.
type Constraint struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Anchor string `json:"anchor,omitempty"`
}

// Decision is one `decisions:` entry, with its own outgoing typed links
// (artifact.Link already carries json tags and is reused unchanged).
type Decision struct {
	ID     string          `json:"id"`
	Text   string          `json:"text"`
	Anchor string          `json:"anchor,omitempty"`
	Links  []artifact.Link `json:"links,omitempty"`
}

// OpenQuestion is one `open_questions:` entry.
type OpenQuestion struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Anchor string `json:"anchor,omitempty"`
}

// Stub is one `stubs:` entry (DC-4's two mutually exclusive arms are
// preserved exactly as declared; this projection never re-derives them).
type Stub struct {
	Slug               string   `json:"slug"`
	Spike              bool     `json:"spike,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	Resolves           []string `json:"resolves,omitempty"`
}

// SpecContent is one spec document's AC-5/AC-6 content: its identity, its
// problem and outcome, its declared pinned context references, its typed
// links, and every semantic object AC-1's closed operation vocabulary can
// address. Collections are always arrays, never null, so a caller never
// has to distinguish "absent" from "empty" (CO-1: silence is never a
// pass).
type SpecContent struct {
	ID                 string                `json:"id"`
	Kind               string                `json:"kind"`
	Class              string                `json:"class"`
	Title              string                `json:"title"`
	Owners             []string              `json:"owners"`
	Story              string                `json:"story,omitempty"`
	Problem            *artifact.Attribute   `json:"problem"`
	Outcome            *artifact.Attribute   `json:"outcome"`
	Context            []string              `json:"context"`
	Links              []artifact.Link       `json:"links"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria"`
	Constraints        []Constraint          `json:"constraints"`
	Decisions          []Decision            `json:"decisions"`
	OpenQuestions      []OpenQuestion        `json:"open_questions"`
	Stubs              []Stub                `json:"stubs"`
}

// projectSpec converts one decoded corpus spec into its bounded wire
// projection. A nil input projects to nil (an absent parent feature is
// absent, never an empty document).
func projectSpec(spec *artifact.SpecFrontmatter) *SpecContent {
	if spec == nil {
		return nil
	}
	return &SpecContent{
		ID:                 spec.ID,
		Kind:               string(spec.Kind),
		Class:              string(spec.Class),
		Title:              spec.Title,
		Owners:             copyStrings(spec.Owners),
		Story:              spec.Story,
		Problem:            copyAttribute(spec.Problem),
		Outcome:            copyAttribute(spec.Outcome),
		Context:            copyStrings(spec.Context),
		Links:              projectLinks(spec.Links),
		AcceptanceCriteria: projectAcceptanceCriteria(spec.AcceptanceCriteria),
		Constraints:        projectConstraints(spec.Constraints),
		Decisions:          projectDecisions(spec.Decisions),
		OpenQuestions:      projectOpenQuestions(spec.OpenQuestions),
		Stubs:              projectStubs(spec.Stubs),
	}
}

func copyStrings(values []string) []string {
	return append([]string{}, values...)
}

func copyAttribute(attribute *artifact.Attribute) *artifact.Attribute {
	if attribute == nil {
		return nil
	}
	copied := *attribute
	return &copied
}

func projectLinks(links []artifact.Link) []artifact.Link {
	return append([]artifact.Link{}, links...)
}

func projectAcceptanceCriteria(criteria []artifact.AcceptanceCriterion) []AcceptanceCriterion {
	projected := make([]AcceptanceCriterion, 0, len(criteria))
	for _, criterion := range criteria {
		projected = append(projected, AcceptanceCriterion{
			ID:       criterion.ID,
			Text:     criterion.Text,
			Evidence: append([]artifact.EvidenceKind{}, criterion.Evidence...),
			Anchor:   criterion.Anchor,
		})
	}
	return projected
}

func projectConstraints(constraints []artifact.Constraint) []Constraint {
	projected := make([]Constraint, 0, len(constraints))
	for _, constraint := range constraints {
		projected = append(projected, Constraint{ID: constraint.ID, Text: constraint.Text, Anchor: constraint.Anchor})
	}
	return projected
}

func projectDecisions(decisions []artifact.Decision) []Decision {
	projected := make([]Decision, 0, len(decisions))
	for _, decision := range decisions {
		entry := Decision{ID: decision.ID, Text: decision.Text, Anchor: decision.Anchor}
		if len(decision.Links) > 0 {
			entry.Links = projectLinks(decision.Links)
		}
		projected = append(projected, entry)
	}
	return projected
}

func projectOpenQuestions(questions []artifact.OpenQuestion) []OpenQuestion {
	projected := make([]OpenQuestion, 0, len(questions))
	for _, question := range questions {
		projected = append(projected, OpenQuestion{ID: question.ID, Text: question.Text, Anchor: question.Anchor})
	}
	return projected
}

func projectStubs(stubs []artifact.Stub) []Stub {
	projected := make([]Stub, 0, len(stubs))
	for _, stub := range stubs {
		entry := Stub{Slug: stub.Slug, Spike: stub.Spike}
		if len(stub.AcceptanceCriteria) > 0 {
			entry.AcceptanceCriteria = copyStrings(stub.AcceptanceCriteria)
		}
		if len(stub.Resolves) > 0 {
			entry.Resolves = copyStrings(stub.Resolves)
		}
		projected = append(projected, entry)
	}
	return projected
}
