package recovery

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/reclaim"
	"github.com/jyang234/verdi/internal/store"
)

// Derive is a pure function of f alone (no I/O, no wall-clock, no
// randomness): each recognizer inspects Facts and emits the
// RecognizedStates it finds evidence for; R-RR3-8 withholds an executable
// choice when two states share a branch; the result is sorted by
// (code, target) before it is returned. Derive itself never calls
// Validate (2B-F5) — codec.Canonical, the seam every caller (Task 3/4)
// goes through, is what validates, downstream of every caller of Derive.
func Derive(f Facts) Projection {
	var states []RecognizedState
	states = append(states, recognizeEmptyBranchCut(f)...)
	states = append(states, recognizeScaffoldUnstaged(f)...)
	states = append(states, recognizeArtifactsStagedUncommitted(f)...)
	states = append(states, recognizeArchiveMoveUncommitted(f)...)
	states = append(states, recognizeClosureUnpublished(f)...)
	states = append(states, recognizeBoardPushFailed(f)...)
	states = append(states, recognizeStaleLock(f)...)
	states = append(states, recognizeGovernedActionInterrupted(f)...)
	states = append(states, recognizeStrandedResidue(f)...)
	states = append(states, recognizeUnrecognized(f)...)
	if states == nil {
		states = []RecognizedState{}
	}

	sort.Slice(states, func(i, j int) bool { return stateTargetKey(states[i]) < stateTargetKey(states[j]) })

	disclosures := append([]string{}, f.Disclosures...)
	disclosures = append(disclosures, lockUndecidableDisclosures(f)...)
	sort.Strings(disclosures)
	disclosures = dedupSorted(disclosures)
	if disclosures == nil {
		disclosures = []string{}
	}

	branch := f.CurrentBranch
	if branch == "" {
		branch = "HEAD" // a detached checkout; an identity token, never a guessed branch name
	}
	head := f.Head
	if head == "" {
		head = "unknown" // HEAD itself could not be resolved (disclosed above)
	}

	return Projection{
		Schema:      SchemaID,
		Ref:         f.Ref.String(),
		Branch:      branch,
		Head:        head,
		States:      states,
		Disclosures: disclosures,
	}
}

// headInvariant is the one invariant every recognized state can always
// truthfully name regardless of its own specifics (2B-F4): Gather and
// Derive together make zero mutating git calls (proven by the AST gate),
// so HEAD's own value never moves and no ritual branch is ever touched
// by the act of producing this projection.
func headInvariant(f Facts) string {
	head := f.Head
	if head == "" {
		head = "unknown"
	}
	return fmt.Sprintf("HEAD is %s and no ritual branch was modified by this run", head)
}

// --- empty-branch-cut (R-RR3-5, R-RR3-8) -----------------------------

// ritualCandidate pairs one of the four ritual branches with its scope
// and its cut mechanism (R-RR3-5): "current" (build start, close — cut
// from whatever was checked out) or "resolved-base" (design start,
// policy adopt — cut from the resolved default branch, independent of
// the checkout).
type ritualCandidate struct {
	rb        RitualBranch
	scope     Scope
	mechanism string
}

func ritualCandidates(f Facts) []ritualCandidate {
	return []ritualCandidate{
		{f.Design, ScopeRef, "resolved-base"},
		{f.Feature, ScopeRef, "current"},
		{f.Close, ScopeRef, "current"},
		{f.PolicyAdopt, ScopeStore, "resolved-base"},
	}
}

// ambiguousClosureUncertainty is R-RR3-8's reciprocal uncertainty (review
// 2B-F2): both partner states in a same-branch ambiguity name each
// other, so an operator reading either one alone still learns the other
// exists and that no executable choice was guessed. Scoped by R-RR3-8's
// own amendment (review 2B-F8) to the SAME branch name only — an
// unrelated ritual branch's empty cut is never withheld by this ref's
// close/<name> staged closure or archive move.
func ambiguousClosureUncertainty(branch string, otherState StateCode) Uncertainty {
	return Uncertainty{
		Text:    fmt.Sprintf("%s also shows evidence of %s — withholding any executable choice until this is resolved (never guess which the operator intended)", branch, otherState),
		Witness: fmt.Sprintf("resolve the %s state first, then re-run `verdi recover`", otherState),
	}
}

// closeRitualAmbiguous reports whether f.Close (the only ritual branch
// R-RR3-8's staged-closure/archive-move partners can ever be about —
// review 2B-F8) is itself an empty cut, and if so which of the two
// artifact-shaped states it is ambiguous with.
func closeRitualAmbiguous(f Facts) (other StateCode, ambiguous bool) {
	if !f.Close.Exists || !f.Close.Empty() {
		return "", false
	}
	if closureStagedSpecName(f.StagedPaths) == f.Name {
		return StateArtifactsStagedUncommitted, true
	}
	if f.ArchiveSpecOnDisk && !f.ActiveSpecOnDisk && f.ActiveSpecAtHead && !f.ArchiveSpecAtHead {
		return StateArchiveMoveUncommitted, true
	}
	return "", false
}

