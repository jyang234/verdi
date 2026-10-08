package workbench

// The Commit and push fragment (spec/wall-strip-and-drawer-v2 ac-3; ledger
// SI-368 (8)): the uncommitted indicator, the "n changes" count and the
// three-state changes body — typed changes with their badges, unclassified
// changes with their reasons, and an unreadable comparison, disclosed —
// rendered once on the server from the wall-changes summary the snapshot
// already carries (dc-3: no git read of its own), and swapped from the
// snapshot's own field. The markup is structural only; its placement and
// styling are the strip lane's (F3a).

import (
	stdhtml "html"
	"strings"

	"github.com/jyang234/verdi/internal/designprovenance"
)

// The fragment's data-changes states: which of ac-3's three states the
// summary is in (mixed is typed and unclassified side by side, wall-changes
// dc-2), or none when nothing is listed.
const (
	uncommittedNone         = "none"
	uncommittedTyped        = "typed"
	uncommittedUnclassified = "unclassified"
	uncommittedMixed        = "mixed"
	uncommittedUnreadable   = "unreadable"
)

// uncommittedNotComputed is the unreadable reason of a render that carries
// no changes summary at all: every authoring wall's page, snapshot and
// mutation response compute one (loadASD), so this fails closed rather
// than reading as a clean tree.
const uncommittedNotComputed = "no comparison with HEAD was computed for this render"

// wallUncommitted is the fragment's facts, read from one boardGitState.
type wallUncommitted struct {
	// Set is the uncommitted indicator: git status reports the tree dirty,
	// or the summary lists any change. A comparison with zero recognized
	// operations therefore never clears it while another change remains
	// (ac-3; wall-changes ac-2), whatever git status's own bit says.
	Set bool
	// Unreadable is the comparison's unreadable reason; "" when readable.
	Unreadable string
	// Typed is the recognized operations; always empty when Unreadable is
	// set (wall-changes dc-2: its reason instead of a typed list).
	Typed []designprovenance.Change
	// Unclassified is every other change git reports, listed in both the
	// readable and the unreadable state (wall-changes dc-2).
	Unclassified []wallUnclassifiedChange
}

// deriveWallUncommitted reads the fragment's facts from git: its changes
// summary and its dirty bit. No summary at all is an unreadable
// comparison, never a clean one.
func deriveWallUncommitted(git *boardGitState) wallUncommitted {
	changes := git.Changes
	if changes == nil {
		return wallUncommitted{Set: git.Dirty, Unreadable: uncommittedNotComputed}
	}
	u := wallUncommitted{Unreadable: changes.UnreadableReason, Unclassified: changes.Unclassified}
	if u.Unreadable == "" {
		u.Typed = changes.Typed
	}
	u.Set = git.Dirty || len(u.Typed) > 0 || len(u.Unclassified) > 0
	return u
}

// state is the summary's data-changes state.
func (u wallUncommitted) state() string {
	switch {
	case u.Unreadable != "":
		return uncommittedUnreadable
	case len(u.Typed) > 0 && len(u.Unclassified) > 0:
		return uncommittedMixed
	case len(u.Typed) > 0:
		return uncommittedTyped
	case len(u.Unclassified) > 0:
		return uncommittedUnclassified
	}
	return uncommittedNone
}

// countLabel is the Commit and push count (SI-368 (8)): typed plus
// unclassified changes, or "unreadable" for an unreadable comparison —
// never a number there, since a count of what could not be compared
// would claim more than is known.
func (u wallUncommitted) countLabel() string {
	if u.Unreadable != "" {
		return uncommittedUnreadable
	}
	return asdCountLabel(len(u.Typed)+len(u.Unclassified), "change", "changes")
}

// plainWire writes a hyphenated wire value as words (prose-or-body-text →
// "prose or body text"); the value itself stays in the data attribute.
func plainWire(value string) string {
	return strings.ReplaceAll(value, "-", " ")
}

// wallUncommittedFragment is the snapshot's Commit and push fragment for
// one loaded wall: present where Commit and push is (the authoring wall),
// "" on every other wall.
func wallUncommittedFragment(p *BoardProjection, git *boardGitState) string {
	if p.Mode != modeAuthoring || git == nil {
		return ""
	}
	return renderWallUncommitted(deriveWallUncommitted(git))
}

// renderWallUncommitted renders the fragment: its root carries the state;
// inside are the indicator (the uncommitted-indicator test id, hidden only
// when clear), the count, and the changes body. A typed change shows its
// target and its operation's own kind as the badge; an unclassified change
// its path and reason; an unreadable comparison its reason. A clean tree
// says so, and a change git reports that the summary does not list keeps
// the indicator set and says that instead.
func renderWallUncommitted(u wallUncommitted) string {
	esc := stdhtml.EscapeString
	var b strings.Builder
	b.WriteString(`<div class="wall-commit" data-testid="wall-commit" data-changes="` + u.state() + `">`)
	b.WriteString(`<span class="uncommitted" data-testid="uncommitted-indicator"`)
	if !u.Set {
		b.WriteString(` hidden`)
	}
	b.WriteString(`>uncommitted changes</span>`)
	b.WriteString(`<span class="wall-commit-count" data-testid="wall-commit-count">` + esc(u.countLabel()) + `</span>`)
	b.WriteString(`<div class="wall-commit-changes" data-testid="wall-commit-changes">`)
	if u.Unreadable != "" {
		b.WriteString(`<p class="wall-commit-unreadable" data-testid="wall-commit-unreadable" role="status">The comparison with HEAD is unreadable: ` + esc(strings.TrimSuffix(u.Unreadable, ".")) + `.</p>`)
	}
	if len(u.Typed) > 0 {
		b.WriteString(`<section class="wall-commit-typed" data-testid="wall-commit-typed"><h3>Typed changes</h3><ul>`)
		for _, c := range u.Typed {
			b.WriteString(`<li data-target="` + esc(c.Target) + `" data-change="` + esc(string(c.Change)) + `">` +
				`<span class="wall-commit-target">` + esc(c.Target) + `</span> ` +
				`<span class="wall-commit-badge">` + esc(plainWire(string(c.Change))) + `</span></li>`)
		}
		b.WriteString(`</ul></section>`)
	}
	if len(u.Unclassified) > 0 {
		b.WriteString(`<section class="wall-commit-unclassified" data-testid="wall-commit-unclassified"><h3>Unclassified changes</h3><ul>`)
		for _, c := range u.Unclassified {
			b.WriteString(`<li data-path="` + esc(c.Path) + `" data-reason="` + esc(string(c.Reason)) + `">` +
				`<span class="wall-commit-path">` + esc(c.Path) + `</span> ` +
				`<span class="wall-commit-reason">` + esc(plainWire(string(c.Reason))) + `</span></li>`)
		}
		b.WriteString(`</ul></section>`)
	}
	if u.Unreadable == "" && len(u.Typed)+len(u.Unclassified) == 0 {
		if u.Set {
			b.WriteString(`<p class="wall-commit-unlisted" data-testid="wall-commit-unlisted" role="status">Git reports uncommitted changes that this list does not name.</p>`)
		} else {
			b.WriteString(`<p class="wall-commit-none" data-testid="wall-commit-none">No uncommitted changes.</p>`)
		}
	}
	b.WriteString(`</div></div>`)
	return b.String()
}
