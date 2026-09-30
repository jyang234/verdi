// Package filelock implements I-12's per-checkout writer-lock algorithm as
// a shared primitive (CLAUDE.md: "anything used by two or more packages
// lives in a shared internal/ package"). It was born as
// internal/mcpserve/lock.go, guarding exactly one `verdi serve`/`verdi mcp`
// process per checkout's data/writer.lock; spec/worktree-manager dc-2
// widens its packaging — not its algorithm — so a second caller
// (internal/wtmanager, guarding one managed-worktree lockfile per design
// branch) can use the EXACT SAME O_CREATE|O_EXCL {pid,start} JSON body,
// kill(pid,0)-plus-`ps -o lstart=` liveness cross-check, and stale-lock
// takeover without copy-pasting a second implementation. Base algorithm is
// the wave-4 S4 spike's proven design (read-only reference, reimplemented
// here); the PID-reuse gap closure (ps -o lstart= cross-check, with a
// documented kill-probe-only fallback) is this package's own addition, per
// PLAN.md Phase 9 exit criteria. SI-177 (Wave 6 design §6.1.2) is this
// package's second own addition: a synchronized process-local ownership
// registry recording exact successful Acquire handles, and the read-only
// HeldByCurrentProcess query over it. Acquire itself stays non-reentrant —
// a live existing lock always returns ErrHeld, even to its own owning
// process — the registry only lets a caller PROVE after the fact that an
// ErrHeld it just received actually names this process's own still-open
// lock, so it can make its own reuse decision instead of failing outright.
package filelock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
)

// Info is the lock body: O_CREATE|O_EXCL JSON {pid, start}.
type Info struct {
	PID int `json:"pid"`
	// Start is the unix seconds the holder process started, per
	// `ps -o lstart=` read in UTC (SI-300, ownProcessStart); locks written
	// before SI-300, and locks whose holder could not read its own start
	// at acquisition, carry their creation time instead.
	Start int64 `json:"start"`
}

// ErrHeld means the lock is held by a live process: the caller should
// proxy (dial the socket) or reuse the winner's result rather than
// proceeding as if it owned the resource. Info is zero when no holder has
// recorded itself in the lock yet: a young empty or partial body (its
// creator is mid-flush), or a stale lock another detector holds the
// takeover flock on (SI-302, takeOverStale).
type ErrHeld struct {
	Info Info
}

// Error names the holder's pid and the start its lock body records — the
// holder's process start for an SI-300 lock, the creation time for an
// older or fallback one (Info.Start), so it says "recorded start". For a
// zero Info it says the holder has not recorded itself yet, rather than
// naming pid 0 and the Unix epoch.
func (e *ErrHeld) Error() string {
	if e.Info == (Info{}) {
		return "filelock: lock held, but its holder has not recorded itself in it yet (the lock is being created or taken over)"
	}
	return fmt.Sprintf("filelock: lock held by live pid %d (recorded start %s)", e.Info.PID, time.Unix(e.Info.Start, 0).Format(time.RFC3339))
}

// strictUnmarshal decodes raw into dst with DisallowUnknownFields and
// trailing-data rejection, delegating to internal/artifact's own
// DecodeStrictJSON rather than reimplementing the same json.NewDecoder
// posture a second time. A lock file is one this package itself writes
// (Acquire, below), so an unexpected extra field is never a forward-compat
// signal to tolerate — it means a malformed or foreign lock file, and the
// read refuses it BY NAME rather than silently dropping the field
// (mirrors spec/fail-loud's strict-decode posture for verdi-owned files).
func strictUnmarshal(raw []byte, dst any) error {
	return artifact.DecodeStrictJSON(raw, dst)
}

