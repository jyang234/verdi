package gitforbid

import (
	"reflect"
	"testing"
)

// TestTokens_HoldsTheSevenTokens pins the list itself: ritual-write-scope-v3
// ac-3's six tokens and recovery's -f (ledger SI-224). The list is a floor,
// so a drop fails here, and an addition must be a deliberate edit of this
// pin.
func TestTokens_HoldsTheSevenTokens(t *testing.T) {
	want := []string{"reset", "restore", "clean", "stash", "--force", "-f", "update-ref"}
	if got := Tokens(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Tokens() = %q, want %q", got, want)
	}
}

// TestTokens_ReturnsAFreshSlice proves the list is not shared state: a
// caller that changes the slice it got changes neither the next call's
// list nor what Forbids matches.
func TestTokens_ReturnsAFreshSlice(t *testing.T) {
	got := Tokens()
	for i := range got {
		got[i] = "mutated"
	}
	if again := Tokens(); again[0] != "reset" {
		t.Fatalf("Tokens() leaked a caller's change: %q", again)
	}
	if !Forbids([]string{"reset"}) {
		t.Fatal("Forbids lost reset after a caller changed its copy of Tokens()")
	}
}

// TestForbids_EveryTokenAsAWholeElement proves each token is forbidden as
// a whole argv element, in any position, and that an element merely
// starting or ending with a token that does not begin with "--" is not.
func TestForbids_EveryTokenAsAWholeElement(t *testing.T) {
	for _, tok := range Tokens() {
		t.Run(tok, func(t *testing.T) {
			cases := []struct {
				name string
				argv []string
				want bool
			}{
				{"alone", []string{tok}, true},
				{"first", []string{tok, "x"}, true},
				{"last", []string{"x", tok}, true},
				{"middle", []string{"x", tok, "y"}, true},
				{"suffixed", []string{tok + "x"}, tok[:2] == "--"},
				{"prefixed", []string{"x" + tok}, false},
				{"inside a longer element", []string{"a " + tok + " b"}, false},
			}
			for _, tc := range cases {
				if got := Forbids(tc.argv); got != tc.want {
					t.Errorf("%s: Forbids(%q) = %v, want %v", tc.name, tc.argv, got, tc.want)
				}
			}
		})
	}
}

// TestForbids_Table covers the matching rule's edges: the "--" prefix rule
// (only for a token that begins with "--"), case, message text, git's
// global options, and empty input.
func TestForbids_Table(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want bool
	}{
		{"reset --hard", []string{"reset", "--hard"}, true},
		{"force-with-lease matches --force as a prefix", []string{"push", "--force-with-lease"}, true},
		{"force-if-includes matches --force as a prefix", []string{"push", "--force-if-includes"}, true},
		{"--force=value matches --force as a prefix", []string{"push", "--force=yes"}, true},
		{"short force flag", []string{"checkout", "-f"}, true},
		{"update-ref with a full refname", []string{"update-ref", "refs/heads/design/x", "abc"}, true},
		{"clean -fd", []string{"clean", "-fd"}, true},
		{"stash push", []string{"stash", "push"}, true},
		{"restore --staged", []string{"restore", "--staged", "a"}, true},
		{"a combined short flag is not -f", []string{"-fd"}, false},
		{"-ff is not -f", []string{"-ff"}, false},
		{"-force is not --force", []string{"-force"}, false},
		{"a single-dash token takes no prefix rule", []string{"-fx"}, false},
		{"resets is not reset", []string{"resets"}, false},
		{"update-refs is not update-ref", []string{"update-refs"}, false},
		{"stash@{0} is not stash", []string{"stash@{0}"}, false},
		{"matching is case-sensitive", []string{"Reset", "--FORCE", "-F", "UPDATE-REF"}, false},
		{"message text is one element, never scanned", []string{"commit", "-m", "reset the counter"}, false},
		{"a config override is not a force flag", []string{"-c", "status.showUntrackedFiles=all", "worktree", "remove", "/tmp/wt"}, false},
		{"read-only", []string{"rev-parse", "--verify", "HEAD"}, false},
		{"branch at a commit", []string{"branch", "design/x", "abc"}, false},
		{"branch -d", []string{"branch", "-d", "close/x"}, false},
		{"empty element", []string{""}, false},
		{"empty argv", []string{}, false},
		{"nil argv", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Forbids(tc.argv); got != tc.want {
				t.Fatalf("Forbids(%q) = %v, want %v", tc.argv, got, tc.want)
			}
		})
	}
}
