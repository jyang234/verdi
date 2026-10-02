package ritualwitness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
// carries no trace, exiting 2.
func helperVerb(spec string) int {
	switch spec {
	case "panic":
		panic("helper verb panicked")
	case "panic-words":
		fmt.Fprintln(os.Stderr, "panic: the refusal's own words, not a trace")
		return 2
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
