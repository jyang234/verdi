// Spike closure on ratified CSE experiment evidence (Wave 5C Task 10;
// design §9's final paragraph, §10; CSE AC-5/AC-7, DC-1/DC-7/DC-9/DC-16,
// CO-1; SI-146 option c). The existing spike-close service (runClose,
// close.go) receives an ADDITIVE evidence provider: a comparison-backed
// close target — one that carries at least one comparative-spike-
// experiment under `.verdi/specs/active/<name>/experiments/` (worktree or
// accepted tree, either zone) — must additionally clear this gate before
// the ritual is allowed to mutate anything. A target that carries no
// experiment evidence at all is untouched: the gate reports zero
// experiments and runClose proceeds exactly as it always has.
//
// Design §9's singular rule is: a comparison-backed close requires an
// accepted ratification; SELECTING that ratification (select-recommended
// or select-other) additionally requires a byte-verified capsule; the
// ratified answer then flows through the spike's EXISTING `resolves` edge
// (no new edge is ever written, no parent-feature spec is ever touched —
// SI-146 option c). A non-selecting disposition (reject-all, misframed,
// request-new-revision) is an honest terminal response and does NOT
// satisfy closure by itself.
//
// Task 10's correction (SI-150, controller pins P1/P2/P5) moved the whole
// accepted-use re-verification and capsule byte-verification INTO the
// application core: internal/experimentapp.Service.VerifyAcceptedClosureEvidence
// resolves one accepted snapshot, re-verifies the retained V3 ratification
// proof, and — for a selecting disposition — re-derives and byte-compares
// the committed capsule manifest against the recomputed binding. This file
// therefore composes that ONE seam call per experiment (the brief's
// disclosed composition, adjudicated by the controller, not re-decided
// here): a comparison-backed target may close only when EVERY discovered
// experiment's evidence is clean AND at least one experiment supplies a
// valid selecting ratification (a clean, selecting outcome the seam only
// ever returns alongside a byte-verified capsule). CO-1 fail-closed: any
// experiment whose evidence is a verdict, or any operational outcome,
// blocks closure outright even if another experiment in the same target
// already satisfies it; a non-selecting experiment merely fails to
// CONTRIBUTE the satisfying selection, it does not itself poison an
// otherwise-satisfied target.
//
// design §10 / CO-4's exit floor governs every path here exactly as it
// governs the rest of close.go: 0 clean, 1 a well-formed but unsatisfied
// authority/lifecycle/evidence verdict, 2 an uninterpretable or unsafe
// operational condition. No missing fact is ever interpreted favorably
// (a provider error, or any per-experiment operational Outcome, is 2 —
// never silently read as "no experiments" or as a verdict); no
// operational result is ever softened into a verdict.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentapp"
)

// closeExperimentEvidence is the consumer-owned typed evidence this gate
// judges, one per experiment discovered under the closure target — the
// application core's own byte-verified closure facts
// (experimentapp.ClosureEvidenceResult), reshaped to exactly what the pure
// judgment below reads. Outcome alone is meaningful for a non-clean
// experiment; Disposition/Selecting/CapsuleVerified/SelectedCandidate are
// populated only when Outcome.Classification is ClassificationClean
// (design §9, controller pin P2): the seam never returns capsule facts
// alongside a non-clean outcome, and this adapter never invents one.
//
// CapsuleVerified/SelectedCandidate are derived from the seam's own
// Capsule field (present exactly when the committed manifest's exact
// canonical bytes equalled the recomputed experiment.BindCapsuleManifest
// encoding) — this file re-derives NO identity and re-implements NO
// capsule algorithm; every comparison design §9 requires already ran once,
// inside VerifyAcceptedClosureEvidence.
type closeExperimentEvidence struct {
	ExperimentID string
	Outcome      experimentapp.Outcome
	// Disposition is the accepted ratification's own disposition, valid
	// only for a clean Outcome.
	Disposition experiment.Disposition
	// Selecting reports whether Disposition is one of the two selecting
	// dispositions (select-recommended, select-other) — the seam's own
	// computed split, never re-derived here.
	Selecting bool
	// CapsuleVerified is true exactly when the seam's ClosureEvidenceResult
	// carried a non-nil Capsule: a selecting disposition whose committed
	// capsule manifest byte-verified against the recomputed binding. It is
	// always false for a non-selecting disposition (design §9: there is no
	// capsule to verify).
	CapsuleVerified bool
	// SelectedCandidate is the seam's own Capsule.Selected when
	// CapsuleVerified is true, "" otherwise.
	SelectedCandidate string
}

