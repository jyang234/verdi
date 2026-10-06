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
// lands (dc-1) — and Evaluate then reports the absence as its own
// unattributable verdict and attributes nothing from the log: every effect
// that only the log could attribute is unattributable, never inferred from
// an absent log's zero value.
type CommandLog struct {
	Calls []Call
	OK    bool
}

// Ritual is a synthetic, in-process ritual: git operations run against dir
// through ctx (so a Driver's gitx.Observer attaches), reporting the exit
// classification every verb has — 0 clean, 1 verdict failure, 2
// operational refusal — the same shape a built binary's exit code carries.
type Ritual func(ctx context.Context, dir string) (exit int, err error)

// Driver runs one ritual against a prepared, already-seeded fixture
// directory and reports its exit classification and git command log.
// spec/ritual-effect-witness dc-1 names the entry points: InProcess runs a
// synthetic ritual; Binary (driver_binary.go) runs a CLI verb as the built
// binary; Workbench (driver_workbench.go) sends a workbench action through
// the running server's handler; MCP (driver_mcp.go) calls one tool on the
// MCP server, run in process. The last three cannot supply a command log
// before spec/gitx-recorder-seam lands one.
type Driver interface {
	Run(ctx context.Context, dir string) (exit int, log CommandLog, err error)
}

// InProcess is the in-process Driver: it runs a Ritual function directly
// against dir with the harness's own gitx.Observer attached to ctx, so its
// CommandLog is always available.
type InProcess struct {
	Fn Ritual
}

// Run implements Driver.
func (d InProcess) Run(ctx context.Context, dir string) (int, CommandLog, error) {
	rec := &recorder{}
	exit, err := d.Fn(gitx.WithObserver(ctx, rec), dir)
	return exit, CommandLog{Calls: rec.snapshot(), OK: true}, err
}

// recorder implements gitx.Observer, copying every call's args (gitx never
// copies them) so a later change to a caller-owned slice cannot corrupt a
// recorded call. Safe for concurrent use, as gitx.Observer requires.
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
