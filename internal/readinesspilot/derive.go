package readinesspilot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// TargetFacts identifies the exact startup target and its existing board
// surface, when one is servable.
type TargetFacts struct {
	Ref       string
	Class     string
	Branch    string
	Head      string
	BoardPath string
}

// ShapeFacts is the decoded proposal content needed by the readiness pilot.
type ShapeFacts struct {
	ProblemPresent    bool
	OutcomePresent    bool
	DeclaredObjectIDs []string
	OpenQuestionIDs   []string
}

// ProvenanceFacts carries the already-classified design provenance and draft
// mutation postures. Derive never reads or reclassifies their sources.
type ProvenanceFacts struct {
	ChainState        State
	ChainWitnesses    []string
	MutationState     State
	MutationWitnesses []string
}

// BoardItem identifies one open scratch-board item.
type BoardItem struct {
	ID   string
	Kind string
}

// BoardFacts carries the already-classified scratch-board enumeration.
type BoardFacts struct {
	State     State
	OpenItems []BoardItem
	Witnesses []string
}

// Fallbacks are exact CLI token vectors supplied by the adapter for each area.
type Fallbacks struct {
	Shape   []string
	Success []string
	Context []string
	Review  []string
}

// Input is the complete decoded operand set for one readiness projection.
type Input struct {
	Target        TargetFacts
	Shape         ShapeFacts
	Provenance    ProvenanceFacts
	Board         BoardFacts
	Journey       journey.Record
	Conflict      policyconflict.Report
	Fallbacks     Fallbacks
	RequestDigest string
}

// Derive returns the deterministic, pure readiness projection. It delegates
// journey and policy-conflict proof validation to their owning packages.
func Derive(input Input) (Snapshot, error) {
	if err := input.validate(); err != nil {
		return Snapshot{}, err
	}

	concerns := make([]Concern, 0)
	concerns = append(concerns, deriveShape(input)...)
	concerns = append(concerns, deriveSuccess(input)...)
	concerns = append(concerns, deriveContext(input)...)
	concerns = append(concerns, deriveReview(input)...)
	sort.Slice(concerns, func(i, j int) bool { return concernLess(concerns[i], concerns[j]) })

	states := aggregateAreaStates(concerns)
	areas := make([]Area, len(orderedAreas))
	currentFocus := AreaID("")
	for i, definition := range orderedAreas {
		areas[i] = Area{ID: definition.ID, Label: definition.Label, State: states[definition.ID]}
		if currentFocus == "" && areas[i].State != StateProven {
			currentFocus = areas[i].ID
		}
	}

	attention := make([]Concern, 0)
	for _, concern := range concerns {
		if concern.State != StateProven {
			attention = append(attention, concern)
		}
	}
	sort.Slice(attention, func(i, j int) bool { return attentionLess(attention[i], attention[j]) })

	snapshot := Snapshot{
		TargetRef:     input.Target.Ref,
		TargetClass:   input.Target.Class,
		Branch:        input.Target.Branch,
		Head:          input.Target.Head,
		RequestDigest: input.RequestDigest,
		Areas:         areas,
		CurrentFocus:  currentFocus,
		Attention:     attention,
		AllConcerns:   concerns,
		StaleNotice:   fmt.Sprintf("Startup snapshot at %s; restart verdi serve after an edit.", input.Target.Head),
	}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, fmt.Errorf("readinesspilot: derived invalid snapshot: %w", err)
	}
	return snapshot, nil
}