// closeExperimentEvidenceProvider is the port both runClose (close.go,
// between the closure gate's ok check and foldStory) and its preflight
// rehearsal (runStoryPreflightGate, closepreflight.go, once its own
// closure-gate outcome already holds) call. nil/empty evidence means the
// target is not comparison-backed: ordinary closure, zero behavior change.
// A returned error is always operational (2) — a provider that cannot
// answer this question honestly must never be read as "no experiments".
type closeExperimentEvidenceProvider interface {
	CloseEvidence(ctx context.Context, root string, spec *artifact.SpecFrontmatter) ([]closeExperimentEvidence, error)
}

// closeExperimentCondition is one experiment's rendered per-condition
// judgment (the reportClosureGateConditions [PASS]/[FAIL] idiom, applied
// to CSE evidence rather than AC evidence).
type closeExperimentCondition struct {
	id     string
	ok     bool
	reason string
}

// closeExperimentEvaluate is the pure core judgment (§9's singular rule
// composed per experiment, disclosed to Codex adjudication in the task
// report): it reads evidence only, never mutates the caller's injected
// slice. evidence must not contain an operational Outcome; the caller
// filters that case before this runs (a provider/port failure is 2, never
// folded into this judgment's 0/1 verdict).
//
// proceed is true exactly when every experiment's evidence is clean AND at
// least one experiment supplies a valid selecting ratification. A verdict
// Outcome, or a selecting disposition the seam did NOT byte-verify a
// capsule for, is a HARD failure that blocks closure outright (CO-1
// fail-closed) even alongside another experiment that already satisfies
// it; a non-selecting disposition merely fails to contribute the
// satisfying selection and does not itself block an otherwise-satisfied
// target.
//
// A selecting-but-CapsuleVerified-false evidence row is defensive: the
// application core's VerifyAcceptedClosureEvidence never returns a clean,
// selecting outcome without a byte-verified capsule (controller pin P2),
// so this branch should be unreachable through the production seam. It is
// still judged as a hard failure — never silently treated as satisfying —
// because a consumer must fail closed on a shape its own contract forbids
// rather than trust that the seam's own invariant always holds.
func closeExperimentEvaluate(evidence []closeExperimentEvidence) (proceed bool, conditions []closeExperimentCondition) {
	hardFailure := false
	anySelecting := false
	conditions = make([]closeExperimentCondition, 0, len(evidence))
	for _, e := range evidence {
		switch e.Outcome.Classification {
		case experimentapp.ClassificationVerdict:
			hardFailure = true
			conditions = append(conditions, closeExperimentCondition{
				id: e.ExperimentID, reason: fmt.Sprintf("%s [%s]", e.Outcome.Detail, e.Outcome.Code),
			})
			continue
		case experimentapp.ClassificationClean:
			// fall through to the selecting/capsule judgment below.
		default:
			// An operational Outcome must never reach this pure judgment
			// (the caller filters and exits 2 first); a defensive fail-
			// closed line rather than a silent skip if it ever does.
			hardFailure = true
			conditions = append(conditions, closeExperimentCondition{
				id: e.ExperimentID, reason: fmt.Sprintf("unexpected outcome classification %q", e.Outcome.Classification),
			})
			continue
		}
		if !e.Selecting {
			conditions = append(conditions, closeExperimentCondition{
				id: e.ExperimentID, reason: fmt.Sprintf("disposition %q is an honest terminal response and does not select a candidate; it does not satisfy closure by itself (design §9)", e.Disposition),
			})
			continue
		}
		if !e.CapsuleVerified {
			hardFailure = true
			conditions = append(conditions, closeExperimentCondition{
				id: e.ExperimentID, reason: fmt.Sprintf("disposition %q selects a candidate but its capsule evidence was not byte-verified by the closure seam", e.Disposition),
			})
			continue
		}
		anySelecting = true
		conditions = append(conditions, closeExperimentCondition{
			id: e.ExperimentID, ok: true,
			reason: fmt.Sprintf("valid ratified selection of candidate %q, capsule identity confirmed", e.SelectedCandidate),
		})
	}
	return !hardFailure && anySelecting, conditions
}

