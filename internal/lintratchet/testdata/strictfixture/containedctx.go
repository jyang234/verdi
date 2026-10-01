package strictfixture

import "context"

// holder stores a context in a struct: containedctx's one finding.
type holder struct {
	ctx context.Context
}

// done uses the stored context.
func (h holder) done() <-chan struct{} { return h.ctx.Done() }

// Done exposes done so holder is used.
func Done() <-chan struct{} { return holder{ctx: context.TODO()}.done() }
