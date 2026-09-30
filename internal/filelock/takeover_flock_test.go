//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package filelock

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// firstCallerGate pauses exactly one caller of a seam: the first to pass
// it signals paused and blocks until resume is closed; every later caller
// passes straight through. It lets a test hold one detector at a chosen
// point of acquire while another runs to completion.
type firstCallerGate struct {
	calls  atomic.Int32
	paused chan struct{}
	resume chan struct{}
}

func newFirstCallerGate() *firstCallerGate {
	return &firstCallerGate{paused: make(chan struct{}), resume: make(chan struct{})}
}

func (g *firstCallerGate) pass() {
	if g.calls.Add(1) != 1 {
		return
	}
	close(g.paused)
	<-g.resume
}

// waitPaused waits until the gate's first caller has paused.
func (g *firstCallerGate) waitPaused(t *testing.T) {
	t.Helper()
	select {
	case <-g.paused:
	case <-time.After(30 * time.Second):
		t.Fatal("no detector ever reached the gate")
	}
}

// awaitResult waits for one detector's Acquire outcome.
func awaitResult(t *testing.T, ch <-chan acquireResult) acquireResult {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(30 * time.Second):
		t.Fatal("a detector never finished")
	}
	return acquireResult{}
}

// acquireResult is one detector's Acquire outcome.
type acquireResult struct {
	f   *os.File
	err error
}

// assertOneIntactHolder fails t unless exactly one of results acquired
// path, every other result is *ErrHeld, and the winner's own lock file —
// the exact inode it created, holding want — is the one on disk. Every
// acquired handle is released when t ends.
func assertOneIntactHolder(t *testing.T, path string, want Info, results ...acquireResult) {
	t.Helper()
	var winner *os.File
	holders := 0
	for i, r := range results {
		if r.err == nil {
			holders++
			winner = r.f
			f := r.f
			t.Cleanup(func() { _ = Release(f, path) })
			continue
		}
		if !errors.As(r.err, new(*ErrHeld)) {
			t.Errorf("detector %d: Acquire = %v, want success or *ErrHeld", i+1, r.err)
		}
	}
	if holders != 1 {
		t.Fatalf("%d detectors acquired the lock, want exactly one (results: %+v)", holders, results)
	}
	ours, err := stillOurRegisteredFile(winner, path)
	if err != nil || !ours {
		t.Fatalf("the winner's lock is not the file on disk (same file %t, err %v): another detector replaced it", ours, err)
	}
	if got := readLockBody(t, path); got != want {
		t.Fatalf("lock body = %+v, want the winner's %+v", got, want)
	}
}

