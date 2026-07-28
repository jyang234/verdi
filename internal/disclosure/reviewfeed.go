package disclosure

import "fmt"

// The review-feed disclosure family lives here, in the seam package
// itself, because it has THREE producers in two different trees —
// cmd/verdi's startup wiring (serve.go/mcp.go), internal/workbench's board
// render, and internal/mcpserve's list_annotations — and CLAUDE.md's rule
// is that anything used by two or more packages lives in one shared
// internal/ package. cmd/verdi cannot be that home (nothing under
// internal/ may import a main package), and neither surface package may
// own it without the other importing it for the text alone. Authoring the
// text is a disclosure-vocabulary concern, which is precisely this
// package's single concern, so the constructors sit next to New/Render:
// one home, no copy to drift.
//
// The migration this file completes: the startup state already rendered
// through the seam, but each surface hand-authored its own render-time
// transport-failure string ("review feed unavailable: %v" on the board,
// "review population unavailable: %v" in list_annotations), so ONE
// underlying state — a configured forge that cannot be consulted — read
// in two vocabularies (spec/disclosure-seam-v2 ac-1/ac-2,
// spec/disclosure-legibility#ac-1).

// SourceReviewFeed is the source id every review-feed disclosure carries,
// whichever surface renders it. Both states below are the same producing
// condition — a configured forge that could not be consulted — so they
// share one source rather than splitting into a per-cause taxonomy;
// their difference lives in the text, which names the cause.
const SourceReviewFeed = "mcp:review-feed"

// reviewUnavailableConsequence is the clause every review-feed disclosure
// closes with. It is shared rather than repeated so the family stays
// recognizable by its ending as well as its source (ac-2's "a reader who
// has learned to recognize one disclosure recognizes all of them"), and
// so a reword can never land on one state and miss the other.
const reviewUnavailableConsequence = "; review state cannot be shown"

// ReviewUnavailableNoCredentials is the startup-time state: verdi.yaml
// names a forge of the given kind, but no credentials were found to build
// a live adapter for it, so review state is disclosed-unproven rather
// than silently reported as "not under review" (I-1(b)).
func ReviewUnavailableNoCredentials(kind string) Disclosure {
	text := fmt.Sprintf("forge %q is configured (verdi.yaml) but no credentials are available to reach it", kind) + reviewUnavailableConsequence
	return New(SourceReviewFeed, "", text)
}

// ReviewUnavailableTransport is the render-time state: a live adapter
// existed and the call was attempted, but the forge could not be
// consulted. Callers reach this only on a failed consult, so a nil err is
// still disclosed — naming the missing detail rather than emitting a
// half-formed line or, worse, falling silent (fail closed).
func ReviewUnavailableTransport(err error) Disclosure {
	cause := "unknown error"
	if err != nil {
		cause = err.Error()
	}
	return New(SourceReviewFeed, "", "the configured forge could not be consulted ("+cause+")"+reviewUnavailableConsequence)
}