// lockStartTolerance bounds how far a live pid's actual process start time
// (per `ps -o lstart=`) may drift from the lock's recorded start before
// it is treated as a DIFFERENT process that happens to have reused the
// pid, rather than the lock's genuine holder. Since SI-300 the recorded
// start is the holder's own `ps -o lstart=` reading (ownProcessStart),
// taken in the probe's zone and locale whatever the two processes' own
// environments (psEnvOverride), so a genuine SI-300 holder agrees with the
// probe however long it ran before acquiring: both are ps's whole-second
// reading of one kernel record of that process's start. (A platform that
// derives lstart from its boot time, as Linux does, moves every reading
// when the wall clock is stepped between the two; the tolerance absorbs
// such a step up to its bound, and a larger one is the open exposure
// BL-109 records.) The tolerance keeps its pre-SI-300 value for the locks
// that still record their creation time — those written by an older
// binary, and those whose holder could not read its own start
// (ownProcessStart's fallback) — which are judged by the same rule as
// before: live only if created within this long of their holder's process
// start (a start now read alike in every environment, so a prober in a
// skewed zone or locale no longer misreads them). A true pid-reuse
// collision is expected to differ by much more than this in practice (a
// different, unrelated process started at an unrelated time).
const lockStartTolerance = 5 * time.Minute

// ownProcessStart is the start Acquire records (SI-300, 01 §D3): this
// process's own OS start time, read through psLstart on os.Getpid() — the
// same source, environment, parsing, and whole-second resolution probe
// reads a holder's start through — so both sides of the liveness
// comparison read one clock whatever TZ or locale either process runs
// under (psLstart's environment override). It is read afresh on every
// call, never cached, so it always reflects the current psLstart (test
// seams included). The process start itself cannot change, but its
// reading can: a platform that derives lstart from its boot time, as
// Linux does, moves every later reading when the wall clock is stepped,
// exactly as lockStartTolerance describes — the tolerance absorbs a step
// up to its bound, and a larger one is the open exposure BL-109 records.
//
// Disclosed fallback: when the process's own start cannot be read (ps
// unavailable, failing, unparseable, or not answering within psTimeout),
// it returns the current time — the lock's acquisition time, exactly the
// value recorded before SI-300 — and acquisition proceeds. Such a lock is
// judged like a pre-SI-300 one: a prober whose ps also fails keeps it live
// through probe's kill-probe-only fallback, but a prober whose ps works
// judges it by the tolerance, so a holder that acquired more than
// lockStartTolerance after it started stays exposed to BL-101's stale
// misjudgment in this fallback case only.
func ownProcessStart() int64 {
	if st, err := psLstart(os.Getpid()); err == nil {
		return st.Unix()
	}
	return time.Now().Unix()
}

// psLstart reads the named process's actual start time from
// `ps -o lstart= -p <pid>` — the cross-check I-12 asks for — bounded by
// psTimeout (readLstart has the environment). Overridable in tests (both
// to avoid a real ps dependency in some paths and to exercise the "ps
// output unparseable" fallback deterministically).
var psLstart = func(pid int) (time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), psTimeout)
	defer cancel()
	return readLstart(ctx, pid)
}

// psEnvOverride is appended to every `ps -o lstart=` exec's environment,
// where it wins over the inherited values (exec.Cmd keeps the last value
// of a repeated key): TZ=UTC0 makes ps print the start in UTC whatever
// zone this process runs in — including a POSIX TZ rule string such as
// JST-9, which ps honours but Go's time.Local does not — and LC_ALL=C
// makes it print the English day and month names lstartLayouts parse.
// parseLstart reads the output in time.UTC to match, so a recording holder
// and a probing process read one clock whatever their own environments
// (SI-300 as amended, FL-R1).
func psEnvOverride() []string { return []string{"TZ=UTC0", "LC_ALL=C"} }

// lstartCommand builds the `ps -o lstart= -p <pid>` exec readLstart runs
// under ctx. A var only so a test can substitute a command that checks its
// environment or does not answer (the psTimeout bound).
var lstartCommand = func(ctx context.Context, pid int) *exec.Cmd {
	return exec.CommandContext(ctx, "ps", "-o", "lstart=", "-p", strconv.Itoa(pid))
}

// psTimeout bounds one `ps -o lstart=` exec: past it the exec is killed and
// the read fails, which is ownProcessStart's acquisition-time fallback on
// the recording side and probe's undecided (kill-probe-only) answer on the
// probing side. psWaitDelay then bounds how long the exec's output pipe may
// stay open after the kill (a ps wrapper whose own child still holds it),
// so a read returns within psTimeout+psWaitDelay once the killed process
// has exited. Vars only so tests can shrink them.
var (
	psTimeout   = 5 * time.Second
	psWaitDelay = time.Second
)