func (in Input) validate() error {
	if err := validateIdentity("target ref", in.Target.Ref); err != nil {
		return err
	}
	if in.Target.Class != "feature" && in.Target.Class != "story" {
		return fmt.Errorf("readinesspilot: unknown target class %q", in.Target.Class)
	}
	if err := validateIdentity("target branch", in.Target.Branch); err != nil {
		return err
	}
	if in.Target.Head == "" || containsControl(in.Target.Head) {
		return fmt.Errorf("readinesspilot: target head must be non-empty and control-free")
	}
	if in.Target.BoardPath != "" && (!strings.HasPrefix(in.Target.BoardPath, "/") || containsControl(in.Target.BoardPath)) {
		return fmt.Errorf("readinesspilot: target board path must be root-relative and control-free")
	}
	if !digestPattern.MatchString(in.RequestDigest) {
		return fmt.Errorf("readinesspilot: request digest %q must match sha256:<64 lowercase hex>", in.RequestDigest)
	}
	if err := validateSourceIDs("declared object ids", in.Shape.DeclaredObjectIDs); err != nil {
		return err
	}
	if err := validateSourceIDs("open question ids", in.Shape.OpenQuestionIDs); err != nil {
		return err
	}
	if err := in.Provenance.ChainState.validate("chain state"); err != nil {
		return err
	}
	if err := validateWitnesses("chain witnesses", in.Provenance.ChainWitnesses); err != nil {
		return err
	}
	if in.Provenance.ChainState != StateProven && len(in.Provenance.ChainWitnesses) == 0 {
		return fmt.Errorf("readinesspilot: unresolved chain state requires a witness")
	}
	if in.Provenance.MutationState != StateProven && in.Provenance.MutationState != StateUnproven {
		return fmt.Errorf("readinesspilot: mutation state %q must be proven or unproven", in.Provenance.MutationState)
	}
	if err := validateWitnesses("mutation witnesses", in.Provenance.MutationWitnesses); err != nil {
		return err
	}
	if in.Provenance.MutationState == StateUnproven && len(in.Provenance.MutationWitnesses) == 0 {
		return fmt.Errorf("readinesspilot: unproven mutation state requires a witness")
	}
	if in.Board.State != StateProven && in.Board.State != StateUnproven {
		return fmt.Errorf("readinesspilot: board state %q must be proven or unproven", in.Board.State)
	}
	if in.Board.OpenItems == nil {
		return fmt.Errorf("readinesspilot: board open items must be non-nil")
	}
	if err := validateWitnesses("board witnesses", in.Board.Witnesses); err != nil {
		return err
	}
	if in.Board.State == StateUnproven && len(in.Board.Witnesses) == 0 {
		return fmt.Errorf("readinesspilot: unproven board state requires a witness")
	}
	boardIDs := make(map[string]bool, len(in.Board.OpenItems))
	for _, item := range in.Board.OpenItems {
		if item.Kind != "question" && item.Kind != "agent-task" {
			return fmt.Errorf("readinesspilot: board item kind %q is not question or agent-task", item.Kind)
		}
		if err := validateIdentity("board item id", item.ID); err != nil {
			return err
		}
		key := item.Kind + "\x00" + item.ID
		if boardIDs[key] {
			return fmt.Errorf("readinesspilot: duplicate board item %q/%q", item.Kind, item.ID)
		}
		boardIDs[key] = true
	}
	if err := validateFallback("shape", in.Fallbacks.Shape); err != nil {
		return err
	}
	if err := validateFallback("success", in.Fallbacks.Success); err != nil {
		return err
	}
	if err := validateFallback("context", in.Fallbacks.Context); err != nil {
		return err
	}
	if err := validateFallback("review", in.Fallbacks.Review); err != nil {
		return err
	}
	if err := in.Journey.Validate(); err != nil {
		return fmt.Errorf("readinesspilot: journey operand: %w", err)
	}
	if err := in.Conflict.Validate(); err != nil {
		return fmt.Errorf("readinesspilot: policy conflict operand: %w", err)
	}
	return nil
}

