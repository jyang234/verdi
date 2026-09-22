// apply.go implements R-RR3-9's whole apply protocol for `verdi recover
// --apply` (ac-9, ac-10, Task 4 of the wave-3 plan): derive the projection
// fresh, find the named choice, refuse a non-executable one, re-prove the
// choice's own preconditions from a SECOND, immediately-pre-execution
// Gather (never string-matching a precondition's rendered prose — every
// check here is a structural comparison over Facts, using the identical
// predicates derive.go used to decide whether the choice existed at all:
// resolveReturnBranch, ritualBranchByName, and a fresh reclaim.Compute
// over a fresh residue.Scan), execute via exactly one of the two
// executors this feature admits (dc-4), then re-derive and report every
// declared postcondition with its observed value.
//
// Nothing is written before the executor itself runs: an unknown choice
// id, a choice with no executor, and a failed precondition all return
// before any mutating call. The two executors are branchcut.Unwind
// (already close's own unwind sequence, spec/readiness-recovery-v2 ac-9)
// and reclaim.Apply (already gc's own reclaim-unmanaged sweep) — this
// file adds no new git primitive of its own (dc-4).
package recovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/branchcut"
	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/reclaim"
	"github.com/jyang234/verdi/internal/residue"
	"github.com/jyang234/verdi/internal/store"
)

// ErrNoExecutor is Apply's refusal for a choice whose Executor is "none"
// (R-RR3-7: every such choice carries its own exact manual commands
// instead).
var ErrNoExecutor = errors.New("recovery: choice has no executor")

// ErrPreconditionFailed is Apply's refusal when a fresh, immediately-
// pre-execution Gather shows a choice's own precondition no longer holds
// (R-RR3-9). Nothing is executed and nothing changes.
var ErrPreconditionFailed = errors.New("recovery: precondition no longer holds")

// ErrUnknownChoice is Apply's refusal for a choice id absent from the
// freshly derived projection (R-RR3-9); the error text lists every known
// choice id.
var ErrUnknownChoice = errors.New("recovery: unknown choice")

// PostconditionResult is one declared postcondition's own observed
// outcome, printed by cmdRecover as "postcondition: <Text>:
// held|VIOLATED (<Observed>)".
type PostconditionResult struct {
	Text     string
	Held     bool
	Observed string
}

// Outcome is what --apply observed: the choice, each postcondition with
// its observed value, the re-derived projection, and the re-derived
// journey record (nil when the journey re-derivation itself failed —
// JourneyErr carries why; R-RR3-9: "the journey re-derivation failing is
// operational... after the executor ran; the projection still prints
// what it observed").
type Outcome struct {
	ChoiceID       string
	Postconditions []PostconditionResult
	After          Projection
	Journey        *journey.Record
	JourneyErr     error
}

// applyReproveHook, when non-nil, runs immediately after Apply finds an
// executable choice in its first (discovery) Gather and immediately
// before its second, immediately-pre-execution Gather (R-RR3-9's own
// "re-prove... immediately before execution" window) — a test-only seam,
// mirroring cmd/verdi/recover.go's own recoverObserverHook, that lets a
// test simulate a mutation landing in that narrow window (the ONLY way a
// single, otherwise-synchronous Apply call can ever observe the fresh
// Gather disagreeing with the discovery Gather: a single mutation applied
// before Apply is even called is seen identically by both of Apply's own
// Gather calls, since nothing else in Apply does I/O between them).
// Always nil in production.
var applyReproveHook func()

// applyJourneyHook, when non-nil, runs immediately after the post-
// execution Gather and immediately before the journey re-derivation —
// the ONLY window in which R-RR3-9's "the journey re-derivation failing
// is operational (exit 2) AFTER the executor ran" branch is reachable in
// a single synchronous call, since every input the journey reads is
// already consistent by then unless something else writes to the store.
// A test-only seam, mirroring applyReproveHook above; always nil in
// production, assigned only from apply_test.go under t.Cleanup, and
// unreachable from the CLI or MCP.
var applyJourneyHook func()

