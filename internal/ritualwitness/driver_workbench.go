package ritualwitness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
)

// Workbench is the workbench-handler Driver (spec/ritual-effect-witness
// dc-1): Serve builds the running server's handler for the fixture
// directory, as `verdi serve` builds it for its store root
// (internal/workbench.NewHandler, for one), and Run serves it on a
// loopback httptest server and sends it one request.
//
// The workbench has no exit codes of its own: a 2xx answer is exit 0, and
// any other answer is 2, the operational class, with the status and body in
// the error. Run follows no redirect, so a 3xx answer is itself exit 2 and
// never judged by its target, which is never requested (ledger SI-334
// (3)).
//
// The server runs in the test process, so its command log is a
// gitx.Observer (spec/gitx-recorder-seam ac-2; parent dc-5; ledger SI-359
// (1)): Run attaches one through the server's base context, without the
// caller's cancellation, so every request's context, and every context a
// handler derives from it, carries it. Run closes the server, which waits
// for its handlers, before it reads the log. A run that ends in a verb's
// exit reports that log with CommandLog.OK true; one that ends in no
// verb's exit (-1) reports none. A handler that roots a context of its own
// (context.Background or TODO) would drop the observer; the static
// context-root guard in internal/specalign fails on any such root the
// workbench reaches (SI-359 (7), (14)), and the ritual-effect producer
// compares each in-process run's log with a process-wide VERDI_GITLOG
// record for record.
type Workbench struct {
	// Serve builds the handler for a store root.
	Serve func(root string) http.Handler
	// Method, Path, Body, and Header make up the one request.
	Method string
	Path   string
	Body   []byte
	Header http.Header
}

// Run implements Driver. A request that cannot be built or answered
// returns -1, which is no verb's exit class, so RunOn refuses the run
// instead of judging it, with no log.
func (d Workbench) Run(ctx context.Context, dir string) (int, CommandLog, error) {
	if d.Serve == nil {
		return -1, CommandLog{}, errors.New("ritualwitness: Workbench: no handler constructor")
	}
	rec := &recorder{}
	srv := httptest.NewUnstartedServer(d.Serve(dir))
	base := gitx.WithObserver(context.WithoutCancel(ctx), rec)
	srv.Config.BaseContext = func(net.Listener) context.Context { return base }
	srv.Start()
	defer srv.Close()
	// logged is the observer's log, read once the server has closed, so
	// every handler has returned.
	logged := func() CommandLog {
		srv.Close()
		return CommandLog{Calls: rec.snapshot(), OK: true}
	}
	req, err := http.NewRequestWithContext(ctx, d.Method, srv.URL+d.Path, bytes.NewReader(d.Body))
	if err != nil {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: Workbench: building the request: %w", err)
	}
	for k, vs := range d.Header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: Workbench: %s %s: %w", d.Method, d.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: Workbench: reading the answer to %s %s: %w", d.Method, d.Path, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return 0, logged(), nil
	}
	return 2, logged(), fmt.Errorf("ritualwitness: Workbench: %s %s answered %d: %s", d.Method, d.Path, resp.StatusCode, strings.TrimSpace(string(body)))
}
