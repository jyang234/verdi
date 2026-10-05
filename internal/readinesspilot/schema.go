// Package readinesspilot derives the Wave 3.5 in-memory readiness snapshot.
// It owns no codec, persistence path, lifecycle state, or source proof logic.
package readinesspilot

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/jyang234/verdi/internal/journey"
)

// State is the closed three-valued readiness state.
type State string

const (
	StateProven   State = "proven"
	StateViolated State = "violated-with-witness"
	StateUnproven State = "unproven"
)

// AreaID is one of the pilot's four ordered presentation areas.
type AreaID string

const (
	AreaShape   AreaID = "shape-proposal"
	AreaSuccess AreaID = "show-success"
	AreaContext AreaID = "check-context"
	AreaReview  AreaID = "request-review"
)

// Timing distinguishes a current concern from a journey-derived eventual one.
type Timing string

const (
	TimingCurrent  Timing = "current"
	TimingEventual Timing = "eventual"
)

// Destination is exactly one corrective board path or CLI token vector for an
// unresolved concern. A proven concern carries neither.
type Destination struct {
	BoardPath string
	CLI       []string
}

// Concern is one lossless source-derived readiness row.
//
// Guidance is the row's source-derived corrective guidance (SI-338 (1);
// Wave 6 §3.1): non-empty exactly when the row is not proven, empty when it
// is. Object is the declared object id at the row's family's
// object-bearing segment (SI-338 (2); SI-345 (1); objectSegment), or empty
// when the family carries none or that segment is undeclared. Neither is
// a new fact: both are pure functions of the derivation's own inputs, and
// no document byte, gate, verdict, or lifecycle input reads them.
type Concern struct {
	ID          string
	Area        AreaID
	State       State
	Blocking    bool
	Timing      Timing
	WorkClass   journey.BlockerClass
	Summary     string
	Guidance    string
	Object      string
	Witnesses   []string
	Destination Destination
}

// HumanReview reports whether the row is human-review work (SI-338 (3);
// Wave 6 §3.1 "human-review work labeled plainly as human review"): a
// review/role/* principal requirement, or a journey blocker of work class
// governance (principal resolution, countersign, exemption). Judgmental
// rows (author vouch, outcome floor, semantic judge) are not. It is a pure
// function of the id and the work class.
func (c Concern) HumanReview() bool {
	return strings.HasPrefix(c.ID, "review/role/") || c.WorkClass == journey.ClassGovernance
}

// Area is one aggregate presentation row.
type Area struct {
	ID    AreaID
	Label string
	State State
}

// Snapshot is the immutable, in-memory readiness projection consumed by the
// workbench. BoardPath is the target's root-relative wall address when one
// is servable (the loader's own board href), else empty.
type Snapshot struct {
	TargetRef     string
	TargetTitle   string
	TargetClass   string
	Branch        string
	Head          string
	BoardPath     string
	RequestDigest string
	Areas         []Area
	CurrentFocus  AreaID
	Attention     []Concern
	AllConcerns   []Concern
	StaleNotice   string
}

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

var orderedAreas = []Area{
	{ID: AreaShape, Label: "Define the work"},
	{ID: AreaSuccess, Label: "Define success"},
	{ID: AreaContext, Label: "Check constraints"},
	{ID: AreaReview, Label: "Get approval"},
}

var areaOrder = map[AreaID]int{
	AreaShape:   0,
	AreaSuccess: 1,
	AreaContext: 2,
	AreaReview:  3,
}