// TestAcquire_ConcurrentStaleDetectorsLeaveOneHolder is BL-108's race
// witness (SI-302): two detectors of one stale lock, the second held at a
// point where it has already read (or judged) the stale body while the
// first takes the lock over and acquires it. Before SI-302 the second then
// removed the path by name — deleting the first detector's fresh lock —
// and created its own, so both held the lock. Each row pauses the second
// detector through a seam the pre-SI-302 code already had, for one kind of
// stale body: a dead pid, a live pid whose process start disagrees with
// the recorded one (a reused pid), and an empty body older than the
// mid-flush window.
func TestAcquire_ConcurrentStaleDetectorsLeaveOneHolder(t *testing.T) {
	self := os.Getpid()
	selfStart := secondsAgo(time.Hour)
	cases := []struct {
		name  string
		setup func(t *testing.T, path string, gate *firstCallerGate)
	}{
		{"dead-pid body, second detector paused after reading it", func(t *testing.T, path string, gate *firstCallerGate) {
			fakeProcessStarts(t, map[int]time.Time{self: selfStart})
			writeLockInfo(t, path, Info{PID: reapedPID(t), Start: selfStart.Unix()})
			orig := lockReadFile
			lockReadFile = func(name string) ([]byte, error) {
				data, err := orig(name)
				gate.pass()
				return data, err
			}
			t.Cleanup(func() { lockReadFile = orig })
		}},
		{"reused-pid body, second detector paused inside its liveness probe", func(t *testing.T, path string, gate *firstCallerGate) {
			sleeper := startSleeper(t)
			sleeperStart := secondsAgo(30 * time.Minute)
			writeLockInfo(t, path, Info{PID: sleeper, Start: sleeperStart.Add(-3 * time.Hour).Unix()})
			orig := psLstart
			psLstart = func(pid int) (time.Time, error) {
				switch pid {
				case self:
					return selfStart, nil
				case sleeper:
					gate.pass()
					return sleeperStart, nil
				}
				return time.Time{}, fmt.Errorf("fake ps: no process start known for pid %d", pid)
			}
			t.Cleanup(func() { psLstart = orig })
		}},
		{"aged empty body, second detector paused after aging it", func(t *testing.T, path string, gate *firstCallerGate) {
			fakeProcessStarts(t, map[int]time.Time{self: selfStart})
			writeAgedEmptyLock(t, path)
			orig := lockStat
			lockStat = func(name string) (os.FileInfo, error) {
				st, err := orig(name)
				gate.pass()
				return st, err
			}
			t.Cleanup(func() { lockStat = orig })
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "writer.lock")
			gate := newFirstCallerGate()
			tc.setup(t, path, gate)

			second := make(chan acquireResult, 1)
			go func() {
				f, err := Acquire(path)
				second <- acquireResult{f, err}
			}()
			gate.waitPaused(t)
			f1, err1 := Acquire(path)
			close(gate.resume)
			r2 := awaitResult(t, second)
			assertOneIntactHolder(t, path, Info{PID: self, Start: selfStart.Unix()}, acquireResult{f1, err1}, r2)
		})
	}
}

// writeAgedEmptyLock writes an empty lock body at path whose mtime lies a
// minute past the mid-flush window: a writer that crashed between its
// exclusive create and its flush.
func writeAgedEmptyLock(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-lockMidFlushWindow - time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

// racerHelperEnv marks a re-executed test binary as TestHelperLockRacer's
// child process.
const racerHelperEnv = "VERDI_FILELOCK_TEST_RACER"

// raceRoundsEnv overrides how many rounds
// TestAcquire_RealProcessesRaceForAStaleLock runs (default
// defaultRaceRounds), for a longer evidence run.
const raceRoundsEnv = "VERDI_FILELOCK_TEST_RACE_ROUNDS"

const (
	defaultRaceRounds = 50
	racersPerRound    = 4
)

// TestHelperLockRacer is not a test on its own: it is the racer child
// process startRacers re-executes this test binary as. It reads its own
// process start once (the start Acquire would record; a process's start
// never changes) and prints "ready". Then, for each round: on "go <path>"
// it races for the lock at path through acquire with that start — so the
// ps exec Acquire costs does not spread the racers apart — and prints
// "acquired", "held", or "error: …"; on "check" it prints whether the lock
// file on disk is still the exact file it acquired ("intact", "replaced",
// or "none" when it acquired nothing); on "release" it releases what it
// holds and prints "released". It exits when its stdin closes.
func TestHelperLockRacer(t *testing.T) {
	if os.Getenv(racerHelperEnv) == "" {
		t.Skip("helper child process only; driven by startRacers")
	}
	start := ownProcessStart()
	in := bufio.NewScanner(os.Stdin)
	fmt.Println("ready")
	var path string
	var f *os.File
	for in.Scan() {
		cmd, arg, _ := strings.Cut(in.Text(), " ")
		switch cmd {
		case "go":
			path = arg
			var err error
			f, err = acquire(path, 5, start)
			switch {
			case err == nil:
				fmt.Println("acquired")
			case errors.As(err, new(*ErrHeld)):
				fmt.Println("held")
			default:
				fmt.Printf("error: %q\n", err.Error())
			}
		case "check":
			if f == nil {
				fmt.Println("none")
			} else if ours, err := stillOurRegisteredFile(f, path); err != nil {
				fmt.Printf("error: %q\n", err.Error())
			} else if ours {
				fmt.Println("intact")
			} else {
				fmt.Println("replaced")
			}
		case "release":
			if f != nil {
				if err := Release(f, path); err != nil {
					fmt.Printf("error: %q\n", err.Error())
					continue
				}
				f = nil
			}
			fmt.Println("released")
		default:
			fmt.Printf("error: unknown command %q\n", in.Text())
		}
	}
	os.Exit(0)
}

// racer is one TestHelperLockRacer child: its stdin, and its stdout lines.
type racer struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	lines chan string
}