func deriveShape(input Input) []Concern {
	concerns := []Concern{
		presenceConcern("shape/problem", "Problem statement is present", "Problem statement is missing", input.Shape.ProblemPresent, input, true),
		presenceConcern("shape/outcome", "Intended outcome is present", "Intended outcome is missing", input.Shape.OutcomePresent, input, true),
	}
	for _, id := range sortedCopy(input.Shape.OpenQuestionIDs) {
		concerns = append(concerns, newConcern(
			"shape/question/"+id, AreaShape, StateUnproven, true, TimingCurrent, "",
			"Declared open question remains unresolved", []string{id}, boardDestination(input, input.Fallbacks.Shape),
		))
	}
	concerns = append(concerns, newConcern(
		"shape/provenance", AreaShape, input.Provenance.ChainState, false, TimingCurrent, "",
		fmt.Sprintf("Design provenance posture for %d declared objects", len(input.Shape.DeclaredObjectIDs)),
		mergeStrings(input.Shape.DeclaredObjectIDs, input.Provenance.ChainWitnesses),
		cliDestination(input.Provenance.ChainState, input.Fallbacks.Shape),
	))
	concerns = append(concerns, newConcern(
		"shape/mutation", AreaShape, input.Provenance.MutationState, false, TimingCurrent, "",
		// vocab:identity — "Draft mutation" names the ASD draftmutation source-fact domain, not a lifecycle-state display label.
		"Draft mutation residue posture", input.Provenance.MutationWitnesses,
		cliDestination(input.Provenance.MutationState, input.Fallbacks.Shape),
	))
	concerns = append(concerns, newConcern(
		"shape/board", AreaShape, input.Board.State, false, TimingCurrent, "",
		"Scratch board enumeration posture", input.Board.Witnesses,
		cliDestination(input.Board.State, input.Fallbacks.Shape),
	))
	items := append([]BoardItem(nil), input.Board.OpenItems...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].ID < items[j].ID
	})
	for _, item := range items {
		witnesses := appendCopy(input.Board.Witnesses, item.ID)
		concerns = append(concerns, newConcern(
			"shape/board/"+item.Kind+"/"+item.ID, AreaShape, StateUnproven, false, TimingCurrent, "",
			"Scratch board item remains open", witnesses, boardDestination(input, input.Fallbacks.Shape),
		))
	}
	return concerns
}

func deriveSuccess(input Input) []Concern {
	concerns := make([]Concern, 0, len(input.Journey.Evidence.Contributors)+len(input.Journey.Blockers.Current)+len(input.Journey.Blockers.Eventual.Items))
	for _, contributor := range input.Journey.Evidence.Contributors {
		state := State(contributor.Resolution)
		destination := Destination{CLI: []string{}}
		if state != StateProven {
			destination = boardDestination(input, input.Fallbacks.Success)
		}
		concerns = append(concerns, newConcern(
			"success/contributor/"+contributor.ID, AreaSuccess, state, false, TimingCurrent, "",
			"Journey evidence contributor "+contributor.ID, []string{contributor.Witness}, destination,
		))
	}
	for _, blocker := range input.Journey.Blockers.Current {
		if strings.HasPrefix(blocker.ID, "obligation-quality/") {
			concerns = append(concerns, blockerConcern(blocker, AreaSuccess, TimingCurrent, input.Fallbacks.Success))
		}
	}
	for _, blocker := range input.Journey.Blockers.Eventual.Items {
		if strings.HasPrefix(blocker.ID, "obligation-quality/") {
			concerns = append(concerns, blockerConcern(blocker, AreaSuccess, TimingEventual, input.Fallbacks.Success))
		}
	}
	return concerns
}

