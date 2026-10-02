package nolintprobe

import "context"

func capitalisedWait(ctx context.Context) error { return ctx.Err() }

// capitalisedFresh starts from a new context instead of taking one.
// contextcheck matches its own name case-sensitively, so a capitalised one in
// its doc's directive skips nothing and the call below is reported: the
// witness neither counts nor refuses the directive.
//
//nolint:unused // ContextCheck, capitalised, in the probe's reason
func capitalisedFresh() error { return capitalisedWait(context.Background()) }

// CapitalisedCaller has a context but calls capitalisedFresh, which does not
// take it.
func CapitalisedCaller(ctx context.Context) error { _ = ctx; return capitalisedFresh() }
