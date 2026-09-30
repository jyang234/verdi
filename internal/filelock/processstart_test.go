package filelock

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeProcessStarts installs a psLstart seam answering from starts (pid ->
// that pid's OS process start) and failing for any pid it does not name,
// exactly as a real `ps -o lstart=` fails for a pid it cannot see. The
// original seam is restored when t ends.
func fakeProcessStarts(t *testing.T, starts map[int]time.Time) {
	t.Helper()
	orig := psLstart
	psLstart = func(pid int) (time.Time, error) {
		if st, ok := starts[pid]; ok {
			return st, nil
		}
		return time.Time{}, fmt.Errorf("fake ps: no process start known for pid %d", pid)
	}
	t.Cleanup(func() { psLstart = orig })
}

// secondsAgo is now minus d, truncated to the whole-second resolution
// `ps -o lstart=` reports a process start in.
func secondsAgo(d time.Duration) time.Time {
	return time.Unix(time.Now().Add(-d).Unix(), 0)
}

// readLockBody strict-decodes the lock body at path, exactly as Acquire,
// Peek, and Inspect themselves read it.
func readLockBody(t *testing.T, path string) Info {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading lock body: %v", err)
	}
	var info Info
	if err := strictUnmarshal(data, &info); err != nil {
		t.Fatalf("decoding lock body %q: %v", string(data), err)
	}
	return info
}

// startSleeper starts a child process that stays alive for the test — a
// live pid that is not this process — and reaps it when t ends.
func startSleeper(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("sleep", "300")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting a live child process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return cmd.Process.Pid
}

// TestAcquire_LongRunningHolderIsJudgedLive is BL-101's RED witness
// (SI-300): a process whose OS start (simulated through the psLstart seam)
// lies ten minutes, twice lockStartTolerance, before it acquires a lock
// must still be judged that lock's live holder by every prober, and a
// second acquirer must get ErrHeld rather than take the lock over. Before
// SI-300 Acquire recorded the lock's creation time, the probe compared it
// with the ten-minutes-earlier process start, and the live lock read as a
// reused pid. The parent pid's start is also known to the seam and differs,
// so a lock recording any process's start but the acquirer's own fails the
// body check.
func TestAcquire_LongRunningHolderIsJudgedLive(t *testing.T) {
	probers := []struct {
		name  string
		judge func(t *testing.T, path string, info Info)
	}{
		{"alive", func(t *testing.T, _ string, info Info) {
			if !alive(info.PID, info.Start) {
				t.Fatalf("alive(%d, %d) = false, want true: the lock's own live holder was judged a reused pid", info.PID, info.Start)
			}
		}},
		{"Peek", func(t *testing.T, path string, info Info) {
			got, held, err := Peek(path)
			if err != nil {
				t.Fatalf("Peek: %v", err)
			}
			if !held || got != info {
				t.Fatalf("Peek = %+v, held %t, want %+v, held true", got, held, info)
			}
		}},
		{"Inspect", func(t *testing.T, path string, info Info) {
			got, err := Inspect(path)
			if err != nil {
				t.Fatalf("Inspect: %v", err)
			}
			if got.Status != LockHeld || got.Info != info {
				t.Fatalf("Inspect = %+v, want status %s with info %+v", got, LockHeld, info)
			}
		}},
		{"second Acquire gets ErrHeld and takes nothing over", func(t *testing.T, path string, info Info) {
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			f2, err := Acquire(path)
			if err == nil {
				_ = Release(f2, path)
				t.Fatal("second Acquire took the live holder's lock over, want *ErrHeld")
			}
			var held *ErrHeld
			if !errors.As(err, &held) {
				t.Fatalf("second Acquire error = %T %v, want *ErrHeld", err, err)
			}
			if held.Info != info {
				t.Fatalf("ErrHeld.Info = %+v, want %+v", held.Info, info)
			}
			after, err := os.Stat(path)
			if err != nil {
				t.Fatalf("the live lock is gone after a second Acquire: %v", err)
			}
			if !os.SameFile(before, after) {
				t.Fatal("the live lock file was replaced by a second Acquire")
			}
		}},
	}
	for _, p := range probers {
		t.Run(p.name, func(t *testing.T) {
			ownStart := secondsAgo(10 * time.Minute)
			fakeProcessStarts(t, map[int]time.Time{
				os.Getpid():  ownStart,
				os.Getppid(): secondsAgo(3 * time.Hour),
			})
			path := filepath.Join(t.TempDir(), "writer.lock")
			f, err := Acquire(path)
			if err != nil {
				t.Fatalf("Acquire: %v", err)
			}
			t.Cleanup(func() { _ = Release(f, path) })

			info := readLockBody(t, path)
			if info.PID != os.Getpid() || info.Start != ownStart.Unix() {
				t.Errorf("lock body = %+v, want {PID:%d Start:%d}: start must be the holder's own OS process start, not the lock's creation time", info, os.Getpid(), ownStart.Unix())
			}
			p.judge(t, path, info)
		})
	}
}

