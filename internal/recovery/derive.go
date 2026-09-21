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
// (code, target) and validated before it is returned.
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

	claimed := make(map[string]bool, len(states))
	for _, s := range states {
		claimed[s.Target] = true
	}
	states = append(states, recognizeUnrecognized(f, claimed)...)
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
			InvariantsHeld: []string{},
			Choices:        []Choice{},
		}

		// R-RR3-8: withhold the unwind choice when this SAME ref's index
		// or disk also shows an uncommitted closure move — never guess
		// which the operator intended.
		if c.scope == ScopeRef && f.Name != "" {
			staged := closureStagedSpecName(f.StagedPaths) == f.Name
			archiveMove := f.ArchiveSpecOnDisk && !f.ActiveSpecOnDisk && f.ActiveSpecAtHead && !f.ArchiveSpecAtHead
			if staged || archiveMove {
				other := StateArchiveMoveUncommitted
				if staged {
					other = StateArtifactsStagedUncommitted
				}
				state.Uncertainties = append(state.Uncertainties, Uncertainty{
					Text:    fmt.Sprintf("%s looks like an empty branch cut, but spec/%s's index/disk also shows evidence of %s — withholding the unwind choice rather than guessing which the operator intended", rb.Name, f.Name, other),
					Witness: fmt.Sprintf("resolve the %s state first, then re-run `verdi recover`", other),
				})
				states = append(states, state)
				continue
			}
		}

		originalBranch, candidateNames, undecidable := resolveReturnBranch(f, c, rb, ritualNames)
		if undecidable {
			state.Uncertainties = append(state.Uncertainties, undecidableReturnBranchUncertainty(c, rb, candidateNames))
			states = append(states, state)
			continue
		}

		id := "unwind-branch-cut:" + rb.Name
		state.Choices = append(state.Choices, Choice{
			ID:      id,
			Summary: fmt.Sprintf("unwind the empty %s branch cut", rb.Name),
			Preconditions: []string{
				fmt.Sprintf("%s still points at %s", rb.Name, rb.Tip),
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
				fmt.Sprintf("HEAD is %s", rb.Tip),
			},
			Executor:       "branchcut.Unwind",
			ManualCommands: []string{},
		})
		states = append(states, state)
	}
	return states
}

// resolveReturnBranch implements R-RR3-5's two-way rule: a cut-from-
// current branch's return target is the unique OTHER local branch its
// own ancestry predicate already named (excluding the four ritual
// branches themselves); a cut-from-resolved-base branch's return target
// is the freshly re-resolved default branch. Either can be undecidable.
func resolveReturnBranch(f Facts, c ritualCandidate, rb RitualBranch, ritualNames map[string]bool) (originalBranch string, candidateNames []string, undecidable bool) {
	if c.mechanism == "resolved-base" {
		if f.DefaultBranchResolved && f.DefaultBranch.BranchName != "" {
			return f.DefaultBranch.BranchName, nil, false
		}
		return "", nil, true
	}
	for _, w := range rb.EmptyWitnesses {
		if ritualNames[w] {
			continue
		}
		candidateNames = append(candidateNames, w)
	}
	sort.Strings(candidateNames)
	if len(candidateNames) == 1 {
		return candidateNames[0], candidateNames, false
	}
	return "", candidateNames, true
}

