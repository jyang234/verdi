// Package readinesstest is a small, shared readinesspilot.Snapshot
// fixture builder for tests across packages that need one — internal/
// mcpserve and cmd/verdi both render a document over a real, valid
// snapshot (task-4-review.md fix round 1, F5: "the repo already has the
// precedent (fixturegit, jiratest)"; CLAUDE.md: "Anything used by two or
// more packages lives in a shared internal/ package ... never copy-paste
// across packages"). A plain .go file, not a _test.go one, so it can be
// imported from another package's own tests — mirroring internal/
// fixturegit and internal/forge/forgetest's own precedent for shared test
// scaffolding.
//
// internal/readinesspilot's OWN schema_test.go deliberately does not use
// this package: it is package-internal (`package readinesspilot`, not
// `readinesspilot_test`), and this package imports readinesspilot, so
// schema_test.go importing readinesstest back would be an import cycle.
// It keeps its own unexported validSnapshot()/validConcern() instead.
package readinesstest

import "github.com/jyang234/verdi/internal/readinesspilot"

// ValidSnapshot returns a Snapshot targeting targetRef at head that
// passes Snapshot.Validate(): every one of the four fixed areas proven,
// no attention items. Mirrors internal/readinesspilot/schema_test.go's
// own (unexported) validSnapshot()/validConcern(). The four AllConcerns
// entries use the closed concern-identity vocabulary
// (readinesspilot/schema.go's unexported concernIdentity): shape/problem,
// success/contributor/static, context/verdict, and review/action — their
// areas and blocking flags are fixed by that vocabulary, not arbitrary,
// so callers must not invent other concern ids without checking it.
//
// TargetTitle/TargetClass/Branch/RequestDigest only need to satisfy
// Validate(); internal/specdoc/readiness.go's WithReadiness — the only
// consumer a rendered document has — never reads them, only TargetRef,
// Head, CurrentFocus, StaleNotice, Areas, and Attention.
func ValidSnapshot(targetRef, head string) readinesspilot.Snapshot {
	concern := func(id string, area readinesspilot.AreaID, blocking bool) readinesspilot.Concern {
		return readinesspilot.Concern{
			ID: id, Area: area, State: readinesspilot.StateProven, Blocking: blocking,
			Timing: readinesspilot.TimingCurrent, Summary: "source-derived readiness fact",
			Witnesses: []string{}, Destination: readinesspilot.Destination{CLI: []string{}},
		}
	}
	return readinesspilot.Snapshot{
		TargetRef:     targetRef,
		TargetTitle:   "Readiness test target",
		TargetClass:   "feature",
		Branch:        "main",
		Head:          head,
		RequestDigest: "sha256:" + digestFiller,
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaSuccess, Label: "Define success", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaContext, Label: "Check constraints", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaReview, Label: "Get approval", State: readinesspilot.StateProven},
		},
		CurrentFocus: "",
		Attention:    []readinesspilot.Concern{},
		AllConcerns: []readinesspilot.Concern{
			concern("shape/problem", readinesspilot.AreaShape, true),
			concern("success/contributor/static", readinesspilot.AreaSuccess, false),
			concern("context/verdict", readinesspilot.AreaContext, true),
			concern("review/action", readinesspilot.AreaReview, true),
		},
		StaleNotice: "Derived at HEAD " + head + " for this request.",
	}
}

// digestFiller is 64 lowercase hex characters — RequestDigest's
// Validate()-required shape (sha256:<64 hex>) is the only constraint on
// its value; no consumer reads it beyond that check (see the doc comment
// above).
const digestFiller = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