// TestAcquire_RecordsOwnProcessStartThroughTheSeam pins where Acquire's
// recorded start comes from: psLstart on this process's own pid, read
// afresh by every Acquire, so each row's seam value (never an earlier
// row's, which a stale process-wide cache would return) lands in the body,
// whatever the process's age. Each row also confirms the probe then judges
// the holder alive.
func TestAcquire_RecordsOwnProcessStartThroughTheSeam(t *testing.T) {
	for _, age := range []time.Duration{
		time.Second,
		lockStartTolerance - time.Second,
		lockStartTolerance + time.Second,
		10 * time.Minute,
		30 * 24 * time.Hour,
	} {
		t.Run(age.String(), func(t *testing.T) {
			ownStart := secondsAgo(age)
			fakeProcessStarts(t, map[int]time.Time{os.Getpid(): ownStart})
			path := filepath.Join(t.TempDir(), "writer.lock")
			f, err := Acquire(path)
			if err != nil {
				t.Fatalf("Acquire: %v", err)
			}
			t.Cleanup(func() { _ = Release(f, path) })
			if got := readLockBody(t, path); got.Start != ownStart.Unix() {
				t.Fatalf("recorded start = %d, want the seam's own process start %d", got.Start, ownStart.Unix())
			}
			if !alive(os.Getpid(), ownStart.Unix()) {
				t.Fatal("alive(own pid, own recorded start) = false, want true")
			}
		})
	}
}

// TestAcquire_OwnStartUnreadableRecordsAcquisitionTime is SI-300's
// disclosed fallback: when this process's own OS start cannot be read at
// acquisition, Acquire still acquires the lock and records the acquisition
// time (the pre-SI-300 value), and while ps stays unavailable the probe's
// kill-probe-only fallback keeps the live holder held against a second
// acquirer.
func TestAcquire_OwnStartUnreadableRecordsAcquisitionTime(t *testing.T) {
	fakeProcessStarts(t, nil)
	path := filepath.Join(t.TempDir(), "writer.lock")

	before := time.Now().Unix()
	f, err := Acquire(path)
	after := time.Now().Unix()
	if err != nil {
		t.Fatalf("Acquire with its own start unreadable: %v, want the lock acquired", err)
	}
	t.Cleanup(func() { _ = Release(f, path) })

	info := readLockBody(t, path)
	if info.PID != os.Getpid() || info.Start < before || info.Start > after {
		t.Fatalf("lock body = %+v, want {PID:%d Start in [%d, %d]} (the acquisition time)", info, os.Getpid(), before, after)
	}
	if held, herr := HeldByCurrentProcess(path); herr != nil || !held {
		t.Fatalf("HeldByCurrentProcess = %t, %v, want true, nil", held, herr)
	}
	taken, err := Acquire(path)
	if err == nil {
		_ = Release(taken, path)
		t.Fatal("second Acquire with ps unavailable took the live lock over, want *ErrHeld")
	}
	if !errors.As(err, new(*ErrHeld)) {
		t.Fatalf("second Acquire with ps unavailable = %v, want *ErrHeld", err)
	}
}

