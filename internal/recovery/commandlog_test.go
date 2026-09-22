package recovery

import (
	"reflect"
	"testing"
)

func TestCommandLog_ForbiddenTokens(t *testing.T) {
	var l CommandLog
	l.Observe("/r", []string{"rev-parse", "HEAD"})
	l.Observe("/r", []string{"branch", "-d", "close/x"})
	l.Observe("/r", []string{"push", "--force-with-lease"})
	l.Observe("/r", []string{"restore", "--staged", "a"})
	got := l.Forbidden()
	want := [][]string{{"push", "--force-with-lease"}, {"restore", "--staged", "a"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if len(l.Entries()) != 4 {
		t.Fatal("entries lost")
	}
}

// TestCommandLog_ForbiddenTokens_ShortForceFlag is R-RR3-17: the short
// "-f" force flag (git push -f / git branch -f / git checkout -f) is
// forbidden too, not just "--force" and its long-form derivatives.
func TestCommandLog_ForbiddenTokens_ShortForceFlag(t *testing.T) {
	var l CommandLog
	l.Observe("/r", []string{"push", "-f"})
	l.Observe("/r", []string{"rev-parse", "HEAD"})
	got := l.Forbidden()
	want := [][]string{{"push", "-f"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestCommandLog_MessageTextNeverScanned proves the token check is over
// argv elements, never message text: a commit message that happens to
// start with a forbidden word is not itself forbidden.
func TestCommandLog_MessageTextNeverScanned(t *testing.T) {
	var l CommandLog
	l.Observe("/r", []string{"commit", "-m", "reset the counter"})
	if got := l.Forbidden(); len(got) != 0 {
		t.Fatalf("Forbidden() = %v, want none (message text is not an argv token)", got)
	}
}

// TestIsForbiddenArgv table-tests the exported seam fix round 1 (M4)
// added, over each rule ForbiddenTokens names: exact match, the
// "--"-prefix rule, R-RR3-17's short "-f" flag, and a clean argv.
func TestIsForbiddenArgv(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want bool
	}{
		{"exact_match", []string{"reset", "--hard"}, true},
		{"force_with_lease_prefix", []string{"push", "--force-with-lease"}, true},
		{"short_force_flag", []string{"checkout", "-f"}, true},
		{"clean", []string{"rev-parse", "HEAD"}, false},
		{"empty_argv", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsForbiddenArgv(tc.argv); got != tc.want {
				t.Fatalf("IsForbiddenArgv(%v) = %v, want %v", tc.argv, got, tc.want)
			}
		})
	}
}

func TestCommandLog_EntriesAreCopies(t *testing.T) {
	var l CommandLog
	l.Observe("/r", []string{"rev-parse", "HEAD"})
	got := l.Entries()
	got[0][0] = "mutated"
	got2 := l.Entries()
	if got2[0][0] != "rev-parse" {
		t.Fatalf("Entries() leaked internal state: got %v", got2)
	}
}
