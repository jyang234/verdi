package strictfixture

import "context"

// fresh starts from a new context instead of taking one.
func fresh() error {
	return wait(context.Background())
}

func wait(ctx context.Context) error { return ctx.Err() }

// Caller has a context but calls fresh, which does not take it:
// contextcheck's one finding.
func Caller(ctx context.Context) error {
	_ = ctx
	return fresh()
}