// TestProbe_ProcessStartDrift pins the comparison SI-300 keeps (01 §D3): a
// live pid is its lock's holder iff its OS process start agrees with the
// recorded start within lockStartTolerance, in either direction; outside
// it a different process reused the pid. The rows cover SI-300's own locks
// (recorded start == process start), locks written by an older binary
// (recorded start == creation time, some while after the process start,
// judged exactly as before SI-300 — including a long-running older holder,
// which stays exposed), the tolerance boundary on both sides, and a holder
// pid other than the prober's own.
func TestProbe_ProcessStartDrift(t *testing.T) {
	self := os.Getpid()
	child := startSleeper(t)
	selfStart := secondsAgo(2 * time.Hour)
	childStart := secondsAgo(30 * time.Minute)
	fakeProcessStarts(t, map[int]time.Time{self: selfStart, child: childStart})

	cases := []struct {
		name     string
		pid      int
		recorded time.Time
		want     bool
	}{
		{"SI-300 lock: recorded start is the holder's process start", self, selfStart, true},
		{"older-binary lock created seconds after process start", self, selfStart.Add(3 * time.Second), true},
		// Literal durations: the pre-SI-300 judgement of an older binary's
		// lock depends on the tolerance staying exactly five minutes.
		{"older-binary lock created 4m59s after process start is live, as before", self, selfStart.Add(4*time.Minute + 59*time.Second), true},
		{"older-binary lock created 5m1s after process start is stale, as before", self, selfStart.Add(5*time.Minute + time.Second), false},
		{"older-binary lock created at exactly the tolerance", self, selfStart.Add(lockStartTolerance), true},
		{"older-binary lock created past the tolerance is judged as before", self, selfStart.Add(lockStartTolerance + time.Second), false},
		{"recorded start precedes the process start by exactly the tolerance", self, selfStart.Add(-lockStartTolerance), true},
		{"recorded start precedes the process start past the tolerance: reused pid", self, selfStart.Add(-lockStartTolerance - time.Second), false},
		{"recorded start an hour after the process start: reused pid", self, selfStart.Add(time.Hour), false},
		{"another live holder recording its own start", child, childStart, true},
		{"another live holder's pid recorded with the prober's start: reused pid", child, selfStart, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isAlive, decided, reason := probe(tc.pid, tc.recorded.Unix())
			if !decided {
				t.Fatalf("probe undecided (%s), want a decided answer", reason)
			}
			if isAlive != tc.want {
				t.Fatalf("probe(pid %d, recorded %s) alive = %t, want %t (reason %q)", tc.pid, tc.recorded.Format(time.RFC3339), isAlive, tc.want, reason)
			}
			if !tc.want && !strings.Contains(reason, "reused this pid") {
				t.Fatalf("stale reason = %q, want it to name pid reuse", reason)
			}
			if got := alive(tc.pid, tc.recorded.Unix()); got != tc.want {
				t.Fatalf("alive = %t, want %t", got, tc.want)
			}
		})
	}
}

