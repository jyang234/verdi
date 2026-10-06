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

// observe is gitx's single observe point, called at the top of all three
// of gitx's exec sites: run, ConfigValue, and runStdin (plumbing.go) —
// R-RR3-2 amended after Task 1 review corrected the original premise that
// ConfigValue was the only exec site bypassing run. It first appends the
// call's record to the VERDI_GITLOG file when that variable is set
// (gitlog.go), then notifies the context's observer, if any. A non-nil
// error means the record failed: the exec site returns it without running
// git, and no observer has been told of a call that never ran.
func observe(ctx context.Context, dir string, args []string) error {
	if err := appendGitLog(dir, args); err != nil {
		return err
	}
	if obs, ok := ctx.Value(observerKey{}).(Observer); ok && obs != nil {
		obs.Observe(dir, args)
	}
	return nil
}