// Validate rejects a snapshot that is not closed, deterministic, lossless, or
// internally consistent with the fixed readiness contract.
func (s Snapshot) Validate() error {
	if err := validateIdentity("target ref", s.TargetRef); err != nil {
		return err
	}
	if s.TargetTitle == "" || containsControl(s.TargetTitle) {
		return fmt.Errorf("readinesspilot: target title must be non-empty and control-free")
	}
	if s.TargetClass != "feature" && s.TargetClass != "story" {
		return fmt.Errorf("readinesspilot: unknown target class %q", s.TargetClass)
	}
	if s.Branch == "" || containsControl(s.Branch) {
		return fmt.Errorf("readinesspilot: branch must be non-empty and control-free")
	}
	if s.Head == "" || containsControl(s.Head) {
		return fmt.Errorf("readinesspilot: head must be non-empty and control-free")
	}
	if s.StaleNotice == "" || containsControl(s.StaleNotice) {
		return fmt.Errorf("readinesspilot: stale notice must be non-empty and control-free")
	}
	if s.BoardPath != "" && (!strings.HasPrefix(s.BoardPath, "/") || containsControl(s.BoardPath)) {
		return fmt.Errorf("readinesspilot: board path %q must be root-relative and control-free", s.BoardPath)
	}
	if !digestPattern.MatchString(s.RequestDigest) {
		return fmt.Errorf("readinesspilot: request digest %q must match sha256:<64 lowercase hex>", s.RequestDigest)
	}
	if s.Areas == nil {
		return fmt.Errorf("readinesspilot: areas must be non-nil")
	}
	if s.Attention == nil {
		return fmt.Errorf("readinesspilot: attention must be non-nil")
	}
	if s.AllConcerns == nil {
		return fmt.Errorf("readinesspilot: all concerns must be non-nil")
	}
	if len(s.Areas) != len(orderedAreas) {
		return fmt.Errorf("readinesspilot: areas has %d rows, want %d", len(s.Areas), len(orderedAreas))
	}
	for i, want := range orderedAreas {
		got := s.Areas[i]
		if got.ID != want.ID || got.Label != want.Label {
			return fmt.Errorf("readinesspilot: area[%d] = (%q, %q), want (%q, %q)", i, got.ID, got.Label, want.ID, want.Label)
		}
		if err := got.State.validate("area state"); err != nil {
			return err
		}
	}

	counts := make(map[AreaID]int, len(orderedAreas))
	for i, concern := range s.AllConcerns {
		if err := concern.validate(); err != nil {
			return fmt.Errorf("readinesspilot: all concerns[%d]: %w", i, err)
		}
		if i > 0 && !concernLess(s.AllConcerns[i-1], concern) {
			return fmt.Errorf("readinesspilot: all concerns must be strictly sorted by area and id order")
		}
		if classifyConcern(concern.ID) == familySuccessCoverage && s.TargetClass != "feature" {
			// vocab:identity — names the fixed artifact class identity in a schema diagnostic, not display prose
			return fmt.Errorf("readinesspilot: all concerns[%d]: coverage concern %q applies only to a feature target, not %q", i, concern.ID, s.TargetClass)
		}
		counts[concern.Area]++
	}
	for _, area := range orderedAreas {
		if counts[area.ID] == 0 {
			return fmt.Errorf("readinesspilot: area %q has no concern; a proven area cannot be vacuous", area.ID)
		}
	}

	states := aggregateAreaStates(s.AllConcerns)
	wantFocus := AreaID("")
	for i, area := range orderedAreas {
		if got, want := s.Areas[i].State, states[area.ID]; got != want {
			return fmt.Errorf("readinesspilot: area %q state %q does not equal derived state %q", area.ID, got, want)
		}
		if wantFocus == "" && states[area.ID] != StateProven {
			wantFocus = area.ID
		}
	}
	if s.CurrentFocus != wantFocus {
		return fmt.Errorf("readinesspilot: current focus %q, want first non-proven area %q", s.CurrentFocus, wantFocus)
	}

	unresolved := make(map[string]Concern)
	for _, concern := range s.AllConcerns {
		if concern.State != StateProven {
			unresolved[concern.ID] = concern
		}
	}
	seen := make(map[string]bool, len(s.Attention))
	for i, concern := range s.Attention {
		if err := concern.validate(); err != nil {
			return fmt.Errorf("readinesspilot: attention[%d]: %w", i, err)
		}
		if seen[concern.ID] {
			return fmt.Errorf("readinesspilot: attention contains duplicate concern %q", concern.ID)
		}
		seen[concern.ID] = true
		canonical, ok := unresolved[concern.ID]
		if !ok {
			return fmt.Errorf("readinesspilot: attention contains non-unresolved concern %q", concern.ID)
		}
		if !reflect.DeepEqual(concern, canonical) {
			return fmt.Errorf("readinesspilot: attention concern %q differs from all concerns", concern.ID)
		}
	}
	for _, concern := range s.AllConcerns {
		if concern.State != StateProven && !seen[concern.ID] {
			return fmt.Errorf("readinesspilot: attention omits unresolved concern %q", concern.ID)
		}
	}
	for i := 1; i < len(s.Attention); i++ {
		if !attentionLessForFocus(wantFocus, s.Attention[i-1], s.Attention[i]) {
			return fmt.Errorf("readinesspilot: attention order is not the fixed current-focus grouping at %q", s.Attention[i].ID)
		}
	}
	return nil
}

func (s State) validate(field string) error {
	switch s {
	case StateProven, StateViolated, StateUnproven:
		return nil
	default:
		return fmt.Errorf("readinesspilot: %s has unknown state %q", field, s)
	}
}

