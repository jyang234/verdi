package mcpserve

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// startTestServerWithErrLog is startTestServer's twin (server_test.go),
// except it hands back the *Server itself so a test can set ErrLog before
// Serve starts accepting — startTestServer's own helper constructs and
// hides the *Server, which the dc-3 tests below need to configure.
func startTestServerWithErrLog(t *testing.T, root string) (sockPath string, srv *Server, stop func()) {
	t.Helper()
	sockPath, err := SocketPath(filepath.Join(t.TempDir(), "checkout"))
	if err != nil {
		t.Fatalf("SocketPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(sockPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(sockPath), err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(sockPath)) })

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	srv = NewServer(root)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(context.Background(), ln)
	}()
	return sockPath, srv, func() {
		ln.Close()
		<-done
	}
}

// TestServeErrLog_DroppedConnectionLogsOneLine proves dc-3's socket-side
// half: a connection ServeConn cannot make sense of (here, a single line
// so oversized it blows the framing scanner's max token size — bufio's
// ErrTooLong, a genuine non-EOF failure) writes exactly one line to the
// injected ErrLog. Before this change, Server.Serve discarded the
// ServeConn error entirely (`_ = ServeConn(...)`) — this same garbage
// would have left zero trace anywhere.
func TestServeErrLog_DroppedConnectionLogsOneLine(t *testing.T) {
	var buf bytes.Buffer
	sockPath, srv, stop := startTestServerWithErrLog(t, mustRepoDir(t))
	srv.ErrLog = &buf
	defer stop()

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dialing %s: %v", sockPath, err)
	}

	// One line with no newline, well past ServeConn's 1<<24 (16MiB) max
	// token size (wire.go) — bufio.Scanner errors out with ErrTooLong as
	// soon as it accumulates past that ceiling, deterministically, with no
	// dependence on connection-close timing.
	// The write is expected to fail partway through on some platforms: as
	// soon as the server side hits ErrTooLong it returns from ServeConn and
	// Serve's goroutine closes conn (its defer), which can race ahead of
	// this Write finishing and surface as a broken-pipe error here. That
	// is itself evidence the server-side condition fired, not a test
	// failure — the error is deliberately ignored rather than asserted on.
	garbage := bytes.Repeat([]byte{'x'}, (1<<24)+(1<<20))
	_, _ = conn.Write(garbage)
	conn.Close()

	// stop() joins Server.Serve's WaitGroup, which does not return until
	// every accepted connection's handler (including this one) has
	// returned — so by the time stop() itself returns, logConnErr has
	// already run (or determinately has not), no sleep/poll needed.
	stop()

	got := buf.String()
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("ErrLog got %d line(s), want exactly 1: %q", strings.Count(got, "\n"), got)
	}
	if !strings.Contains(got, "mcpserve:") {
		t.Fatalf("ErrLog line missing the mcpserve: prefix: %q", got)
	}
}

// TestServeErrLog_CleanCloseLeavesNoTrace proves dc-3's other half: a
// client that simply disconnects (no request ever sent) is a CLEAN close —
// bufio.Scanner's Err() is nil on plain EOF, so ServeConn returns nil and
// logConnErr is never even reached. The injected ErrLog stays byte-empty.
func TestServeErrLog_CleanCloseLeavesNoTrace(t *testing.T) {
	var buf bytes.Buffer
	sockPath, srv, stop := startTestServerWithErrLog(t, mustRepoDir(t))
	srv.ErrLog = &buf

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dialing %s: %v", sockPath, err)
	}
	conn.Close() // disconnect without ever sending a request: a clean EOF

	stop() // blocks until this connection's handler has returned

	if buf.Len() != 0 {
		t.Fatalf("ErrLog got %d byte(s) for a clean close, want 0: %q", buf.Len(), buf.String())
	}
}

// TestServeErrLog_NilErrLogIsSilent proves the zero-value Server (ErrLog
// nil, e.g. the fixture every other server_test.go case builds via
// startTestServer/NewServer) never panics logConnErr and never writes
// anywhere — nil is silence, not a crash.
func TestServeErrLog_NilErrLogIsSilent(t *testing.T) {
	sockPath, srv, stop := startTestServerWithErrLog(t, mustRepoDir(t))
	if srv.ErrLog != nil {
		t.Fatalf("NewServer's zero-value ErrLog = %v, want nil", srv.ErrLog)
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dialing %s: %v", sockPath, err)
	}
	// The write is expected to fail partway through on some platforms: as
	// soon as the server side hits ErrTooLong it returns from ServeConn and
	// Serve's goroutine closes conn (its defer), which can race ahead of
	// this Write finishing and surface as a broken-pipe error here. That
	// is itself evidence the server-side condition fired, not a test
	// failure — the error is deliberately ignored rather than asserted on.
	garbage := bytes.Repeat([]byte{'x'}, (1<<24)+(1<<20))
	_, _ = conn.Write(garbage)
	conn.Close()

	stop() // would panic here if logConnErr dereferenced a nil ErrLog
}
