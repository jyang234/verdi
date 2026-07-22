// verdi init [--wizard] (05 §CLI candidate, spec/init-wizard, ledger
// L-N5, R4-I-56): the two-path store-creation verb. Bare (no flags) is
// R4-I-56's conservative, non-interactive scaffold wrapper: it writes
// exactly the .verdi/verdi.yaml skeleton the README's own manual
// bootstrap steps describe, and nothing more. --wizard opts explicitly
// into the guide Part II interview (internal/initwizard), requiring an
// attached terminal and refusing without one.
//
// Design doc §12 rules W-1..W-4/W-3b (this story's own binding
// contract): BOTH paths build the complete candidate store in a
// same-filesystem sibling temporary directory — never os.TempDir, so
// promotion can never cross a filesystem boundary — and never write to
// the real .verdi path until a single, final os.Rename promotes the
// whole staged tree at once (W-2). Promotion is gated on running the
// FULL runModelCheck core (model.go) over the staged root, exactly as
// `verdi model check` itself would (W-1) — never a decode-only stand-in
// that would leave a wizard-written template override unvalidated —
// and, when the wizard diverged from canonical, on the staged
// model.yaml decoding back to a value identical to the interview's own
// in-memory candidate (W-4). Both paths refuse on ANY existing .verdi/
// directory at all, not merely an existing manifest (W-3/W-3b), because
// the single-rename promotion itself requires .verdi to be absent or it
// fails outright with ENOTEMPTY. The staged temporary directory is
// removed on any pre-promotion error or refusal, and on a mid-interview
// abort (stdin ending before every prompt is answered) — no real-store
// write ever happens before that one rename.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/jyang234/verdi/internal/initwizard"
	"github.com/jyang234/verdi/internal/model"
	"golang.org/x/term"
)

// verdiDirName is the one directory name both the existence-refusal
// check (W-3/W-3b) and the final promotion (W-2) key on.
const verdiDirName = ".verdi"

// cmdInit is `verdi init`'s real entry point, invoked by dispatch.go. It
// parses --wizard, resolves the real working directory and the real TTY
// predicate (isRealStdinTTY — term.IsTerminal, with the disclosed
// VERDI_INIT_ASSUME_TTY test override), and delegates to runInit, the
// testable core every built-binary test in init_test.go drives through
// the compiled binary rather than calling directly (CLAUDE.md: CLI
// behavioral paths get built-binary Go e2e tests).
func cmdInit(args []string, stdout, stderr io.Writer) int {
	wizard := false
	for _, a := range args {
		switch a {
		case "--wizard":
			wizard = true
		default:
			// vocab:identity — CLI usage/flag grammar (identity)
			fmt.Fprintf(stderr, "init: unknown argument %q (usage: verdi init [--wizard])\n", a)
			return 2
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "init:", err)
		return 2
	}

	return runInit(cwd, wizard, os.Stdin, isRealStdinTTY(), stdout, stderr)
}