func deriveContext(input Input) []Concern {
	verdictState := StateProven
	switch input.Conflict.Verdict {
	case policyconflict.VerdictBlockedViolated:
		verdictState = StateViolated
	case policyconflict.VerdictBlockedUnproven:
		verdictState = StateUnproven
	}
	verdictWitnesses := []string{}
	if verdictState != StateProven {
		verdictWitnesses = []string{"policy-conflict verdict: " + string(input.Conflict.Verdict)}
	}
	concerns := []Concern{newConcern(
		"context/verdict", AreaContext, verdictState, true, TimingCurrent, "",
		"Policy-conflict verdict", verdictWitnesses, cliDestination(verdictState, input.Fallbacks.Context),
	)}
	for _, row := range input.Conflict.Mechanical {
		state := State(row.State)
		concerns = append(concerns, newConcern(
			"context/mechanical/"+row.ID, AreaContext, state, false, TimingCurrent, "",
			"Mechanical policy-conflict detail", reasonWitnesses(row.Reasons), cliDestination(state, input.Fallbacks.Context),
		))
	}
	for _, row := range input.Conflict.Semantic {
		state := State(row.State)
		concerns = append(concerns, newConcern(
			"context/semantic/"+row.ID, AreaContext, state, false, TimingCurrent, "",
			"Semantic policy-conflict detail", reasonWitnesses(row.Reasons), cliDestination(state, input.Fallbacks.Context),
		))
	}
	for _, disclosure := range input.Conflict.Disclosures {
		witnesses := appendCopy(disclosure.Witnesses, "policy-conflict disclosure: "+string(disclosure.Code))
		concerns = append(concerns, newConcern(
			"context/disclosure/"+string(disclosure.Code), AreaContext, StateUnproven, false, TimingCurrent, "",
			"Policy-conflict disclosure", witnesses, cliDestination(StateUnproven, input.Fallbacks.Context),
		))
	}
	return concerns
}

func deriveReview(input Input) []Concern {
	concerns := make([]Concern, 0, len(input.Journey.Blockers.Current)+len(input.Journey.Blockers.Eventual.Items)+len(input.Journey.Principals.Required)+1)
	for _, blocker := range input.Journey.Blockers.Current {
		if !strings.HasPrefix(blocker.ID, "obligation-quality/") {
			concerns = append(concerns, blockerConcern(blocker, AreaReview, TimingCurrent, input.Fallbacks.Review))
		}
	}
	for _, blocker := range input.Journey.Blockers.Eventual.Items {
		if !strings.HasPrefix(blocker.ID, "obligation-quality/") {
			concerns = append(concerns, blockerConcern(blocker, AreaReview, TimingEventual, input.Fallbacks.Review))
		}
	}
	for _, role := range input.Journey.Principals.Required {
		state := roleState(role.Resolution)
		witnesses := []string{}
		if state != StateProven {
			witnesses = appendCopy(input.Journey.Principals.Disclosures,
				fmt.Sprintf("principal requirement %s/%s is %s", role.Transition, role.Obligation, role.Resolution))
		}
		concerns = append(concerns, newConcern(
			"review/role/"+role.Transition+"/"+role.Obligation, AreaReview, state, false, TimingCurrent, "",
			fmt.Sprintf("%d principal role required for review", role.Count), witnesses,
			cliDestination(state, input.Fallbacks.Review),
		))
	}

	actionState := StateProven
	witnesses := actionIDs(input.Journey.Actions.Safe)
	if input.Journey.Lifecycle.State == "unproven" || input.Journey.Lifecycle.Relation == "unproven" || input.Journey.Lifecycle.Posture == "unknown" ||
		!input.Journey.Blockers.Eventual.Derived ||
		!input.Journey.Principals.ProfileAdopted || len(input.Journey.Actions.Safe) == 0 {
		actionState = StateUnproven
		witnesses = mergeStrings(
			input.Journey.Lifecycle.Disclosures,
			input.Journey.Blockers.Eventual.Disclosures,
			input.Journey.Principals.Disclosures,
			input.Journey.Actions.NeededFacts,
		)
		if len(witnesses) == 0 {
			witnesses = []string{"safe review action is unavailable"}
		}
	}
	concerns = append(concerns, newConcern(
		"review/action", AreaReview, actionState, true, TimingCurrent, "",
		"Lifecycle and safe-action posture can advance review", witnesses,
		cliDestination(actionState, input.Fallbacks.Review),
	))
	return concerns
}

