package workbench

// The record drawer Review tab's acceptance facts (spec/wall-strip-and-
// drawer-v2 ac-5, ac-7, dc-4; ledger SI-368 (3), (32) B3): the retired
// wall shell's review/acceptance row, moved claim for claim to its home.
// Acceptance is human review: the owner's merge of the proposal is the
// single acceptance decision, so the tab labels the row human review in
// plain words, keeps the formal obligation (AC-6/DC-15's profile-required
// review) secondary with its witness, and says how acceptance is reached
// from where the wall stands. It is derived from the wall's own state,
// mode and branch, which the drawer is rendered with; the readiness
// loader has no review/acceptance family (SI-338), and nothing here is a
// readiness verdict.

import (
	stdhtml "html"
	"strings"

	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/specstate"
)

// reviewAcceptanceID is the acceptance row's formal concern id, the one
// the retired wall shell gave it.
const reviewAcceptanceID = "review/acceptance"

// reviewAcceptance is the Review tab's acceptance facts: its formal state
// (proven or unproven), the fact, the guidance an unproven row carries,
// and the witnesses — the formal obligation among them.
type reviewAcceptance struct {
	State     readinesspilot.State
	Summary   string
	Guidance  string
	Witnesses []string
}

// reviewAcceptanceFor derives the acceptance facts of a wall whose spec's
// Git-derived state is stateFormal, in mode, on branch, as the retired
// wall shell did: an accepted revision is proven; an open review mirrors
// the proposal's merge request; any other wall is not yet accepted. A
// wall that takes no edit proposes nothing from itself (SI-368 (28)(b)),
// so its guidance names the proposal's own design branch rather than a
// pull request from this one.
func reviewAcceptanceFor(stateFormal string, mode boardModeKind, branch string) reviewAcceptance {
	// vocab:identity — non-vocabulary homograph: the forge's merge request/authorizes-merge, never the `merge` lifecycle transition word
	const obligation = "AC-6/DC-15: the profile-required review of the exact proposed head authorizes merge"
	switch {
	case stateFormal == string(specstate.AcceptedPendingBuild):
		return reviewAcceptance{
			State: readinesspilot.StateProven,
			// vocab:identity — non-vocabulary homograph: the forge's merge (owner's merge of the PR), never the `merge` lifecycle transition word
			Summary:   "This revision is accepted: the owner's merge made it reachable from the default branch.",
			Witnesses: []string{"Git-derived state " + stateFormal},
		}
	case mode == modeReview:
		return reviewAcceptance{
			State: readinesspilot.StateUnproven,
			// vocab:identity — non-vocabulary homograph: the forge's merge request/owner's merge, never the `merge` lifecycle transition word
			Summary: "Human review is open: this wall mirrors the proposal's merge request.",
			// vocab:identity — non-vocabulary homograph: the forge's merge request/owner's merge, never the `merge` lifecycle transition word
			Guidance: "The owner's merge of the open merge request is the single acceptance decision — no second ceremony.",
			// vocab:identity — non-vocabulary homograph: the forge's merge request, never the `merge` lifecycle transition word
			Witnesses: []string{"an open merge request mirrors this spec", obligation},
		}
	}
	// vocab:identity — non-vocabulary homograph: the forge's owner's-merge of the pull request, never the `merge` lifecycle transition word
	guidance := "Request the owner's review of the proposal from its own design branch — the owner's merge is the single acceptance decision."
	if mode == modeAuthoring {
		// vocab:identity — non-vocabulary homograph: the forge's owner's-merge of the pull request, never the `merge` lifecycle transition word
		guidance = "Derive the semantic review packet (below), open a pull request from " + branch + ", and request the owner's review — the owner's merge is the single acceptance decision."
	}
	return reviewAcceptance{
		State:     readinesspilot.StateUnproven,
		Summary:   "Human review has not accepted this proposal yet.",
		Guidance:  guidance,
		Witnesses: []string{"Git-derived state " + stateFormal, obligation + "; no separate acceptance command exists"},
	}
}

// reviewAcceptanceOf is the acceptance facts of the wall p rendered with
// asd: the spec's Git-derived state and the branch from the wall's own
// view, its mode from the projection. A render without a view reads as
// an unresolved state on no branch, never as accepted.
func reviewAcceptanceOf(p *BoardProjection, asd *asdView) reviewAcceptance {
	state, branch := "", ""
	if asd != nil {
		state, branch = asd.StateFormal, asd.Branch
	}
	return reviewAcceptanceFor(state, p.Mode, branch)
}

// writeReviewAcceptance writes the Review tab's acceptance section: the
// plain human-review label with the formal concern secondary, the
// primary line — the guidance, or the fact of a proven row — then the
// fact, and the formal state and each witness as rows. It is markup only:
// no control opens, approves or merges anything (co-2).
func writeReviewAcceptance(b *strings.Builder, a reviewAcceptance) {
	esc := stdhtml.EscapeString
	b.WriteString(`<section class="record-section" data-testid="record-review-acceptance" data-concern-id="` + reviewAcceptanceID + `" data-state="` + string(a.State) + `"`)
	if a.State == readinesspilot.StateProven {
		b.WriteString(` data-tone="evidenced"`)
	}
	b.WriteString(`>`)
	b.WriteString(`<div class="record-section-head"><h3 class="record-section-title">Acceptance</h3>`)
	b.WriteString(`<span class="record-section-aside" data-testid="record-human-review">Human review<span class="record-note"> · <code>` + reviewAcceptanceID + `</code></span></span></div>`)
	primary, fact := a.Guidance, a.Summary
	if primary == "" {
		primary, fact = a.Summary, ""
	}
	b.WriteString(`<p class="record-prose" data-testid="record-acceptance-primary">` + esc(primary) + `</p>`)
	if fact != "" {
		b.WriteString(`<p class="record-prose" data-testid="record-acceptance-fact">` + esc(fact) + `</p>`)
	}
	b.WriteString(`<dl class="record-rows" data-testid="record-acceptance-formal">`)
	writeAcceptanceRow(b, "state", string(a.State))
	for _, w := range a.Witnesses {
		writeAcceptanceRow(b, "witness", w)
	}
	b.WriteString(`</dl></section>`)
}

// writeAcceptanceRow writes one key and value row in the drawer's row
// markup (walldrawer.js's rows), the value monospaced and escaped.
func writeAcceptanceRow(b *strings.Builder, key, value string) {
	b.WriteString(`<div class="record-row"><dt class="record-key">` + key + `</dt><dd class="record-value record-mono"><span class="record-text">` + stdhtml.EscapeString(value) + `</span></dd></div>`)
}
