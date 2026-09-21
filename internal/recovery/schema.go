// Package recovery derives the readiness-recovery projection (ac-8): a
// read-only report of which of a closed inventory of interrupted-ritual
// states a ref's repository and store facts currently match, each with
// its evidence, uncertainties, and — when one exists — the exact,
// re-provable executable or manual choice that resolves it.
//
// The projection is never authority (co-2): no readiness or recovery
// file, cache, status field, transition, receipt, event log, or artifact
// kind is added by this package. Removing this package leaves every
// canonical artifact, gate, transition, and recovery fact intact — the
// projection is derived fresh from facts that already exist (git,
// on-disk store state, and the two existing scan/plan primitives
// internal/residue and internal/reclaim already expose), never cached as
// its own truth.
//
// recover.go's package comment (cmd/verdi, a later task) documents this
// package's downstream exit-code divergence from `verdi journey`'s "exit
// 1 is unreachable" doctrine (R-RR3-1, ledger SI-220): a RecognizedState
// here is a verdict about recognized state, not a lifecycle authority
// claim.
package recovery

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// SchemaID is the only accepted Projection.Schema value.
const SchemaID = "verdi.recovery-projection/v1"

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// StateCode is the closed inventory of interrupted-ritual states this
// package recognizes (dc-7: nothing outside this closed set is inferred).
type StateCode string

const (
	StateEmptyBranchCut             StateCode = "empty-branch-cut"
	StateScaffoldUnstaged           StateCode = "scaffold-unstaged"
	StateArtifactsStagedUncommitted StateCode = "artifacts-staged-uncommitted"
	StateArchiveMoveUncommitted     StateCode = "archive-move-uncommitted"
	StateClosureUnpublished         StateCode = "closure-unpublished"
	StateBoardPushFailed            StateCode = "board-push-failed"
	StateStaleLock                  StateCode = "stale-lock"
	StateGovernedActionInterrupted  StateCode = "governed-action-interrupted"
	StateStrandedResidue            StateCode = "stranded-residue"
	StateUnrecognized               StateCode = "unrecognized"
)

// validStateCodes is StateCode's closed set (Validate fails closed on any
// other value).
var validStateCodes = map[StateCode]bool{
	StateEmptyBranchCut:             true,
	StateScaffoldUnstaged:           true,
	StateArtifactsStagedUncommitted: true,
	StateArchiveMoveUncommitted:     true,
	StateClosureUnpublished:         true,
	StateBoardPushFailed:            true,
	StateStaleLock:                  true,
	StateGovernedActionInterrupted:  true,
	StateStrandedResidue:            true,
	StateUnrecognized:               true,
}

// Scope is a recognized state's reach: "ref" (this ref's own ritual
// branches/artifacts) or "store" (R-RR3-4: policy/adopt, locks,
// execution-workspace residue, and the draft-mutation journal, reported
// under every ref's projection).
type Scope string

const (
	ScopeRef   Scope = "ref"
	ScopeStore Scope = "store"
)

// Reversibility is a choice's own closed posture.
type Reversibility string

const (
	ReversibilityReversible   Reversibility = "reversible"
	ReversibilityIrreversible Reversibility = "irreversible"
	ReversibilityNoneNeeded   Reversibility = "none-needed"
)

const executorNone = "none"

var validExecutors = map[string]bool{
	"branchcut.Unwind": true,
	"reclaim.Apply":    true,
	executorNone:       true,
}

var validReversibilities = map[Reversibility]bool{
	ReversibilityReversible:   true,
	ReversibilityIrreversible: true,
	ReversibilityNoneNeeded:   true,
}

var validScopes = map[Scope]bool{ScopeRef: true, ScopeStore: true}

// Uncertainty is one unresolved fact the projection discloses rather than
// guesses (co-6), together with the witness that would settle it.
type Uncertainty struct {
	Text    string `json:"text"`
	Witness string `json:"witness"` // the command or observation that would settle it
}

// Choice is one way to resolve a recognized state: either executable
// under `verdi recover --apply <id>` (Executor names one of the two
// permitted executors and ManualCommands is empty), or manual (Executor
// is "none" and ManualCommands carries the exact command line(s) an
// operator would run — R-RR3-7).
type Choice struct {
	ID             string        `json:"id"`
	Summary        string        `json:"summary"`
	Preconditions  []string      `json:"preconditions"`
	Effects        []string      `json:"effects"`
	Reversibility  Reversibility `json:"reversibility"`
	Confirmation   string        `json:"confirmation"` // "--apply <id>" for executable choices; "none: no executor" otherwise
	Postconditions []string      `json:"postconditions"`
	Executor       string        `json:"executor"`        // "branchcut.Unwind" | "reclaim.Apply" | "none"
	ManualCommands []string      `json:"manual_commands"` // exact command lines; empty for executable choices
}

