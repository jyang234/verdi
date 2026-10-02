package nolintprobe

import "context"

func reasonWait(ctx context.Context) error { return ctx.Err() }

// reasonFresh starts from a new context instead of taking one. Its doc's
// directive names only an ungated linter, but contextcheck reads its word in
// the reason and skips the function, so the call below is suppressed: the
// witness counts the directive as naming contextcheck.
//
//nolint:unused // contextcheck: the probe's word in the reason
func reasonFresh() error { return reasonWait(context.Background()) }

// ReasonCaller has a context but calls reasonFresh, which does not take it.
func ReasonCaller(ctx context.Context) error { _ = ctx; return reasonFresh() }
