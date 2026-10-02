package nolintprobe

import "context"

func detachedWait(ctx context.Context) error { return ctx.Err() }

//nolint:unused // contextcheck: the probe's word, detached from the function

// detachedFresh starts from a new context instead of taking one. A blank line
// separates the directive above from this doc comment, and contextcheck reads
// only a function declaration's doc, so the call below is reported: the
// witness neither counts nor refuses the directive.
func detachedFresh() error { return detachedWait(context.Background()) }

// DetachedCaller has a context but calls detachedFresh, which does not take
// it.
func DetachedCaller(ctx context.Context) error { _ = ctx; return detachedFresh() }