// Apply implements R-RR3-9: derive fresh -> find the choice -> refuse a
// non-executable one (ErrNoExecutor with the manual commands in the error
// text) -> re-prove preconditions from a fresh Gather -> execute ->
// re-derive -> check postconditions. It never executes anything for an
// unknown id or a failed precondition (those errors wrap the sentinel and
// name the id / the failed precondition); nothing is written before the
// executor runs.
func Apply(ctx context.Context, cfg *store.Config, ref, choiceID string, stderr io.Writer) (Outcome, error) {
	g := NewGatherer()

	facts, err := g.Gather(ctx, cfg, ref)
	if err != nil {
		return Outcome{}, err
	}
	before := Derive(facts)

	choice, state, ok := findChoice(before, choiceID)
	if !ok {
		// Wave review m1: a projection with no choices at all renders its
		// own fact, never a "known choices:" label with nothing after it.
		if known := choiceIDs(before); len(known) != 0 {
			return Outcome{}, fmt.Errorf("%w: %q; known choices: %s", ErrUnknownChoice, choiceID, strings.Join(known, ", "))
		}
		return Outcome{}, fmt.Errorf("%w: %q; no choices are offered for %s", ErrUnknownChoice, choiceID, before.Ref)
	}
	if choice.Executor == executorNone {
		return Outcome{}, fmt.Errorf("%w: %s; the manual commands are: %s", ErrNoExecutor, choiceID, strings.Join(choice.ManualCommands, "; "))
	}

	if applyReproveHook != nil {
		applyReproveHook()
	}

	fresh, err := g.Gather(ctx, cfg, ref) // re-prove immediately before execution (R-RR3-9)
	if err != nil {
		return Outcome{}, err
	}
	failedPrecondition, err := reprove(ctx, cfg.Root, facts, choice, fresh)
	if err != nil {
		return Outcome{}, err
	}
	if failedPrecondition != "" {
		return Outcome{}, fmt.Errorf("%w: %s", ErrPreconditionFailed, failedPrecondition)
	}

	var reclaimRows []reclaim.Row
	switch choice.Executor {
	case "branchcut.Unwind":
		executeUnwind(ctx, cfg.Root, state, fresh, stderr)
		// Outcome != Unwound is reported below by the postcondition check
		// against the re-gathered facts: nothing else to do here.
	case "reclaim.Apply":
		rows, rerr := executeReclaim(ctx, cfg.Root, state, fresh, stderr)
		if rerr != nil {
			return Outcome{}, rerr
		}
		reclaimRows = rows
	default:
		// vocab:identity — "feature" names this wave's own scope of work (dc-4), never a spec class
		return Outcome{}, fmt.Errorf("recovery: executor %q is not one of the two this feature admits (dc-4)", choice.Executor)
	}

	afterFacts, err := g.Gather(ctx, cfg, ref)
	if err != nil {
		return Outcome{}, err
	}

	var postconditions []PostconditionResult
	switch choice.Executor {
	case "branchcut.Unwind":
		postconditions = checkUnwindPostconditions(facts, choice, state, afterFacts)
	case "reclaim.Apply":
		postconditions = checkReclaimPostconditions(choice, state, afterFacts, reclaimRows)
	}
	if err := assertPostconditionsDeclared(choice, postconditions); err != nil {
		return Outcome{}, err
	}

	out := Outcome{
		ChoiceID:       choiceID,
		After:          Derive(afterFacts),
		Postconditions: postconditions,
	}
	if applyJourneyHook != nil {
		applyJourneyHook()
	}
	rec, jerr := journey.NewProjector().Project(ctx, cfg, ref)
	if jerr != nil {
		out.JourneyErr = jerr
	} else {
		out.Journey = &rec
	}
	return out, nil
}

// findChoice returns the first choice (executable or manual alike) whose
// ID matches id, along with the state it belongs to.
func findChoice(p Projection, id string) (Choice, RecognizedState, bool) {
	for _, s := range p.States {
		for _, c := range s.Choices {
			if c.ID == id {
				return c, s, true
			}
		}
	}
	return Choice{}, RecognizedState{}, false
}