// startRacers re-executes this test binary as n TestHelperLockRacer
// children at once and waits for each to report ready. Every racer is
// stopped when t ends.
func startRacers(t *testing.T, n int) []*racer {
	t.Helper()
	racers := make([]*racer, n)
	for i := range racers {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHelperLockRacer$", "-test.count=1")
		cmd.Env = append(os.Environ(), racerHelperEnv+"=1")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatalf("starting a racer child: %v", err)
		}
		r := &racer{cmd: cmd, stdin: stdin, lines: make(chan string, 8)}
		go func() {
			defer close(r.lines)
			sc := bufio.NewScanner(stdout)
			for sc.Scan() {
				r.lines <- sc.Text()
			}
		}()
		t.Cleanup(func() {
			if err := r.stop(); err != nil {
				t.Errorf("racer child exited uncleanly: %v", err)
			}
		})
		racers[i] = r
	}
	for _, r := range racers {
		if line := r.next(t); line != "ready" {
			t.Fatalf("racer child's first line = %q, want ready", line)
		}
	}
	return racers
}

// next returns the racer's next stdout line.
func (r *racer) next(t *testing.T) string {
	t.Helper()
	select {
	case line, ok := <-r.lines:
		if !ok {
			t.Fatal("racer child exited early")
		}
		return line
	case <-time.After(60 * time.Second):
		t.Fatal("racer child never answered")
	}
	return ""
}

// send writes one line to the racer's stdin.
func (r *racer) send(t *testing.T, line string) {
	t.Helper()
	if _, err := io.WriteString(r.stdin, line+"\n"); err != nil {
		t.Fatalf("writing %q to a racer child: %v", line, err)
	}
}

// stop closes the racer's stdin — it then exits — drains its stdout to EOF,
// and returns its exit error; past a minute it kills the racer.
func (r *racer) stop() error {
	_ = r.stdin.Close()
	timeout := time.After(60 * time.Second)
	for {
		select {
		case _, ok := <-r.lines:
			if ok {
				continue
			}
			return r.cmd.Wait()
		case <-timeout:
			_ = r.cmd.Process.Kill()
			timeout = nil
		}
	}
}

// each sends line to every racer and returns their answers, in order.
func each(t *testing.T, racers []*racer, line string) []string {
	t.Helper()
	for _, r := range racers {
		r.send(t, line)
	}
	answers := make([]string, len(racers))
	for i, r := range racers {
		answers[i] = r.next(t)
	}
	return answers
}

// TestAcquire_RealProcessesRaceForAStaleLock is SI-302's seam-free,
// cross-process witness: racersPerRound real child processes race, round
// after round, for a fresh lock naming a dead pid, released together at the
// lock. While every racer still holds whatever it got, the holders are
// counted: exactly one may acquire, every other must get ErrHeld, and the
// winner's own lock file must still be the one on disk, naming its pid.
func TestAcquire_RealProcessesRaceForAStaleLock(t *testing.T) {
	rounds := defaultRaceRounds
	if s := os.Getenv(raceRoundsEnv); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			t.Fatalf("%s=%q: want a positive integer", raceRoundsEnv, s)
		}
		rounds = n
	}
	racers := startRacers(t, racersPerRound)
	dir := t.TempDir()
	for round := 0; round < rounds; round++ {
		path := filepath.Join(dir, fmt.Sprintf("writer-%d.lock", round))
		writeLockInfo(t, path, Info{PID: reapedPID(t), Start: time.Now().Add(-time.Hour).Unix()})
		outcomes := each(t, racers, "go "+path)
		checks := each(t, racers, "check")
		onDisk := readLockBody(t, path)
		released := each(t, racers, "release")

		holders := 0
		for i, r := range racers {
			switch outcomes[i] {
			case "acquired":
				holders++
				if checks[i] != "intact" {
					t.Errorf("round %d racer %d acquired, but its lock on disk is %q, want intact", round, i, checks[i])
				}
				if onDisk.PID != r.cmd.Process.Pid {
					t.Errorf("round %d: lock body names pid %d, want the winner's %d", round, onDisk.PID, r.cmd.Process.Pid)
				}
			case "held":
			default:
				t.Errorf("round %d racer %d: %s, want acquired or held", round, i, outcomes[i])
			}
			if released[i] != "released" {
				t.Errorf("round %d racer %d release: %s", round, i, released[i])
			}
		}
		if holders != 1 {
			t.Fatalf("round %d: %d real processes hold the lock at once (outcomes %v, checks %v), want exactly one", round, holders, outcomes, checks)
		}
	}
	t.Logf("%d rounds of %d real racer processes over a dead-pid lock: exactly one holder every round", rounds, racersPerRound)
}

