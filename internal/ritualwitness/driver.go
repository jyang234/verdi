package ritualwitness

import (
	"context"
	"sync"

	"github.com/jyang234/verdi/internal/gitx"
)

// Call is one git invocation a ritual issued, as gitx.Observer reports it:
// the directory it ran in and the exact argv after "git".
type Call struct {
	Dir  string
	Args []string
}

// CommandLog is what a Driver recorded of a ritual's git invocations, or
// the disclosed absence of one. OK is false when the driver cannot supply a
// log — a built binary before spec/gitx-recorder-seam's command-log hook
// lands (dc-1) — and then Calls is always nil: every effect that only the
// log could attribute is unattributable, never inferred from an absent
// log's zero value.
type CommandLog struct {
	Calls []Call
	OK    bool
}

// Mentions reports whether any recorded call's argv contains token
// verbatim. It is the harness's attribution primitive for ref and worktree
// effects (dc-2: the log catches a mutation made outside gitx — one whose
// state change has no corresponding call here at all). It is always false
// when the log is unavailable.
func (l CommandLog) Mentions(token string) bool {
	if !l.OK {
		return false
	}
	for _, c := range l.Calls {
		for _, a := range c.Args {
			if a == token {
				return true
			}
		}
	}
	return false
}

// HasVerb reports whether any recorded call's first argument (the git
// subcommand) equals verb — used where the token an effect would name
// (e.g. a pushed branch's remote-tracking ref) does not literally appear
// in the argv gitx.Push issues ("push --set-upstream origin HEAD" never
// names the branch).
func (l CommandLog) HasVerb(verb string) bool {
	if !l.OK {
		return false
	}
	for _, c := range l.Calls {
		if len(c.Args) > 0 && c.Args[0] == verb {
			return true
		}
	}
	return false
}

// Ritual is a synthetic, in-process ritual: git operations run against dir
// through ctx (so a Driver's gitx.Observer attaches), reporting the exit
// classification ground-rules' three-way split gives every verb — 0 clean,
// 1 verdict failure, 2 operational refusal — the same shape a built
// binary's real exit code will carry for the driver spec/ritual-effect-
// witness's R3 lane adds.
type Ritual func(ctx context.Context, dir string) (exit int, err error)

// Driver runs one ritual against a prepared, already-seeded fixture
// directory and reports its exit classification and git command log.
// Parent dc-1 names three more drivers spec/ritual-effect-witness's R3 lane
// adds — the built binary, the workbench's handlers, and MCP — each with
// its own way of driving a ritual and its own way (or inability, before
// spec/gitx-recorder-seam lands) to supply a command log; this interface is
// shaped so all four drop in unchanged. InProcess, below, is the only one
// built here.
type Driver interface {
	Run(ctx context.Context, dir string) (exit int, log CommandLog, err error)
}

// InProcess is the in-process Driver (dc-1's first of four): it runs a
// Ritual function directly against dir, with the harness's own
// gitx.Observer attached to ctx, so its CommandLog is always available.
type InProcess struct {
	Fn Ritual
}

// Run implements Driver.
func (d InProcess) Run(ctx context.Context, dir string) (int, CommandLog, error) {
	rec := &recorder{}
	exit, err := d.Fn(gitx.WithObserver(ctx, rec), dir)
	return exit, CommandLog{Calls: rec.snapshot(), OK: true}, err
}

// recorder implements gitx.Observer, copying every call's args (gitx's own
// doc: "gitx never copies args") so a later mutation of a caller-owned
// slice can never corrupt an already-recorded call. Safe for concurrent
// use, as gitx.Observer requires.
type recorder struct {
	mu    sync.Mutex
	calls []Call
}

func (r *recorder) Observe(dir string, args []string) {
	cp := append([]string(nil), args...)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, Call{Dir: dir, Args: cp})
}

func (r *recorder) snapshot() []Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Call(nil), r.calls...)
}
