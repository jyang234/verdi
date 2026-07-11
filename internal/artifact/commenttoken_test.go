package artifact

import "testing"

// TestParseCommentToken pins the exact comment-token grammar 02 §Record
// schemas defines — the single grammar both internal/forge (gate + mcp)
// and internal/workbench (board projection) now share. The cases below
// cover every input the two previously-divergent regexes each accepted, so
// this test is the regression fence against the drift M-3 flagged.
func TestParseCommentToken(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		wantID string
		wantOK bool
	}{
		// Well-formed typed-prefix ids: accepted by both old regexes.
		{"typed prefix ac", "[vd:ac-2] reword this", "ac-2", true},
		{"typed prefix oq only token", "[vd:oq-1]", "oq-1", true},
		{"no space after token", "[vd:dc-3]a note with no space", "dc-3", true},
		{"unresolvable but well-formed token", "[vd:zz-99] still a token", "zz-99", true},

		// Non-tokens: rejected by both old regexes.
		{"no token", "nit: no vd token here", "", false},
		{"token not at start", "see [vd:ac-2] above", "", false},
		{"empty id", "[vd:] empty", "", false},
		{"empty body", "", "", false},
		{"unterminated bracket", "[vd:ac-2 no closing bracket", "", false},

		// The grammar is permissive on id SHAPE (02 puts id well-formedness
		// on the object-id definition + declared-object resolution, not the
		// token parser): these parse as tokens and route to the tray only
		// because they resolve against no declared object — they are NOT
		// non-tokens. This is the behaviour the strict workbench copy got
		// wrong; the permissive forge grammar is authoritative.
		{"no hyphen still a token", "[vd:ac2] parses", "ac2", true},
		{"uppercase still a token", "[vd:AC-2] parses", "AC-2", true},
		{"numeric still a token", "[vd:123] parses", "123", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := ParseCommentToken(tc.body)
			if ok != tc.wantOK || id != tc.wantID {
				t.Errorf("ParseCommentToken(%q) = (%q, %v), want (%q, %v)", tc.body, id, ok, tc.wantID, tc.wantOK)
			}
		})
	}
}