// stopAt makes takeoverHook pass every detector reaching step through
// gate, restoring the hook when t ends.
func stopAt(t *testing.T, step takeoverStep, gate *firstCallerGate) {
	t.Helper()
	orig := takeoverHook
	takeoverHook = func(s takeoverStep) {
		if s == step {
			gate.pass()
		}
	}
	t.Cleanup(func() { takeoverHook = orig })
}

// staleBodies are the two kinds of stale lock SI-302's takeover protects:
// a dead holder's body and an empty body older than the mid-flush window.
var staleBodies = []struct {
	name string
	seed func(t *testing.T, path string)
}{
	{"dead-pid body", func(t *testing.T, path string) {
		writeLockInfo(t, path, Info{PID: reapedPID(t), Start: secondsAgo(2 * time.Hour).Unix()})
	}},
	{"aged empty body", writeAgedEmptyLock},
}

// TestTakeover_TwoDetectorsOfOneJudgedFile interleaves two detectors that
// have both judged the same stale file through their own handles (SI-302),
// for each kind of stale body. When the second reaches the flock only
// after the first has unlinked the file and created its own lock, the
// re-check under the flock finds the path naming another file and the
// second re-evaluates, reading the winner's live lock. When the second
// reaches the flock while the first still holds it, EWOULDBLOCK answers
// held. Either way exactly one holds the lock and its file is intact.
func TestTakeover_TwoDetectorsOfOneJudgedFile(t *testing.T) {
	self := os.Getpid()
	selfStart := secondsAgo(time.Hour)
	winner := Info{PID: self, Start: selfStart.Unix()}
	for _, body := range staleBodies {
		t.Run(body.name+": the second takes the flock after the first finished", func(t *testing.T) {
			fakeProcessStarts(t, map[int]time.Time{self: selfStart})
			path := filepath.Join(t.TempDir(), "writer.lock")
			body.seed(t, path)
			gate := newFirstCallerGate()
			stopAt(t, takeoverJudged, gate)

			second := make(chan acquireResult, 1)
			go func() {
				f, err := Acquire(path)
				second <- acquireResult{f, err}
			}()
			gate.waitPaused(t)
			f1, err1 := Acquire(path)
			close(gate.resume)
			r2 := awaitResult(t, second)

			var held *ErrHeld
			if !errors.As(r2.err, &held) || held.Info != winner {
				t.Fatalf("second detector = %v, want *ErrHeld naming the winner %+v", r2.err, winner)
			}
			assertOneIntactHolder(t, path, winner, acquireResult{f1, err1}, r2)
		})
		t.Run(body.name+": the second reaches the flock while the first holds it", func(t *testing.T) {
			fakeProcessStarts(t, map[int]time.Time{self: selfStart})
			path := filepath.Join(t.TempDir(), "writer.lock")
			body.seed(t, path)
			gate := newFirstCallerGate()
			stopAt(t, takeoverRechecked, gate)

			first := make(chan acquireResult, 1)
			go func() {
				f, err := Acquire(path)
				first <- acquireResult{f, err}
			}()
			gate.waitPaused(t)
			f2, err2 := Acquire(path)
			close(gate.resume)
			r1 := awaitResult(t, first)

			var held *ErrHeld
			if !errors.As(err2, &held) || held.Info != (Info{}) {
				t.Fatalf("second detector = %v, want *ErrHeld with a zero Info (another detector holds the takeover flock)", err2)
			}
			assertOneIntactHolder(t, path, winner, r1, acquireResult{f2, err2})
		})
	}
}