// TestAcquireAndPeek_JudgeTheRecordedHolderNotTheCaller pins which side of
// the comparison Acquire and Peek probe: the pid the lock body records,
// against that pid's own OS start — never the calling process's pid or
// start. The acquirer's own start (two hours ago) differs from the live
// child holder's (thirty minutes ago), so probing the wrong pid flips
// every row.
func TestAcquireAndPeek_JudgeTheRecordedHolderNotTheCaller(t *testing.T) {
	self := os.Getpid()
	child := startSleeper(t)
	selfStart := secondsAgo(2 * time.Hour)
	childStart := secondsAgo(30 * time.Minute)
	fakeProcessStarts(t, map[int]time.Time{self: selfStart, child: childStart})

	cases := []struct {
		name string
		body Info
		held bool
	}{
		{"a live other holder recording its own start is held", Info{PID: child, Start: childStart.Unix()}, true},
		{"a live other pid recording the caller's start is a reused pid", Info{PID: child, Start: selfStart.Unix()}, false},
		{"a dead holder recording the caller's own start is stale", Info{PID: reapedPID(t), Start: selfStart.Unix()}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "writer.lock")
			writeLockInfo(t, path, tc.body)

			if _, held, err := Peek(path); err != nil || held != tc.held {
				t.Fatalf("Peek = held %t, %v, want held %t, nil", held, err, tc.held)
			}
			f, err := Acquire(path)
			if tc.held {
				if err == nil {
					_ = Release(f, path)
					t.Fatal("Acquire took a live holder's lock over, want *ErrHeld")
				}
				var held *ErrHeld
				if !errors.As(err, &held) || held.Info != tc.body {
					t.Fatalf("Acquire = %v, want *ErrHeld naming %+v", err, tc.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("Acquire over a stale lock: %v, want a takeover", err)
			}
			t.Cleanup(func() { _ = Release(f, path) })
			if got := readLockBody(t, path); got != (Info{PID: self, Start: selfStart.Unix()}) {
				t.Fatalf("lock body after takeover = %+v, want {PID:%d Start:%d}", got, self, selfStart.Unix())
			}
		})
	}
}

// TestAcquire_LongRunningWriterKeepsItsLockAcrossANestedWriter is the
// in-package regression witness for BL-101's witnessed failure,
// internal/workbench's TestSpecImport_LabeledMarkdown_PreviewCorrectApplyRecord
// ("writer lock is gone after apply" once its package ran past ~5 minutes):
// a long-running writer holds its lifetime lock (its OS start simulated an
// hour before acquisition, as for a `verdi serve` or a long test package)
// and runs a nested writer transaction the way internal/draftmutation does —
// Acquire, and on ErrHeld prove ownership with a lease. The nested Acquire
// must answer ErrHeld (not a takeover), the lease must prove ownership, and
// the same lock file must survive the transaction.
func TestAcquire_LongRunningWriterKeepsItsLockAcrossANestedWriter(t *testing.T) {
	fakeProcessStarts(t, map[int]time.Time{os.Getpid(): secondsAgo(time.Hour)})
	path := filepath.Join(t.TempDir(), "writer.lock")
	outer, err := Acquire(path)
	if err != nil {
		t.Fatalf("acquiring the lifetime lock: %v", err)
	}
	t.Cleanup(func() { _ = Release(outer, path) })
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	nested, err := Acquire(path)
	if err == nil {
		_ = Release(nested, path)
		t.Fatal("the nested writer's Acquire took the long-running writer's own lock over, want *ErrHeld")
	}
	if !errors.As(err, new(*ErrHeld)) {
		t.Fatalf("nested Acquire error = %T %v, want *ErrHeld", err, err)
	}
	release, owned, err := LeaseIfHeldByCurrentProcess(path)
	if err != nil || !owned {
		t.Fatalf("LeaseIfHeldByCurrentProcess = %t, %v, want true, nil", owned, err)
	}
	release()

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("writer lock is gone after the nested transaction: %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("writer lock file identity changed across the nested transaction")
	}
}

// holderHelperEnv names the lock path a re-executed test binary acquires
// when it runs as TestHelperLockHolder's child process.
const holderHelperEnv = "VERDI_FILELOCK_TEST_HOLDER_LOCK"

// holderHelperDelay is how long the helper child waits after starting
// before it acquires: long enough that the lock's creation time lies at
// least one whole second after the child's OS start, so a lock recording
// its creation time can never equal `ps -o lstart=` for the child.
const holderHelperDelay = 1500 * time.Millisecond