func (t Timing) validate() error {
	switch t {
	case TimingCurrent, TimingEventual:
		return nil
	default:
		return fmt.Errorf("readinesspilot: unknown timing %q", t)
	}
}

func (c Concern) validate() error {
	if err := c.Timing.validate(); err != nil {
		return err
	}
	inferredArea, journeyDerived, expectedBlocking, err := concernIdentity(c.ID, c.Timing)
	if err != nil {
		return err
	}
	if c.Area != inferredArea {
		return fmt.Errorf("concern %q area %q does not match id-derived area %q", c.ID, c.Area, inferredArea)
	}
	if err := c.State.validate("concern state"); err != nil {
		return err
	}
	if c.Blocking != expectedBlocking {
		return fmt.Errorf("concern %q blocking=%v, want %v", c.ID, c.Blocking, expectedBlocking)
	}
	if journeyDerived {
		if !validWorkClass(c.WorkClass) {
			return fmt.Errorf("concern %q journey concern work class %q is missing or invalid", c.ID, c.WorkClass)
		}
	} else if c.WorkClass != "" {
		return fmt.Errorf("concern %q is a non-journey concern but carries work class %q", c.ID, c.WorkClass)
	}
	if c.Summary == "" {
		return fmt.Errorf("concern %q summary must be non-empty", c.ID)
	}
	if err := c.validateFamilyPosture(); err != nil {
		return err
	}
	if c.State == StateProven {
		if c.Guidance != "" {
			return fmt.Errorf("concern %q proven concern must carry no guidance", c.ID)
		}
	} else if c.Guidance == "" || containsControl(c.Guidance) {
		return fmt.Errorf("concern %q unresolved concern guidance must be non-empty and control-free", c.ID)
	}
	if c.Object != "" && c.Object != objectSegment(c.ID) {
		return fmt.Errorf("concern %q object %q is not its id's object-bearing segment", c.ID, c.Object)
	}
	if err := validateWitnesses(fmt.Sprintf("concern %q witnesses", c.ID), c.Witnesses); err != nil {
		return err
	}
	if !strictlySorted(c.Witnesses) {
		return fmt.Errorf("concern %q witnesses must be sorted and deduplicated", c.ID)
	}
	if c.State != StateProven && len(c.Witnesses) == 0 {
		return fmt.Errorf("concern %q unresolved concern must carry a witness", c.ID)
	}
	return c.Destination.validate(c.State, c.ID)
}

func (d Destination) validate(state State, concernID string) error {
	if d.CLI == nil {
		return fmt.Errorf("concern %q destination CLI must be non-nil", concernID)
	}
	if state == StateProven {
		if d.BoardPath != "" || len(d.CLI) != 0 {
			return fmt.Errorf("concern %q proven concern must carry no destination", concernID)
		}
		return nil
	}
	hasBoard := d.BoardPath != ""
	hasCLI := len(d.CLI) != 0
	if hasBoard == hasCLI {
		return fmt.Errorf("concern %q unresolved concern requires exactly one destination", concernID)
	}
	if hasBoard {
		if !strings.HasPrefix(d.BoardPath, "/") || containsControl(d.BoardPath) {
			return fmt.Errorf("concern %q board path %q must be root-relative and control-free", concernID, d.BoardPath)
		}
		return nil
	}
	for _, token := range d.CLI {
		if token == "" {
			return fmt.Errorf("concern %q has an empty CLI token", concernID)
		}
		if containsControl(token) {
			return fmt.Errorf("concern %q has a control-bearing CLI token", concernID)
		}
	}
	return nil
}

// concernFamily is one family of the closed concern-identity vocabulary.
// classifyConcern is its one classifier: concernIdentity fixes each
// family's area and blocking rule from it, and guidanceFor its corrective
// guidance, so a family added here without guidance fails the guidance
// table test (TestGuidanceFor_EveryFamilyOfTheClosedVocabulary) rather than
// shipping a row Validate would refuse.
type concernFamily int

const (
	familyUnknown concernFamily = iota
	familyShapeProblem
	familyShapeOutcome
	familyShapeProvenance
	familyShapeMutation
	familyShapeBoard
	familyShapeQuestion
	familyShapeBoardItem
	familySuccessCriteria
	familySuccessCoverage
	familySuccessContributor
	familySuccessBlocker
	familyContextVerdict
	familyContextMechanical
	familyContextSemantic
	familyContextDisclosure
	familyReviewBlocker
	familyReviewRole
	familyReviewAction
	familyReviewEventualDerivation
	// familyEnd bounds the enumeration; it is not a family.
	familyEnd
)

