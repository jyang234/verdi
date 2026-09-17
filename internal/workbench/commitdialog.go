package workbench

import (
	stdhtml "html"
	"regexp"
	"strings"
)

// The board's "Commit & push" dialog (spec/uat-round-1 ac-9, closes
// UAT-019). In the UAT the proposal commit was typed as "accepted F13
// spec": a lifecycle state the commit cannot make true, because acceptance
// is the owner's merge on main and a design-branch commit only PROPOSES.
// Two mechanical aids, neither of which changes what is committed or
// pushed, nor the server's message rules (boardspecapi.go's git-commit
// case is untouched):
//
//   - a proposal-shaped template naming the spec, restored on every open
//     (the client already reset the field to "" per open — there was no
//     draft memory to preserve) and freely replaceable; and
//   - a non-blocking note beside the field while the message contains a
//     lifecycle word the commit cannot make true. The button stays
//     enabled; the note leaves when the words do.
//
// The word test has ONE source: commitLifecyclePattern is proven in Go
// (commitMessageNamesLifecycle's table) and shipped verbatim on the note
// element as data-pattern, which the client compiles with the "i" flag —
// never a second, hand-copied regex that could drift from the proof.

// commitLifecyclePattern matches, as whole words and (once flagged) in any
// case, the four lifecycle states a proposal commit cannot make true —
// accepted, closed, merged, superseded — and their verb variants. The
// alternation is deliberately narrow: "acceptance" ("add acceptance
// criteria") and "closure" are ordinary summary words and stay silent, and
// \b keeps "unacceptable"/"disclosure" silent too. Written in the SYNTAX
// RE2 and ECMAScript share: no inline flags, no lookaround, no named
// groups. Case-folding is NOT shared and is proven for ASCII input only:
// Go's (?i) applies Unicode simple folding while the browser's /i without
// the u flag folds ASCII alone (review-measured: "cloſed", U+017F, matches
// in Go and not in JS). The browser is the only production matcher; the
// Go regexp below exists for the table test and has no production caller.
const commitLifecyclePattern = `\b(?:accept(?:ed|s|ing)?|clos(?:e|ed|es|ing)|merg(?:e|ed|es|ing)|supersed(?:e|ed|es|ing))\b`

var commitLifecycleRE = regexp.MustCompile(`(?i)` + commitLifecyclePattern)

// commitMessageNamesLifecycle reports whether msg contains a lifecycle
// word the commit itself cannot make true. Advisory only: the caller
// shows a note, never blocks.
func commitMessageNamesLifecycle(msg string) bool {
	return commitLifecycleRE.MatchString(msg)
}

// commitMessageTemplate is the proposal-shaped prefill: it names the spec
// and leaves the summary to the author.
func commitMessageTemplate(spec string) string {
	return "Propose spec/" + spec + ": " // vocab:identity — commit-subject grammar naming the spec ref
}

// writeCommitDialog renders the authoring-mode commit dialog: the same
// chrome as before (05 §Workbench: "a commit/push button (message
// prompt, ...)") plus the ac-9 template attribute and the hidden note.
func writeCommitDialog(b *strings.Builder, spec string) {
	esc := stdhtml.EscapeString
	b.WriteString(`
<div role="dialog" aria-label="Commit &amp; push" class="board-dialog" id="commit-dialog" hidden>
<h2>Commit &amp; push</h2>
<p class="ritual-note">Commits the working tree on this design branch and pushes it.</p>
<div class="field"><label for="commit-message">Commit message</label><input id="commit-message" autocomplete="off" data-template="` + esc(commitMessageTemplate(spec)) + `">`)
	// The note is a live status region toggled by the client (hidden until
	// the message contains a lifecycle word), NOT an aria-describedby
	// target: a described-by reference reads a hidden element's text too,
	// which would attach the warning to every message.
	b.WriteString(`
<p class="ritual-note" id="commit-lifecycle-note" data-testid="commit-lifecycle-note" role="status" data-pattern="` + esc(commitLifecyclePattern) + `" hidden>`)
	// vocab:identity — the correction itself: names the owner's act (merge) and this commit's (propose); no state id is spoken
	b.WriteString(`This commit only proposes the spec. Acceptance is the owner&#8217;s merge on main, so a message announcing a lifecycle state names something this commit cannot make true.</p></div>`)
	b.WriteString(`
<div class="dialog-actions"><button type="button" id="commit-dialog-ok">Commit</button>
<button type="button" id="commit-dialog-cancel">Cancel</button></div>
</div>`)
}