// closeExperimentGateCore is the seam both runClose (close.go, between the
// closure gate's ok check and foldStory, per the controller's fixed
// ordering: AC-5 — the ratification record joins the normal closure
// review, still strictly pre-effect) and its preflight rehearsal
// (runStoryPreflightGate, closepreflight.go) call, through the two thin
// wrappers below. It returns 0 to proceed (the caller continues
// unchanged), 1 for an unsatisfied verdict (the per-experiment
// [PASS]/[FAIL] condition lines are already printed to stdout; it prints
// NO terminal summary of its own — that is each wrapper's own line, so
// runClose's real-ritual wording and preflight's own "NOT READY" summary
// each speak their own verb without a bare, unprefixed "close: FAIL" line
// leaking into a dry run that mutated nothing), or 2 for an uninterpretable
// operational condition (the offending line is already printed to stderr).
func closeExperimentGateCore(evidence []closeExperimentEvidence, stdout, stderr io.Writer) int {
	if len(evidence) == 0 {
		// Not comparison-backed: zero behavior change.
		return 0
	}
	// design §10 / CO-4, fail-closed on an unknown enum value: the pure
	// judgment below is defined ONLY over the two interpretable
	// classifications, so anything that is neither clean nor verdict —
	// operational, or a classification this build does not recognize at
	// all — is an uninterpretable condition and exits 2 here. Reading an
	// unrecognized classification as a verdict would be exactly the
	// operational→verdict collapse §10 forbids.
	for _, e := range evidence {
		switch e.Outcome.Classification {
		case experimentapp.ClassificationClean, experimentapp.ClassificationVerdict:
			continue
		default:
			fmt.Fprintf(stderr, "close: experiment %s: %s [%s]\n", e.ExperimentID, e.Outcome.Detail, e.Outcome.Code)
			return 2
		}
	}
	proceed, conditions := closeExperimentEvaluate(evidence)
	if proceed {
		return 0
	}
	sort.Slice(conditions, func(i, j int) bool { return conditions[i].id < conditions[j].id })
	for _, c := range conditions {
		if c.ok {
			fmt.Fprintf(stdout, "[PASS] experiment %s: %s\n", c.id, c.reason)
		} else {
			fmt.Fprintf(stdout, "[FAIL] experiment %s: %s\n", c.id, c.reason)
		}
	}
	return 1
}

// closeExperimentGate is runClose's (close.go) own thin wrapper over
// closeExperimentGateCore: it adds runClose's exact, existing terminal FAIL
// summary line on an unsatisfied verdict (rc == 1) so real close's output
// stays byte-identical to before this correction, while an uninterpretable
// operational condition (rc == 2, whose own line is already on stderr)
// gets no summary at all. closepreflight.go's preflight rehearsal calls
// closeExperimentGateCore directly instead, so its OWN "NOT READY"/
// operational-exit-2 lines are the only summary a dry run ever prints —
// never this bare, unprefixed "close:" line, which would read as a real
// ritual having run (the exact defect class this file's preflight-parity
// wiring guards against elsewhere in close's own family, closepreflight.go's
// header comment).
func closeExperimentGate(evidence []closeExperimentEvidence, stdout, stderr io.Writer) int {
	rc := closeExperimentGateCore(evidence, stdout, stderr)
	if rc == 1 {
		fmt.Fprintln(stdout, "close: FAIL (experiment evidence not satisfied; see conditions above)")
	}
	return rc
}

