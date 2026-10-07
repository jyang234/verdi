package gitx

import "context"

// Observer receives every git invocation gitx issues on a context it is
// attached to — dir and the exact argv after "git" — BEFORE the command
// runs (ritual-write-scope-v2 dc-5: a consumer-defined one-method observer
// attached through the context every gitx call already receives; no
// signature change; no package-level state). A nil or absent observer is
// a no-op. Observers must be safe for concurrent use if the caller runs
// gitx calls concurrently; gitx never copies args.
type Observer interface {
	Observe(dir string, args []string)
}

type observerKey struct{}

// WithObserver returns a context carrying obs.
func WithObserver(ctx context.Context, obs Observer) context.Context {
	if obs == nil {
		return ctx
	}
	return context.WithValue(ctx, observerKey{}, obs)
}

// SessionObserver is the optional half of an Observer that a structural
// witness implements (Wave 6 §5.3; ledger SI-356): an observer whose
// dynamic type also has ObserveSession is told, besides each process it
// sees through Observe, of what a read session (WithReadSession) does
// without starting one — so a witness counts every read a projection
// makes, not only the processes it launches. A replayed ref read or an
// object a batch process answers still names the revision git resolves,
// and a witness that saw processes alone would miss it. An observer
// without the method sees processes alone, exactly as before.
type SessionObserver interface {
	ObserveSession(dir string, event SessionEvent, args []string)
}

// SessionEvent names what a read session did without a process.
type SessionEvent string

const (
	// SessionOpened: WithReadSession opened a session for dir (args is
	// nil). A context that already carries one opens none.
	SessionOpened SessionEvent = "opened"
	// SessionReplayed: a read whose identical argv already ran in this
	// session was answered from that run (args is the argv).
	SessionReplayed SessionEvent = "replayed"
	// SessionBatched: an object name was sent to the session's
	// `cat-file --batch` process (args is the one name).
	SessionBatched SessionEvent = "batched"
)

// observeSession notifies the context's observer of a session event when
// the observer implements SessionObserver. It starts no process, so it is
// not one of observe's exec sites, and it appends no VERDI_GITLOG record:
// the log holds one record per git execution (ledger SI-359 (1)).
func observeSession(ctx context.Context, dir string, event SessionEvent, args []string) {
	if obs, ok := ctx.Value(observerKey{}).(SessionObserver); ok && obs != nil {
		obs.ObserveSession(dir, event, args)
	}
}

// observe is gitx's single observe point, called at the top of all four
// of gitx's exec sites: execGit (exec.go), ConfigValue (configvalue.go),
// runStdin (plumbing.go), and the read session's batch process
// (startCatFileBatch, readsession.go) — R-RR3-2 amended after Task 1
// review corrected the original premise that ConfigValue was the only
// exec site bypassing run.
// It first appends the call's record to the VERDI_GITLOG file when that
// variable is set (gitlog.go), then notifies the context's observer, if
// any. A non-nil error means the record failed: the exec site returns it
// without running git, and no observer has been told of a call that never
// ran.
func observe(ctx context.Context, dir string, args []string) error {
	if err := appendGitLog(dir, args); err != nil {
		return err
	}
	if obs, ok := ctx.Value(observerKey{}).(Observer); ok && obs != nil {
		obs.Observe(dir, args)
	}
	return nil
}