// TestTakeover_ReChecksUnderTheFlock changes the lock at the one point
// between a detector judging it stale through its own handle and taking
// the flock (takeoverJudged), one row per path through the takeover:
// another detector already holding the flock (EWOULDBLOCK), the path now
// naming another file (the inode re-check), the body rewritten in place
// (the body re-check — including a creator's late flush into an aged empty
// lock), an aged empty lock made young (the re-judgement), the lock
// vanishing, and nothing changing. Only the last two take the lock; every
// other row leaves the lock found at the path exactly as it was.
func TestTakeover_ReChecksUnderTheFlock(t *testing.T) {
	self := os.Getpid()
	selfStart := secondsAgo(time.Hour)
	live := Info{PID: self, Start: selfStart.Unix()}
	writeLive := func(t *testing.T, path string) { writeLockInfo(t, path, live) }
	cases := []struct {
		name     string
		seed     func(t *testing.T, path string)
		atJudged func(t *testing.T, path string)
		// held is the ErrHeld.Info the detector must answer; nil means it
		// must acquire the lock instead.
		held *Info
		// sameFile: the file found at the path afterwards is the seeded one
		// (true) or the one atJudged put there (false); unused on acquire.
		sameFile bool
	}{
		{"another detector holds the takeover flock", staleBodies[0].seed, func(t *testing.T, path string) {
			other, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = other.Close() })
			if err := lockFlock(other); err != nil {
				t.Fatalf("taking the other detector's flock: %v", err)
			}
		}, &Info{}, true},
		{"the path now names a live holder's lock", staleBodies[0].seed, func(t *testing.T, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			writeLive(t, path)
		}, &live, false},
		{"the body was rewritten in place with a live holder's", staleBodies[0].seed, writeLive, &live, true},
		{"the creator flushed its body late into the aged empty lock", staleBodies[1].seed, writeLive, &live, true},
		{"the aged empty lock was made young", staleBodies[1].seed, func(t *testing.T, path string) {
			now := time.Now()
			if err := os.Chtimes(path, now, now); err != nil {
				t.Fatal(err)
			}
		}, &Info{}, true},
		{"the lock vanished", staleBodies[0].seed, func(t *testing.T, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}, nil, false},
		{"nothing changed", staleBodies[0].seed, func(*testing.T, string) {}, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeProcessStarts(t, map[int]time.Time{self: selfStart})
			path := filepath.Join(t.TempDir(), "writer.lock")
			tc.seed(t, path)
			seeded, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			var found os.FileInfo
			orig := takeoverHook
			takeoverHook = func(s takeoverStep) {
				if s != takeoverJudged || found != nil {
					return
				}
				tc.atJudged(t, path)
				found, _ = os.Lstat(path)
				if found == nil {
					found = seeded // the lock vanished; nothing to compare
				}
			}
			t.Cleanup(func() { takeoverHook = orig })

			f, err := Acquire(path)
			if found == nil {
				t.Fatal("the detector never judged the lock stale through its own handle")
			}
			if tc.held == nil {
				if err != nil {
					t.Fatalf("Acquire = %v, want the lock taken over", err)
				}
				t.Cleanup(func() { _ = Release(f, path) })
				if got := readLockBody(t, path); got != live {
					t.Fatalf("lock body after takeover = %+v, want ours %+v", got, live)
				}
				return
			}
			if err == nil {
				_ = Release(f, path)
				t.Fatal("Acquire took the lock over, want *ErrHeld")
			}
			var held *ErrHeld
			if !errors.As(err, &held) || held.Info != *tc.held {
				t.Fatalf("Acquire = %v, want *ErrHeld naming %+v", err, *tc.held)
			}
			after, err := os.Lstat(path)
			if err != nil {
				t.Fatalf("the lock is gone: %v", err)
			}
			want := found
			if tc.sameFile {
				want = seeded
			}
			if !os.SameFile(after, want) {
				t.Fatal("the lock found at the path was replaced")
			}
		})
	}
}