// RecognizedState is one member of the closed inventory this projection
// found evidence for.
type RecognizedState struct {
	Code           StateCode     `json:"code"`
	Scope          Scope         `json:"scope"`
	Target         string        `json:"target"` // branch, lock path, journal path, workspace id
	Facts          []string      `json:"facts"`
	Uncertainties  []Uncertainty `json:"uncertainties"`
	StepsCompleted []string      `json:"steps_completed"`
	InvariantsHeld []string      `json:"invariants_held"`
	Choices        []Choice      `json:"choices"`
}

// Projection is this package's whole output (ac-8): the ref and its
// repository identity, the recognized states (sorted by (code, target)),
// and any facts that could not be gathered at all (co-6), plus a
// self-digest (Canonical, below).
type Projection struct {
	Schema      string            `json:"schema"`
	Ref         string            `json:"ref"`
	Branch      string            `json:"branch"`
	Head        string            `json:"head"`
	States      []RecognizedState `json:"states"`      // sorted by (code, target)
	Disclosures []string          `json:"disclosures"` // facts that could not be gathered (co-6)
	Digest      string            `json:"digest"`
}

// Recognized reports whether at least one state was recognized — ac-8's
// exit-1 predicate. An `unrecognized` state counts: something being
// outside the closed inventory is itself a recognized fact.
func (p Projection) Recognized() bool {
	return len(p.States) > 0
}

// stateTargetKey is the (code, target) ordering key RecognizedState's own
// slice must be strictly ascending by. Neither field can carry \x00
// (both are control-character-free), so lexicographic order over the
// joined key matches tuple order exactly.
func stateTargetKey(s RecognizedState) string {
	return string(s.Code) + "\x00" + s.Target
}

// Validate reports the first rule p violates, fail-closed: an unknown
// schema id or state code, an out-of-order or duplicated (code, target)
// pair, a control character in any prose/identity string, an
// inconsistency between a choice's Executor and its ManualCommands or
// Confirmation, an executable choice ID that does not name its state's
// target after the colon (R-RR3-3), an uncertainty with no witness, or a
// malformed digest.
func (p Projection) Validate() error {
	if p.Schema != SchemaID {
		return fmt.Errorf("recovery: projection: schema %q, want %q", p.Schema, SchemaID)
	}
	if p.Ref == "" || containsControl(p.Ref) {
		return fmt.Errorf("recovery: projection: ref must be non-empty and control-free")
	}
	if p.Branch == "" || containsControl(p.Branch) {
		return fmt.Errorf("recovery: projection: branch must be non-empty and control-free")
	}
	if p.Head == "" || containsControl(p.Head) {
		return fmt.Errorf("recovery: projection: head must be non-empty and control-free")
	}
	if p.States == nil {
		return fmt.Errorf("recovery: projection: states must be non-nil (an explicitly empty set is [])")
	}
	seen := make(map[string]bool, len(p.States))
	for i, s := range p.States {
		field := fmt.Sprintf("states[%d]", i)
		if err := s.validate(field); err != nil {
			return err
		}
		key := stateTargetKey(s)
		if seen[key] {
			return fmt.Errorf("recovery: projection: duplicate state (code=%q, target=%q)", s.Code, s.Target)
		}
		seen[key] = true
		if i > 0 && stateTargetKey(p.States[i-1]) >= key {
			return fmt.Errorf("recovery: projection: states must be strictly ascending by (code, target)")
		}
	}
	if p.Disclosures == nil {
		return fmt.Errorf("recovery: projection: disclosures must be non-nil (an explicitly empty set is [])")
	}
	if !isSortedDeduped(p.Disclosures) {
		return fmt.Errorf("recovery: projection: disclosures must be sorted and deduplicated")
	}
	for i, d := range p.Disclosures {
		if d == "" || containsControl(d) {
			return fmt.Errorf("recovery: projection: disclosures[%d] must be non-empty and control-free", i)
		}
	}
	if p.Digest != "" && !digestRe.MatchString(p.Digest) {
		return fmt.Errorf("recovery: projection: digest %q is not empty or a valid sha256:<hex> digest", p.Digest)
	}
	return nil
}

