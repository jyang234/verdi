package specdoc

// Words carries every class word the renderer speaks, resolved through the
// model display chain at Build time so no renderer literal names a class.
// StoryPlural/SpikePlural are the model's own DisplayClassPlural result
// (irregular-plural-aware, e.g. "story" -> "stories"); the renderer never
// hand-pluralizes a class word itself (fix round 1, F1).
type Words struct {
	Feature     string
	Story       string
	StoryPlural string
	Spike       string
	SpikePlural string
}

// KV is one identity row.
type KV struct {
	Label string
	Value string
}

// Item is a decision or constraint: its id, its declared text, and the
// rationale found under its body heading (empty when the body has none).
type Item struct {
	ID     string
	Text   string
	Detail string
}

// Criterion is one acceptance criterion with its evidence kinds (as
// declared, verbatim) and its computed coverage.
type Criterion struct {
	ID       string
	Text     string
	Evidence []string
	// Coverage lists covering stub slugs; CoverageKnown is false when the
	// facts did not supply coverage.
	Coverage      []string
	CoverageKnown bool
	Detail        string
}

// Question is one open question with its claiming spike stubs.
type Question struct {
	ID          string
	Text        string
	Claims      []string
	ClaimsKnown bool
	Detail      string
}

// PlanItem is one stub: a planned story covering criteria, or a spike
// resolving questions.
type PlanItem struct {
	Slug     string
	Spike    bool
	Criteria []string
	Resolves []string
}

// EvidenceRow is one criterion's matrix state.
type EvidenceRow struct {
	ID      string
	Status  string
	Summary string
	Stories []string
	Kinds   []KindEvidence
}

// Document is the fully resolved model a renderer walks. Every slice is in
// declaration order; nothing is sorted at render time.
type Document struct {
	Kind     Kind
	Stamp    Stamp
	Sections []SectionID
	Words    Words

	Title    string
	Identity []KV

	// ProblemText/OutcomeText are the declared attribute texts; the
	// *Known flags are false when the spec declares none (a story spec).
	ProblemText  string
	ProblemKnown bool
	OutcomeText  string
	OutcomeKnown bool

	Decisions   []Item
	Constraints []Item
	Criteria    []Criterion
	Questions   []Question
	Plan        []PlanItem

	Evidence       []EvidenceRow
	EvidenceKnown  bool
	EvidenceSource string

	Readiness      *ReadinessFacts
	ReadinessKnown bool
}
