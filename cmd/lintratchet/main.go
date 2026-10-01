// Command lintratchet is the thin command make lint-strict runs after
// golangci-lint (spec/strict-lint-gate dc-1). All of its logic lives in
// internal/lintratchet; this command parses arguments and exits with the
// status that package decides: 0 (clean), 1 (verdict), 2 (operational).
//
//	lintratchet check    -lint-exit <status> -report <report.json> -baseline <baseline.json>
//	lintratchet baseline -lint-exit <status> -report <report.json> -baseline <baseline.json>
//
// check compares the report's findings with the committed baseline and that
// baseline with the one at the merge base with the default branch (make
// lint-strict). baseline regenerates the baseline from the report (make
// lint-strict-baseline, never part of make verify). -lint-exit is
// golangci-lint's own exit status: it runs with --issues-exit-code=0, so any
// nonzero status is operational (ledger SI-311). The baseline path is
// relative to the working directory, inside the repository.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/lintratchet"
)

const usage = "usage: lintratchet check|baseline -lint-exit <status> -report <report.json> -baseline <baseline.json>"

func main() {
	os.Exit(run(context.Background(), ".", os.Args[1:], os.Stdout, os.Stderr))
}

// run executes one invocation in dir and returns its exit status.
func run(ctx context.Context, dir string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return lintratchet.ExitOperational
	}
	sub := args[0]
	if sub != "check" && sub != "baseline" {
		fmt.Fprintf(stderr, "lintratchet: unknown subcommand %q\n%s\n", sub, usage)
		return lintratchet.ExitOperational
	}
	fs := flag.NewFlagSet("lintratchet "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	lintExit := fs.Int("lint-exit", -1, "golangci-lint's exit status (required; any nonzero status is operational)")
	report := fs.String("report", "", "golangci-lint's JSON report (required)")
	baseline := fs.String("baseline", "", "the baseline, relative to the working directory (required)")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return lintratchet.ExitClean
		}
		return lintratchet.ExitOperational
	}
	switch {
	case fs.NArg() != 0:
		fmt.Fprintf(stderr, "lintratchet: unexpected argument %q\n%s\n", fs.Arg(0), usage)
		return lintratchet.ExitOperational
	case *lintExit < 0:
		fmt.Fprintf(stderr, "lintratchet: -lint-exit is required: golangci-lint's own exit status\n%s\n", usage)
		return lintratchet.ExitOperational
	case *report == "":
		fmt.Fprintf(stderr, "lintratchet: -report is required\n%s\n", usage)
		return lintratchet.ExitOperational
	case *baseline == "":
		fmt.Fprintf(stderr, "lintratchet: -baseline is required\n%s\n", usage)
		return lintratchet.ExitOperational
	}

	if sub == "baseline" {
		return lintratchet.WriteBaseline(lintratchet.BaselineInput{LintExit: *lintExit, ReportPath: *report, BaselinePath: inDir(dir, *baseline)}, stdout, stderr)
	}
	earlier, err := lintratchet.NewGitEarlier(ctx, dir, *baseline)
	if err != nil {
		fmt.Fprintln(stderr, "lint-strict:", err)
		return lintratchet.ExitOperational
	}
	return lintratchet.Check(ctx, lintratchet.CheckInput{
		LintExit:     *lintExit,
		ReportPath:   *report,
		BaselinePath: inDir(dir, *baseline),
		Earlier:      earlier,
	}, stdout, stderr)
}

// inDir resolves p against dir unless it is absolute.
func inDir(dir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}
