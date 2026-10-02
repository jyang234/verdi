package nolintprobe

import "context"

func pluralWait(ctx context.Context) error { return ctx.Err() }

// pluralFresh starts from a new context instead of taking one. Its doc's
// directive names no linter golangci-lint knows, but contextcheck finds its
// own name inside that one and skips the function, so the call below is
// suppressed: the witness counts the directive as naming contextcheck.
//
//nolint:contextchecks
func pluralFresh() error { return pluralWait(context.Background()) }

// PluralCaller has a context but calls pluralFresh, which does not take it.
func PluralCaller(ctx context.Context) error { _ = ctx; return pluralFresh() }
