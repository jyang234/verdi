package ritualwitness

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
)

// helperEnv switches this test binary into a stand-in for a built verb
// (helperVerb), so the Binary driver's tests exec a real subprocess with
// no network and no second build.
const helperEnv = "RITUALWITNESS_HELPER_VERB"

// TestMain isolates the whole test binary from ambient git configuration
// (IsolateGitConfig), so tests using Build may run in parallel. Run with
// helperEnv set, it is helperVerb instead.
func TestMain(m *testing.M) {
	if spec := os.Getenv(helperEnv); spec != "" {
		os.Exit(helperVerb(spec))
	}
	restore, err := IsolateGitConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	code := m.Run()
	restore()
	os.Exit(code)
}

// helperVerb is a stand-in verb, driven by spec "<exit>[:<branch>]": it
// creates <branch> in its working directory with plain git when one is
// named, then writes a line to stdout and to stderr and exits <exit>. Spec
// "panic" panics, which the Go runtime reports with a trace and exit 2;
// spec "panic-words" is a refusal whose own words begin "panic: " but that
// carries no trace, exiting 2; spec "stack-overflow" recurses past a
// 64 KiB stack limit, which the runtime reports as "fatal error: stack
// overflow" with a trace and exit 2 — a real runtime fatal error, reached
// deterministically on every platform and under -race, unlike a deadlock,
// whose detection the race runtime can withhold; spec "fatal-words" is a
// refusal that mentions a fatal error mid-line, exiting 2; spec
// "gotraceback" prints its GOTRACEBACK and exits 1; spec "stdin" prints
// what it read from standard input and exits 1; spec "fd3" writes a line
// to file descriptor 3 and exits 0, or exits 3 when it cannot; spec
// "cienv" prints CI, GITHUB_ACTIONS, and GITHUB_BASE_REF, each with
// whether it is set at all, and exits 1.
func helperVerb(spec string) int {
	switch spec {
	case "stdin":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "helper: reading stdin:", err)
			return 3
		}
		fmt.Fprintf(os.Stderr, "stdin=%q\n", data)
		return 1
	case "fd3":
		if _, err := os.NewFile(3, "fd3").WriteString("written to fd 3\n"); err != nil {
			fmt.Fprintln(os.Stderr, "helper: writing fd 3:", err)
			return 3
		}
		return 0
	case "cienv":
		for _, k := range []string{"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF"} {
			v, ok := os.LookupEnv(k)
			fmt.Fprintf(os.Stderr, "%s=%q set=%v\n", k, v, ok)
		}
		return 1
	case "panic":
		panic("helper verb panicked")
	case "panic-words":
		fmt.Fprintln(os.Stderr, "panic: the refusal's own words, not a trace")
		return 2
	case "stack-overflow":
		debug.SetMaxStack(64 << 10)
		return overflowStack(0)
	case "fatal-words":
		fmt.Fprintln(os.Stderr, "close: refused: the index holds a fatal error: foreign-staged.txt is staged")
		return 2
	case "gotraceback":
		fmt.Fprintf(os.Stderr, "GOTRACEBACK=%s\n", os.Getenv("GOTRACEBACK"))
		return 1
	}
	code, branch, _ := strings.Cut(spec, ":")
	exit, err := strconv.Atoi(code)
	if err != nil {
		fmt.Fprintln(os.Stderr, "helper: bad exit:", err)
		return 3
	}
	if branch != "" {
		if out, err := exec.CommandContext(context.Background(), "git", "branch", branch).CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "helper: git branch %s: %v\n%s", branch, err, out)
			return 3
		}
	}
	fmt.Println("helper stdout")
	fmt.Fprintln(os.Stderr, "helper stderr")
	return exit
}

// overflowStack recurses until the goroutine's stack exceeds its limit.
// Every frame holds a buffer the next call reads, so no frame can be
// elided; the guard on n, which never holds, keeps the recursion from
// being unconditional.
func overflowStack(n int) int {
	if n < 0 {
		return 0
	}
	var frame [512]byte
	frame[n%len(frame)] = byte(n)
	return overflowStack(n+1) + int(frame[(n+1)%len(frame)])
}
