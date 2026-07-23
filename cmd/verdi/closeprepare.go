package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// runPrepare derives the next closure-session state for an explicit story or
// feature ref. It may refresh only the target's living deviation report; all
// judgment remains a human-authored disposition and final closure remains a
// separate invocation of the existing close ritual.
func runPrepare(ctx context.Context, root, storyArg string, manifest *store.Manifest, deps closeDeps, forceLocal bool, stdout, stderr io.Writer) int {
	spec, err := storyresolve.Resolve(root, storyArg)
	if err != nil {
		fmt.Fprintln(stderr, "close: --prepare:", err)
		return 2
	}
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "close: --prepare: internal error: resolved spec has an invalid id:", err)
		return 2
	}
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "close: --prepare:", err)
		return 2
	}
	reportPath := store.DeviationReportPath(root, store.ZoneActive, specRef.Name)
	report, _, err := loadExistingReport(reportPath)
	if err != nil {
		fmt.Fprintln(stderr, "close: --prepare:", err)
		return 2
	}

	if report == nil || report.Covers != head {
		freshness := "absent"
		if report != nil {
			freshness = fmt.Sprintf("stale (covers %s)", report.Covers)
		}
		modelDigest, err := resolveModelDigest(root)
		if err != nil {
			fmt.Fprintln(stderr, "close: --prepare:", err)
			return 2
		}
		alignD := alignDeps{
			Runner:        deps.Runner,
			JudgeCmd:      deps.JudgeCmd,
			JudgeRequired: deps.JudgeRequired,
			JudgeTimeout:  deps.JudgeTimeout,
			ModelDigest:   modelDigest,
		}
		if rc := runAlignForSpec(ctx, root, spec, head, false, alignD, stdout, stderr); rc != 0 {
			return rc
		}
		fmt.Fprintf(stdout, "close: --prepare: ALIGNMENT REQUIRED (living report was %s for HEAD %s; the existing align engine refreshed it)\n", freshness, head)

		report, _, err = loadExistingReport(reportPath)
		if err != nil {
			fmt.Fprintln(stderr, "close: --prepare:", err)
			return 2
		}
		if report == nil {
			fmt.Fprintf(stderr, "close: --prepare: align returned success but %s is absent\n", reportPath)
			return 2
		}
	}

	undispositioned := make([]artifact.Finding, 0, len(report.Findings))
	for _, finding := range report.Findings {
		if !finding.Dispositioned() {
			undispositioned = append(undispositioned, finding)
		}
	}
	if len(undispositioned) > 0 {
		fmt.Fprintf(stdout, "close: --prepare: JUDGMENT REQUIRED (%d undispositioned finding(s) in %s)\n", len(undispositioned), store.DeviationReportRelPath(store.ZoneActive, specRef.Name))
		for _, finding := range undispositioned {
			fmt.Fprintf(
				stdout,
				"verdi disposition --rationale %s -- %s %s %s\n",
				shellQuoteWord("<human-authored rationale>"),
				shellQuoteWord(specRef.String()),
				shellQuoteWord(finding.ID),
				shellQuoteWord("<human-authored-disposition:fixed|accepted-deviation>"),
			)
		}
		return 1
	}

	rc := runPreflight(ctx, root, storyArg, manifest, deps.Model, deps.Forge, forceLocal, stdout, stderr)
	if rc == 1 {
		fmt.Fprintln(stdout, "close: --prepare: MECHANICAL WORK REQUIRED (closure preflight is NOT READY; see its diagnostics above)")
		return 1
	}
	if rc != 0 {
		return rc
	}

	forceArg := ""
	if forceLocal {
		forceArg = " --force-local"
	}
	fmt.Fprintf(stdout, "close: --prepare: next command: verdi close %s%s\n", storyArg, forceArg)
	return 0
}

// shellQuoteWord renders one argument as a copyable POSIX-shell word. The
// artifact schema deliberately accepts arbitrary non-empty finding IDs, so
// presentation must quote rather than narrow that compatibility boundary.
func shellQuoteWord(word string) string {
	if word != "" {
		safe := true
		for _, r := range word {
			if !isSafeShellWordRune(r) {
				safe = false
				break
			}
		}
		if safe {
			return word
		}
	}
	return "'" + strings.ReplaceAll(word, "'", `'"'"'`) + "'"
}

func isSafeShellWordRune(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' ||
		strings.ContainsRune("_@%+=:,./-", r)
}
