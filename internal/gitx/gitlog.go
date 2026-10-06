package gitx

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/canonjson"
)

// GitLogEnv names the file that every git execution gitx issues is
// appended to, one GitLogRecord per line, when the variable is set
// (spec/gitx-recorder-seam dc-2; parent spec/ritual-write-scope-v3 dc-10;
// ledger SI-359 (1), (3)). It is a test hook: the product never sets it.
// A test that drives the built binary sets it to record the verb's whole
// git command log, which no gitx.Observer a test attaches can reach,
// because the binary roots its own context.
//
// Unset or empty, nothing is recorded and gitx behaves exactly as without
// it. Set, a record that cannot be opened or appended fails the git
// execution with an operational error before git runs, so a log never
// silently lacks a call that ran. The variable is read at each execution;
// the sink keeps no package-level state (SI-219).
//
// VERDI_RECOVERY_GITLOG (`verdi recover`'s own hook, SI-305) is separate
// and keeps working (SI-359 (2)).
//
// Name a file outside every worktree the verb touches. A relative value
// resolves against the recording process's working directory, and a file
// inside a worktree changes what the verb sees: git lists it as
// untracked, a whole-tree guard counts it as the operator's work, and
// `git add -A` stages it (R5c1 review R5C1R-3). The ritual witness's
// Binary driver names an absolute temporary file for that reason.
const GitLogEnv = "VERDI_GITLOG"

// GitLogRecord is one line of the VERDI_GITLOG file: the argv after "git",
// the absolute, cleaned directory git runs in, and the pid of the process
// that ran it. A child process that inherits the variable records its own
// pid, so a reader keeps the records of the process it drove (SI-359 (3)).
// Each line is the record's canonical JSON (internal/canonjson): keys
// sorted, no HTML escaping, ending in one newline.
//
// Disclosed, not lossless (R5c1 review R5C1R-2): canonjson, like
// encoding/json, writes each invalid UTF-8 byte of an argument or of the
// directory as U+FFFD, so a record names such a path in a spelling no file
// has. A reader that attributes effects by path then matches nothing for
// it, which fails closed (unattributable, never within). Forbidden-token
// matching is unaffected: every token is ASCII, an element equal to one is
// valid UTF-8 and kept byte for byte, and a "--" token's prefix survives
// the replacement of any later byte.
type GitLogRecord struct {
	Args []string `json:"args"`
	Dir  string   `json:"dir"`
	PID  int      `json:"pid"`
}

// appendGitLog appends the record of a git execution of args in dir to the
// file GitLogEnv names, or does nothing when the variable is unset or
// empty. The record is written by one write to a file opened with
// O_APPEND, so concurrent executions in one process each append one whole
// line and never interleave. Any failure is returned for the exec site to
// refuse before git runs.
func appendGitLog(dir string, args []string) error {
	path := os.Getenv(GitLogEnv)
	if path == "" {
		return nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("gitx: %s: resolving the directory %q: %w", GitLogEnv, dir, err)
	}
	line, err := canonjson.Marshal(GitLogRecord{Args: append([]string{}, args...), Dir: abs, PID: os.Getpid()})
	if err != nil {
		return fmt.Errorf("gitx: %s: encoding the record: %w", GitLogEnv, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("gitx: %s: opening the log: %w", GitLogEnv, err)
	}
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		return fmt.Errorf("gitx: %s: appending to %s: %w", GitLogEnv, path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("gitx: %s: closing %s: %w", GitLogEnv, path, err)
	}
	return nil
}