// TestTakeOverStale_Direct drives takeOverStale itself over each lock it can
// find, including its operational failures: it removes only a stale lock,
// answers nil (re-evaluate) for every other, and reports an error — having
// removed nothing — when it cannot open, flock, or unlink the lock.
func TestTakeOverStale_Direct(t *testing.T) {
	self := os.Getpid()
	selfStart := secondsAgo(time.Hour)
	notRoot := func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root: permission bits do not restrict access")
		}
	}
	cases := []struct {
		name    string
		seed    func(t *testing.T, path string)
		wantErr string // "" for nil
		removed bool
	}{
		{"no lock at the path", func(*testing.T, string) {}, "", true},
		{"a live holder's lock", func(t *testing.T, path string) {
			writeLockInfo(t, path, Info{PID: self, Start: selfStart.Unix()})
		}, "", false},
		{"a live holder's lock another handle has flocked", func(t *testing.T, path string) {
			writeLockInfo(t, path, Info{PID: self, Start: selfStart.Unix()})
			other, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = other.Close() })
			if err := lockFlock(other); err != nil {
				t.Fatal(err)
			}
		}, "", false},
		{"a garbled body", func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "", false},
		{"a young empty body", func(t *testing.T, path string) {
			if err := os.WriteFile(path, nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}, "", false},
		{"a dead holder's lock", staleBodies[0].seed, "", true},
		{"an aged empty body", staleBodies[1].seed, "", true},
		{"the lock cannot be opened", func(t *testing.T, path string) {
			notRoot(t)
			staleBodies[0].seed(t, path)
			if err := os.Chmod(path, 0o000); err != nil {
				t.Fatal(err)
			}
		}, "opening it", false},
		{"the flock fails with another error", func(t *testing.T, path string) {
			staleBodies[0].seed(t, path)
			orig := lockFlock
			lockFlock = func(*os.File) error { return syscall.ENOLCK }
			t.Cleanup(func() { lockFlock = orig })
		}, "taking its takeover flock", false},
		{"the unlink fails", func(t *testing.T, path string) {
			notRoot(t)
			staleBodies[0].seed(t, path)
			dir := filepath.Dir(path)
			orig := takeoverHook
			takeoverHook = func(s takeoverStep) {
				if s == takeoverRechecked {
					if err := os.Chmod(dir, 0o555); err != nil {
						t.Error(err)
					}
				}
			}
			t.Cleanup(func() {
				takeoverHook = orig
				_ = os.Chmod(dir, 0o755)
			})
		}, "unlinking it", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeProcessStarts(t, map[int]time.Time{self: selfStart})
			path := filepath.Join(t.TempDir(), "writer.lock")
			tc.seed(t, path)

			err := takeOverStale(path)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("takeOverStale = %v, want nil", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("takeOverStale = %v, want an error naming %q", err, tc.wantErr)
			}
			_ = os.Chmod(filepath.Dir(path), 0o755)
			_, statErr := os.Lstat(path)
			if gone := errors.Is(statErr, os.ErrNotExist); gone != tc.removed {
				t.Fatalf("lock removed = %t (%v), want %t", gone, statErr, tc.removed)
			}
		})
	}
}

// TestAcquire_TakeoverFailureIsOperational pins acquire's answer when the
// takeover itself fails for a reason other than another detector holding
// its flock: an operational error naming the stale lock — never *ErrHeld,
// never a takeover — with the lock left in place.
func TestAcquire_TakeoverFailureIsOperational(t *testing.T) {
	for _, body := range staleBodies {
		t.Run(body.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "writer.lock")
			body.seed(t, path)
			orig := lockFlock
			lockFlock = func(*os.File) error { return syscall.ENOLCK }
			t.Cleanup(func() { lockFlock = orig })

			f, err := Acquire(path)
			if err == nil {
				_ = Release(f, path)
				t.Fatal("Acquire took the lock over although its flock failed")
			}
			if errors.As(err, new(*ErrHeld)) || !errors.Is(err, syscall.ENOLCK) || !strings.Contains(err.Error(), "could not take it over") || !strings.Contains(err.Error(), path) {
				t.Fatalf("Acquire = %v, want an operational error naming %s and wrapping the flock failure", err, path)
			}
			if _, err := os.Lstat(path); err != nil {
				t.Fatalf("the stale lock is gone after a failed takeover: %v", err)
			}
		})
	}
}

