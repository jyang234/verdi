package upstream

import (
	"context"
	"fmt"
)

// operationalExit is upstream's own exit-2 convention (PLAN.md §3: "gate
// verbs use the exit-code contract 0 = clean, 1 = verdict failure,
// 2 = operational error").
const operationalExit = 2

// RunGraph runs `flowmap graph -stamp <stamp> [-entry <entry>] <dir>` and
// strict-decodes its stdout. flowmap graph is verdict-neutral (it prints a
// view, never gates): any nonzero exit is an operational error (bad flags,
// unreadable dir).
//
// entry, when non-empty, appends flowmap's own `-entry` flag (spec/
// verification-extractor dc-3: the JSON-decodable, build-time equivalent of
// the render-time-only `--root` flag over the same selector value space),
// scoping the build to one entry point's reachable subgraph. Every call
// site that existed before spec/verification-extractor passes entry == ""
// and gets byte-for-byte today's unscoped argv and behavior — this is the
// same RunGraph, extended in place, not a second exec path.
func RunGraph(ctx context.Context, runner Runner, dir, stamp, entry string) (*Graph, error) {
	flags := []string{"-stamp", stamp}
	if entry != "" {
		flags = append(flags, "-entry", entry)
	}
	req := Request{
		Bin:        "flowmap",
		Subcommand: "graph",
		Flags:      flags,
		Positional: []string{dir},
	}
	res, err := runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("upstream: flowmap graph %s: %w", dir, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("upstream: flowmap graph %s: exit %d: %s", dir, res.ExitCode, res.Stderr)
	}
	g, err := DecodeGraph(res.Stdout)
	if err != nil {
		return nil, fmt.Errorf("upstream: flowmap graph %s: %w", dir, err)
	}
	return g, nil
}

// BoundaryGenerate runs `flowmap boundary <dir>`, which writes (never
// prints) `<dir>/.flowmap/boundary-contract.json` (spike S1: "flowmap
// boundary has no stdout mode or output flag — it always writes there").
// Callers read the file back themselves; this function only runs the
// command and reports operational failure.
func BoundaryGenerate(ctx context.Context, runner Runner, dir string) error {
	req := Request{Bin: "flowmap", Subcommand: "boundary", Positional: []string{dir}}
	res, err := runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("upstream: flowmap boundary %s: %w", dir, err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("upstream: flowmap boundary %s: exit %d: %s", dir, res.ExitCode, res.Stderr)
	}
	return nil
}

// BoundaryCheck runs `flowmap boundary -check <dir>`: upstream's own
// currency gate for the committed boundary contract (PLAN.md §3). Exit 0
// is current; exit 1 is stale (a verdict failure, returned as a non-nil
// error so callers can distinguish it from exit 2 only by inspecting the
// error — v0 has no caller that needs to tell them apart, so both surface
// as an error here).
func BoundaryCheck(ctx context.Context, runner Runner, dir string) error {
	req := Request{Bin: "flowmap", Subcommand: "boundary", Flags: []string{"-check"}, Positional: []string{dir}}
	res, err := runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("upstream: flowmap boundary -check %s: %w", dir, err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("upstream: flowmap boundary -check %s: exit %d (stale or operational error): %s", dir, res.ExitCode, res.Stderr)
	}
	return nil
}

// RunReview runs `groundwork review -json [-expect <sha>] <policy> <base> <branch>`
// and strict-decodes its stdout. A BLOCK verdict exits 1 (upstream's own
// contract: "BLOCK exits non-zero") but still emits valid JSON on stdout,
// so Review decodes on both exit 0 and exit 1 and lets the caller read
// Review.Verdict / Review.Blocking() for the three-valued outcome; only
// exit 2 (or an exec-level failure) is treated as an operational error.
func RunReview(ctx context.Context, runner Runner, policyPath, basePath, branchPath, expect string) (*Review, error) {
	flags := []string{"-json"}
	if expect != "" {
		flags = append(flags, "-expect", expect)
	}
	req := Request{
		Bin:        "groundwork",
		Subcommand: "review",
		Flags:      flags,
		Positional: []string{policyPath, basePath, branchPath},
	}
	res, err := runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("upstream: groundwork review: %w", err)
	}
	if res.ExitCode == operationalExit {
		return nil, fmt.Errorf("upstream: groundwork review: operational error (exit 2): %s", res.Stderr)
	}
	r, err := DecodeReview(res.Stdout)
	if err != nil {
		return nil, fmt.Errorf("upstream: groundwork review: %w", err)
	}
	return r, nil
}

// CrossCheckDiff runs `groundwork diff <base-contract> <branch-contract>`
// for its exit code alone (0 clean, 1 breaking — S1: no --json mode exists,
// so its stdout text is never parsed) and asserts it agrees with
// wantBreaking, the verdict ComputeBoundaryDiff already reached from the
// two strict-decoded contracts (I-3: "cross-checks its breaking verdict
// against groundwork diff's exit code in tests and CI"). A disagreement is
// a hard error — it means verdi's own diff computation has drifted from
// upstream's, which must never fail silently.
func CrossCheckDiff(ctx context.Context, runner Runner, baseContractPath, branchContractPath string, wantBreaking bool) error {
	req := Request{
		Bin:        "groundwork",
		Subcommand: "diff",
		Positional: []string{baseContractPath, branchContractPath},
	}
	res, err := runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("upstream: groundwork diff: %w", err)
	}
	switch res.ExitCode {
	case 0:
		if wantBreaking {
			return fmt.Errorf("upstream: groundwork diff exited 0 (clean) but verdi's computed boundary diff found a breaking change")
		}
	case 1:
		if !wantBreaking {
			return fmt.Errorf("upstream: groundwork diff exited 1 (breaking) but verdi's computed boundary diff found no breaking change")
		}
	default:
		return fmt.Errorf("upstream: groundwork diff: operational error (exit %d): %s", res.ExitCode, res.Stderr)
	}
	return nil
}

// Version runs `<bin> version` and returns its trimmed stdout (PLAN.md §3:
// "record flowmap version / groundwork version output in evidence
// provenance").
func Version(ctx context.Context, runner Runner, bin string) (string, error) {
	req := Request{Bin: bin, Subcommand: "version"}
	res, err := runner.Run(ctx, req)
	if err != nil {
		return "", fmt.Errorf("upstream: %s version: %w", bin, err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("upstream: %s version: exit %d: %s", bin, res.ExitCode, res.Stderr)
	}
	return trimTrailingNewline(res.Stdout), nil
}

func trimTrailingNewline(b []byte) string {
	s := string(b)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
