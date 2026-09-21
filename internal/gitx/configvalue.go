package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrConfigUnset identifies the benign "no value here" state: the named
// key carries no usable value in the repository scope, either because
// the checkout does not configure it at all or because it configures it
// to the empty string, which names nobody. Both are the state a checkout legitimately has, mirroring
// RemoteURL's own ErrNoSuchRemote split between "absent" and
// "operationally broken" (ADJ-64: never conflate unreadable with
// absent). Callers use errors.Is to tell a valueless key apart from a
// real read error they must surface as operational.
var ErrConfigUnset = errors.New("gitx: git config key is unset")

// ConfigValue reads one git-config string value from dir's OWN
// repository configuration (.git/config), distinguishing the states
// local-operator
// identity resolution depends on (2026-09-05 local-operator disposition
// design §2.1). Because that value is an authorization input, every
// judgment here fails closed rather than guessing:
//
//   - set: exactly one value, returned as git's own bytes. ConfigValue
//     strips the single trailing newline git appends and nothing else —
//     never leading or trailing whitespace, which git itself preserves
//     for a quoted value, and trimming which would silently return an
//     identity other than the configured one.
//   - set-empty: the key is configured to the empty string. Git exits 0
//     and prints an empty line, so an exit-code-only reading would report
//     a usable identity of "". An empty value identifies nobody:
//     ConfigValue reports ErrConfigUnset.
//   - absent: the checkout's own configuration does not set the key.
//     Git documents exit code 1 with no stderr for exactly this case;
//     ConfigValue reports ErrConfigUnset (errors.Is-able), never
//     conflated with a real read failure.
//   - malformed key: git itself refuses the key's syntax (e.g. a key with
//     no section separator, or a variable name not starting with a
//     letter). Git also exits 1 for this, but WITH an explanatory stderr
//     message. That message is git's own localized prose (verified: it
//     is translated under a non-English locale), so absence-vs-malformed
//     is decided from whether stderr is empty, never by matching its
//     text (ADJ-64's locale-independence precedent, RemoteURL). Reported
//     as a plain operational error, never ErrConfigUnset: the caller
//     passed a key git cannot even parse, not a legitimately-unset one.
//   - broken configuration: any other git failure (a corrupt config
//     file, an unreadable repository, a missing git binary). Reported as
//     a plain operational error carrying git's own stderr.
//
// The read is scoped with `--local`, so the global and system scopes
// deliberately never participate. The design calls this "the store's Git
// identity" (§2.1): a self-asserted identity must at least be the
// checkout's own declaration, not whatever the machine happens to be
// configured with — a developer's global user.email would otherwise
// silently become the identity of every store on that machine. It also
// keeps the scopes from being mistaken for values: a global identity
// plus a repository override is the standard developer setup (and what
// internal/fixturegit and cmd/e2eharness provision), not an ambiguity.
//
// Within that scope the read uses `--get-all`, never `--get`: for a
// multi-valued key `--get` exits 0 and silently returns the LAST value,
// which for an authorization input is a wrong answer dressed as a right
// one. ConfigValue refuses any key carrying more than one value in
// .git/config as an operational error naming the key. Because
// `--get-all` separates values by newline, a single value that itself
// contains a newline is indistinguishable from two values and is refused
// the same way — the fail-closed direction, and never an identity
// ConfigValue invented.
func ConfigValue(ctx context.Context, dir, key string) (string, error) {
	observe(ctx, dir, []string{"config", "--local", "--get-all", key})
	cmd := exec.CommandContext(ctx, "git", "config", "--local", "--get-all", key)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		// git terminates every value it prints with one newline; the
		// value's own bytes are everything before it.
		out := strings.TrimSuffix(stdout.String(), "\n")
		if n := strings.Count(out, "\n") + 1; n > 1 {
			return "", fmt.Errorf("gitx: ConfigValue(%q): key is ambiguous: git config --local --get-all (dir %s) returned %d values", key, dir, n)
		}
		if out == "" {
			return "", fmt.Errorf("gitx: ConfigValue(%q): configured to the empty string: %w", key, ErrConfigUnset)
		}
		return out, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.TrimSpace(stderr.String()) == "" {
		return "", fmt.Errorf("gitx: ConfigValue(%q): %w", key, ErrConfigUnset)
	}
	return "", fmt.Errorf("gitx: ConfigValue(%q): git config --local --get-all (dir %s): %w: %s", key, dir, err, strings.TrimSpace(stderr.String()))
}