// readLstart runs lstartCommand for pid under ctx with psEnvOverride and
// parses its stdout as the process's start. A read cut off by ctx says so,
// naming the bound it did not answer within.
func readLstart(ctx context.Context, pid int) (time.Time, error) {
	cmd := lstartCommand(ctx, pid)
	cmd.Env = append(os.Environ(), psEnvOverride()...)
	cmd.WaitDelay = psWaitDelay
	out, err := cmd.Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return time.Time{}, fmt.Errorf("filelock: ps -o lstart= -p %d: no answer within %s: %w", pid, psTimeout, ctxErr)
		}
		return time.Time{}, fmt.Errorf("filelock: ps -o lstart= -p %d: %w", pid, err)
	}
	return parseLstart(strings.TrimSpace(string(out)))
}

// lstartLayouts are the reference-time layouts `ps -o lstart=` is known to
// emit (BSD/macOS and GNU/Linux both print "Www Mmm [ ]d HH:MM:SS YYYY" in
// the C/POSIX locale; "_2" absorbs the space-padded single-digit day both
// platforms use).
var lstartLayouts = []string{
	"Mon Jan _2 15:04:05 2006",
	"Mon Jan 2 15:04:05 2006",
}

// parseLstart reads s, ps's lstart output under psEnvOverride, as a UTC
// time — never time.Local, which differs between processes (and ignores a
// POSIX TZ rule string ps honours).
func parseLstart(s string) (time.Time, error) {
	for _, layout := range lstartLayouts {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("filelock: unparseable ps -o lstart= output %q", s)
}

// alive reports whether pid names a live process that is plausibly the
// SAME process that wrote recordedStart, closing S4's documented
// PID-reuse gap. It delegates to probe and keeps probe's documented
// kill-probe-only fallback: when probe cannot decide (the ps cross-check
// failed, timed out, or its output was unparseable), alive reports true
// rather than guessing stale (the narrow, disclosed limitation S4 and
// PLAN.md's ledger both name). Inspect (inspect.go, R-RR3-6) calls probe directly
// so it can report that same undecided case as LockUndecidable instead —
// this function's own contract, and Peek's built on it, are unchanged.
func alive(pid int, recordedStart int64) bool {
	isAlive, decided, _ := probe(pid, recordedStart)
	if !decided {
		return true // documented fallback: kill-probe-only
	}
	return isAlive
}

// probe is alive's split-out decision core (R-RR3-6): a classic
// kill(pid,0) liveness probe (no signal delivered, existence/permission
// only) first, then, for a live pid, a cross-check of its actual start
// time against recordedStart within lockStartTolerance. decided is false
// in exactly one case — the ps cross-check itself could not be completed
// (its output could not be obtained within psTimeout, or not parsed) —
// with reason carrying the ps error text; every other outcome (pid
// absent/not signalable, pid alive and start time within tolerance, pid
// alive but start time drifted far enough that a different process must
// have reused it) is decided, with reason set only for the not-alive
// decided cases (empty for a decided-alive result).
func probe(pid int, recordedStart int64) (isAlive, decided bool, reason string) {
	if pid <= 0 {
		return false, true, fmt.Sprintf("pid %d is not a valid process id", pid)
	}
	proc, err := os.FindProcess(pid) // always succeeds on Unix; not the real check
	if err != nil {
		return false, true, fmt.Sprintf("pid %d: %v", pid, err)
	}
	sigErr := proc.Signal(syscall.Signal(0))
	switch {
	case sigErr == nil:
		// Exists; fall through to the start-time cross-check.
	case errors.Is(sigErr, os.ErrProcessDone):
		return false, true, fmt.Sprintf("pid %d is not alive (process already reaped)", pid)
	case errors.Is(sigErr, syscall.ESRCH):
		return false, true, fmt.Sprintf("pid %d is not alive (no such process)", pid)
	case errors.Is(sigErr, syscall.EPERM):
		// Exists, we just can't signal it — still alive. ps may also be
		// permission-restricted for this pid; the fallback below covers it.
	default:
		return false, true, fmt.Sprintf("pid %d: %v", pid, sigErr)
	}

	actual, perr := psLstart(pid)
	if perr != nil {
		return true, false, perr.Error()
	}
	diff := actual.Unix() - recordedStart
	if diff < 0 {
		diff = -diff
	}
	if time.Duration(diff)*time.Second <= lockStartTolerance {
		return true, true, ""
	}
	return false, true, fmt.Sprintf("pid %d start time drifted %s from the lock's recorded start: a different process reused this pid", pid, time.Duration(diff)*time.Second)
}

// registryMu/registry are SI-177's synchronized process-local ownership
// registry: on every successful Acquire, the exact *os.File handle this
// process was just granted is recorded here, keyed by the exact path
// string Acquire was called with. HeldByCurrentProcess and
// LeaseIfHeldByCurrentProcess (below) are the only readers; Release is the
// only remover. Neither the lock body's PID text nor liveness is ever
// consulted here — registration begins only at a genuine successful
// O_CREATE|O_EXCL and ends only when the exact registered handle is
// released, so a caller-authored or foreign lock file naming this
// process's own pid/start bytes is never proof of ownership by itself
// (the "forged-lock rule": matching bytes without a registry entry is not
// held by us).
//
// Each entry also carries a LEASE COUNT: the number of in-flight
// operations that have proven ownership of this exact handle and are
// still running under it (design §6.1.2's continuous exclusion, as
// corrected in Codex review round 1). Ownership proof alone is a
// point-in-time fact; a lease turns it into custody for the length of the
// work, because Release BLOCKS while any lease on that exact handle is
// outstanding. registryCond (over registryMu) is how a draining Release
// waits — a synchronized wait, never a poll.
var (
	registryMu   sync.Mutex
	registryCond = sync.NewCond(&registryMu)
	registry     = map[string]*registryEntry{}
)

// registryEntry is one registered acquisition: the exact granted handle
// plus the number of outstanding leases against it. Both fields are
// guarded by registryMu.
type registryEntry struct {
	file   *os.File
	leases int
}

func registerAcquired(path string, f *os.File) {
	key := filepath.Clean(path)
	registryMu.Lock()
	registry[key] = &registryEntry{file: f}
	registryMu.Unlock()
}

// deregisterAcquired removes path's registry entry only if it still names
// exactly f — the handle THIS release call owns — so a mismatched or
// stale Release call can never evict another, still-valid registration.
// It BLOCKS while that entry has outstanding leases, waking on
// registryCond each time a lease drains and re-reading the registry (the
// entry may have been deregistered, or the path re-registered under a
// different handle, while this call waited — either case means the entry
// is no longer ours to remove and the wait ends).
func deregisterAcquired(path string, f *os.File) {
	key := filepath.Clean(path)
	registryMu.Lock()
	defer registryMu.Unlock()
	for {
		entry, ok := registry[key]
		if !ok || entry.file != f {
			return
		}
		if entry.leases == 0 {
			delete(registry, key)
			return
		}
		registryCond.Wait()
	}
}

// releaseLease drops one lease taken by LeaseIfHeldByCurrentProcess and
// wakes any Release draining that entry. It operates on the entry pointer
// the lease was taken against, so it stays correct even if the path has
// meanwhile been deregistered or re-registered under a different handle.
// The zero guard is defence in depth only: each granted lease is returned
// wrapped in a sync.Once, so a repeated release never reaches here and the
// count cannot go negative.
func releaseLease(entry *registryEntry) {
	registryMu.Lock()
	if entry.leases > 0 {
		entry.leases--
	}
	registryMu.Unlock()
	registryCond.Broadcast()
}

// HeldByCurrentProcess is SI-177's read-only ownership query. It reports
// true only when path resolves to the exact still-open file identity this
// process's own successful Acquire registered — never from PID/start text,
// liveness, or a caller-authored lock body alone. A replaced lock (the
// path now names a different file, even one with identical bytes), a
// removed/recreated lock, a symlinked path, or a path this process never
// itself Acquired (no registry entry at all — the forged-lock rule) all
// report false. A path that cannot be inspected right now (a transient
// stat failure distinct from the file simply being gone) is reported as an
// error rather than a guessed answer, so a caller treats it as unproven
// rather than as proven ownership.
func HeldByCurrentProcess(path string) (bool, error) {
	key := filepath.Clean(path)
	registryMu.Lock()
	entry, ok := registry[key]
	registryMu.Unlock()
	if !ok {
		return false, nil
	}
	return stillOurRegisteredFile(entry.file, path)
}

// stillOurRegisteredFile is the identity half both ownership queries
// share: does path right now name the exact still-open file this process
// registered? os.SameFile compares device+inode; a symlink, a
// removed-and-recreated file, or any other replacement at path never
// shares the open handle's identity, so this single comparison covers
// every "not actually the file we hold" case without a separate
// symlink-mode check (os.Lstat, never os.Stat, so a symlink AT path is
// never followed to the file it names).
func stillOurRegisteredFile(held *os.File, path string) (bool, error) {
	heldInfo, err := held.Stat()
	if err != nil {
		return false, fmt.Errorf("filelock: stating held lock handle for %s: %w", path, err)
	}
	diskInfo, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil // removed out from under the registered handle
	}
	if err != nil {
		return false, fmt.Errorf("filelock: stating lock path %s: %w", path, err)
	}
	return os.SameFile(heldInfo, diskInfo), nil
}