// isRealStdinTTY reports whether the real os.Stdin is attached to an
// interactive terminal (golang.org/x/term.IsTerminal — a real ioctl
// check, not a heuristic that would misclassify a redirected /dev/null
// as a terminal). VERDI_INIT_ASSUME_TTY=1 is a disclosed, test-only
// override — mirroring serve.go's own VERDI_REVIEW_FEED/
// VERDI_OPENMR_FEED canned-injection precedent — that lets a
// built-binary test (init_test.go) drive the real wizard interview over
// a scripted stdin pipe (this story's own disclosed stdin-script
// harness, chosen over a pty harness for hermetic, dependency-free CI
// portability) without a real terminal ever being attached. It changes
// nothing about a real user's invocation: no production flag or
// documented surface ever sets it.
func isRealStdinTTY() bool {
	if os.Getenv("VERDI_INIT_ASSUME_TTY") == "1" {
		return true
	}
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// runInit is the testable core: given an already-resolved cwd (the
// directory that will become the store root — .verdi lands directly
// under it) and injected TTY/stdin, runs the whole init ritual and
// returns the exit code. It never partially writes to the real root: a
// refusal or any staging/gate failure leaves cwd exactly as it found it
// (the sibling temp directory is always cleaned up), and the only write
// that ever touches the real .verdi path is the single terminal
// os.Rename.
func runInit(cwd string, wizard bool, stdin io.Reader, isTTY bool, stdout, stderr io.Writer) int {
	verdiDir := filepath.Join(cwd, verdiDirName)

	if desc, exists := describeExistingVerdiDir(verdiDir); exists {
		fmt.Fprintf(stderr, "init: refusing — %s already exists (verdi init is create-only; hand-edit .verdi/model.yaml and validate with `verdi model check` to reconfigure an existing store)\n", desc)
		return 2
	}

	if wizard && !isTTY {
		fmt.Fprintln(stderr, "init: --wizard requires an attached terminal; refusing rather than silently falling back to the bare defaults (run `verdi init` without --wizard for the non-interactive scaffold, or attach a TTY)")
		return 2
	}

	tempRoot, err := os.MkdirTemp(cwd, ".verdi-init-tmp-*")
	if err != nil {
		fmt.Fprintln(stderr, "init:", err)
		return 2
	}
	promoted := false
	defer func() {
		if !promoted {
			_ = os.RemoveAll(tempRoot)
		}
	}()

	candidate, err := stageCandidateStore(tempRoot, wizard, stdin, stdout, stderr)
	if err != nil {
		return reportInitStagingError(stderr, err)
	}

	// W-1: gate promotion on the FULL runModelCheck core over the staged
	// root — the same function `verdi model check` itself calls — never
	// a decode-only stand-in.
	var checkOut, checkErrBuf discardOrCapture
	if code := runModelCheck(tempRoot, &checkOut, &checkErrBuf); code != 0 {
		fmt.Fprintf(stderr, "init: the staged store failed verdi model check (exit %d), refusing to promote: %s\n", code, checkErrBuf.String())
		return 2
	}

	// W-4: when the wizard diverged from canonical, the staged
	// model.yaml must decode back to a value identical to the
	// interview's own in-memory candidate — proven by re-reading and
	// re-decoding the ACTUAL staged bytes, never trusting what was
	// rendered in memory.
	if candidate != nil {
		stagedPath := filepath.Join(tempRoot, verdiDirName, "model.yaml")
		stagedBytes, rerr := os.ReadFile(stagedPath)
		if rerr != nil {
			fmt.Fprintln(stderr, "init: internal error: staged model.yaml disappeared before the decode-compare check:", rerr)
			return 2
		}
		staged, derr := model.DecodeModel(stagedBytes)
		if derr != nil {
			fmt.Fprintln(stderr, "init: internal error: staged model.yaml failed to decode at the compare step:", derr)
			return 2
		}
		if !reflect.DeepEqual(staged, candidate) {
			fmt.Fprintln(stderr, "init: internal error: staged model.yaml does not decode-compare equal to the interview's own intent, refusing to promote")
			return 2
		}
	}

	// W-2: promotion is exactly one os.Rename of the staged .verdi
	// subtree onto the real path — no other write ever touches the real
	// root, before or after.
	if err := os.Rename(filepath.Join(tempRoot, verdiDirName), verdiDir); err != nil {
		fmt.Fprintln(stderr, "init:", err)
		return 2
	}
	promoted = true
	_ = os.Remove(tempRoot) // best-effort: the now-empty staging wrapper

	fmt.Fprintf(stdout, "init: created %s\n", verdiDir)
	return 0
}

// describeExistingVerdiDir reports whether path already exists (any
// entry at all — W-3b's unified predicate, since os.Rename's promotion
// target must be absent or it fails with ENOTEMPTY regardless of
// whether a manifest lives inside) and, when it does, a plain-language
// description of exactly what: a real store (verdi.yaml present) or a
// stray, non-manifest directory — so the refusal names what an operator
// actually finds there rather than a generic "already exists."
func describeExistingVerdiDir(path string) (description string, exists bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", false
	}
	if !info.IsDir() {
		return fmt.Sprintf("%s (not a directory)", path), true
	}
	if _, err := os.Stat(filepath.Join(path, "verdi.yaml")); err == nil {
		return fmt.Sprintf("%s (an existing store — verdi.yaml is present)", path), true
	}
	return fmt.Sprintf("%s (a directory with no verdi.yaml inside it)", path), true
}