// classifyConcern returns id's family in the closed vocabulary, or
// familyUnknown. It checks shape only; validateIdentity checks the id's
// components first.
func classifyConcern(id string) concernFamily {
	parts := strings.Split(id, "/")
	switch {
	case id == "shape/problem":
		return familyShapeProblem
	case id == "shape/outcome":
		return familyShapeOutcome
	case id == "shape/provenance":
		return familyShapeProvenance
	case id == "shape/mutation":
		return familyShapeMutation
	case id == "shape/board":
		return familyShapeBoard
	case len(parts) >= 3 && parts[0] == "shape" && parts[1] == "question":
		return familyShapeQuestion
	case len(parts) >= 4 && parts[0] == "shape" && parts[1] == "board" && (parts[2] == "question" || parts[2] == "agent-task"):
		return familyShapeBoardItem
	case id == "success/criteria":
		return familySuccessCriteria
	case len(parts) == 3 && parts[0] == "success" && parts[1] == "coverage":
		return familySuccessCoverage
	case len(parts) >= 3 && parts[0] == "success" && parts[1] == "contributor":
		return familySuccessContributor
	case len(parts) >= 3 && parts[0] == "success" && parts[1] == "blocker":
		return familySuccessBlocker
	case id == "context/verdict":
		return familyContextVerdict
	case len(parts) >= 3 && parts[0] == "context" && parts[1] == "mechanical":
		return familyContextMechanical
	case len(parts) >= 3 && parts[0] == "context" && parts[1] == "semantic":
		return familyContextSemantic
	case len(parts) >= 3 && parts[0] == "context" && parts[1] == "disclosure":
		return familyContextDisclosure
	case len(parts) >= 3 && parts[0] == "review" && parts[1] == "blocker":
		return familyReviewBlocker
	case len(parts) >= 5 && parts[0] == "review" && parts[1] == "role":
		return familyReviewRole
	case id == "review/action":
		return familyReviewAction
	case id == "review/eventual-derivation":
		return familyReviewEventualDerivation
	default:
		return familyUnknown
	}
}

func concernIdentity(id string, timing Timing) (AreaID, bool, bool, error) {
	if err := validateIdentity("concern id", id); err != nil {
		return "", false, false, fmt.Errorf("invalid id %q: %w", id, err)
	}
	current := timing == TimingCurrent
	switch classifyConcern(id) {
	case familyShapeProblem, familyShapeOutcome:
		return AreaShape, false, true, nil
	case familyShapeProvenance, familyShapeMutation, familyShapeBoard:
		return AreaShape, false, false, nil
	case familyShapeQuestion:
		// An unclaimed question blocks now (current); a question a spike
		// stub claims is a later-timed, non-blocking readiness concern
		// (PLAN.md §7 I-128 option (a); spec/uat-round-1 ac-10) — the same
		// timing-derives-blocking shape blockerConcern already uses.
		return AreaShape, false, current, nil
	case familyShapeBoardItem:
		return AreaShape, false, false, nil
	// SI-338 (5): whether the spec declares acceptance criteria at all,
	// blocking — the success area's own anchor.
	case familySuccessCriteria:
		return AreaSuccess, false, true, nil
	// SI-338 (4): an acceptance criterion no non-spike stub lists, on a
	// feature only — the wall's "no stub", never a blocker.
	case familySuccessCoverage:
		return AreaSuccess, false, false, nil
	case familySuccessContributor:
		return AreaSuccess, false, false, nil
	case familySuccessBlocker:
		return AreaSuccess, true, current, nil
	case familyContextVerdict:
		return AreaContext, false, true, nil
	case familyContextMechanical, familyContextSemantic, familyContextDisclosure:
		return AreaContext, false, false, nil
	case familyReviewBlocker:
		return AreaReview, true, current, nil
	case familyReviewRole:
		return AreaReview, false, false, nil
	case familyReviewAction:
		return AreaReview, false, true, nil
	// SI-213 / R-RRF-1: the completeness of the journey's own eventual
	// derivation, blocking and current — a readiness conclusion drawn over
	// a partial derivation is not a conclusion. Not journey-derived in the
	// work-class sense: it describes the derivation, not one blocker.
	case familyReviewEventualDerivation:
		return AreaReview, false, true, nil
	default:
		// vocab:identity — "closed" describes enum closure in a schema diagnostic, not the renameable lifecycle state.
		return "", false, false, fmt.Errorf("invalid id %q: not in the closed concern identity vocabulary", id)
	}
}