// LeaseIfHeldByCurrentProcess is the CUSTODY form of the ownership query
// above, for a caller that will go on to do work under an exclusion this
// process already owns (internal/draftmutation's writer-lock reuse path).
// It answers exactly the question HeldByCurrentProcess answers — same
// registry, same forged-lock rule, same exact-open-file identity test —
// and, when the answer is yes, takes a LEASE against that registration in
// the SAME critical section, so no concurrent Release can slip between the
// proof and the lease. While a lease is outstanding, Release of that exact
// handle BLOCKS: the lock file stays on disk and no second process can
// acquire the checkout, which is what makes the proven ownership hold for
// the whole length of the caller's work rather than for one instant.
//
// A lease NEVER releases the outer lock and is not a claim on it: only the
// original acquirer's own Release closes and removes the lock file. The
// lease's only effect is to make that owner's Release wait.
//
// held is true only together with a non-nil release func; the caller must
// call it (defer) as soon as its work under the exclusion is done, or the
// owner's Release blocks forever. Calling the returned func more than once
// is INERT (it is idempotent by construction), so a defensive extra call
// during error unwinding can never corrupt another lease's accounting.
// (false, nil) and any error carry no lease and no release func — the
// caller must treat both as unproven and refuse, exactly as with
// HeldByCurrentProcess.
func LeaseIfHeldByCurrentProcess(path string) (release func(), held bool, err error) {
	key := filepath.Clean(path)
	registryMu.Lock()
	entry, ok := registry[key]
	if ok {
		// Taken BEFORE the identity check and inside the same lock the
		// registry itself is guarded by: from here on a concurrent Release
		// of this entry must wait for us, so the proof below cannot be
		// invalidated by a release racing between query and lease. A proof
		// that then fails simply drops the lease again.
		entry.leases++
	}
	registryMu.Unlock()
	if !ok {
		return nil, false, nil
	}

	ours, err := stillOurRegisteredFile(entry.file, path)
	if err != nil || !ours {
		releaseLease(entry)
		return nil, false, err
	}
	var once sync.Once
	return func() { once.Do(func() { releaseLease(entry) }) }, true, nil
}

