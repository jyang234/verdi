// Package readcensus counts the git reads one projection makes, for the
// structural witnesses of Wave 6 §5.3 (ledger SI-168, SI-356): one
// accepted-HEAD resolution and at most one accepted-tree enumeration per
// page projection, and one application projection and no mutation per
// conditional refresh. A Census is attached through gitx.WithObserver, so
// it counts at gitx's exec seam: every process gitx starts, and — through
// gitx.SessionObserver — every read a read session answers without one (a
// replayed ref read, an object name sent to the batch process) and every
// session opened. Counting replays and batched names matters: a ref named
// as an operand of a batched `<ref>:<path>` read is resolved inside git
// although no process starts for it.
//
// It is test support, like internal/fixturegit: production code never
// attaches one.
package readcensus

import (
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/gitx"
)

// Exec is the Kind of an Event that is a git process.
const Exec = "exec"

// Event is one read: a process (Kind Exec, Args its argv) or something a
// read session did without one (Kind a gitx.SessionEvent).
type Event struct {
	Kind string
	Args []string
}

// Census records every Event of the contexts it is attached to. Its zero
// value is ready to use and safe for concurrent use.
type Census struct {
	mu     sync.Mutex
	events []Event
}

// Observe records a process (gitx.Observer).
func (c *Census) Observe(_ string, args []string) {
	c.record(Exec, args)
}

// ObserveSession records a read session's event (gitx.SessionObserver).
func (c *Census) ObserveSession(_ string, event gitx.SessionEvent, args []string) {
	c.record(string(event), args)
}

func (c *Census) record(kind string, args []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, Event{Kind: kind, Args: append([]string(nil), args...)})
}

// Events returns a copy of every event recorded so far, in order.
func (c *Census) Events() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Event(nil), c.events...)
}

// Accepted names the accepted HEAD a Budget is counted against: every
// spelling of its ref a consumer could resolve (the short name, such as
// "origin/main", and the full refname) and the object ids it resolves to.
type Accepted struct {
	Spellings []string
	IDs       []string
}

// Budget is one projection's counts against Wave 6 §5.3.
type Budget struct {
	// Processes counts every git process.
	Processes int
	// Sessions counts the read sessions opened.
	Sessions int
	// Batches counts `cat-file --batch` processes.
	Batches int
	// Chains counts default-branch discoveries begun: the
	// `symbolic-ref ... refs/remotes/<remote>/HEAD` read each one starts
	// with, run or replayed.
	Chains int
	// Explicit counts `rev-parse --verify` reads of the accepted ref, with
	// or without ^{commit}, run or replayed.
	Explicit int
	// Operand lists every other read that names the accepted ref, so that
	// git resolves it again: a process's or a replay's argv, or a name
	// sent to the batch process.
	Operand []string
	// Enumerations lists every recursive listing of the accepted tree (by
	// ref or by id) that ran; Relisted, every one a session replayed,
	// which reuses a listing already made.
	Enumerations, Relisted []string
	// Writes lists every process that can write the index, a ref, the
	// object store or the working tree: `status` without
	// --no-optional-locks (which may refresh the index) and every writing
	// subcommand.
	Writes []string
}

// Resolutions is one discovery chain, one explicit resolution and no
// operand use: the one accepted-HEAD resolution §5.3 allows a projection.
func (b Budget) Resolutions() (chains, explicit, operand int) {
	return b.Chains, b.Explicit, len(b.Operand)
}

// Budget counts the census's events against acc.
func (c *Census) Budget(acc Accepted) Budget {
	var b Budget
	for _, e := range c.Events() {
		line := strings.Join(e.Args, " ")
		switch e.Kind {
		case string(gitx.SessionOpened):
			b.Sessions++
			continue
		case string(gitx.SessionBatched):
			for _, name := range e.Args {
				if namesAny(name, acc.Spellings) {
					b.Operand = append(b.Operand, "batch "+name)
				}
			}
			continue
		case Exec:
			b.Processes++
		}
		replayed := e.Kind == string(gitx.SessionReplayed)
		sub, rest := subcommand(e.Args)
		switch {
		case sub == "cat-file" && contains(rest, "--batch"):
			b.Batches++
		case isChainStart(sub, rest):
			b.Chains++
		case isExplicit(sub, rest, acc.Spellings):
			b.Explicit++
		case sub == "show-ref":
			// The discovery chain's own existence check of the full
			// refname (refs/remotes/origin/main): part of the chain.
		default:
			if argvNames(rest, acc.Spellings) {
				b.Operand = append(b.Operand, e.Kind+" "+line)
			}
		}
		if sub == "ls-tree" && recursive(rest) && namesAny(revision(rest), append(append([]string(nil), acc.Spellings...), acc.IDs...)) {
			if replayed {
				b.Relisted = append(b.Relisted, line)
			} else {
				b.Enumerations = append(b.Enumerations, line)
			}
		}
		if !replayed && writes(sub, e.Args, rest) {
			b.Writes = append(b.Writes, line)
		}
	}
	return b
}

