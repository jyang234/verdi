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

// observe notifies the context's observer, if any. Called at the top of
// all three of gitx's exec sites: run, ConfigValue, and runStdin
// (plumbing.go) — R-RR3-2 amended after Task 1 review corrected the
// original premise that ConfigValue was the only exec site bypassing run.
func observe(ctx context.Context, dir string, args []string) {
	if obs, ok := ctx.Value(observerKey{}).(Observer); ok && obs != nil {
		obs.Observe(dir, args)
	}
}