// Acquire implements I-12(a) end to end: create path with O_CREATE|O_EXCL
// and write {pid,start} JSON on success, start being this process's own OS
// start time (ownProcessStart, SI-300). If the path already exists,
// inspect the holder recorded inside — alive (per alive, above) yields
// ErrHeld (the caller should proxy/reuse rather than proceed);
// dead/stale takes the lock over (takeOverStale, SI-302: on platforms with
// flock(2) the detector removes only the exact file it judged, under a
// flock on that file, so two detectors of one stale lock can never both
// remove it) and retries acquisition, up to a small bound: a takeover
// that finds the lock changed under it re-evaluates from scratch, reading
// the new holder's live lock, and only one O_EXCL create can win.
//
// The own start is read once per call, BEFORE the exclusive create, so the
// ps exec it may cost never widens the window between the create and the
// body flush that racing readers must treat as mid-flush.
func Acquire(path string) (*os.File, error) {
	return acquire(path, 5, ownProcessStart())
}

func acquire(path string, retriesLeft int, start int64) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		info := Info{PID: os.Getpid(), Start: start}
		if encErr := json.NewEncoder(f).Encode(info); encErr != nil {
			_ = f.Close()
			_ = os.Remove(path)
			return nil, fmt.Errorf("filelock: writing lock %s: %w", path, encErr)
		}
		registerAcquired(path, f)
		return f, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("filelock: acquiring lock %s: %w", path, err)
	}

	data, rerr := lockReadFile(path)
	if rerr != nil {
		// Lost the race with the remover between our OpenFile and this Read.
		if errors.Is(rerr, os.ErrNotExist) && retriesLeft > 0 {
			return acquire(path, retriesLeft-1, start)
		}
		return nil, fmt.Errorf("filelock: lock %s exists but is unreadable: %w", path, rerr)
	}
	info, jerr := decodeLockInfo(path, data)
	if jerr != nil {
		// A decode failure is a HARD malformed error ONLY for a
		// complete-but-garbled body. An empty or truncated body is the
		// signature of a mid-flush partial write: Acquire's own
		// O_CREATE|O_EXCL makes the path exist an instant before the winner
		// flushes its {pid,start} JSON, so a racing acquirer can read the
		// bytes-not-landed-yet file (the ""/EOF this closes). Resolve that by
		// the file's age rather than failing hard.
		if !lockBodyIncomplete(jerr) {
			return nil, fmt.Errorf("filelock: lock %s exists but is malformed (%q): %w", path, string(data), jerr)
		}
		young, serr := lockFileYoung(path)
		if serr != nil {
			// Lost the race with a concurrent remover between our read and
			// this stat — retry acquisition rather than fail hard.
			if errors.Is(serr, os.ErrNotExist) && retriesLeft > 0 {
				return acquire(path, retriesLeft-1, start)
			}
			return nil, fmt.Errorf("filelock: lock %s exists but its empty/partial body could not be aged: %w", path, serr)
		}
		if young {
			// Freshly created, not-yet-flushed: HELD, never a hard error —
			// the winner is mid-write, so the caller must keep polling. No
			// {pid} body has landed yet, hence the zero-value Info.
			return nil, &ErrHeld{Info: Info{}}
		}
		// An empty/partial body older than the mid-flush window is a writer
		// that crashed between create and flush. No {pid} survives for a
		// liveness probe, so age is the honest staleness signal: take it over
		// exactly like a dead-pid lock.
		if retriesLeft <= 0 {
			return nil, fmt.Errorf("filelock: stale lock %s (empty/partial body older than %s) but exceeded takeover retries", path, lockMidFlushWindow)
		}
		if terr := takeOverStale(path); terr != nil {
			return nil, takeoverFailed(path, "empty/partial body", terr)
		}
		return acquire(path, retriesLeft-1, start)
	}
	if alive(info.PID, info.Start) {
		return nil, &ErrHeld{Info: info}
	}
	if retriesLeft <= 0 {
		return nil, fmt.Errorf("filelock: stale lock %s (pid %d dead) but exceeded takeover retries", path, info.PID)
	}
	if terr := takeOverStale(path); terr != nil {
		return nil, takeoverFailed(path, fmt.Sprintf("pid %d dead", info.PID), terr)
	}
	return acquire(path, retriesLeft-1, start)
}

