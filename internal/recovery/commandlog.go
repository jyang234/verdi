package recovery

import (
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/gitx"
)

// ForbiddenTokens is dc-4/ac-9's own command-surface guard: no recovery
// run's git command log may contain any of these as a whole argv
// element, or — for a "--"-prefixed flag — as an argv element's prefix
// ("--force" matches both "--force" and "--force-with-lease"). "-f"
// (R-RR3-17) is the short force flag `git push -f` / `git branch -f` /
// `git checkout -f` all accept, which "--force" alone does not catch; a
// recovery run never legitimately passes it to any command it issues.
// The check is over argv ELEMENTS only, never message TEXT: `commit -m
// "reset the counter"` is not forbidden merely because its (whole,
// single) message argument happens to start with the word "reset" — an
// exact one-word match ("commit -m reset") would still be forbidden by
// construction, since the check cannot distinguish a message argument
// from any other bare argv element, but this is unreachable inside a
// recovery run: neither of this feature's two executors
// (branchcut.Unwind, reclaim.Apply) ever issues a `git commit`.
var ForbiddenTokens = []string{"reset", "restore", "clean", "stash", "--force", "-f", "update-ref"}

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

// _ is the compile-time proof that CommandLog implements gitx.Observer
// (2A-M4): gitx cannot host this assertion itself (internal/gitx
// importing internal/recovery would cycle, since facts.go imports
// gitx), so it lives on this, the implementing, side.
var _ gitx.Observer = (*CommandLog)(nil)

// Observe implements gitx.Observer's Observe(dir string, args []string)
// method. dir is deliberately not recorded: Forbidden's contract is over
// argv shape alone.
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
