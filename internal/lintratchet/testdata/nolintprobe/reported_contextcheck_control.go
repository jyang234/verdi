package nolintprobe

import "context"

func controlWait(ctx context.Context) error { return ctx.Err() }

// controlFresh starts from a new context instead of taking one. Its doc's
// directive names only an ungated linter and its reason lacks contextcheck's
// word, so the call below is reported: the witness neither counts nor
// refuses the directive.
//
//nolint:unused // the probe's reason, which names no linter
func controlFresh() error { return controlWait(context.Background()) }

// ControlCaller has a context but calls controlFresh, which does not take it.
func ControlCaller(ctx context.Context) error { _ = ctx; return controlFresh() }