// takeoverFailed is acquire's answer when takeOverStale did not leave the
// lock to re-evaluate: its *ErrHeld (another detector is taking the lock
// over) passes through unchanged, and any other error is operational, named
// with the stale lock it concerned.
func takeoverFailed(path, why string, err error) error {
	var held *ErrHeld
	if errors.As(err, &held) {
		return held
	}
	return fmt.Errorf("filelock: stale lock %s (%s) but could not take it over: %w", path, why, err)
}

// lockReadFile and lockStat are acquire's read of an existing lock's body
// and lockFileYoung's stat of it — os.ReadFile and os.Stat, vars only so a
// test can remove the lock between two of acquire's steps, the concurrent
// remover acquire's two ENOENT retry branches exist for.
var (
	lockReadFile = os.ReadFile
	lockStat     = os.Stat
)

// Release closes f and removes path — the holder's own clean path — but
// only while path still names f itself (SI-302): the identity is checked
// while f is still open, so its inode cannot have been reused. When path
// names another file — the lock was taken from this holder and another
// process now holds one there (an older binary's by-name takeover, say),
// even with byte-identical contents — Release leaves that file in place and
// returns nil, exactly as when path is already gone: this holder's own
// release is complete. When path cannot be inspected, Release returns an
// error and removes nothing. Holders take no flock (takeOverStale says
// why), so the check and the remove are two steps; only a detector that
// misjudged this live holder stale, or an older binary, can act between
// them. A crash leaves the lock behind on disk exactly as I-12 intends: the
// next acquirer's alive() probe discovers the dead pid and takes over.
// Release is the sole owner release and deregisters exactly the
// registered handle (deregisterAcquired only removes a registry entry
// that still names this exact *os.File), before closing it — so no
// window exists where a still-registered entry names an already-closing
// handle.
//
// BLOCKING: Release waits, uncancellably and without a timeout, until
// every lease taken against this exact handle
// (LeaseIfHeldByCurrentProcess) has been released; only then does it
// deregister, close, and remove. That wait is the point — it is what keeps
// the on-disk exclusion continuously in force while an operation this
// process already admitted under the lock is still running, so a shutdown
// path that does not itself wait for in-flight work (an http.Server.Close
// on SIGTERM, say) can never drop the lock under a running writer and let
// a second process in. A caller that never releases its lease therefore
// hangs its own Release: the lease contract is defer-and-release.
// Releasing a handle with no leases (the common case) never waits.
func Release(f *os.File, path string) error {
	deregisterAcquired(path, f)
	ours, identErr := stillOurRegisteredFile(f, path)
	if cerr := f.Close(); cerr != nil {
		return fmt.Errorf("filelock: closing lock %s: %w", path, cerr)
	}
	if identErr != nil {
		return fmt.Errorf("filelock: lock %s left in place: %w", path, identErr)
	}
	if !ours {
		return nil
	}
	if rerr := os.Remove(path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		return fmt.Errorf("filelock: removing lock %s: %w", path, rerr)
	}
	return nil
}