func presenceConcern(id, provenSummary, missingSummary string, present bool, input Input, boardCorrectable bool) Concern {
	if present {
		return newConcern(id, AreaShape, StateProven, true, TimingCurrent, "", provenSummary, []string{}, Destination{CLI: []string{}})
	}
	destination := cliDestination(StateViolated, input.Fallbacks.Shape)
	if boardCorrectable {
		destination = boardDestination(input, input.Fallbacks.Shape)
	}
	return newConcern(id, AreaShape, StateViolated, true, TimingCurrent, "", missingSummary, []string{missingSummary}, destination)
}

func blockerConcern(blocker journey.Blocker, area AreaID, timing Timing, fallback []string) Concern {
	prefix := "review/blocker/"
	if area == AreaSuccess {
		prefix = "success/blocker/"
	}
	return newConcern(
		prefix+blocker.ID, area, StateViolated, timing == TimingCurrent, timing, blocker.Class,
		blocker.ClearingCondition, blocker.Witnesses, cliDestination(StateViolated, fallback),
	)
}

func newConcern(id string, area AreaID, state State, blocking bool, timing Timing, workClass journey.BlockerClass, summary string, witnesses []string, destination Destination) Concern {
	if destination.CLI == nil {
		destination.CLI = []string{}
	}
	return Concern{
		ID: id, Area: area, State: state, Blocking: blocking, Timing: timing,
		WorkClass: workClass, Summary: summary, Witnesses: normalizedStrings(witnesses), Destination: destination,
	}
}

func boardDestination(input Input, fallback []string) Destination {
	if input.Target.BoardPath != "" {
		return Destination{BoardPath: input.Target.BoardPath, CLI: []string{}}
	}
	return Destination{CLI: append([]string(nil), fallback...)}
}

func cliDestination(state State, fallback []string) Destination {
	if state == StateProven {
		return Destination{CLI: []string{}}
	}
	return Destination{CLI: append([]string(nil), fallback...)}
}

func roleState(resolution string) State {
	switch resolution {
	case "authenticated":
		return StateProven
	case "violated-with-witness":
		return StateViolated
	default:
		return StateUnproven
	}
}

func reasonWitnesses(reasons []policyconflict.ReasonCode) []string {
	out := make([]string, len(reasons))
	for i, reason := range reasons {
		out[i] = string(reason)
	}
	return out
}

func actionIDs(actions []journey.Action) []string {
	out := make([]string, len(actions))
	for i, action := range actions {
		out[i] = action.ID
	}
	return normalizedStrings(out)
}

func validateSourceIDs(field string, ids []string) error {
	if ids == nil {
		return fmt.Errorf("readinesspilot: %s must be non-nil", field)
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if err := validateIdentity(field, id); err != nil {
			return err
		}
		if seen[id] {
			return fmt.Errorf("readinesspilot: %s contains duplicate %q", field, id)
		}
		seen[id] = true
	}
	return nil
}

func validateFallback(name string, tokens []string) error {
	if tokens == nil {
		return fmt.Errorf("readinesspilot: %s fallback must be non-nil", name)
	}
	if len(tokens) == 0 || tokens[0] != "verdi" {
		return fmt.Errorf("readinesspilot: %s fallback must be a nonempty CLI vector beginning with verdi", name)
	}
	for _, token := range tokens {
		if token == "" || containsControl(token) {
			return fmt.Errorf("readinesspilot: %s fallback contains an empty or control-bearing token", name)
		}
	}
	return nil
}

func normalizedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	if len(out) == 0 {
		return []string{}
	}
	n := 1
	for i := 1; i < len(out); i++ {
		if out[i] != out[n-1] {
			out[n] = out[i]
			n++
		}
	}
	return out[:n]
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func appendCopy(values []string, extras ...string) []string {
	out := append([]string(nil), values...)
	return append(out, extras...)
}

func mergeStrings(sets ...[]string) []string {
	var out []string
	for _, set := range sets {
		out = append(out, set...)
	}
	return normalizedStrings(out)
}
