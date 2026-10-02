package ritualwitness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
// (3)). Its actions root their own contexts (the commit-to-design
// action runs commitdesign.Run under context.Background), so no observer
// on the request reaches their gitx calls: Run reports the log
// unavailable, as Binary does, until spec/gitx-recorder-seam threads one.
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
// instead of judging it.
func (d Workbench) Run(ctx context.Context, dir string) (int, CommandLog, error) {
	if d.Serve == nil {
		return -1, CommandLog{}, errors.New("ritualwitness: Workbench: no handler constructor")
	}
	srv := httptest.NewServer(d.Serve(dir))
	defer srv.Close()
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
		return 0, CommandLog{}, nil
	}
	return 2, CommandLog{}, fmt.Errorf("ritualwitness: Workbench: %s %s answered %d: %s", d.Method, d.Path, resp.StatusCode, strings.TrimSpace(string(body)))
}