// stagedPathsPhrase renders f.StagedPaths for an operator: the count
// always, the first few paths themselves, and the remainder as a count —
// never an unbounded dump into a single canonical line.
func stagedPathsPhrase(paths []string) string {
	const shown = 3
	if len(paths) <= shown {
		return fmt.Sprintf("%d staged: %s", len(paths), strings.Join(paths, ", "))
	}
	return fmt.Sprintf("%d staged: %s, and %d more", len(paths), strings.Join(paths[:shown], ", "), len(paths)-shown)
}

// uncleanTreeUncertainty implements R-RR3-21: the unwind's own "index is
// empty" and "working tree is clean" preconditions are evaluated at
// DERIVE time, from the same Facts the choice would be built from. When
// either is already false the choice cannot prove where it starts, so
// parent DC-13 leaves diagnosis only: the state is still emitted, no
// executable choice is offered, and this uncertainty names what it can
// (the staged paths from Facts; the working tree's own changed paths are
// not gathered) with the witness that settles both. Reports false when
// the tree is clean, in which case no uncertainty is added at all.
func uncleanTreeUncertainty(f Facts, branch string) (Uncertainty, bool) {
	var reasons []string
	if len(f.StagedPaths) != 0 {
		reasons = append(reasons, fmt.Sprintf("the index is not empty (%s)", stagedPathsPhrase(f.StagedPaths)))
	}
	if f.Dirty {
		reasons = append(reasons, "the working tree is not clean")
	}
	if len(reasons) == 0 {
		return Uncertainty{}, false
	}
	return Uncertainty{
		Text:    fmt.Sprintf("no unwind of %s is offered: %s — a branch cut is only unwound from an empty index and a clean working tree, and this run cannot prove that starting point", branch, strings.Join(reasons, "; ")),
		Witness: "git status --porcelain",
	}, true
}

func recognizeEmptyBranchCut(f Facts) []RecognizedState {
	ritualNames := map[string]bool{
		f.Design.Name: true, f.Feature.Name: true, f.Close.Name: true, f.PolicyAdopt.Name: true,
	}

	var states []RecognizedState
	for _, c := range ritualCandidates(f) {
		rb := c.rb
		if !rb.Exists || !rb.Empty() {
			continue
		}

		state := RecognizedState{
			Code:   StateEmptyBranchCut,
			Scope:  c.scope,
			Target: rb.Name,
			Facts: []string{
				fmt.Sprintf("%s exists at tip %s", rb.Name, rb.Tip),
				fmt.Sprintf("%s carries no commits of its own (ancestor of: %s)", rb.Name, strings.Join(rb.EmptyWitnesses, ", ")),
			},
			Uncertainties:  []Uncertainty{},
			StepsCompleted: []string{rb.Name + " was cut"},
			InvariantsHeld: []string{
				fmt.Sprintf("no commit exists on %s that is not already on %s", rb.Name, strings.Join(rb.EmptyWitnesses, " or ")),
				headInvariant(f),
			},
			Choices: []Choice{},
		}

		// R-RR3-8/2B-F8: withhold the unwind choice ONLY when THIS is the
		// close/<name> branch and this ref's own index/disk also shows an
		// uncommitted closure move — never a design/<name> or
		// feature/<name> cut, which those artifacts are never about.
		if rb.Name == "close/"+f.Name {
			if other, ambiguous := closeRitualAmbiguous(f); ambiguous {
				state.Uncertainties = append(state.Uncertainties, ambiguousClosureUncertainty(rb.Name, other))
				states = append(states, state)
				continue
			}
		}

		// R-RR3-21: both tree preconditions are evaluated HERE, not left
		// for the executor to refuse. The uncertainty is recorded before
		// the return-branch resolution below so a state that is both
		// unclean AND undecidable carries both diagnoses.
		treeUncertainty, treeUnclean := uncleanTreeUncertainty(f, rb.Name)
		if treeUnclean {
			state.Uncertainties = append(state.Uncertainties, treeUncertainty)
		}

		originalBranch, tipEqual, containing, undecidable := resolveReturnBranch(f, c, rb, ritualNames)
		if undecidable {
			state.Uncertainties = append(state.Uncertainties, undecidableReturnBranchUncertainty(c, rb, tipEqual, containing))
			states = append(states, state)
			continue
		}
		if treeUnclean {
			states = append(states, state)
			continue
		}

		id := "unwind-branch-cut:" + rb.Name
		state.Choices = append(state.Choices, Choice{
			ID:      id,
			Summary: fmt.Sprintf("unwind the empty %s branch cut", rb.Name),
			Preconditions: []string{
				fmt.Sprintf("%s still points at %s", rb.Name, rb.Tip),
				// ac-9's literal third clause, DECLARED as well as
				// re-proved (Task 4 re-review N1): apply.go's
				// reproveUnwind re-checks Empty() on its own.
				fmt.Sprintf("%s has no commits of its own", rb.Name),
				"index is empty",
				"working tree is clean",
				fmt.Sprintf("%s resolves", originalBranch),
			},
			Effects: []string{
				fmt.Sprintf("switch back to %s", originalBranch),
				fmt.Sprintf("delete %s with git branch -d", rb.Name),
			},
			Reversibility: ReversibilityNoneNeeded,
			Confirmation:  "--apply " + id,
			Postconditions: []string{
				fmt.Sprintf("%s does not exist", rb.Name),
				fmt.Sprintf("current branch is %s", originalBranch),
				// R-RR3-19: the third postcondition names the RETURN
				// BRANCH'S OWN TIP, never the cut point. R-RR3-5 is
				// explicit that "the return branch may sit ahead of the
				// ritual tip" — the containing tier resolves to a branch
				// that has since moved on, and a resolved-base cut
				// returns to a freshly re-resolved default branch under
				// no obligation to still sit at the cut point at all. A
				// correct unwind leaves HEAD wherever that branch now
				// is, which is exactly what the choice's own effects
				// promise ("switch back to <branch>").
				fmt.Sprintf("HEAD is the tip of %s", originalBranch),
			},
			Executor:       "branchcut.Unwind",
			ManualCommands: []string{},
		})
		states = append(states, state)
	}
	return states
}

