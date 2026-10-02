package nolintprobe

import (
	"context"
	"net/http"
)

func requestControlWait(ctx context.Context) error { return ctx.Err() }

func requestControlFresh[T any](v T) error { _ = v; return requestControlWait(context.Background()) }

// requestControlMid is suppressed_contextcheck_request.go's shape without
// contextcheck's request flag: the call below is reported.
func requestControlMid(r *http.Request) error { return requestControlFresh(r) }

// RequestControlCaller has a context but calls requestControlMid, which does
// not take it.
func RequestControlCaller(ctx context.Context, r *http.Request) error {
	_ = ctx
	return requestControlMid(r)
}
