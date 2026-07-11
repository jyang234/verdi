// verdi gate (I-7, PLAN.md Phase 8; R4-I-8 extends it): the merge-gate
// verb, 03 §Gates' "merge gate" — four conditions, all fail-closed:
//
//  1. the story's spec exists on the DEFAULT branch with status
//     accepted-pending-build (read via git, never the working tree — a
//     build branch must never be trusted to self-report this);
//  2. no AC is violated at the build head, per the fold, authoritative
//     (source: ci) evidence only — never --preview;
//  3. a fresh alignment report is present in the spec's directory
//     (`covers` == HEAD) with EVERY finding — computed and judged,
//     including the synthetic absence finding — dispositioned (I-9's
//     ratified reading of 03 §Gates, "every computed finding" corrected to
//     "every finding");
//  4. (V1-P4, 03 §The amendment ladder rung 4) no unresolved rung-4
//     cascade block: a story whose edges are CascadeInvalidated by a
//     merged feature supersession, or CascadeStale without a matching
//     re-affirmation record, is refused — "the merge gate and verdi build
//     start refuse a story whose edges carry unresolved stale flags"
//     (cascadecheck.go, shared with buildstart.go).
//
// This file also builds (V1-P4) the CLOSURE-gate condition set — spec-stale
// and pending-supersession, 03 §Gates' "closure gate" — as a self-contained,
// separately testable function (runClosureGate, closuregate.go) rather than
// folding those two conditions into runGate above: 03 is explicit that
// spec-stale and pending-supersession "block closure, not merge — builds
// keep moving", so mixing them into the merge-gate conditions above would
// be a spec violation, not just an organizational choice. `verdi close`
// (the verb that would dispatch a closure-MR run of this condition set) is
// out of this phase's scope (05 §CLI's close row), so runClosureGate is
// unwired to any CLI verb yet — built cleanly extensible so a `verdi close`
// phase can call it directly, and so V1-P5's declared-decision-conflict
// condition and V1-P7's review-thread condition (05 §CLI's gate row,
// SPEC-MR half — a THIRD, still-different condition set this phase
// deliberately does not touch) have an established sibling pattern to
// follow rather than needing to invent gate.go's next extension shape.
//
// gate takes no story/spec argument, like align — both infer the build's
// spec from the feature/<name> branch convention (internal/storyresolve.
// ResolveBuildSpec). Not named in 05 §CLI's table (I-7 notes this); exit
// contract mirrors upstream's own convention: 0 all conditions hold, 1 any
// condition fails, 2 operational error.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/evidence"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/lint"
	"github.com/OWNER/verdi/internal/store"
	"github.com/OWNER/verdi/internal/storyresolve"
)

// cmdGate is `verdi gate`'s entry point, invoked by dispatch.go.
func cmdGate(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "gate: usage: verdi gate (no arguments; operates on the current build branch)")
		return 2
	}

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}
	branch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}
	spec, err := storyresolve.ResolveBuildSpec(root, branch)
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}

	return runGate(ctx, root, spec, head, resolveDefaultBranch(ctx, root), stdout, stderr)
}

// resolveDefaultBranch mirrors cmd/verdi/lint.go's buildLintContext exactly
// (CI_DEFAULT_BRANCH, else the configured remote's HEAD symbolic ref, else
// "" — unknown, never guessed): the same "which branch is the default"
// resolution the rest of this module already uses (CLAUDE.md: don't invent
// a second one).
func resolveDefaultBranch(ctx context.Context, root string) string {
	if env := lint.ReadCIEnv(); env.DefaultBranch != "" {
		return env.DefaultBranch
	}
	if branch, err := gitx.DefaultBranch(ctx, root); err == nil {
		return branch
	}
	return ""
}

// gateCondition is one of the three merge-gate conditions' outcome.
type gateCondition struct {
	Name   string
	OK     bool
	Reason string
}

// runGate is the testable core: given an already-resolved root, the
// build-head spec (resolved from the working tree — condition 1 still
// reads the default branch's OWN copy of it via git, never trusting the
// working tree's status field), the build head commit, and a resolved
// default-branch ref (branch name or "" if unknown — condition 1 then
// fails closed), evaluates all three conditions independently, prints each
// with its reason, and returns the exit code.
func runGate(ctx context.Context, root string, spec *artifact.SpecFrontmatter, head, defaultBranchRef string, stdout, stderr io.Writer) int {
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "gate: internal error: resolved spec has an invalid id:", err)
		return 2
	}

	cond1 := checkAcceptedOnDefaultBranch(ctx, root, specRef.Name, defaultBranchRef)

	cond2, err := checkNoACViolated(ctx, root, spec, head)
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}

	cond3, err := checkFreshFullyDispositioned(root, specRef.Name, head)
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}

	cond4, err := checkCascadeCondition(root, spec)
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}

	allOK := true
	for _, c := range []gateCondition{cond1, cond2, cond3, cond4} {
		status := "PASS"
		if !c.OK {
			status = "FAIL"
			allOK = false
		}
		fmt.Fprintf(stdout, "[%s] %s\n", status, c.Name)
		if !c.OK {
			fmt.Fprintf(stdout, "       %s\n", c.Reason)
		}
	}

	if allOK {
		fmt.Fprintln(stdout, "gate: PASS")
		return 0
	}
	fmt.Fprintln(stdout, "gate: FAIL")
	return 1
}