// productionCloseExperimentEvidence is the real closeExperimentEvidenceProvider
// (closeDeps.Experiments == nil wires this, mirroring the State field's
// nil-is-production precedent). It discovers every experiment id under the
// closure target (worktree ∪ accepted tree, either zone) and asks the
// application core's ONE read-only closure-evidence operation
// (Service.VerifyAcceptedClosureEvidence) to re-verify each one's accepted
// ratification proof and, for a selecting disposition, its committed
// capsule — never re-deriving any of that itself (SI-150: role membership
// is never evidence, and no adapter-side trust fact or capsule algorithm
// remains in this file).
type productionCloseExperimentEvidence struct{}

// CloseEvidence implements closeExperimentEvidenceProvider.
func (productionCloseExperimentEvidence) CloseEvidence(ctx context.Context, root string, spec *artifact.SpecFrontmatter) ([]closeExperimentEvidence, error) {
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: resolved spec has an invalid id: %w", err)
	}
	name := specRef.Name

	accGit := experimentAcceptedGit{}
	branch, err := accGit.ResolveDefaultBranch(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: resolving the accepted default branch: %w", err)
	}
	head := branch.Head

	treeEntries, err := accGit.ListTree(ctx, root, head)
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: listing the accepted tree at %s: %w", head, err)
	}

	ids, err := closeExperimentIDUnion(root, name, treeEntries)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	service, err := newExperimentService(root)
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: constructing the experiment service: %w", err)
	}

	// vocab:identity — the delegated-agent harness identifier this verb
	// registers itself under (experimentapp.NewDelegatedAgent's own
	// grammar), not display prose.
	actor, err := experimentapp.NewDelegatedAgent("verdi-close", "")
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: constructing the delegated-agent actor: %w", err)
	}

	out := make([]closeExperimentEvidence, 0, len(ids))
	for _, id := range ids {
		identity := experimentapp.Identity{
			CheckoutRoot: root, Spike: "spec/" + name, ExperimentID: id,
			ExpectedAcceptedHEAD: head, Actor: actor,
		}
		result := service.VerifyAcceptedClosureEvidence(ctx, identity)
		ev := closeExperimentEvidence{ExperimentID: id, Outcome: result.Outcome}
		if result.Outcome.Classification == experimentapp.ClassificationClean {
			ev.Disposition = result.Disposition
			ev.Selecting = result.Selecting
			ev.CapsuleVerified = result.Capsule != nil
			if result.Capsule != nil {
				ev.SelectedCandidate = result.Capsule.Selected
			}
		}
		out = append(out, ev)
	}
	return out, nil
}

// closeExperimentIDUnion is the union of worktree-side and accepted-tree-
// side experiment ids under the closure target's experiments/ directory
// (the controller's stop-gate audit item 1): worktree ids via os.ReadDir
// (absent dir = none; a non-directory entry is included as an id too —
// a malformed layout must fail closed through Identity validation
// downstream, never be silently skipped), accepted-tree ids via the
// already-listed tree entries under EITHER zone's
// specs/<zone>/<name>/experiments/<id>/ prefix. Sorted, deduplicated.
func closeExperimentIDUnion(root, name string, treeEntries []experimentapp.GitTreeEntry) ([]string, error) {
	ids := map[string]bool{}

	worktreeDir := filepath.Join(root, ".verdi", "specs", "active", name, "experiments")
	dirEntries, err := os.ReadDir(worktreeDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("experiment evidence: reading %s: %w", worktreeDir, err)
	}
	for _, e := range dirEntries {
		ids[e.Name()] = true
	}

	for _, zone := range []string{"active", "archive"} {
		prefix := ".verdi/specs/" + zone + "/" + name + "/experiments/"
		for _, entry := range treeEntries {
			rest, ok := strings.CutPrefix(entry.Path, prefix)
			if !ok {
				continue
			}
			if id, _, _ := strings.Cut(rest, "/"); id != "" {
				ids[id] = true
			}
		}
	}

	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}
