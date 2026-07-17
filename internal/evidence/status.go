package evidence

import "github.com/jyang234/verdi/internal/artifact"

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

// KindResult is one declared evidence kind's folded sub-status within a
// single AC — the fold's OWN per-kind evaluation (foldAC/kindStatus over the
// authoritative candidate set), captured so a disclosure consumer
// (spec/close-preflight) renders exactly the per-kind outcome the verdict
// folded, never a second, differently-filtered per-kind derivation (dc-2;
// ADJ-56). It carries no verdict of its own — Satisfied/Violating are
// projections of the same fold the AC-level Status already reduced.
type KindResult struct {
	// Kind is the declared evidence kind this slot evaluates.
	Kind artifact.EvidenceKind
	// Satisfied reports kindStatus's "satisfied": at least one current
	// passing record of Kind, or — for the attestation kind — an authored
	// attestation. A satisfied kind never blocks the AC.
	Satisfied bool
	// Attestation is the three-way attestation state (absent/unauthored/
	// authored), meaningful only when Kind is EvidenceAttestation — it lets a
	// disclosure tell an absent attestation apart from a scaffolded-but-
	// unauthored one (spec/close-preflight dc-7). Its zero value
	// (AttestationAbsent) is not meaningful for any other kind.
	Attestation AttestationState
	// Violating names a current FAILING record of Kind, when one exists — so a
	// disclosure names a violated kind as a violation (its witness), never as
	// merely-absent evidence (ADJ-56 finding 3). nil when no current record of
	// Kind failed (attestation kinds never populate it — an attestation has no
	// verdict, only presence/authorship).
	Violating *artifact.Evidence
}

// ACResult is one acceptance criterion's folded outcome, enough to render
// one `verdi matrix` row.
type ACResult struct {
	ID      string
	Text    string
	Status  Status
	Summary string // one-line evidence summary, e.g. "static:pass; behavioral:pending(no-record)"
	// Kinds is this AC's per-declared-kind folded evaluation, in the AC's own
	// declared order (empty when the AC declares no evidence kinds) — foldAC's
	// OWN kindStatus results over the authoritative candidate set. A
	// disclosure consumer renders missing/violated/unauthored detail from
	// THIS, never a re-derived per-kind status (dc-2; ADJ-56). It is a
	// projection of the same fold that produced Status: no consumer of Status
	// alone is affected.
	Kinds []KindResult
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