// TestBodyStale pins the staleness rule takeOverStale judges a body by — the
// same rule acquire applies: a complete body by its holder's liveness (an
// undecided probe is live), an empty or truncated one by its age, and a
// garbled one never.
func TestBodyStale(t *testing.T) {
	self := os.Getpid()
	selfStart := secondsAgo(time.Hour)
	sleeper := startSleeper(t)
	fakeProcessStarts(t, map[int]time.Time{self: selfStart})
	young := time.Now()
	aged := time.Now().Add(-lockMidFlushWindow - time.Second)
	body := func(info Info) []byte {
		data, err := json.Marshal(info)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	cases := []struct {
		name    string
		body    []byte
		modTime time.Time
		want    bool
	}{
		{"live holder", body(Info{PID: self, Start: selfStart.Unix()}), aged, false},
		{"dead holder", body(Info{PID: reapedPID(t), Start: selfStart.Unix()}), young, true},
		{"reused pid", body(Info{PID: self, Start: selfStart.Add(-time.Hour).Unix()}), young, true},
		{"live pid whose start cannot be read (undecided)", body(Info{PID: sleeper, Start: selfStart.Unix()}), aged, false},
		{"young empty body", nil, young, false},
		{"aged empty body", nil, aged, true},
		{"young truncated body", []byte(`{"pid":`), young, false},
		{"aged truncated body", []byte(`{"pid":`), aged, true},
		{"aged garbled body", []byte("not json"), aged, false},
		{"aged body with an unknown field", []byte(`{"pid":1,"start":2,"x":3}`), aged, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bodyStale(tc.body, tc.modTime); got != tc.want {
				t.Fatalf("bodyStale(%q) = %t, want %t", tc.body, got, tc.want)
			}
		})
	}
}

// TestTakeoverHandleHelpers covers the helpers takeOverStale reads its open
// handle through: readOpenLock reads the whole body from offset 0 whatever
// the handle's own offset; lockFlock takes the flock once and answers
// EWOULDBLOCK to a second handle on the same file; and every helper reports
// an error on a closed handle rather than a guessed answer.
func TestTakeoverHandleHelpers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	writeLockInfo(t, path, Info{PID: 1, Start: 2})
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	open := func(t *testing.T) *os.File {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}

	t.Run("readOpenLock reads from offset 0", func(t *testing.T) {
		f := open(t)
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			t.Fatal(err)
		}
		got, err := readOpenLock(f)
		if err != nil || string(got) != string(want) {
			t.Fatalf("readOpenLock = %q, %v, want %q", got, err, want)
		}
	})
	t.Run("lockFlock excludes a second handle", func(t *testing.T) {
		first, second := open(t), open(t)
		if err := lockFlock(first); err != nil {
			t.Fatalf("first lockFlock: %v", err)
		}
		if err := lockFlock(second); !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatalf("second lockFlock = %v, want EWOULDBLOCK", err)
		}
		if err := first.Close(); err != nil {
			t.Fatal(err)
		}
		if err := lockFlock(second); err != nil {
			t.Fatalf("lockFlock after the first handle closed: %v", err)
		}
	})
	t.Run("a closed handle is an error everywhere", func(t *testing.T) {
		f := open(t)
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := readOpenLock(f); err == nil {
			t.Error("readOpenLock(closed) = nil error")
		}
		if _, _, err := judgeOpenLock(f); err == nil {
			t.Error("judgeOpenLock(closed) = nil error")
		}
		if _, err := stillStaleUnderFlock(f, path, want); err == nil {
			t.Error("stillStaleUnderFlock(closed) = nil error")
		}
		if err := lockFlock(f); err == nil {
			t.Error("lockFlock(closed) = nil error")
		}
	})
}