// checkAcceptedOnDefaultBranch is condition 1: the story's spec exists on
// the default branch with status accepted-pending-build. Read via
// gitx.Show at the default branch's current tip — never the working tree,
// which a build branch must never be trusted to self-report (03 §Gates:
// "builds reference accepted designs only").
func checkAcceptedOnDefaultBranch(ctx context.Context, root, specName, defaultBranchRef string) gateCondition {
	name := "1. spec accepted-pending-build on the default branch"
	if defaultBranchRef == "" {
		return gateCondition{Name: name, Reason: "cannot determine the default branch (no CI_DEFAULT_BRANCH and no configured git remote HEAD) — failing closed"}
	}

	tip, err := gitx.RevParse(ctx, root, defaultBranchRef)
	if err != nil {
		return gateCondition{Name: name, Reason: fmt.Sprintf("resolving default branch %q: %v", defaultBranchRef, err)}
	}

	relPath := filepath.ToSlash(filepath.Join(".verdi", "specs", "active", specName, "spec.md"))
	raw, err := gitx.Show(ctx, root, tip, relPath)
	if err != nil {
		return gateCondition{Name: name, Reason: fmt.Sprintf("spec/%s not found on default branch %s at %s: %v", specName, defaultBranchRef, tip, err)}
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return gateCondition{Name: name, Reason: fmt.Sprintf("spec/%s on default branch: %v", specName, err)}
	}
	decoded, err := artifact.DecodeSpec(fm)
	if err != nil {
		return gateCondition{Name: name, Reason: fmt.Sprintf("spec/%s on default branch failed to decode: %v", specName, err)}
	}
	if decoded.Status != "accepted-pending-build" {
		return gateCondition{Name: name, Reason: fmt.Sprintf("spec/%s status on default branch %s is %q, want accepted-pending-build", specName, defaultBranchRef, decoded.Status)}
	}
	return gateCondition{Name: name, OK: true}
}

// checkNoACViolated is condition 2: no AC is violated at head, per the
// fold over authoritative (source: ci) evidence only — never --preview
// (03 §Gates: "the gate ... consume[s] authoritative evidence only").
func checkNoACViolated(ctx context.Context, root string, spec *artifact.SpecFrontmatter, head string) (gateCondition, error) {
	name := "2. no AC violated at head (authoritative evidence)"

	derivedRoot := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(ctx, root, derivedRoot, head)
	if err != nil {
		return gateCondition{}, fmt.Errorf("loading evidence records: %w", err)
	}
	slug := store.RefSlug(spec.Story)
	result, err := evidence.Fold(evidence.Input{Spec: spec, Records: records, Preview: false, StoreRoot: root, StorySlug: slug})
	if err != nil {
		return gateCondition{}, fmt.Errorf("folding evidence: %w", err)
	}
	if !result.Violated {
		return gateCondition{Name: name, OK: true}, nil
	}

	var violated []string
	for _, ac := range result.ACs {
		if ac.Status == evidence.StatusViolated {
			violated = append(violated, ac.ID)
		}
	}
	sort.Strings(violated)
	return gateCondition{Name: name, Reason: fmt.Sprintf("violated AC(s): %v", violated)}, nil
}

// checkCascadeCondition is condition 4: no unresolved rung-4 cascade block
// (03 §The amendment ladder rung 4). Thin wrapper around
// checkCascadeReaffirmation (cascadecheck.go, shared with build start)
// rendering its (ok, reason) pair as a gateCondition.
func checkCascadeCondition(root string, spec *artifact.SpecFrontmatter) (gateCondition, error) {
	name := "4. no unresolved rung-4 cascade block (spec-stale re-affirmation / invalidated edges)"
	ok, reason, err := checkCascadeReaffirmation(root, spec)
	if err != nil {
		return gateCondition{}, fmt.Errorf("checking rung-4 cascade: %w", err)
	}
	if !ok {
		return gateCondition{Name: name, Reason: reason}, nil
	}
	return gateCondition{Name: name, OK: true}, nil
}

// checkFreshFullyDispositioned is condition 3: a deviation-report.md is
// present in the spec's directory, its `covers` equals head, and every
// finding — computed and judged, including the synthetic absence finding —
// carries a disposition (I-9's ratified reading of 03 §Gates).
func checkFreshFullyDispositioned(root, specName, head string) (gateCondition, error) {
	name := "3. fresh, fully-dispositioned alignment report"
	path := filepath.Join(root, ".verdi", "specs", "active", specName, "deviation-report.md")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return gateCondition{Name: name, Reason: fmt.Sprintf("no deviation-report.md found at %s (run `verdi align`)", path)}, nil
		}
		return gateCondition{}, fmt.Errorf("reading %s: %w", path, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return gateCondition{Name: name, Reason: fmt.Sprintf("%s: %v", path, err)}, nil
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		return gateCondition{Name: name, Reason: fmt.Sprintf("%s failed to decode: %v", path, err)}, nil
	}
	if decoded.Covers != head {
		return gateCondition{Name: name, Reason: fmt.Sprintf("stale: covers %s, head is %s (run `verdi align` again)", decoded.Covers, head)}, nil
	}

	var undispositioned []string
	for _, f := range decoded.Findings {
		if !f.Dispositioned() {
			undispositioned = append(undispositioned, f.ID)
		}
	}
	if len(undispositioned) > 0 {
		sort.Strings(undispositioned)
		return gateCondition{Name: name, Reason: fmt.Sprintf("undispositioned finding(s): %v", undispositioned)}, nil
	}
	return gateCondition{Name: name, OK: true}, nil
}
