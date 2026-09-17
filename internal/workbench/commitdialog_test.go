package workbench

import (
	stdhtml "html"
	"regexp"
	"strings"
	"testing"
)

// TestCommitDialog_PrefillNamesSpec (spec/uat-round-1 ac-9, closes
// UAT-019: the proposal commit was typed as "accepted F13 spec", a
// lifecycle state no commit can make true): the authoring dialog carries
// a proposal-shaped template naming the spec, which the client restores
// on EVERY open (the client already reset the field to "" per open, so
// no draft memory existed to preserve), and the lifecycle note element,
// hidden until the message earns it, carrying the SAME pattern the Go
// matcher below is proven against — the browser compiles that attribute,
// never a second hand-copied regex. The Commit button stays enabled: the
// note is advisory (contract point 2), the server's empty-message rule is
// untouched (point 3), and nothing about the commit payload changes
// (point 4).
func TestCommitDialog_PrefillNamesSpec(t *testing.T) {
	cases := []struct {
		name string
		spec string
	}{
		{name: "kebab-case spec name", spec: "decline-ledger"},
		{name: "name needing HTML escaping", spec: "a&b<c>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			html := renderBoardDialogs(&BoardProjection{Spec: tc.spec, Mode: modeAuthoring})
			wantTemplate := "Propose spec/" + tc.spec + ": "
			if got := commitMessageTemplate(tc.spec); got != wantTemplate {
				t.Fatalf("commitMessageTemplate(%q) = %q, want %q", tc.spec, got, wantTemplate)
			}
			wantInput := `<input id="commit-message" autocomplete="off" data-template="` + stdhtml.EscapeString(wantTemplate) + `">`
			if !strings.Contains(html, wantInput) {
				t.Fatalf("commit dialog missing the prefill template input %q\n%s", wantInput, html)
			}
			wantNote := `<p class="ritual-note" id="commit-lifecycle-note" data-testid="commit-lifecycle-note" role="status" data-pattern="` + stdhtml.EscapeString(commitLifecyclePattern) + `" hidden>`
			if !strings.Contains(html, wantNote) {
				t.Fatalf("commit dialog missing the hidden lifecycle note %q\n%s", wantNote, html)
			}
			// The note sits beside the field, inside the dialog, and speaks
			// the correction: acceptance is the owner's merge; a commit
			// only proposes.
			dialogAt := strings.Index(html, `id="commit-dialog"`)
			inputAt := strings.Index(html, wantInput)
			noteAt := strings.Index(html, wantNote)
			okAt := strings.Index(html, `id="commit-dialog-ok"`)
			if dialogAt >= inputAt || inputAt >= noteAt || noteAt >= okAt {
				t.Fatalf("order dialog=%d input=%d note=%d ok=%d; want dialog < input < note < ok", dialogAt, inputAt, noteAt, okAt)
			}
			noteText := html[noteAt : strings.Index(html[noteAt:], "</p>")+noteAt]
			for _, want := range []string{"owner", "merge", "propose"} {
				if !strings.Contains(strings.ToLower(noteText), want) {
					t.Errorf("note text %q must mention %q", noteText, want)
				}
			}
			if strings.Contains(html, `id="commit-dialog-ok" disabled`) || strings.Contains(html, `id="commit-dialog-ok" aria-disabled`) {
				t.Fatal("the Commit button must stay enabled: the note is non-blocking")
			}
			if n := strings.Count(html, ` id="commit-lifecycle-note"`); n != 1 {
				t.Fatalf("commit-lifecycle-note elements = %d, want exactly 1", n)
			}
		})
	}
}

// TestCommitDialog_AbsentOutsideAuthoring: the mirror (review) and the
// document (read-only) never carried the commit dialog; ac-9 adds nothing
// there.
func TestCommitDialog_AbsentOutsideAuthoring(t *testing.T) {
	for _, mode := range []boardModeKind{modeReview, modeReadOnly} {
		t.Run(string(mode), func(t *testing.T) {
			html := renderBoardDialogs(&BoardProjection{Spec: "s", Mode: mode})
			for _, absent := range []string{`id="commit-dialog"`, `id="commit-lifecycle-note"`, `data-template=`} {
				if strings.Contains(html, absent) {
					t.Errorf("%s: %q must not render outside authoring", mode, absent)
				}
			}
		})
	}
}

// TestCommitMessageNamesLifecycle: the matcher behind the note. Whole
// words only (contract: "unacceptable"/"disclosure" must NOT trigger),
// case-insensitive, the four states and their verb variants. It is also
// the proof that the pattern is a valid RE2 expression; the browser-side
// half — that the identical source text is a valid ECMAScript RegExp and
// toggles the note — is spec 77's Playwright path.
func TestCommitMessageNamesLifecycle(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		// The UAT-019 message itself, and the four states.
		{"accepted F13 spec", true},
		{"ACCEPTED", true},
		{"spec is closed", true},
		{"merged to main", true},
		{"superseded by v2", true},
		// Verb variants, as whole words, any case.
		{"Accept the spec", true},
		{"accepts F13", true},
		{"accepting F13", true},
		{"Close the story", true},
		{"Closes UAT-019", true},
		{"closing out", true},
		{"Merge branch design/x", true},
		{"merges cleanly", true},
		{"merging now", true},
		{"supersede v1", true},
		{"supersedes v1", true},
		{"superseding v1", true},
		// Boundaries: punctuation and hyphens are boundaries; letters are not.
		{"spec: accepted.", true},
		{"move to accepted-pending-build", true},
		{"(merged)", true},
		// Negatives: the prefill, ordinary summaries, and the contract's
		// two named non-triggers.
		{"", false},
		{"Propose spec/decline-ledger: ", false},
		{"Propose spec/decline-ledger: add F13 summary", false},
		{"unacceptable", false},
		{"disclosure", false},
		{"acceptance criteria added", false},
		{"closure notes", false},
		{"the merger", false},
		{"emerged from review", false},
		{"submerged", false},
		{"enclosed", false},
		{"acceptor", false},
	}
	for _, tc := range cases {
		t.Run(tc.msg, func(t *testing.T) {
			if got := commitMessageNamesLifecycle(tc.msg); got != tc.want {
				t.Fatalf("commitMessageNamesLifecycle(%q) = %v, want %v", tc.msg, got, tc.want)
			}
		})
	}
	// The pattern is shipped WITHOUT flags: Go applies (?i), the browser
	// the "i" flag. It must compile bare too (a leading flag group would
	// be an ECMAScript syntax error).
	if _, err := regexp.Compile(commitLifecyclePattern); err != nil {
		t.Fatalf("commitLifecyclePattern must compile bare: %v", err)
	}
	if strings.HasPrefix(commitLifecyclePattern, "(?") {
		t.Fatalf("commitLifecyclePattern %q must not carry an inline flag group: the browser's RegExp rejects it", commitLifecyclePattern)
	}
}
