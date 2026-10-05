package ritualwitness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

// stdinFeed copies a Binary's Stdin into the binary through a pipe the
// driver owns (R3ab review R3-B1). os/exec copies a Stdin that is not an
// *os.File itself, and its Wait waits for that copy until the reader
// reaches EOF, past the context and past WaitDelay, so a reader that never
// does (an io.Pipe nobody closes, a net.Conn) would hold Run for as long
// as it stays open. Owning the pipe lets Run stop waiting and abandon the
// copy.
type stdinFeed struct {
	// child is the pipe's read end, the binary's standard input.
	child *os.File
	// parent is the pipe's write end, which the copy fills.
	parent *os.File
	// closeParent closes parent once: the copy closes it at the reader's
	// EOF, so the binary reads its input's end, and abandon closes it to
	// stop feeding.
	closeParent func() error
	// copied carries the copy's result once it ends. It is buffered, so a
	// copy Run abandoned can still send its result, which nobody reads,
	// and end once the reader's Read returns.
	copied chan error
}

// errStdinClosed is what the copy's destination (pipeWriter) reports for
// a write to the pipe that failed with EPIPE: the binary no longer reads
// its standard input, because it closed it or exited.
var errStdinClosed = errors.New("ritualwitness: the binary no longer reads its standard input")

// pipeWriter is the copy's destination, the pipe's write end. It reports
// that end's own EPIPE as errStdinClosed, so the copy tells the binary's
// closed input apart from a source reader's error, even one wrapping
// EPIPE.
type pipeWriter struct{ f *os.File }

func (w pipeWriter) Write(p []byte) (int, error) {
	n, err := w.f.Write(p)
	if errors.Is(err, syscall.EPIPE) {
		return n, errStdinClosed
	}
	return n, err
}

// stdinFor returns the binary's standard input for src, and the feed that
// fills it when the driver owns the pipe: for nil, the null device and no
// feed; for an *os.File, the file itself and no feed; for any other
// reader, the read end of a new pipe, which the feed fills from src once
// started.
func stdinFor(src io.Reader) (io.Reader, *stdinFeed, error) {
	switch in := src.(type) {
	case nil:
		return nil, nil, nil
	case *os.File:
		return in, nil, nil
	}
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, fmt.Errorf("ritualwitness: Binary: opening the standard input pipe: %w", err)
	}
	f := &stdinFeed{child: r, parent: w, closeParent: sync.OnceValue(w.Close), copied: make(chan error, 1)}
	return r, f, nil
}

// start closes this process's copy of the read end, which the started
// binary now holds its own copy of, so the copy's writes fail once the
// binary exits, and begins copying src into the pipe. A nil feed does
// nothing.
func (f *stdinFeed) start(src io.Reader) {
	if f == nil {
		return
	}
	_ = f.child.Close()
	go func() {
		_, err := io.Copy(pipeWriter{f.parent}, src)
		if errors.Is(err, errStdinClosed) {
			// A write to the pipe failed with EPIPE: the binary closed
			// its standard input, or exited, before reading the rest. That
			// is its choice, not a fault, as os/exec reads a stdin write's
			// EPIPE too. Only that write's EPIPE counts: a source reader's
			// own error, even one wrapping EPIPE, is a failure to feed.
			err = nil
		}
		if cerr := f.closeParent(); err == nil {
			err = cerr
		}
		f.copied <- err
	}()
}

// abandon stops feeding: it closes both pipe ends this process holds, so
// the binary's standard input ends. A copy still blocked in the reader's
// Read is left behind, and ends when that Read returns. A nil feed does
// nothing.
func (f *stdinFeed) abandon() {
	if f == nil {
		return
	}
	_ = f.child.Close()
	_ = f.closeParent()
}

// wait waits for the copy to end, until ctx ends or binaryWaitDelay
// passes, and then abandons it. It returns nil when the copy reached the
// reader's EOF or the binary stopped reading, and otherwise an error naming
// the reader's failure, or the input that never reached EOF. A nil feed
// returns nil.
func (f *stdinFeed) wait(ctx context.Context) error {
	if f == nil {
		return nil
	}
	select {
	case err := <-f.copied:
		return feedFailure(err)
	default:
	}
	timer := time.NewTimer(binaryWaitDelay)
	defer timer.Stop()
	select {
	case err := <-f.copied:
		return feedFailure(err)
	case <-ctx.Done():
		f.abandon()
		return fmt.Errorf("its standard input never reached EOF before the context ended (%w), so the driver abandoned it", ctx.Err())
	case <-timer.C:
		f.abandon()
		return fmt.Errorf("its standard input never reached EOF within %s of its exit, so the driver abandoned it", binaryWaitDelay)
	}
}

// feedFailure names a copy's failure, or returns nil for a copy that
// ended cleanly.
func feedFailure(err error) error {
	if err != nil {
		return fmt.Errorf("feeding its standard input: %w", err)
	}
	return nil
}