// Peek reports whether path currently names a LIVE lock, without
// creating, removing, or otherwise mutating anything on disk — a
// read-only liveness check for a caller (spec/worktree-manager's `gc`,
// dc-2/dc-4) that needs to know "is this held right now" without racing
// Acquire's own create/takeover side effects. It returns (Info{}, false,
// nil) both when no lock file exists at all and when one exists but its
// recorded holder is not alive (stale) — gc treats a stale lock exactly
// like no lock at all (Acquire's own stale-takeover semantics), without
// actually taking it over here: gc performs its own explicit Acquire
// immediately before its own mutating git call, never relying on a Peek
// result alone to decide it is safe to remove anything.
func Peek(path string) (Info, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Info{}, false, nil
		}
		return Info{}, false, fmt.Errorf("filelock: peeking lock %s: %w", path, err)
	}
	info, jerr := decodeLockInfo(path, data)
	if jerr != nil {
		// Same mid-flush window Acquire honours (above): a complete-but-garbled
		// body is a hard malformed error, but an empty/partial body is judged
		// by age — young = a live holder still mid-write (held); old = a
		// crashed writer (stale, reported not-held exactly like no lock, per
		// gc's Peek contract).
		if !lockBodyIncomplete(jerr) {
			return Info{}, false, fmt.Errorf("filelock: lock %s exists but is malformed (%q): %w", path, string(data), jerr)
		}
		young, serr := lockFileYoung(path)
		if serr != nil {
			if errors.Is(serr, os.ErrNotExist) {
				return Info{}, false, nil // removed under us: no lock at all
			}
			return Info{}, false, fmt.Errorf("filelock: lock %s exists but its empty/partial body could not be aged: %w", path, serr)
		}
		return Info{}, young, nil
	}
	return info, alive(info.PID, info.Start), nil
}