// validateFamilyPosture enforces the two success families' fixed states
// beyond concernIdentity's area and blocking rule: success/criteria is
// proven or violated, never unproven (SI-338 (5)); success/coverage/<ac>
// is unproven and current, never proven (SI-338 (4)) — a declared stub is
// a claim of relatedness, never evidence (index-coverage co-1).
func (c Concern) validateFamilyPosture() error {
	switch classifyConcern(c.ID) {
	case familySuccessCriteria:
		if c.State != StateProven && c.State != StateViolated {
			return fmt.Errorf("concern %q criteria concern must be proven or violated, not %q", c.ID, c.State)
		}
	case familySuccessCoverage:
		if c.State != StateUnproven {
			return fmt.Errorf("concern %q coverage concern must be unproven, not %q", c.ID, c.State)
		}
		if c.Timing != TimingCurrent {
			return fmt.Errorf("concern %q coverage concern must be current, not %q", c.ID, c.Timing)
		}
	}
	return nil
}

// objectSegment returns the one segment of id at its family's
// object-bearing position, or "" for a family that carries no object
// (SI-345 (1)): <oq> in shape/question/<oq>, <ac> in success/coverage/<ac>,
// and, in either blocker family, the object of the journey blocker codes
// outcome-floor/<ac>, question-claimed/<oq> and
// obligation-quality/<ac>/<kind>. Each shape is exact, so an object is
// always one whole segment. Every other segment — a stub slug, verb, role,
// exemption id, conflict id, sticky id, disclosure code, or evidence kind —
// names no object even when its text equals a declared object id: stub
// slugs and object ids share one textual namespace. objectOf derives an
// Object from it and Validate admits no other.
func objectSegment(id string) string {
	parts := strings.Split(id, "/")
	switch classifyConcern(id) {
	case familyShapeQuestion:
		if len(parts) == 3 {
			return parts[2]
		}
	case familySuccessCoverage:
		return parts[2]
	case familySuccessBlocker, familyReviewBlocker:
		switch {
		case len(parts) == 4 && (parts[2] == "outcome-floor" || parts[2] == "question-claimed"):
			return parts[3]
		case len(parts) == 5 && parts[2] == "obligation-quality":
			return parts[3]
		}
	}
	return ""
}

func validWorkClass(class journey.BlockerClass) bool {
	switch class {
	case journey.ClassMechanical, journey.ClassJudgmental, journey.ClassGovernance, journey.ClassExternalWait, journey.ClassUnknown:
		return true
	default:
		return false
	}
}

func aggregateAreaStates(concerns []Concern) map[AreaID]State {
	states := map[AreaID]State{
		AreaShape: StateProven, AreaSuccess: StateProven,
		AreaContext: StateProven, AreaReview: StateProven,
	}
	for _, concern := range concerns {
		if !concern.Blocking {
			continue
		}
		switch {
		case concern.State == StateViolated:
			states[concern.Area] = StateViolated
		case concern.State == StateUnproven && states[concern.Area] == StateProven:
			states[concern.Area] = StateUnproven
		}
	}
	return states
}

func concernLess(a, b Concern) bool {
	if areaOrder[a.Area] != areaOrder[b.Area] {
		return areaOrder[a.Area] < areaOrder[b.Area]
	}
	return a.ID < b.ID
}

func attentionLess(a, b Concern) bool {
	if a.Blocking != b.Blocking {
		return a.Blocking
	}
	if a.Timing != b.Timing {
		return a.Timing == TimingCurrent
	}
	if a.State != b.State {
		return a.State == StateViolated
	}
	return concernLess(a, b)
}

func attentionLessForFocus(currentFocus AreaID, a, b Concern) bool {
	aCurrent := a.Area == currentFocus
	bCurrent := b.Area == currentFocus
	if aCurrent != bCurrent {
		return aCurrent
	}
	return attentionLess(a, b)
}

func validateIdentity(field, value string) error {
	if value == "" || containsControl(value) {
		return fmt.Errorf("readinesspilot: %s must be non-empty and control-free", field)
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" {
			return fmt.Errorf("readinesspilot: %s has an empty component", field)
		}
	}
	return nil
}

func validateWitnesses(field string, witnesses []string) error {
	if witnesses == nil {
		return fmt.Errorf("readinesspilot: %s must be non-nil", field)
	}
	for _, witness := range witnesses {
		if witness == "" {
			return fmt.Errorf("readinesspilot: %s contains an empty witness", field)
		}
	}
	return nil
}

func containsControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}

func strictlySorted(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i] <= values[i-1] {
			return false
		}
	}
	return true
}
