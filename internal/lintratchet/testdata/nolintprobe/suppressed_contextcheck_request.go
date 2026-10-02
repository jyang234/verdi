package nolintprobe

import (
	"context"
	"net/http"
)

func requestWait(ctx context.Context) error { return ctx.Err() }

func requestFresh[T any](v T) error { _ = v; return requestWait(context.Background()) }

// requestMid takes a request and reaches a fresh context through an instance
// of a generic function. contextcheck's request flag below makes it check
// requestMid as a handler, where that instance has no recorded result, so
// the finding reported_contextcheck_request_control.go shows is suppressed
// outright: the witness counts the flag as naming contextcheck.
//
// @contextcheck(req_has_ctx)
func requestMid(r *http.Request) error { return requestFresh(r) }

// RequestCaller has a context but calls requestMid, which does not take it.
func RequestCaller(ctx context.Context, r *http.Request) error { _ = ctx; return requestMid(r) }