// resolveReturnBranch implements R-RR3-5's two-way rule (review 2B-F1: a
// cut-from-current branch's return target is TWO-TIERED, not a single
// flat candidate list): among the OTHER local branches the ancestry
// predicate already named (excluding the four ritual branches
// themselves), tipEqual holds every one whose tip EQUALS rb's own tip
// (the branch this was actually cut from, if still exactly at the cut
// point) and containing holds every one whose tip properly descends
// (contains rb's tip as an ancestor, but has since moved on). The return
// branch is the unique tipEqual candidate when exactly one exists, else
// the unique containing candidate when exactly one exists, else
// UNDECIDABLE (SI-221) — a deleted source branch (both empty) or a tie
// (either list with more than one entry) are both undecidable, never
// guessed. A cut-from-resolved-base branch's return target is instead
// the freshly re-resolved default branch, independent of this predicate.
func resolveReturnBranch(f Facts, c ritualCandidate, rb RitualBranch, ritualNames map[string]bool) (originalBranch string, tipEqual, containing []string, undecidable bool) {
	if c.mechanism == "resolved-base" {
		if f.DefaultBranchResolved && f.DefaultBranch.BranchName != "" {
			return f.DefaultBranch.BranchName, nil, nil, false
		}
		return "", nil, nil, true
	}

	tipByName := make(map[string]string, len(f.LocalBranches))
	for _, bt := range f.LocalBranches {
		tipByName[bt.Name] = bt.Tip
	}
	for _, w := range rb.EmptyWitnesses {
		if ritualNames[w] {
			continue
		}
		if tipByName[w] == rb.Tip {
			tipEqual = append(tipEqual, w)
		} else {
			containing = append(containing, w)
		}
	}
	sort.Strings(tipEqual)
	sort.Strings(containing)

	if len(tipEqual) == 1 {
		return tipEqual[0], tipEqual, containing, false
	}
	if len(tipEqual) == 0 && len(containing) == 1 {
		return containing[0], tipEqual, containing, false
	}
	return "", tipEqual, containing, true
}

func undecidableReturnBranchUncertainty(c ritualCandidate, rb RitualBranch, tipEqual, containing []string) Uncertainty {
	if c.mechanism == "resolved-base" {
		return Uncertainty{
			Text:    fmt.Sprintf("%s's return branch (the re-resolved default branch) could not be determined", rb.Name),
			Witness: "git remote set-head origin --auto, or the CI_DEFAULT_BRANCH environment variable",
		}
	}
	if len(tipEqual) == 0 && len(containing) == 0 {
		return Uncertainty{
			Text:    fmt.Sprintf("%s's original branch cannot be determined: the branch it was cut from was deleted (no local branch reaches its cut point except %s itself)", rb.Name, rb.Name),
			Witness: "inspect the reflog's own \"Created from\" line, corroboration only",
		}
	}
	if len(tipEqual) > 1 {
		// 2B-F1: every one of these is genuinely AT the exact cut point —
		// the only case where "shares the exact cut point" is a true
		// claim.
		return Uncertainty{
			Text:    fmt.Sprintf("%s's original branch is ambiguous: more than one local branch sits at its exact tip", rb.Name),
			Witness: "the candidate branches: " + strings.Join(tipEqual, ", "),
		}
	}
	// len(tipEqual) == 0 && len(containing) != 1: every candidate here
	// only DESCENDS from rb's tip — never claim it "shares the exact cut
	// point" (2B-F1's other concrete defect).
	return Uncertainty{
		Text:    fmt.Sprintf("%s's original branch is ambiguous: more than one local branch descends from its cut point, none of them still at the exact tip", rb.Name),
		Witness: "the candidate branches: " + strings.Join(containing, ", "),
	}
}

// --- scaffold-unstaged -------------------------------------------------

