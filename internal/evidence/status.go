package evidence

// Status is one AC's folded outcome (03 §The fold). Precedence is total:
// waived > violated > evidenced > pending > no-signal — see Rank.
type Status string

const (
	StatusWaived    Status = "waived"
	StatusViolated  Status = "violated"
	StatusEvidenced Status = "evidenced"
	StatusPending   Status = "pending"
	StatusNoSignal  Status = "no-signal"
)

// rank orders Status by 03's total precedence, lowest number first
// (waived ranks 0, no-signal ranks 4) — exposed via Rank for callers
// (tests, rendering) that need to compare two statuses without
// duplicating the precedence table.
var rank = map[Status]int{
	StatusWaived:    0,
	StatusViolated:  1,
	StatusEvidenced: 2,
	StatusPending:   3,
	StatusNoSignal:  4,
}

// Rank returns s's position in 03's total precedence order (lower is
// higher-precedence: waived=0 ... no-signal=4), or -1 for an unknown
// status.
func Rank(s Status) int {
	if r, ok := rank[s]; ok {
		return r
	}
	return -1
}

// ACResult is one acceptance criterion's folded outcome, enough to render
// one `verdi matrix` row.
type ACResult struct {
	ID      string
	Text    string
	Status  Status
	Summary string // one-line evidence summary, e.g. "static:pass; behavioral:pending(no-record)"
}

// StoryResult is a whole story's folded outcome (03 §The fold:
// "story.violated ... story.eligible").
type StoryResult struct {
	Story    string // the spec's story: field (e.g. "jira:LOAN-1482")
	SpecRef  string // the resolved spec's canonical ref (e.g. "spec/stale-decline")
	ACs      []ACResult
	Violated bool
	Eligible bool
}