// stageCandidateStore writes the complete candidate store under
// tempRoot/.verdi/ — verdi.yaml always, and, for the wizard path, a
// model.yaml (only on divergence from canonical) and template overrides
// per the interview's own choices — returning the in-memory candidate
// *model.Model the W-4 decode-compare step proves the staged bytes
// against (nil for the bare path, which never stages a model.yaml at
// all). Every returned error is already a *initError naming which
// staged file (if any) is implicated, so the caller can report it
// without re-deriving that context.
func stageCandidateStore(tempRoot string, wizard bool, stdin io.Reader, stdout, stderr io.Writer) (*model.Model, error) {
	if err := initwizard.WriteVerdiYAML(tempRoot); err != nil {
		return nil, &initStageError{file: "verdi.yaml", err: err}
	}
	if err := simulateCrashAfter("verdi.yaml"); err != nil {
		return nil, &initStageError{file: "verdi.yaml", err: err}
	}

	if !wizard {
		return nil, nil
	}

	result, err := initwizard.RunInterview(stdin, stdout)
	if err != nil {
		return nil, &initStageError{interview: true, err: err}
	}

	var candidate *model.Model
	if !initwizard.VocabularyEmpty(result.Vocabulary) {
		if err := initwizard.WriteModelYAML(tempRoot, result.Vocabulary); err != nil {
			return nil, &initStageError{file: "model.yaml", err: err}
		}
		if err := simulateCrashAfter("model.yaml"); err != nil {
			return nil, &initStageError{file: "model.yaml", err: err}
		}
		candidate = initwizard.CandidateModel(result.Vocabulary)
	}

	if result.CopyTemplates {
		if err := initwizard.CopyCanonicalTemplates(tempRoot); err != nil {
			return nil, &initStageError{file: "templates", err: err}
		}
		if err := simulateCrashAfter("templates"); err != nil {
			return nil, &initStageError{file: "templates", err: err}
		}
	}

	return candidate, nil
}

// initStageError carries enough context for reportInitStagingError to
// print a precise, single refusal line — never a bare "something failed"
// — for every way staging can fail: a write error naming the file, an
// interview error (ErrAborted/ErrDeclinedWrite, each printed as-is since
// initwizard's own error text already names the condition), or a
// simulated crash (also a plain error naming the file, via
// simulateCrashAfter).
type initStageError struct {
	file      string
	interview bool
	err       error
}

func (e *initStageError) Error() string { return e.err.Error() }
func (e *initStageError) Unwrap() error { return e.err }

// reportInitStagingError prints stgErr's message to stderr and returns
// the operational exit code (2) — every staging failure, whatever its
// cause, is "did not complete," never a verdict.
func reportInitStagingError(stderr io.Writer, stgErr error) int {
	fmt.Fprintln(stderr, "init:", stgErr)
	return 2
}

// simulateCrashAfter is a disclosed, test-only crash-injection hook:
// when VERDI_INIT_SIMULATE_CRASH_AFTER equals justStaged, it returns a
// synthetic error simulating a process crash immediately after that
// file (or "templates" for the template-copy step) was staged but
// before promotion — init_test.go's own mid-write-crash pin
// (spec/init-wizard ac-3) drives this to prove the staged temp directory
// is discarded and nothing ever reaches the real root. It changes
// nothing about a real invocation: the env var is never set outside a
// test process's own environment.
func simulateCrashAfter(justStaged string) error {
	if want := os.Getenv("VERDI_INIT_SIMULATE_CRASH_AFTER"); want != "" && want == justStaged {
		return fmt.Errorf("simulated crash after staging %s (VERDI_INIT_SIMULATE_CRASH_AFTER test hook)", justStaged)
	}
	return nil
}

// discardOrCapture is a tiny io.Writer/fmt.Stringer combo used to
// capture runModelCheck's own stderr for the gate-failure message
// without pulling in bytes.Buffer's wider API at the call site.
type discardOrCapture struct {
	data []byte
}

func (d *discardOrCapture) Write(p []byte) (int, error) {
	d.data = append(d.data, p...)
	return len(p), nil
}

func (d *discardOrCapture) String() string { return string(d.data) }
