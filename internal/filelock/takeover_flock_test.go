//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package filelock

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
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
			select {
			case <-gate.paused:
			case <-time.After(30 * time.Second):
				t.Fatal("the second detector never reached the stale body")
			}
			f1, err1 := Acquire(path)
			close(gate.resume)
			var r2 acquireResult
			select {
			case r2 = <-second:
			case <-time.After(30 * time.Second):
				t.Fatal("the second detector never finished")
			}
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