func (s RecognizedState) validate(field string) error {
	if !validStateCodes[s.Code] {
		return fmt.Errorf("recovery: %s: unknown code %q", field, s.Code)
	}
	if !validScopes[s.Scope] {
		return fmt.Errorf("recovery: %s: unknown scope %q", field, s.Scope)
	}
	if s.Target == "" || containsControl(s.Target) {
		return fmt.Errorf("recovery: %s: target must be non-empty and control-free", field)
	}
	if s.Facts == nil {
		return fmt.Errorf("recovery: %s.facts: must be non-nil (an explicitly empty set is [])", field)
	}
	if err := validateStrings(field+".facts", s.Facts); err != nil {
		return err
	}
	if s.Uncertainties == nil {
		return fmt.Errorf("recovery: %s.uncertainties: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, u := range s.Uncertainties {
		if err := u.validate(fmt.Sprintf("%s.uncertainties[%d]", field, i)); err != nil {
			return err
		}
	}
	if s.StepsCompleted == nil {
		return fmt.Errorf("recovery: %s.steps_completed: must be non-nil (an explicitly empty set is [])", field)
	}
	if err := validateStrings(field+".steps_completed", s.StepsCompleted); err != nil {
		return err
	}
	if s.InvariantsHeld == nil {
		return fmt.Errorf("recovery: %s.invariants_held: must be non-nil (an explicitly empty set is [])", field)
	}
	if err := validateStrings(field+".invariants_held", s.InvariantsHeld); err != nil {
		return err
	}
	if s.Choices == nil {
		return fmt.Errorf("recovery: %s.choices: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, c := range s.Choices {
		if err := c.validate(fmt.Sprintf("%s.choices[%d]", field, i), s.Target); err != nil {
			return err
		}
	}
	return nil
}

func (u Uncertainty) validate(field string) error {
	if u.Text == "" || containsControl(u.Text) {
		return fmt.Errorf("recovery: %s: text must be non-empty and control-free", field)
	}
	if u.Witness == "" {
		return fmt.Errorf("recovery: %s: witness must be non-empty (an uncertainty with no witness is a guess)", field)
	}
	if containsControl(u.Witness) {
		return fmt.Errorf("recovery: %s: witness must be control-free", field)
	}
	return nil
}

func (c Choice) validate(field, stateTarget string) error {
	if c.ID == "" || containsControl(c.ID) {
		return fmt.Errorf("recovery: %s: id must be non-empty and control-free", field)
	}
	if c.Summary == "" || containsControl(c.Summary) {
		return fmt.Errorf("recovery: %s: summary must be non-empty and control-free", field)
	}
	if c.Preconditions == nil {
		return fmt.Errorf("recovery: %s.preconditions: must be non-nil (an explicitly empty set is [])", field)
	}
	if err := validateStrings(field+".preconditions", c.Preconditions); err != nil {
		return err
	}
	if len(c.Preconditions) == 0 {
		return fmt.Errorf("recovery: %s.preconditions: must be non-empty", field)
	}
	if c.Effects == nil {
		return fmt.Errorf("recovery: %s.effects: must be non-nil (an explicitly empty set is [])", field)
	}
	if err := validateStrings(field+".effects", c.Effects); err != nil {
		return err
	}
	if len(c.Effects) == 0 {
		return fmt.Errorf("recovery: %s.effects: must be non-empty", field)
	}
	if !validReversibilities[c.Reversibility] {
		return fmt.Errorf("recovery: %s: unknown reversibility %q", field, c.Reversibility)
	}
	if c.Postconditions == nil {
		return fmt.Errorf("recovery: %s.postconditions: must be non-nil (an explicitly empty set is [])", field)
	}
	if err := validateStrings(field+".postconditions", c.Postconditions); err != nil {
		return err
	}
	if len(c.Postconditions) == 0 {
		return fmt.Errorf("recovery: %s.postconditions: must be non-empty", field)
	}
	if !validExecutors[c.Executor] {
		return fmt.Errorf("recovery: %s: unknown executor %q", field, c.Executor)
	}
	if c.ManualCommands == nil {
		return fmt.Errorf("recovery: %s.manual_commands: must be non-nil (an explicitly empty set is [])", field)
	}
	if err := validateStrings(field+".manual_commands", c.ManualCommands); err != nil {
		return err
	}

	executable := c.Executor != executorNone
	if executable {
		if len(c.ManualCommands) != 0 {
			return fmt.Errorf("recovery: %s: executor %q must carry no manual_commands", field, c.Executor)
		}
		wantConfirmation := "--apply " + c.ID
		if c.Confirmation != wantConfirmation {
			return fmt.Errorf("recovery: %s: confirmation %q, want %q", field, c.Confirmation, wantConfirmation)
		}
		if !strings.HasSuffix(c.ID, ":"+stateTarget) {
			return fmt.Errorf("recovery: %s: executable choice id %q must name its state's target %q after a colon", field, c.ID, stateTarget)
		}
	} else {
		if len(c.ManualCommands) == 0 {
			return fmt.Errorf("recovery: %s: executor \"none\" must carry at least one manual command", field)
		}
		if c.Confirmation != "none: no executor" {
			return fmt.Errorf("recovery: %s: confirmation %q, want %q", field, c.Confirmation, "none: no executor")
		}
	}
	return nil
}

// validateStrings requires every element of ss to be non-empty and
// control-character-free (it does not require sorting: unlike
// Disclosures, these lists are prose in evidence order, not identity
// sets).
func validateStrings(field string, ss []string) error {
	for i, s := range ss {
		if s == "" || containsControl(s) {
			return fmt.Errorf("recovery: %s[%d] must be non-empty and control-free", field, i)
		}
	}
	return nil
}

// containsControl reports whether value contains any control character
// (a title/summary/identity string is always single-line prose).
func containsControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}

// isSortedDeduped reports whether ss is in strict ascending order: sorted
// and free of adjacent duplicates in one pass.
func isSortedDeduped(ss []string) bool {
	for i := 1; i < len(ss); i++ {
		if ss[i] <= ss[i-1] {
			return false
		}
	}
	return true
}