// choiceIDs lists every choice id (executable or manual) in p, sorted —
// ErrUnknownChoice's own "known choices" witness.
func choiceIDs(p Projection) []string {
	var ids []string
	for _, s := range p.States {
		for _, c := range s.Choices {
			ids = append(ids, c.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

// ritualBranchByName returns f's own RitualBranch matching name, among
// the four ritual branches only.
func ritualBranchByName(f Facts, name string) (RitualBranch, bool) {
	for _, rb := range []RitualBranch{f.Design, f.Feature, f.Close, f.PolicyAdopt} {
		if rb.Name == name {
			return rb, rb.Exists
		}
	}
	return RitualBranch{}, false
}

// candidateFor returns f's own ritualCandidate matching name (its cut
// mechanism), among the four ritual branches only.
func candidateFor(f Facts, name string) (ritualCandidate, bool) {
	for _, c := range ritualCandidates(f) {
		if c.rb.Name == name {
			return c, true
		}
	}
	return ritualCandidate{}, false
}

func ritualNameSet(f Facts) map[string]bool {
	return map[string]bool{f.Design.Name: true, f.Feature.Name: true, f.Close.Name: true, f.PolicyAdopt.Name: true}
}

// reprove re-evaluates choice's own preconditions against fresh, dispatch
// note (a): "never string-match the sentence" — every check here is a
// structural predicate over Facts, the SAME predicate derive.go's own
// recognizer/resolver functions use, never a parse of the rendered
// precondition text. Returns the first failed precondition's own
// disclosure text ("" when every precondition holds), or a non-nil error
// for a genuine operational failure re-proving a reclaim choice (a
// residue.Scan I/O failure).
func reprove(ctx context.Context, root string, facts Facts, choice Choice, fresh Facts) (string, error) {
	switch choice.Executor {
	case "branchcut.Unwind":
		return reproveUnwind(facts, choice, fresh), nil
	case "reclaim.Apply":
		return reproveReclaim(ctx, root, choice, fresh)
	default:
		// vocab:identity — "feature" names this wave's own scope of work (dc-4), never a spec class
		return fmt.Sprintf("choice has executor %q, outside the two this feature admits (dc-4)", choice.Executor), nil
	}
}

// unwindTarget extracts the branch name an "unwind-branch-cut:<branch>"
// choice targets — its ID's own suffix after the colon, the same
// byte-for-byte target Validate already requires (R-RR3-3), never a
// parse of prose.
func unwindTarget(choice Choice) string {
	_, target, _ := strings.Cut(choice.ID, ":")
	return target
}

// reproveUnwind structurally re-checks every precondition
// recognizeEmptyBranchCut's own choice construction relies on: the
// branch still exists at the same tip, the index is empty, the working
// tree is clean, and the branch's own return branch (R-RR3-5) still
// resolves to the SAME branch it resolved to when the choice was built —
// dispatch note (b): "re-resolve it from fresh facts and refuse if it
// changed."
func reproveUnwind(facts Facts, choice Choice, fresh Facts) string {
	name := unwindTarget(choice)

	origRB, origExists := ritualBranchByName(facts, name)
	if !origExists {
		return fmt.Sprintf("%s no longer exists", name)
	}
	freshRB, freshExists := ritualBranchByName(fresh, name)
	if !freshExists {
		return fmt.Sprintf("%s no longer exists", name)
	}
	if freshRB.Tip != origRB.Tip {
		return fmt.Sprintf("%s no longer points at %s (now %s)", name, origRB.Tip, freshRB.Tip)
	}
	// ac-9's literal third clause, re-proved STRUCTURALLY and on its own
	// (review I1): "has no commits of its own" at execution time. For the
	// cut-from-current mechanism this is transitively implied by the
	// return-branch re-resolution below (its candidates ARE the emptiness
	// witnesses), but a resolved-base cut's return branch (design/<name>,
	// policy/adopt) is the re-resolved default branch and ignores the
	// witnesses entirely — so a branch that LOST its emptiness inside the
	// re-prove window would otherwise reach the executor. R-RR3-9: exit
	// 1, nothing changed.
	if !freshRB.Empty() {
		return fmt.Sprintf("%s now carries commit(s) of its own: no other local branch reaches its tip %s any more", name, freshRB.Tip)
	}
	// R-RR3-21: state the fact this check actually observed. "No longer"
	// would assert a transition nothing here witnessed — the derive-time
	// guard now withholds the choice outright when either is already
	// false, so this path only ever sees the tree as it is right now.
	if len(fresh.StagedPaths) != 0 {
		return "the index is not empty"
	}
	if fresh.Dirty {
		return "the working tree is not clean"
	}

	origCand, ok := candidateFor(facts, name)
	if !ok {
		return fmt.Sprintf("%s is no longer one of the four ritual branches", name)
	}
	origOriginal, _, _, origUndecidable := resolveReturnBranch(facts, origCand, origRB, ritualNameSet(facts))
	if origUndecidable {
		// Unreachable through this package's own construction (the choice
		// would never have been built), but never silently ignored.
		return fmt.Sprintf("%s's original branch is no longer decidable", name)
	}

	freshCand, ok := candidateFor(fresh, name)
	if !ok {
		return fmt.Sprintf("%s is no longer one of the four ritual branches", name)
	}
	freshOriginal, _, _, freshUndecidable := resolveReturnBranch(fresh, freshCand, freshRB, ritualNameSet(fresh))
	if freshUndecidable {
		return fmt.Sprintf("%s's original branch %s can no longer be resolved", name, origOriginal)
	}
	if freshOriginal != origOriginal {
		return fmt.Sprintf("%s's original branch changed from %s to %s", name, origOriginal, freshOriginal)
	}
	return ""
}

// reclaimTarget extracts the branch a "reclaim:<branch>" choice targets.
func reclaimTarget(choice Choice) string {
	_, target, _ := strings.Cut(choice.ID, ":")
	return target
}

// reclaimPlanItem recomputes reclaim's own plan fresh, from a fresh
// residue.Scan (dispatch note (c)): found is false when branch is no
// longer part of the plan at all; unprovenRefusal carries gc's own
// refusal sentence (R-RR3-14) when the fresh scan itself reports
// UnprovenSpecs, in which case found is always false and item is the
// zero value.
func reclaimPlanItem(ctx context.Context, root string, f Facts, branch string) (item reclaim.PlanItem, found bool, unprovenRefusal string, err error) {
	if !f.DefaultBranchResolved {
		return reclaim.PlanItem{}, false, "", fmt.Errorf("recovery: apply: the default branch no longer resolves; cannot recompute the reclaim plan")
	}
	res, err := residue.Scan(ctx, root, f.DefaultBranch.Ref)
	if err != nil {
		return reclaim.PlanItem{}, false, "", fmt.Errorf("recovery: apply: recomputing the residue scan: %w", err)
	}
	if len(res.UnprovenSpecs) > 0 {
		// R-RR3-14: mirrors gc's own refusal — never compute or apply a
		// plan over an incomplete scan.
		return reclaim.PlanItem{}, false, gcUnprovenSpecsRefusal, nil
	}
	plan := reclaim.Compute(res, root, f.CurrentBranch, f.DefaultBranch.BranchName)
	for _, it := range plan.Items {
		if it.Unit.Branch == branch {
			return it, true, "", nil
		}
	}
	return reclaim.PlanItem{}, false, "", nil
}

// reproveReclaim structurally re-checks a reclaim choice's own
// precondition ("reclaim plan still lists <target> as eligible") against
// a freshly recomputed plan. A no-longer-eligible item's own failure text
// IS reclaim's own Row.Line() rendering, verbatim (never a generic
// message) — the operator sees exactly the same disclosure `verdi gc`
// itself would show for the same row.
func reproveReclaim(ctx context.Context, root string, choice Choice, fresh Facts) (string, error) {
	target := reclaimTarget(choice)
	item, found, refusal, err := reclaimPlanItem(ctx, root, fresh, target)
	if err != nil {
		return "", err
	}
	if refusal != "" {
		return refusal, nil
	}
	if !found {
		return fmt.Sprintf("%s is no longer part of the reclaim plan", target), nil
	}
	if !item.Eligible {
		row := reclaim.Row{Kind: reclaim.KindKept, Unit: item.Unit, Reason: item.Reason, Detail: item.Detail}
		return row.Line(), nil
	}
	return "", nil
}

// executeUnwind re-resolves state's own unwind target and return branch
// from fresh (already reproved by reprove above) and runs the shared
// branchcut.Unwind sequence. Its returned Outcome is not itself
// inspected here: a giving-up outcome (anything but Unwound) is reported
// by checkUnwindPostconditions's own structural re-check of the
// subsequent Gather, exactly as an ordinary, successful unwind is.
func executeUnwind(ctx context.Context, root string, state RecognizedState, fresh Facts, stderr io.Writer) branchcut.Outcome {
	name := state.Target
	rb, ok := ritualBranchByName(fresh, name)
	if !ok {
		fmt.Fprintf(stderr, "recover: %s no longer exists; nothing to unwind\n", name)
		return branchcut.LeftUninspectable
	}
	cand, ok := candidateFor(fresh, name)
	if !ok {
		fmt.Fprintf(stderr, "recover: %s is no longer one of the four ritual branches; nothing to unwind\n", name)
		return branchcut.LeftUninspectable
	}
	originalBranch, _, _, undecidable := resolveReturnBranch(fresh, cand, rb, ritualNameSet(fresh))
	if undecidable {
		fmt.Fprintf(stderr, "recover: %s's original branch could not be resolved; nothing unwound\n", name)
		return branchcut.LeftUninspectable
	}
	return branchcut.Unwind(ctx, root, originalBranch, name, rb.Tip, "recover", stderr)
}

// branchTip returns f's own recorded tip for the local branch name.
func branchTip(f Facts, name string) (string, bool) {
	for _, bt := range f.LocalBranches {
		if bt.Name == name {
			return bt.Tip, true
		}
	}
	return "", false
}

// checkUnwindPostconditions structurally re-derives the three values an
// unwind choice's own Postconditions name (does-not-exist, current
// branch, HEAD-is-the-return-branch's-tip) from facts (the ORIGINAL,
// pre-execution gather — which named the return branch the choice
// promised to switch back to) and checks each against afterFacts (the
// post-execution gather), positionally matched against
// choice.Postconditions (recognizeEmptyBranchCut always builds exactly
// these three, in this order; Apply's own assertPostconditionsDeclared
// refuses any drift between the two).
//
// R-RR3-19: the third value is the RETURN BRANCH'S OWN TIP, re-read from
// the post-execution facts, never the cut point — R-RR3-5's return branch
// "may sit ahead of the ritual tip", so comparing HEAD with the cut point
// would report a correct unwind as VIOLATED in the ordinary case (the
// containing tier, and every resolved-base cut whose default branch moved
// on).
func checkUnwindPostconditions(facts Facts, choice Choice, state RecognizedState, afterFacts Facts) []PostconditionResult {
	name := state.Target
	origRB, _ := ritualBranchByName(facts, name)
	originalBranch := ""
	if cand, ok := candidateFor(facts, name); ok {
		originalBranch, _, _, _ = resolveReturnBranch(facts, cand, origRB, ritualNameSet(facts))
	}

	results := make([]PostconditionResult, 0, len(choice.Postconditions))
	text := func(i int) string {
		if i < len(choice.Postconditions) {
			return choice.Postconditions[i]
		}
		return fmt.Sprintf("postcondition[%d]", i)
	}

	afterRB, exists := ritualBranchByName(afterFacts, name)
	obs := "does not exist"
	if exists {
		obs = fmt.Sprintf("exists at %s", afterRB.Tip)
	}
	results = append(results, PostconditionResult{Text: text(0), Held: !exists, Observed: obs})

	results = append(results, PostconditionResult{
		Text:     text(1),
		Held:     afterFacts.CurrentBranch == originalBranch,
		Observed: fmt.Sprintf("current branch is %s", afterFacts.CurrentBranch),
	})

	head := afterFacts.Head
	if head == "" {
		head = "unknown"
	}
	returnTip, returnKnown := branchTip(afterFacts, originalBranch)
	tipObs := returnTip
	if !returnKnown {
		// Fail closed: with no post-execution tip for the return branch
		// there is nothing to compare HEAD against (never "held" from no
		// evidence).
		tipObs = "not a local branch any more"
	}
	results = append(results, PostconditionResult{
		Text:     text(2),
		Held:     returnKnown && head == returnTip,
		Observed: fmt.Sprintf("HEAD is %s, %s is at %s", head, originalBranch, tipObs),
	})

	return results
}

// assertPostconditionsDeclared is M1's own structural pin: every declared
// postcondition sentence was evaluated, in order, under its own declared
// text. The two check functions build their results positionally, so a
// fourth declared postcondition (or a reordering) would otherwise be
// silently neither evaluated nor printed — which ac-10's "prints the
// postconditions" would not survive. A mismatch is this verb's own
// operational failure (exit 2), never a verdict: the executor already
// ran, and the projection cannot honestly say what held.
func assertPostconditionsDeclared(choice Choice, results []PostconditionResult) error {
	if len(results) != len(choice.Postconditions) {
		return fmt.Errorf("recovery: apply: choice %s declares %d postcondition(s) but %d were evaluated; the projection cannot report what held", choice.ID, len(choice.Postconditions), len(results))
	}
	for i, r := range results {
		if r.Text != choice.Postconditions[i] {
			return fmt.Errorf("recovery: apply: choice %s postcondition[%d] was evaluated as %q but declared as %q", choice.ID, i, r.Text, choice.Postconditions[i])
		}
	}
	return nil
}

// checkReclaimPostconditions reports a reclaim choice's own declared
// postconditions (branch gone, and — for a worktree unit — its worktree
// path gone): a KindKept/KindRefused/KindPartial row from the ACTUAL
// execution (rows) fails every declared postcondition at once, carrying
// that row's own Row.Line() verbatim (dispatch note (c)) rather than a
// generic re-derived message; a KindReclaimed row (or, defensively, no
// captured row at all) is checked structurally against afterFacts.
func checkReclaimPostconditions(choice Choice, state RecognizedState, afterFacts Facts, rows []reclaim.Row) []PostconditionResult {
	results := make([]PostconditionResult, 0, len(choice.Postconditions))
	// M4: no row at all is no evidence at all. reclaim.Apply returns
	// exactly one row per applied PlanItem (internal/reclaim/execute.go),
	// so this is unreachable today — but a postcondition that reports
	// "held" from nothing fails OPEN, the wrong direction for a check
	// whose whole job is to verify where the run ended (DC-13).
	if len(rows) != 1 {
		observed := "no reclaim row returned"
		if len(rows) > 1 {
			observed = fmt.Sprintf("%d reclaim rows returned for one plan item", len(rows))
		}
		for _, text := range choice.Postconditions {
			results = append(results, PostconditionResult{Text: text, Held: false, Observed: observed})
		}
		return results
	}
	if rows[0].Kind != reclaim.KindReclaimed {
		line := rows[0].Line()
		for _, text := range choice.Postconditions {
			results = append(results, PostconditionResult{Text: text, Held: false, Observed: line})
		}
		return results
	}

	target := state.Target
	branchGone := true
	for _, bt := range afterFacts.LocalBranches {
		if bt.Name == target {
			branchGone = false
			break
		}
	}
	branchObs := "does not exist"
	if !branchGone {
		branchObs = "still exists"
	}
	if len(choice.Postconditions) > 0 {
		results = append(results, PostconditionResult{Text: choice.Postconditions[0], Held: branchGone, Observed: branchObs})
	}

	if len(choice.Postconditions) > 1 {
		wtPath := rows[0].Unit.WorktreePath
		gone := wtPath == "" || !pathExists(wtPath)
		obs := "does not exist"
		if !gone {
			obs = "still exists"
		}
		results = append(results, PostconditionResult{Text: choice.Postconditions[1], Held: gone, Observed: obs})
	}

	return results
}

// executeReclaim recomputes reclaim's own plan fresh (dispatch note (c))
// and applies exactly the one PlanItem the choice names — never the
// whole plan — printing every returned Row's own Line() to stderr
// verbatim, success or refusal alike, so an operator sees exactly what
// `verdi gc` itself would have shown for this one unit.
func executeReclaim(ctx context.Context, root string, state RecognizedState, fresh Facts, stderr io.Writer) ([]reclaim.Row, error) {
	target := state.Target
	item, found, refusal, err := reclaimPlanItem(ctx, root, fresh, target)
	if err != nil {
		return nil, err
	}
	if refusal != "" {
		return nil, fmt.Errorf("%w: %s", ErrPreconditionFailed, refusal)
	}
	if !found || !item.Eligible {
		return nil, fmt.Errorf("%w: %s is no longer eligible for reclaim", ErrPreconditionFailed, target)
	}

	rows := reclaim.Apply(ctx, root, reclaim.Plan{Items: []reclaim.PlanItem{item}})
	for _, row := range rows {
		// Wave-review m4: the row text is reclaim's own, verbatim, but the
		// LINE belongs to this verb's error stream — carrying the same
		// "recover: " prefix executeUnwind's own lines above (and every
		// line cmd/verdi's recoverErr writes) carry, so no line on that
		// stream is unattributed.
		fmt.Fprintln(stderr, "recover: "+row.Line())
	}
	return rows, nil
}
