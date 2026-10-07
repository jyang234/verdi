package recovery

import (
	"sync"

	"github.com/jyang234/verdi/internal/gitforbid"
	"github.com/jyang234/verdi/internal/gitx"
)

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

// Forbidden returns the recorded commands whose argv contains a forbidden
// token (IsForbiddenArgv), in call order.
func (l *CommandLog) Forbidden() [][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out [][]string
	for _, entry := range l.entries {
		if IsForbiddenArgv(entry) {
			out = append(out, append([]string(nil), entry...))
		}
	}
	return out
}

// IsForbiddenArgv reports whether argv, one command's whole argument list,
// contains a forbidden token: dc-4/ac-9's command-surface guard, no
// recovery run's git command log may contain one. The tokens and the
// matching rule are the one shared list's (gitforbid.Tokens and
// gitforbid.Forbids, spec/gitx-recorder-seam dc-1), which carries -f
// besides ac-9's six (R-RR3-17, ledger SI-224): a recovery run never
// legitimately passes it to any command it issues. Neither of this
// feature's two executors (branchcut.Unwind, reclaim.Apply) issues a `git
// commit`, so the rule's one blind spot, an exact one-word commit message,
// is unreachable here. Exported so a caller gating on the SAME rule
// Forbidden uses (cmd/verdi's recover.go, R-RR3-10's own runtime reaction)
// scans through this one seam rather than a bespoke copy that can silently
// drift from it (fix round 1, M4).
func IsForbiddenArgv(argv []string) bool {
	return gitforbid.Forbids(argv)
}