func recognizeScaffoldUnstaged(f Facts) []RecognizedState {
	if !f.Design.Exists || f.CurrentBranch != f.Design.Name {
		return nil
	}
	activePrefix := store.SpecDirRelPath(store.ZoneActive, f.Name) + "/"

	var changed []string
	for _, p := range f.WorktreeChangedPaths {
		if strings.HasPrefix(p, activePrefix) {
			changed = append(changed, p)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	for _, p := range f.StagedPaths {
		if strings.HasPrefix(p, activePrefix) {
			return nil // already staged: not this state
		}
	}
	sort.Strings(changed)

	id := "commit-scaffold:" + f.Design.Name
	return []RecognizedState{{
		Code:           StateScaffoldUnstaged,
		Scope:          ScopeRef,
		Target:         f.Design.Name,
		Facts:          []string{fmt.Sprintf("%s has unstaged changes under %s: %s", f.Design.Name, activePrefix, strings.Join(changed, ", "))},
		Uncertainties:  []Uncertainty{},
		StepsCompleted: []string{"design start scaffolded and committed the spec"},
		InvariantsHeld: []string{headInvariant(f)},
		Choices: []Choice{{
			ID:             id,
			Summary:        fmt.Sprintf("commit the in-progress scaffold edits on %s", f.Design.Name),
			Preconditions:  []string{fmt.Sprintf("%s is checked out", f.Design.Name)},
			Effects:        []string{"stage and commit the scaffold edits"},
			Reversibility:  ReversibilityReversible,
			Confirmation:   "none: no executor",
			Postconditions: []string{"working tree is clean"},
			Executor:       "none",
			ManualCommands: []string{
				"git add -- " + activePrefix,
				fmt.Sprintf("git commit -m \"design: %s\"", f.Name),
			},
		}},
	}}
}

// --- artifacts-staged-uncommitted / archive-move-uncommitted -----------

// closureStagedSpecName is close.go:1016 (closureResidueName)'s own pure
// index-shape predicate, copied here (recovery cannot import cmd/verdi,
// package main): the index carries nothing but one spec's own closure
// paths (both the active-zone deletion and the archive-zone tree, no
// other path), returning that spec's name or "" otherwise. Pinned
// against close.go's own source text by
// TestClosureAdviceCommandsPinned (2B-F6).
func closureStagedSpecName(paths []string) string {
	const activeRoot = ".verdi/specs/active/"
	const archiveRoot = ".verdi/specs/archive/"

	name := ""
	sawActive, sawArchive := false, false
	for _, p := range paths {
		rest, inArchive := strings.CutPrefix(p, archiveRoot)
		if !inArchive {
			var inActive bool
			if rest, inActive = strings.CutPrefix(p, activeRoot); !inActive {
				return ""
			}
		}
		specName, _, hasChild := strings.Cut(rest, "/")
		if !hasChild || specName == "" {
			return ""
		}
		if name == "" {
			name = specName
		} else if specName != name {
			return ""
		}
		sawArchive = sawArchive || inArchive
		sawActive = sawActive || !inArchive
	}
	if !sawActive || !sawArchive {
		return ""
	}
	return name
}

// close.go:1059 (closureResidueRefusal) and close.go:1123
// (reportUncommittedArchiveMove)'s own advice commands, copied here
// verbatim as templates — recovery cannot import cmd/verdi (package
// main). Pinned by TestClosureAdviceCommandsPinned (2B-F6), which reads
// cmd/verdi/close.go as a file and asserts these literals occur in it,
// so drift in the source trips the recovery copy.
const (
	closureResidueCompleteCommand    = "git commit"
	closureResidueRestoreActiveTmpl  = "git restore --source=HEAD --staged --worktree -- %s"
	closureResidueUnstageArchiveTmpl = "git restore --staged -- %s"
	closureResidueDeleteArchiveTmpl  = "rm -rf %s"
)

func closureResidueManualCommands(active, archive string) []string {
	return []string{
		closureResidueCompleteCommand,
		fmt.Sprintf(closureResidueRestoreActiveTmpl, active),
		fmt.Sprintf(closureResidueUnstageArchiveTmpl, archive),
		fmt.Sprintf(closureResidueDeleteArchiveTmpl, archive),
	}
}

func recognizeArtifactsStagedUncommitted(f Facts) []RecognizedState {
	if closureStagedSpecName(f.StagedPaths) != f.Name || f.Name == "" {
		return nil
	}
	active := store.SpecDirRelPath(store.ZoneActive, f.Name)
	archive := store.SpecDirRelPath(store.ZoneArchive, f.Name)
	target := "close/" + f.Name
	id := "resolve-staged-closure:" + target

	uncertainties := []Uncertainty{}
	if f.Close.Exists && f.Close.Empty() {
		// 2B-F2: reciprocal half of recognizeEmptyBranchCut's own
		// uncertainty — both partner states name each other.
		uncertainties = append(uncertainties, ambiguousClosureUncertainty(target, StateEmptyBranchCut))
	}

	return []RecognizedState{{
		Code:          StateArtifactsStagedUncommitted,
		Scope:         ScopeRef,
		Target:        target,
		Facts:         []string{fmt.Sprintf("the index carries spec/%s's own closure paths (%s, %s) and nothing else", f.Name, active, archive)},
		Uncertainties: uncertainties,
		// vocab:identity — "close" names the git verb/branch-prefix identity, never the renameable lifecycle status
		StepsCompleted: []string{"close staged the active-to-archive move"},
		InvariantsHeld: []string{headInvariant(f)},
		Choices: []Choice{{
			ID:             id,
			Summary:        "resolve the staged, uncommitted closure of spec/" + f.Name,
			Preconditions:  []string{fmt.Sprintf("the index still carries only spec/%s's own closure paths", f.Name)},
			Effects:        []string{"commit the staged closure, or abandon it and restore the checkout"},
			Reversibility:  ReversibilityReversible,
			Confirmation:   "none: no executor",
			Postconditions: []string{fmt.Sprintf("the index no longer carries an uncommitted closure for spec/%s", f.Name)},
			Executor:       "none",
			ManualCommands: closureResidueManualCommands(active, archive),
		}},
	}}
}

func recognizeArchiveMoveUncommitted(f Facts) []RecognizedState {
	if !f.ArchiveSpecOnDisk || f.ActiveSpecOnDisk || !f.ActiveSpecAtHead || f.ArchiveSpecAtHead {
		return nil
	}
	if closureStagedSpecName(f.StagedPaths) == f.Name {
		return nil // classified as artifacts-staged-uncommitted instead
	}
	active := store.SpecDirRelPath(store.ZoneActive, f.Name)
	archive := store.SpecDirRelPath(store.ZoneArchive, f.Name)
	target := "close/" + f.Name
	id := "restore-uncommitted-archive-move:" + target

	uncertainties := []Uncertainty{}
	if f.Close.Exists && f.Close.Empty() {
		uncertainties = append(uncertainties, ambiguousClosureUncertainty(target, StateEmptyBranchCut))
	}

	return []RecognizedState{{
		Code:   StateArchiveMoveUncommitted,
		Scope:  ScopeRef,
		Target: target,
		Facts: []string{
			fmt.Sprintf("%s is absent on disk but present in HEAD's tree", active),
			fmt.Sprintf("%s is present on disk but absent from HEAD's tree", archive),
		},
		Uncertainties: uncertainties,
		// vocab:identity — "close" names the git verb/branch-prefix identity, never the renameable lifecycle status
		StepsCompleted: []string{"close moved the spec directory on disk"},
		InvariantsHeld: []string{headInvariant(f)},
		Choices: []Choice{{
			ID:             id,
			Summary:        "restore the uncommitted archive move for spec/" + f.Name,
			Preconditions:  []string{fmt.Sprintf("%s is still absent on disk", active)},
			Effects:        []string{"restore the checkout to HEAD's shape"},
			Reversibility:  ReversibilityReversible,
			Confirmation:   "none: no executor",
			Postconditions: []string{fmt.Sprintf("%s exists on disk again", active)},
			Executor:       "none",
			ManualCommands: []string{
				fmt.Sprintf(closureResidueRestoreActiveTmpl, active),
				fmt.Sprintf(closureResidueDeleteArchiveTmpl, archive),
			},
		}},
	}}
}

// --- closure-unpublished / board-push-failed (R-RR3-12, R-RR3-13) ------

func remoteComparisonUncertainty() Uncertainty {
	return Uncertainty{
		Text:    "this comparison is against the last-fetched remote-tracking state, not a live one",
		Witness: "git fetch origin",
	}
}

func aheadOrNoRemoteFact(rb RitualBranch) string {
	if !rb.HasRemoteTracking {
		return fmt.Sprintf("%s has no remote-tracking branch", rb.Name)
	}
	return fmt.Sprintf("%s is %d commit(s) ahead of its remote-tracking branch", rb.Name, rb.Ahead)
}

// hasOtherLocalBranch reports whether at least one local branch besides
// rb itself exists (2B-F9): the ancestry predicate needs at least one
// other branch to test against, so when none exists at all, "not empty"
// cannot be told apart from "cannot be decided" — Empty() reads false in
// both cases.
func hasOtherLocalBranch(f Facts, rb RitualBranch) bool {
	for _, bt := range f.LocalBranches {
		if bt.Name != rb.Name {
			return true
		}
	}
	return false
}

// ownCommitUncertainty is 2B-F9's own fix: when no other local branch
// exists at all, closure-unpublished/board-push-failed must not assert
// "carries its own commit" as a fact — it becomes an uncertainty with
// the witness that would decide it.
func ownCommitUncertainty(f Facts, rb RitualBranch) Uncertainty {
	base := "<default branch>"
	if f.DefaultBranchResolved {
		base = f.DefaultBranch.Ref
	}
	return Uncertainty{
		Text:    fmt.Sprintf("whether %s carries a commit of its own could not be determined: no other local branch exists to test its ancestry against", rb.Name),
		Witness: fmt.Sprintf("git log %s..%s", base, rb.Name),
	}
}

func recognizeClosureUnpublished(f Facts) []RecognizedState {
	rb := f.Close
	if !rb.Exists || rb.Empty() || !rb.RemoteChecked {
		return nil
	}
	if rb.HasRemoteTracking && rb.Ahead == 0 {
		return nil
	}
	target := rb.Name
	id := "publish-closure:" + target

	steps := []string{}
	uncertainties := []Uncertainty{remoteComparisonUncertainty()}
	if hasOtherLocalBranch(f, rb) {
		steps = append(steps, target+" carries its own closure commit")
	} else {
		uncertainties = append(uncertainties, ownCommitUncertainty(f, rb))
	}

	return []RecognizedState{{
		Code:           StateClosureUnpublished,
		Scope:          ScopeRef,
		Target:         target,
		Facts:          []string{aheadOrNoRemoteFact(rb)},
		Uncertainties:  uncertainties,
		StepsCompleted: steps,
		InvariantsHeld: []string{headInvariant(f)},
		Choices: []Choice{{
			ID:             id,
			Summary:        "publish " + target,
			Preconditions:  []string{target + " still carries its own closure commit"},
			Effects:        []string{"push " + target + " to origin"},
			Reversibility:  ReversibilityReversible,
			Confirmation:   "none: no executor",
			Postconditions: []string{target + " has a remote-tracking branch with nothing ahead"},
			Executor:       "none",
			ManualCommands: []string{"git push -u origin " + target},
		}},
	}}
}

func recognizeBoardPushFailed(f Facts) []RecognizedState {
	rb := f.Design
	if !rb.Exists || rb.Empty() || !rb.RemoteChecked {
		return nil
	}
	if rb.HasRemoteTracking && rb.Ahead == 0 {
		return nil
	}
	target := rb.Name
	id := "publish-board-branch:" + target

	steps := []string{}
	uncertainties := []Uncertainty{
		remoteComparisonUncertainty(),
		{
			Text:    fmt.Sprintf("whether a push of %s was attempted and failed, or was never attempted at all — no artifact records a failed push", target),
			Witness: fmt.Sprintf("the board's own commit response, or git push -u origin %s", target),
		},
	}
	if hasOtherLocalBranch(f, rb) {
		steps = append(steps, target+" carries a commit not on its remote-tracking branch")
	} else {
		uncertainties = append(uncertainties, ownCommitUncertainty(f, rb))
	}

	return []RecognizedState{{
		Code:           StateBoardPushFailed,
		Scope:          ScopeRef,
		Target:         target,
		Facts:          []string{aheadOrNoRemoteFact(rb)},
		Uncertainties:  uncertainties,
		StepsCompleted: steps,
		InvariantsHeld: []string{headInvariant(f)},
		Choices: []Choice{{
			ID:             id,
			Summary:        "publish " + target,
			Preconditions:  []string{target + " still carries a commit not on its remote-tracking branch"},
			Effects:        []string{"push " + target + " to origin"},
			Reversibility:  ReversibilityReversible,
			Confirmation:   "none: no executor",
			Postconditions: []string{target + " has a remote-tracking branch with nothing ahead"},
			Executor:       "none",
			ManualCommands: []string{"git push -u origin " + target},
		}},
	}}
}

// --- stale-lock (R-RR3-6) ----------------------------------------------

func recognizeStaleLock(f Facts) []RecognizedState {
	var states []RecognizedState
	states = append(states, staleLockState(f, f.WriterLock, ScopeStore)...)
	for _, lf := range f.RitualLocks {
		// A ritual branch's own worktree lock is ref-scoped; the writer
		// lock and every execution-workspace lock are store-scoped.
		states = append(states, staleLockState(f, lf, ScopeRef)...)
	}
	for _, lf := range f.WorkspaceLocks {
		states = append(states, staleLockState(f, lf, ScopeStore)...)
	}
	sort.Slice(states, func(i, j int) bool { return states[i].Target < states[j].Target })
	return states
}

func staleLockState(f Facts, lf LockFact, scope Scope) []RecognizedState {
	if lf.Inspection.Status != filelock.LockStale {
		return nil
	}
	id := "remove-stale-lock:" + lf.Path
	return []RecognizedState{{
		Code:   StateStaleLock,
		Scope:  scope,
		Target: lf.Path,
		Facts: []string{
			fmt.Sprintf("pid %d", lf.Inspection.Info.PID),
			fmt.Sprintf("start %d", lf.Inspection.Info.Start),
			lf.Inspection.Reason,
		},
		Uncertainties:  []Uncertainty{},
		StepsCompleted: []string{},
		InvariantsHeld: []string{headInvariant(f)},
		Choices: []Choice{{
			ID:             id,
			Summary:        "remove the stale lock " + lf.Path,
			Preconditions:  []string{fmt.Sprintf("%s still names pid %d", lf.Path, lf.Inspection.Info.PID)},
			Effects:        []string{"delete " + lf.Path},
			Reversibility:  ReversibilityIrreversible,
			Confirmation:   "none: no executor",
			Postconditions: []string{lf.Path + " does not exist"},
			Executor:       "none",
			ManualCommands: []string{"rm " + lf.Path},
		}},
	}}
}

// lockUndecidableDisclosures implements R-RR3-6/co-6: an undecidable lock
// inspection never becomes a stale-lock state — it is disclosed with the
// witness that would decide it.
func lockUndecidableDisclosures(f Facts) []string {
	var out []string
	all := append([]LockFact{f.WriterLock}, f.RitualLocks...)
	all = append(all, f.WorkspaceLocks...)
	for _, lf := range all {
		if lf.Inspection.Status != filelock.LockUndecidable {
			continue
		}
		out = append(out, fmt.Sprintf(
			"lock %s: pid %d's liveness could not be decided (%s); witness: ps -o lstart= -p %d",
			lf.Path, lf.Inspection.Info.PID, lf.Inspection.Reason, lf.Inspection.Info.PID,
		))
	}
	return out
}

// --- governed-action-interrupted (R-RR3-11) -----------------------------

func recognizeGovernedActionInterrupted(f Facts) []RecognizedState {
	var states []RecognizedState

	if f.Journal.Present && f.Journal.Decoded && knownJournalPhases[f.Journal.Phase] {
		steps := f.Journal.Steps
		if steps == nil {
			steps = []string{}
		}
		id := "resolve-journal:" + f.Journal.Path
		states = append(states, RecognizedState{
			Code:   StateGovernedActionInterrupted,
			Scope:  ScopeRef,
			Target: f.Journal.Path,
			// vocab:identity — "draft-mutation" names the internal/draftmutation package/artifact identity, not a lifecycle status
			Facts:          []string{fmt.Sprintf("the draft-mutation journal for %s is in phase %q", f.Journal.Spec, f.Journal.Phase)},
			Uncertainties:  []Uncertainty{},
			StepsCompleted: steps,
			InvariantsHeld: []string{headInvariant(f)},
			Choices: []Choice{{
				ID: id,
				// vocab:identity — "draft-mutation" names the internal/draftmutation package/artifact identity, not a lifecycle status
				Summary:        "resolve the interrupted draft-mutation journal for " + f.Journal.Spec,
				Preconditions:  []string{fmt.Sprintf("%s is still in phase %q", f.Journal.Path, f.Journal.Phase)},
				Effects:        []string{"complete or roll back the journal"},
				Reversibility:  ReversibilityReversible,
				Confirmation:   "none: no executor",
				Postconditions: []string{f.Journal.Path + " no longer exists"},
				Executor:       "none",
				// vocab:identity — "draft" names internal/draftmutation.LockedWriter.Recover's own board-draft-write identity, not a lifecycle status
				ManualCommands: []string{"next board draft write completes or rolls back this journal (internal/draftmutation.LockedWriter.Recover)"},
			}},
		})
	}

	for _, u := range f.WorkspaceUnits {
		if u.HasUnit || (!u.HasRequestStaging && !u.HasLock) {
			continue
		}
		id := "resolve-orphan-workspace:" + u.ID
		states = append(states, RecognizedState{
			Code:           StateGovernedActionInterrupted,
			Scope:          ScopeStore,
			Target:         u.ID,
			Facts:          []string{fmt.Sprintf("execution-workspace %s has a sibling entry with no unit directory", u.ID)},
			Uncertainties:  []Uncertainty{},
			StepsCompleted: []string{},
			InvariantsHeld: []string{headInvariant(f)},
			Choices: []Choice{{
				ID:             id,
				Summary:        "reclaim the orphan execution-workspace entry " + u.ID,
				Preconditions:  []string{fmt.Sprintf("%s still has no unit directory", u.ID)},
				Effects:        []string{"reclaim the orphan execution-workspace entry"},
				Reversibility:  ReversibilityIrreversible,
				Confirmation:   "none: no executor",
				Postconditions: []string{fmt.Sprintf("%s's siblings no longer exist", u.ID)},
				Executor:       "none",
				ManualCommands: []string{"verdi gc"},
			}},
		})
	}
	return states
}

// --- stranded-residue (R-RR3-14) ----------------------------------------

func recognizeStrandedResidue(f Facts) []RecognizedState {
	var states []RecognizedState
	for _, row := range f.ReclaimRows {
		target := row.Unit.Branch
		switch row.Kind {
		case reclaim.KindEligible:
			id := "reclaim:" + target
			postconditions := []string{fmt.Sprintf("%s does not exist", target)}
			if row.Unit.HasWorktree() {
				postconditions = append(postconditions, fmt.Sprintf("%s does not exist", row.Unit.WorktreePath))
			}
			states = append(states, RecognizedState{
				Code:           StateStrandedResidue,
				Scope:          ScopeRef,
				Target:         target,
				Facts:          []string{row.Line()},
				Uncertainties:  []Uncertainty{},
				StepsCompleted: []string{},
				InvariantsHeld: []string{headInvariant(f)},
				Choices: []Choice{{
					ID:             id,
					Summary:        "reclaim " + target,
					Preconditions:  []string{fmt.Sprintf("reclaim plan still lists %s as eligible", target)},
					Effects:        []string{row.Line()},
					Reversibility:  ReversibilityIrreversible,
					Confirmation:   "--apply " + id,
					Postconditions: postconditions,
					Executor:       "reclaim.Apply",
					ManualCommands: []string{},
				}},
			})
		case reclaim.KindKept:
			states = append(states, RecognizedState{
				Code:           StateStrandedResidue,
				Scope:          ScopeRef,
				Target:         target,
				Facts:          []string{row.Line()},
				Uncertainties:  []Uncertainty{},
				StepsCompleted: []string{},
				InvariantsHeld: []string{headInvariant(f)},
				Choices:        []Choice{},
			})
		}
	}
	return states
}

// --- unrecognized (R-RR3-16) ---------------------------------------------

// knownJournalPhases is the draft-mutation journal's own closed phase
// vocabulary this projection understands — today just "prepared"
// (internal/draftmutation.LockedWriter never lands any other phase in
// the journal document; a value outside this set is not a state a
// governed action recovery reasons about).
var knownJournalPhases = map[string]bool{"prepared": true}

func newUnrecognizedState(f Facts, scope Scope, target string, facts []string, witness string) RecognizedState {
	return RecognizedState{
		Code:           StateUnrecognized,
		Scope:          scope,
		Target:         target,
		Facts:          facts,
		Uncertainties:  []Uncertainty{{Text: "state is outside the ritual inventory (dc-7)", Witness: witness}},
		StepsCompleted: []string{},
		InvariantsHeld: []string{headInvariant(f)},
		Choices:        []Choice{},
	}
}

// recognizeUnrecognized (R-RR3-16, amending Step 13's withdrawn literal
// reading) fires only for an observation that contradicts the closed
// ritual inventory (dc-7), never for a ritual branch that merely carries
// commits of its own — that is ordinary in-progress work, not a
// recovery state:
//
//   - a lock (writer, ritual, or execution-workspace) whose body
//     filelock.Inspect could not even parse (a malformed, complete-but-
//     garbled body — distinct from stale/held/undecidable, all of which
//     Inspect itself resolves);
//   - a draft-mutation journal that is present but did not decode, or
//     decoded to a phase outside knownJournalPhases;
//   - an execution-workspace directory entry ClassifyEntry does not
//     recognize (grammar-external);
//   - the target spec present on disk in BOTH the active and archive
//     zones at once (an on-disk shape no ritual ever produces).
func recognizeUnrecognized(f Facts) []RecognizedState {
	var states []RecognizedState

	states = append(states, unrecognizedLockStates(f, f.WriterLock, ScopeStore)...)
	for _, lf := range f.RitualLocks {
		states = append(states, unrecognizedLockStates(f, lf, ScopeRef)...)
	}
	for _, lf := range f.WorkspaceLocks {
		states = append(states, unrecognizedLockStates(f, lf, ScopeStore)...)
	}

	if f.Journal.Present && (!f.Journal.Decoded || !knownJournalPhases[f.Journal.Phase]) {
		// vocab:identity — "draft-mutation" names the internal/draftmutation package/artifact identity, not a lifecycle status
		fact := fmt.Sprintf("draft-mutation journal %s is present but could not be decoded: %s", f.Journal.Path, f.Journal.DecodeError)
		if f.Journal.Decoded {
			// vocab:identity — "draft-mutation" names the internal/draftmutation package/artifact identity, not a lifecycle status
			fact = fmt.Sprintf("draft-mutation journal %s decoded with phase %q, outside the known set", f.Journal.Path, f.Journal.Phase)
		}
		states = append(states, newUnrecognizedState(f, ScopeRef, f.Journal.Path, []string{fact}, f.Journal.Path))
	}

	for _, name := range f.WorkspaceUnclassified {
		states = append(states, newUnrecognizedState(
			f, ScopeStore, name,
			[]string{fmt.Sprintf("execution-workspace entry %q does not match the workspace-unit grammar", name)},
			name,
		))
	}

	if f.ActiveSpecOnDisk && f.ArchiveSpecOnDisk && f.Name != "" {
		target := f.Ref.String()
		states = append(states, newUnrecognizedState(
			f, ScopeRef, target,
			[]string{fmt.Sprintf("%s exists on disk in both the active and archive zones", target)},
			target,
		))
	}

	sort.Slice(states, func(i, j int) bool { return states[i].Target < states[j].Target })
	return states
}

func unrecognizedLockStates(f Facts, lf LockFact, scope Scope) []RecognizedState {
	if lf.ReadError == "" {
		return nil
	}
	return []RecognizedState{newUnrecognizedState(
		f, scope, lf.Path,
		[]string{fmt.Sprintf("%s could not be inspected: %s", lf.Path, lf.ReadError)},
		lf.Path,
	)}
}