// subcommand splits argv into git's subcommand and its arguments, past
// the global options gitx can pass before it (--no-optional-locks, -c
// <name>=<value>, -C <dir>).
func subcommand(args []string) (string, []string) {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-c" || a == "-C":
			i++
		case strings.HasPrefix(a, "-"):
		default:
			return a, args[i+1:]
		}
	}
	return "", nil
}

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// isChainStart reports a read of a remote's HEAD symbolic ref: the first
// read of every default-branch discovery (gitx.DefaultBranch).
func isChainStart(sub string, rest []string) bool {
	if sub != "symbolic-ref" || len(rest) == 0 {
		return false
	}
	last := rest[len(rest)-1]
	return strings.HasPrefix(last, "refs/remotes/") && strings.HasSuffix(last, "/HEAD")
}

// isExplicit reports `rev-parse --verify [-q] <ref>[^{commit}]`.
func isExplicit(sub string, rest, spellings []string) bool {
	if sub != "rev-parse" || !contains(rest, "--verify") || len(rest) == 0 {
		return false
	}
	last := strings.TrimSuffix(rest[len(rest)-1], "^{commit}")
	for _, s := range spellings {
		if last == s {
			return true
		}
	}
	return false
}

// argvNames reports whether any argument names one of spellings as a
// revision.
func argvNames(args, spellings []string) bool {
	for _, a := range args {
		if namesAny(a, spellings) {
			return true
		}
	}
	return false
}

// namesAny reports whether the revision expression rev names one of
// spellings: the name itself, <name>:<path>, <name>^…, <name>~…,
// <name>@{…}, or either end of a range.
func namesAny(rev string, spellings []string) bool {
	for _, s := range spellings {
		if s == "" {
			continue
		}
		for _, end := range strings.Split(strings.ReplaceAll(rev, "...", ".."), "..") {
			if end == s || strings.HasPrefix(end, s+":") || strings.HasPrefix(end, s+"^") ||
				strings.HasPrefix(end, s+"~") || strings.HasPrefix(end, s+"@") {
				return true
			}
		}
	}
	return false
}

// recursive reports an ls-tree that lists recursively (-r, alone or in a
// cluster such as -rz).
func recursive(rest []string) bool {
	for _, a := range rest {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "r") {
			return true
		}
	}
	return false
}

// revision is an ls-tree's tree-ish: its first argument that is not an
// option.
func revision(rest []string) string {
	for _, a := range rest {
		if a == "--" {
			return ""
		}
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

// writers are the subcommands that write the index, a ref, the object
// store or the working tree in every form gitx could run them.
var writers = map[string]bool{
	"add": true, "am": true, "apply": true, "checkout": true, "cherry-pick": true,
	"clean": true, "commit": true, "commit-tree": true, "fetch": true, "gc": true,
	"merge": true, "mktree": true, "mv": true, "notes": true,
	"pull": true, "push": true, "read-tree": true, "rebase": true, "replace": true,
	"reset": true, "restore": true, "revert": true, "rm": true, "stash": true,
	"switch": true, "update-index": true, "update-ref": true, "write-tree": true,
}

// writes reports whether a process can write: status without
// --no-optional-locks, hash-object -w, a symbolic-ref with a target, a
// branch or tag that is not a listing, a worktree command other than
// list, a config that is not a read, or any writing subcommand.
func writes(sub string, argv, rest []string) bool {
	switch sub {
	case "status":
		return !contains(argv, "--no-optional-locks")
	case "hash-object":
		return contains(rest, "-w")
	case "symbolic-ref":
		return len(positional(rest)) > 1
	case "branch":
		return len(positional(rest)) > 0 && !contains(rest, "--list") && !contains(rest, "-l")
	case "tag":
		return len(positional(rest)) > 0 && !contains(rest, "--list") && !contains(rest, "-l")
	case "worktree":
		return len(rest) == 0 || rest[0] != "list"
	case "config":
		for _, a := range rest {
			if a == "--get" || a == "--get-all" || a == "--get-regexp" || a == "--list" || a == "-l" {
				return false
			}
		}
		return true
	}
	return writers[sub]
}

// positional is rest's arguments that are not options.
func positional(rest []string) []string {
	var out []string
	for _, a := range rest {
		if !strings.HasPrefix(a, "-") {
			out = append(out, a)
		}
	}
	return out
}