// lockMidFlushWindow bounds how long after a lock file's last modification an
// empty or truncated (mid-flush) body is still charitably read as "the winner
// is mid-write, HELD" rather than "a writer crashed between O_CREATE|O_EXCL
// and its flush, stale". Conservative on purpose: the real mid-flush gap is
// sub-millisecond (one Encode call), so 2s is enormously generous for a live
// holder yet still lets a genuinely crashed writer's empty lock be taken over
// promptly. An empty body carries no {pid} for a liveness probe, so age is the
// only honest staleness signal available for it.
const lockMidFlushWindow = 2 * time.Second

// lockBodyIncomplete reports whether a decode error is the signature of a
// mid-flush partial write — an empty file (io.EOF) or a truncated JSON prefix
// (io.ErrUnexpectedEOF) — as opposed to a complete-but-garbled body (a syntax
// or unknown-field error), which is a genuine malformed lock and stays a hard
// error. artifact.DecodeStrictJSON wraps the underlying error with %w, so
// errors.Is sees through it.
func lockBodyIncomplete(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

// lockFileYoung reports whether path was last modified within
// lockMidFlushWindow of now — recent enough that an empty/partial body is a
// live holder still mid-write rather than a crashed one. The stat error is
// returned unwrapped so the caller can distinguish os.ErrNotExist (the file
// was removed out from under us — lost a race) from a real stat failure.
func lockFileYoung(path string) (bool, error) {
	st, err := lockStat(path)
	if err != nil {
		return false, err
	}
	return modifiedWithinMidFlushWindow(st.ModTime()), nil
}

// modifiedWithinMidFlushWindow reports whether modTime lies within
// lockMidFlushWindow of now: lockFileYoung's test, and takeOverStale's for a
// lock it judges through its own open handle.
func modifiedWithinMidFlushWindow(modTime time.Time) bool {
	return time.Since(modTime) <= lockMidFlushWindow
}

// lockDecodeRetries/lockDecodeRetryDelay bound decodeLockInfo's tolerance
// for a benign, extremely short race: Acquire's own O_CREATE|O_EXCL
// succeeds (the path exists) a moment before its JSON body is fully
// flushed, so a concurrent reader (another Acquire call racing for the
// same lock, or a Peek) can observe a transiently empty or truncated
// file. 25 attempts at 2ms apart bounds the wait at ~50ms — generous
// under -race/heavy goroutine contention, still tiny next to any real
// git-worktree-mutating call this lock actually guards (dc-2).
const (
	lockDecodeRetries    = 25
	lockDecodeRetryDelay = 2 * time.Millisecond
)

// decodeLockInfo strict-decodes data (already read from path) as Info. If
// that fails, it re-reads path a bounded number of times before giving
// up — closing the transient partial-write race described above — and
// returns the LAST attempt's decode error if every retry still fails
// (a genuinely malformed or foreign lock file decodes the same way every
// time, so this adds bounded latency to that case, never a wrong
// answer).
func decodeLockInfo(path string, data []byte) (Info, error) {
	var info Info
	err := strictUnmarshal(data, &info)
	if err == nil {
		return info, nil
	}
	for i := 0; i < lockDecodeRetries; i++ {
		time.Sleep(lockDecodeRetryDelay)
		data2, rerr := os.ReadFile(path)
		if rerr != nil {
			continue // e.g. removed by a concurrent takeover; keep retrying within budget
		}
		err = strictUnmarshal(data2, &info)
		if err == nil {
			return info, nil
		}
	}
	return Info{}, err
}
