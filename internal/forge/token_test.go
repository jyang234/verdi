package forge

import "testing"

func TestParseCommentToken(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		wantID string
		wantOK bool
	}{
		{"simple", "[vd:ac-2] outcome AC reads implementation-scoped — reword?", "ac-2", true},
		{"only token", "[vd:oq-1]", "oq-1", true},
		{"em-dash body survives, only prefix parsed", "[vd:dc-3]a note with no space after the token", "dc-3", true},
		{"no token", "nit: this comment has no vd token", "", false},
		{"token not at start fails", "see [vd:ac-2] above", "", false},
		{"empty body", "", "", false},
		{"unterminated bracket", "[vd:ac-2 no closing bracket", "", false},
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