func undecidableReturnBranchUncertainty(c ritualCandidate, rb RitualBranch, candidateNames []string) Uncertainty {
	if c.mechanism == "resolved-base" {
		return Uncertainty{
			Text:    fmt.Sprintf("%s's return branch (the re-resolved default branch) could not be determined", rb.Name),
			Witness: "git remote set-head origin --auto, or the CI_DEFAULT_BRANCH environment variable",
		}
	}
	if len(candidateNames) == 0 {
		return Uncertainty{
			Text:    fmt.Sprintf("%s's original branch cannot be determined: the branch it was cut from was deleted (no local branch reaches its cut point except %s itself)", rb.Name, rb.Name),
			Witness: "inspect the reflog's own \"Created from\" line, corroboration only",
		}
	}
	return Uncertainty{
		Text:    fmt.Sprintf("%s's original branch is ambiguous: more than one local branch shares its exact cut point", rb.Name),
		Witness: "the candidate branches: " + strings.Join(candidateNames, ", "),
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
		InvariantsHeld: []string{},
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
// other path), returning that spec's name or "" otherwise.
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
// main). Pinned by TestClosureAdviceCommandsPinned against close.go's
// own text.
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

	return []RecognizedState{{
		Code:           StateArtifactsStagedUncommitted,
		Scope:          ScopeRef,
		Target:         target,
		Facts:          []string{fmt.Sprintf("the index carries spec/%s's own closure paths (%s, %s) and nothing else", f.Name, active, archive)},
		Uncertainties:  []Uncertainty{},
		StepsCompleted: []string{"close staged the active-to-archive move"},
		InvariantsHeld: []string{},
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

	return []RecognizedState{{
		Code:   StateArchiveMoveUncommitted,
		Scope:  ScopeRef,
		Target: target,
		Facts: []string{
			fmt.Sprintf("%s is absent on disk but present in HEAD's tree", active),
			fmt.Sprintf("%s is present on disk but absent from HEAD's tree", archive),
		},
		Uncertainties:  []Uncertainty{},
		StepsCompleted: []string{"close moved the spec directory on disk"},
		InvariantsHeld: []string{},
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

func recognizeClosureUnpublished(f Facts) []RecognizedState {
	rb := f.Close
	if !rb.Exists || rb.Empty() {
		return nil
	}
	if rb.HasRemoteTracking && rb.Ahead == 0 {
		return nil
	}
	target := rb.Name
	id := "publish-closure:" + target

	return []RecognizedState{{
		Code:           StateClosureUnpublished,
		Scope:          ScopeRef,
		Target:         target,
		Facts:          []string{aheadOrNoRemoteFact(rb)},
		Uncertainties:  []Uncertainty{remoteComparisonUncertainty()},
		StepsCompleted: []string{target + " carries its own closure commit"},
		InvariantsHeld: []string{},
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
	if !rb.Exists || rb.Empty() {
		return nil
	}
	if rb.HasRemoteTracking && rb.Ahead == 0 {
		return nil
	}
	target := rb.Name
	id := "publish-board-branch:" + target

	return []RecognizedState{{
		Code:   StateBoardPushFailed,
		Scope:  ScopeRef,
		Target: target,
		Facts:  []string{aheadOrNoRemoteFact(rb)},
		Uncertainties: []Uncertainty{
			remoteComparisonUncertainty(),
			{
				Text:    fmt.Sprintf("whether a push of %s was attempted and failed, or was never attempted at all — no artifact records a failed push", target),
				Witness: fmt.Sprintf("the board's own commit response, or git push -u origin %s", target),
			},
		},
		StepsCompleted: []string{target + " carries a commit not on its remote-tracking branch"},
		InvariantsHeld: []string{},
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
	states = append(states, staleLockState(f.WriterLock, ScopeStore)...)
	for _, lf := range f.RitualLocks {
		// A ritual branch's own worktree lock is ref-scoped; the writer
		// lock and every execution-workspace lock are store-scoped.
		states = append(states, staleLockState(lf, ScopeRef)...)
	}
	for _, lf := range f.WorkspaceLocks {
		states = append(states, staleLockState(lf, ScopeStore)...)
	}
	sort.Slice(states, func(i, j int) bool { return states[i].Target < states[j].Target })
	return states
}

func staleLockState(lf LockFact, scope Scope) []RecognizedState {
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
		InvariantsHeld: []string{},
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

	if f.Journal.Present && f.Journal.Phase == "prepared" {
		steps := f.Journal.Steps
		if steps == nil {
			steps = []string{}
		}
		id := "resolve-journal:" + f.Journal.Path
		states = append(states, RecognizedState{
			Code:           StateGovernedActionInterrupted,
			Scope:          ScopeRef,
			Target:         f.Journal.Path,
			Facts:          []string{fmt.Sprintf("the draft-mutation journal for %s is in phase %q", f.Journal.Spec, f.Journal.Phase)},
			Uncertainties:  []Uncertainty{},
			StepsCompleted: steps,
			InvariantsHeld: []string{},
			Choices: []Choice{{
				ID:             id,
				Summary:        "resolve the interrupted draft-mutation journal for " + f.Journal.Spec,
				Preconditions:  []string{fmt.Sprintf("%s is still in phase %q", f.Journal.Path, f.Journal.Phase)},
				Effects:        []string{"complete or roll back the journal"},
				Reversibility:  ReversibilityReversible,
				Confirmation:   "none: no executor",
				Postconditions: []string{f.Journal.Path + " no longer exists"},
				Executor:       "none",
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
			InvariantsHeld: []string{},
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
				InvariantsHeld: []string{},
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
				InvariantsHeld: []string{},
				Choices:        []Choice{},
			})
		}
	}
	return states
}

// --- unrecognized --------------------------------------------------------

// recognizeUnrecognized fires for a ritual branch that exists with
// commits beyond its cut (never empty) and whose name was not already
// claimed as another recognized state's own Target (dc-7: the closed
// inventory's own catch-all — "state is outside the ritual inventory").
func recognizeUnrecognized(f Facts, claimed map[string]bool) []RecognizedState {
	var states []RecognizedState
	for _, c := range ritualCandidates(f) {
		rb := c.rb
		if !rb.Exists || rb.Empty() || claimed[rb.Name] {
			continue
		}
		states = append(states, RecognizedState{
			Code:   StateUnrecognized,
			Scope:  c.scope,
			Target: rb.Name,
			Facts:  []string{fmt.Sprintf("%s exists at tip %s with commits beyond its cut, and matches no recognized ritual state", rb.Name, rb.Tip)},
			Uncertainties: []Uncertainty{{
				Text:    "state is outside the ritual inventory (dc-7)",
				Witness: "manual inspection of " + rb.Name,
			}},
			StepsCompleted: []string{},
			InvariantsHeld: []string{},
			Choices:        []Choice{},
		})
	}
	return states
}
