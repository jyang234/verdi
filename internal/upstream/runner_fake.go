package upstream

import (
	"context"
	"fmt"
	"sync"
)

// FakeRunner serves canned Results keyed by (Bin, Subcommand), with no
// exec and no network — the hermetic double every test in this module (and
// internal/bundle, cmd/verdi) uses in place of RealRunner (CLAUDE.md: "No
// network in any test"). Responses are queued FIFO per key so a test that
// invokes the same (bin, subcommand) more than once (e.g. `flowmap graph`
// for a base graph and then a branch graph) can script each call's answer
// independently; a key with a single registered response keeps serving it
// for every subsequent call to that key (the common case).
//
// FakeRunner also records every Request it received, in call order, so
// tests can assert on the exact argv a caller built — in particular, that
// flags always precede positional arguments.
type FakeRunner struct {
	mu sync.Mutex

	queued map[string][]fakeResponse
	sticky map[string]fakeResponse // last response for a key once its queue drains
	Calls  []Request
}

type fakeResponse struct {
	result Result
	err    error
}

// NewFakeRunner returns an empty FakeRunner: no responses registered, no
// calls recorded.
func NewFakeRunner() *FakeRunner {
	return &FakeRunner{
		queued: make(map[string][]fakeResponse),
		sticky: make(map[string]fakeResponse),
	}
}

func fakeKey(bin, subcommand string) string { return bin + " " + subcommand }

// Enqueue arranges for the next call to (bin, subcommand) to return result
// with a nil error. Call it multiple times to script a sequence of
// distinct responses for repeated calls to the same key; after the queue
// drains, the last enqueued response keeps being served.
func (f *FakeRunner) Enqueue(bin, subcommand string, result Result) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fakeKey(bin, subcommand)
	f.queued[key] = append(f.queued[key], fakeResponse{result: result})
}

// EnqueueError arranges for the next call to (bin, subcommand) to return an
// exec-level error (RealRunner's failure mode for a missing binary, a
// cancelled context, etc. — distinct from a nonzero-but-clean ExitCode).
func (f *FakeRunner) EnqueueError(bin, subcommand string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fakeKey(bin, subcommand)
	f.queued[key] = append(f.queued[key], fakeResponse{err: err})
}

// Run implements Runner.
func (f *FakeRunner) Run(ctx context.Context, req Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Calls = append(f.Calls, req)
	key := fakeKey(req.Bin, req.Subcommand)

	if queue := f.queued[key]; len(queue) > 0 {
		resp := queue[0]
		f.queued[key] = queue[1:]
		f.sticky[key] = resp
		return resp.result, resp.err
	}
	if resp, ok := f.sticky[key]; ok {
		return resp.result, resp.err
	}
	return Result{}, fmt.Errorf("upstream: FakeRunner: no response registered for %s %s (call Enqueue first)", req.Bin, req.Subcommand)
}

var _ Runner = (*FakeRunner)(nil)
