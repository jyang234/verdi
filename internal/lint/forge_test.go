package lint

import "testing"

func TestGeneratedAttrToken(t *testing.T) {
	cases := []struct {
		forge, want string
	}{
		{"gitlab", "gitlab-generated"},
		{"github", "linguist-generated"},
		{"", "gitlab-generated"}, // omitted forge: defaults to gitlab (02's own literal example)
	}
	for _, tc := range cases {
		t.Run(tc.forge, func(t *testing.T) {
			if got := generatedAttrToken(tc.forge); got != tc.want {
				t.Fatalf("generatedAttrToken(%q) = %q, want %q", tc.forge, got, tc.want)
			}
		})
	}
}

func TestParseGitAttributes(t *testing.T) {
	data := []byte("# comment\n\n.verdi/specs/*/*/board.json          gitlab-generated\n.verdi/specs/*/*/rollup.json gitlab-generated\nmalformed-line-no-token\n")
	got := parseGitAttributes(data)

	if got[".verdi/specs/*/*/board.json"] != "gitlab-generated" {
		t.Fatalf("board.json token = %q, want gitlab-generated", got[".verdi/specs/*/*/board.json"])
	}
	if got[".verdi/specs/*/*/rollup.json"] != "gitlab-generated" {
		t.Fatalf("rollup.json token = %q, want gitlab-generated", got[".verdi/specs/*/*/rollup.json"])
	}
	if _, ok := got["malformed-line-no-token"]; ok {
		t.Fatal("malformed single-field line should not produce an entry")
	}
}

func TestParseGitAttributes_Empty(t *testing.T) {
	got := parseGitAttributes(nil)
	if len(got) != 0 {
		t.Fatalf("parseGitAttributes(nil) = %v, want empty", got)
	}
}