// TestHelperLockHolder is not a test on its own: it is the holder child
// process startHolderChild re-executes this test binary as. In that child
// it waits holderHelperDelay, acquires the lock named by holderHelperEnv,
// reports "acquired", holds the lock until its stdin closes, then releases
// it and exits.
func TestHelperLockHolder(t *testing.T) {
	path := os.Getenv(holderHelperEnv)
	if path == "" {
		t.Skip("helper child process only; driven by startHolderChild")
	}
	time.Sleep(holderHelperDelay)
	f, err := Acquire(path)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(3)
	}
	fmt.Println("acquired")
	_, _ = io.Copy(io.Discard, os.Stdin)
	if err := Release(f, path); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(4)
	}
	os.Exit(0)
}

// startHolderChild re-executes this test binary as TestHelperLockHolder
// with env (the parent's environment when nil) plus the helper's lock
// path, waits until the child reports it acquired path, and returns the
// child's pid and a stop func. stop closes the child's stdin — the child
// then releases the lock and exits — and returns the child's exit error.
// A child whose stop was never called is killed when t ends.
func startHolderChild(t *testing.T, path string, env []string) (int, func() error) {
	t.Helper()
	if env == nil {
		env = os.Environ()
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperLockHolder$", "-test.count=1")
	cmd.Env = append(env[:len(env):len(env)], holderHelperEnv+"="+path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the holder child: %v", err)
	}

	// The reader drains the child's stdout to EOF and only then signals
	// drained, so Wait (which closes the pipe) never races a pending read.
	lines := make(chan string, 1)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if line := sc.Text(); line == "acquired" || strings.HasPrefix(line, "error: ") {
				lines <- line
				break
			}
		}
		close(lines)
		_, _ = io.Copy(io.Discard, stdout)
	}()
	stopped := false
	stop := func() error {
		stopped = true
		_ = stdin.Close()
		select {
		case <-drained:
		case <-time.After(60 * time.Second):
			_ = cmd.Process.Kill()
			<-drained
		}
		return cmd.Wait()
	}
	t.Cleanup(func() {
		if !stopped {
			_ = cmd.Process.Kill()
			_ = stop()
		}
	})

	select {
	case line, ok := <-lines:
		if !ok || line != "acquired" {
			t.Fatalf("holder child did not acquire the lock: %q", line)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("holder child never reported acquiring the lock")
	}
	return cmd.Process.Pid, stop
}

// TestAcquire_RealHolderRecordsItsOSProcessStart is the seam-free,
// cross-process witness of SI-300: a real child process acquires a lock
// holderHelperDelay after it started, and this process — a different
// prober running the real `ps -o lstart=` on the child's pid — finds the
// recorded start EQUAL to that OS process start (one source, one
// resolution), judges the child the live holder, and gets ErrHeld from its
// own Acquire. A lock recording its creation time fails the equality.
func TestAcquire_RealHolderRecordsItsOSProcessStart(t *testing.T) {
	if _, err := psLstart(os.Getpid()); err != nil {
		t.Skipf("ps -o lstart= unavailable/unparseable on this platform: %v", err)
	}
	path := filepath.Join(t.TempDir(), "writer.lock")
	child, stop := startHolderChild(t, path, nil)

	info := readLockBody(t, path)
	osStart, err := psLstart(child)
	if err != nil {
		t.Fatalf("ps -o lstart= for the holder child: %v", err)
	}
	if info.PID != child || info.Start != osStart.Unix() {
		t.Fatalf("lock body = %+v, want {PID:%d Start:%d}: the child's OS process start per ps -o lstart=, not the lock's creation time", info, child, osStart.Unix())
	}
	if _, held, err := Peek(path); err != nil || !held {
		t.Fatalf("Peek(real holder's lock) = held %t, %v, want true, nil", held, err)
	}
	taken, err := Acquire(path)
	if err == nil {
		_ = Release(taken, path)
		t.Fatal("Acquire took the real live holder's lock over, want *ErrHeld")
	}
	if !errors.As(err, new(*ErrHeld)) {
		t.Fatalf("Acquire against the real holder = %v, want *ErrHeld", err)
	}

	if err := stop(); err != nil {
		t.Fatalf("holder child exited uncleanly: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lock still present after the holder released it: %v", err)
	}
}
