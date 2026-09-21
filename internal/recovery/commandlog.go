package recovery

import (
	"strings"
	"sync"
)

// ForbiddenTokens is dc-4/ac-9's own command-surface guard: no recovery
// run's git command log may contain any of these as a whole argv
// element, or — for a "--"-prefixed flag — as an argv element's prefix
// ("--force" matches both "--force" and "--force-with-lease"). Message
// TEXT (a commit message, say) is never scanned for these words: the
// check is over argv elements only, so `commit -m "reset the counter"`
// is never forbidden merely because its (whole, single) message argument
// happens to start with the word "reset".
var ForbiddenTokens = []string{"reset", "restore", "clean", "stash", "--force", "update-ref"}

// CommandLog records every gitx invocation on a context via the
// gitx.Observer seam (R-RR3-2): recover.go (a later task) attaches one
// for the whole run (R-RR3-10) so a violation of dc-4/ac-9's "no git
// primitive beyond branchcut.Unwind's and reclaim.Apply's own calls"
// contract can be detected and reported as an operational failure of the
// verb itself, never as a verdict. The zero value is ready to use.
type CommandLog struct {
	mu      sync.Mutex
	entries [][]string
}

// Observe implements the method signature gitx.Observer declares
// (Observe(dir string, args []string)) — Task 1, running concurrently,
// adds that interface and the compile-time `var _ gitx.Observer =
// (*CommandLog)(nil)` assertion at integration; this package does not
// import or reference gitx.Observer itself. dir is deliberately not
// recorded: Forbidden's contract is over argv shape alone.
func (l *CommandLog) Observe(dir string, args []string) {
	entry := make([]string, len(args))
	copy(entry, args)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

// Entries returns every recorded command's argv, in call order.
func (l *CommandLog) Entries() [][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([][]string, len(l.entries))
	for i, e := range l.entries {
		out[i] = append([]string(nil), e...)
	}
	return out
}

// Forbidden returns the recorded commands whose argv contains any
// ForbiddenTokens match, in call order.
func (l *CommandLog) Forbidden() [][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out [][]string
	for _, entry := range l.entries {
		for _, arg := range entry {
			if matchesForbiddenToken(arg) {
				out = append(out, append([]string(nil), entry...))
				break
			}
		}
	}
	return out
}

// matchesForbiddenToken reports whether arg — one whole argv element —
// matches a ForbiddenTokens entry: exact equality always counts; for a
// "--"-prefixed flag token, arg being prefixed by it also counts (closing
// "--force-with-lease" over "--force" without opening plain tokens like
// "reset" to a raw string-prefix match, which would wrongly flag an
// unrelated argument that merely starts with the same word, such as a
// commit message).
func matchesForbiddenToken(arg string) bool {
	for _, ft := range ForbiddenTokens {
		if arg == ft {
			return true
		}
		if strings.HasPrefix(ft, "--") && strings.HasPrefix(arg, ft) {
			return true
		}
	}
	return false
}
