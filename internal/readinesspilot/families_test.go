package readinesspilot

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/journey"
)

// TestDeriveSuccessCriteria is SI-338 (5): success/criteria moves into the
// derivation, proven when the spec declares acceptance criteria and
// violated with a witness when it declares none — blocking either way. A
// story with no criteria used to fail every readiness surface
// operationally (the success area was vacuous); it now reads violated.
func TestDeriveSuccessCriteria(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		class      string
		criteria   []string
		wantState  State
		wantArea   State
		wantFocus  AreaID
		wantSumm   string
		wantWitnes []string
	}{
		{
			name: "declared criteria", class: "feature", criteria: []string{"ac-1", "ac-2"},
			wantState: StateProven, wantArea: StateProven, wantFocus: "",
			wantSumm: "Acceptance criteria are declared", wantWitnes: []string{"ac-1", "ac-2"},
		},
		{
			name: "zero-criteria story", class: "story", criteria: []string{},
			wantState: StateViolated, wantArea: StateViolated, wantFocus: AreaSuccess,
			wantSumm: "Acceptance criteria are missing", wantWitnes: []string{"Acceptance criteria are missing"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput(t)
			in.Target.Class = tt.class
			in.Journey.Target.Class = tt.class
			in.Journey.Lifecycle.Class = tt.class
			in.Shape.DeclaredObjectIDs = append([]string{}, tt.criteria...)
			in.Success = SuccessFacts{CriterionIDs: tt.criteria, UncoveredCriteria: []string{}}
			snapshot := mustDerive(t, in)
			c := mustConcern(t, snapshot, "success/criteria")
			if c.Area != AreaSuccess || c.State != tt.wantState || !c.Blocking || c.Timing != TimingCurrent || c.WorkClass != "" {
				t.Fatalf("success/criteria = %+v, want area %q state %q blocking current unclassified", c, AreaSuccess, tt.wantState)
			}
			if c.Summary != tt.wantSumm || !reflect.DeepEqual(c.Witnesses, tt.wantWitnes) {
				t.Fatalf("success/criteria summary/witnesses = %q/%q, want %q/%q", c.Summary, c.Witnesses, tt.wantSumm, tt.wantWitnes)
			}
			if got := snapshot.Areas[1].State; got != tt.wantArea {
				t.Fatalf("success area state = %q, want %q", got, tt.wantArea)
			}
			if snapshot.CurrentFocus != tt.wantFocus {
				t.Fatalf("current focus = %q, want %q", snapshot.CurrentFocus, tt.wantFocus)
			}
			if tt.wantState == StateProven {
				if c.Guidance != "" || c.Destination.BoardPath != "" || len(c.Destination.CLI) != 0 {
					t.Fatalf("proven success/criteria carries guidance or destination: %+v", c)
				}
				return
			}
			if c.Guidance != Guidance(GuidanceCriteria, GuidanceFacts{}) {
				t.Fatalf("violated success/criteria guidance = %q, want the shared criteria sentence", c.Guidance)
			}
			if c.Destination.BoardPath != in.Target.BoardPath {
				t.Fatalf("violated success/criteria destination = %+v, want the board", c.Destination)
			}
			if !attentionContains(snapshot.Attention, "success/criteria") {
				t.Fatalf("violated success/criteria is missing from attention")
			}
		})
	}
}

// TestDeriveSuccessCoverage is SI-338 (4): one success/coverage/<ac> row
// per acceptance criterion no non-spike stub lists, on features only —
// unproven (never proven), non-blocking, current, with a board or CLI
// destination and the criterion as its Object. A covered criterion emits
// no row at all.
func TestDeriveSuccessCoverage(t *testing.T) {
	t.Parallel()

	t.Run("uncovered criteria on a feature", func(t *testing.T) {
		in := baseInput(t)
		in.Shape.DeclaredObjectIDs = []string{"ac-1", "ac-2", "ac-3"}
		in.Success = SuccessFacts{CriterionIDs: []string{"ac-1", "ac-2", "ac-3"}, UncoveredCriteria: []string{"ac-3", "ac-2"}}
		snapshot := mustDerive(t, in)
		for _, ac := range []string{"ac-2", "ac-3"} {
			c := mustConcern(t, snapshot, "success/coverage/"+ac)
			if c.Area != AreaSuccess || c.State != StateUnproven || c.Blocking || c.Timing != TimingCurrent || c.WorkClass != "" {
				t.Fatalf("coverage concern = %+v, want success, unproven, nonblocking, current, unclassified", c)
			}
			if c.Object != ac {
				t.Fatalf("coverage concern %q Object = %q, want %q", c.ID, c.Object, ac)
			}
			if c.Summary != "No stub covers acceptance criterion "+ac || !reflect.DeepEqual(c.Witnesses, []string{"declared stub coverage count for " + ac + " is 0"}) {
				t.Fatalf("coverage concern summary/witnesses = %q/%q", c.Summary, c.Witnesses)
			}
			if c.Destination.BoardPath != in.Target.BoardPath {
				t.Fatalf("coverage destination = %+v, want the board", c.Destination)
			}
			if c.Guidance != Guidance(GuidanceCoverage, GuidanceFacts{Object: ac}) {
				t.Fatalf("coverage guidance = %q", c.Guidance)
			}
			if !attentionContains(snapshot.Attention, c.ID) {
				t.Fatalf("coverage concern %q is missing from attention", c.ID)
			}
		}
		for _, c := range snapshot.AllConcerns {
			if c.ID == "success/coverage/ac-1" {
				t.Fatalf("covered criterion ac-1 emitted a coverage row: %+v", c)
			}
		}
		if snapshot.Areas[1].State != StateProven || snapshot.CurrentFocus != "" {
			t.Fatalf("non-blocking coverage moved the success area or focus: area=%q focus=%q", snapshot.Areas[1].State, snapshot.CurrentFocus)
		}
	})

	t.Run("CLI destination without a board", func(t *testing.T) {
		in := baseInput(t)
		in.Target.BoardPath = ""
		in.Shape.DeclaredObjectIDs = []string{"ac-1", "ac-2"}
		in.Success = SuccessFacts{CriterionIDs: []string{"ac-1", "ac-2"}, UncoveredCriteria: []string{"ac-2"}}
		c := mustConcern(t, mustDerive(t, in), "success/coverage/ac-2")
		if c.Destination.BoardPath != "" || !reflect.DeepEqual(c.Destination.CLI, in.Fallbacks.Success) {
			t.Fatalf("coverage destination = %+v, want the success CLI fallback", c.Destination)
		}
	})

	t.Run("a story never carries coverage", func(t *testing.T) {
		in := baseInput(t)
		in.Target.Class = "story"
		in.Journey.Target.Class = "story"
		in.Journey.Lifecycle.Class = "story"
		in.Shape.DeclaredObjectIDs = []string{"ac-1", "ac-2"}
		in.Success = SuccessFacts{CriterionIDs: []string{"ac-1", "ac-2"}, UncoveredCriteria: []string{"ac-2"}}
		// The input is refused before anything is derived; Snapshot.Validate
		// refuses the same row again (TestValidateSuccessFamilies).
		if _, err := Derive(in); err == nil || !strings.Contains(err.Error(), "only to a feature") || strings.Contains(err.Error(), "derived invalid snapshot") {
			t.Fatalf("Derive(story with uncovered criteria) error = %v, want the input's features-only refusal", err)
		}
		in.Success.UncoveredCriteria = []string{}
		for _, c := range mustDerive(t, in).AllConcerns {
			if strings.HasPrefix(c.ID, "success/coverage/") {
				t.Fatalf("story derivation emitted coverage concern %q", c.ID)
			}
		}
	})
}

func TestValidateInputRejectsInvalidSuccessFacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Input)
		wantErr string
	}{
		{name: "nil criterion ids", mutate: func(in *Input) { in.Success.CriterionIDs = nil }, wantErr: "criterion ids must be non-nil"},
		{name: "nil uncovered criteria", mutate: func(in *Input) { in.Success.UncoveredCriteria = nil }, wantErr: "uncovered criteria must be non-nil"},
		{name: "duplicate criterion", mutate: func(in *Input) { in.Success.CriterionIDs = []string{"ac-1", "ac-1"} }, wantErr: "duplicate"},
		{name: "undeclared criterion", mutate: func(in *Input) { in.Success.CriterionIDs = []string{"ac-1", "ac-9"} }, wantErr: "not a declared object"},
		{name: "uncovered criterion not declared as a criterion", mutate: func(in *Input) { in.Success.UncoveredCriteria = []string{"ac-9"} }, wantErr: "not a declared acceptance criterion"},
		{name: "duplicate uncovered criterion", mutate: func(in *Input) { in.Success.UncoveredCriteria = []string{"ac-1", "ac-1"} }, wantErr: "duplicate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput(t)
			tt.mutate(&in)
			if _, err := Derive(in); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Derive() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestValidateSuccessFamilies pins the closed grammar's two new success
// families: success/criteria is blocking and proven or violated, never
// unproven; success/coverage/<ac> is non-blocking, current, unproven, and
// only on a feature target.
func TestValidateSuccessFamilies(t *testing.T) {
	t.Parallel()

	coverageRow := func() Concern {
		c := validConcern("success/coverage/ac-1", AreaSuccess, StateUnproven, false, TimingCurrent)
		c.Witnesses = []string{"declared stub coverage count for ac-1 is 0"}
		c.Destination.BoardPath = "/b/design%2Fexample/board/spec/example"
		c.Guidance = "Plan the delivery."
		c.Object = "ac-1"
		return c
	}
	criteriaRow := func(state State) Concern {
		c := validConcern("success/criteria", AreaSuccess, state, true, TimingCurrent)
		if state != StateProven {
			c.Witnesses = []string{"Acceptance criteria are missing"}
			c.Destination.BoardPath = "/b/design%2Fexample/board/spec/example"
			c.Guidance = "Declare what must be true."
		}
		return c
	}
	// assemble adds rows to the valid all-proven snapshot and recomputes
	// every derived aggregate, so each case below isolates one concern rule.
	assemble := func(rows ...Concern) Snapshot {
		s := validSnapshot()
		s.AllConcerns = append(s.AllConcerns, rows...)
		sort.Slice(s.AllConcerns, func(i, j int) bool { return concernLess(s.AllConcerns[i], s.AllConcerns[j]) })
		states := aggregateAreaStates(s.AllConcerns)
		s.CurrentFocus = ""
		for i, area := range orderedAreas {
			s.Areas[i].State = states[area.ID]
			if s.CurrentFocus == "" && states[area.ID] != StateProven {
				s.CurrentFocus = area.ID
			}
		}
		s.Attention = []Concern{}
		for _, c := range s.AllConcerns {
			if c.State != StateProven {
				s.Attention = append(s.Attention, c)
			}
		}
		focus := s.CurrentFocus
		sort.Slice(s.Attention, func(i, j int) bool { return attentionLessForFocus(focus, s.Attention[i], s.Attention[j]) })
		return s
	}

	if err := assemble(coverageRow(), criteriaRow(StateProven)).Validate(); err != nil {
		t.Fatalf("proven criteria and one coverage row: Validate() = %v", err)
	}
	if err := assemble(criteriaRow(StateViolated)).Validate(); err != nil {
		t.Fatalf("violated criteria: Validate() = %v", err)
	}

	tests := []struct {
		name    string
		snap    func() Snapshot
		wantErr string
	}{
		{name: "proven coverage", snap: func() Snapshot {
			c := coverageRow()
			c.State, c.Witnesses, c.Destination, c.Guidance = StateProven, []string{}, Destination{CLI: []string{}}, ""
			return assemble(c)
		}, wantErr: "coverage concern must be unproven"},
		{name: "violated coverage", snap: func() Snapshot {
			c := coverageRow()
			c.State = StateViolated
			return assemble(c)
		}, wantErr: "coverage concern must be unproven"},
		{name: "eventual coverage", snap: func() Snapshot {
			c := coverageRow()
			c.Timing = TimingEventual
			return assemble(c)
		}, wantErr: "coverage concern must be current"},
		{name: "blocking coverage", snap: func() Snapshot {
			c := coverageRow()
			c.Blocking = true
			return assemble(c)
		}, wantErr: "blocking="},
		{name: "coverage on a story", snap: func() Snapshot {
			s := assemble(coverageRow())
			s.TargetClass = "story"
			return s
		}, wantErr: "only to a feature"},
		{name: "coverage id with a tail", snap: func() Snapshot {
			c := coverageRow()
			c.ID = "success/coverage/ac-1/extra"
			return assemble(c)
		}, wantErr: "not in the closed concern identity vocabulary"},
		{name: "unproven criteria", snap: func() Snapshot {
			return assemble(criteriaRow(StateUnproven))
		}, wantErr: "criteria concern must be proven or violated"},
		{name: "non-blocking criteria", snap: func() Snapshot {
			c := criteriaRow(StateProven)
			c.Blocking = false
			return assemble(c)
		}, wantErr: "blocking="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := tt.snap()
			if err := snapshot.Validate(); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestDeriveObject is SI-338 (2): Object is the declared object id that
// one segment of the concern id names, matched exactly (never by prefix)
// against the declared object ids, otherwise empty. Every family whose id
// carries an object is listed.
func TestDeriveObject(t *testing.T) {
	t.Parallel()

	fixture := mustJourney(t)
	owner := fixture.Blockers.Current[0].Owner
	quality := fixture.Blockers.Current[1]
	quality.ID = "obligation-quality/ac-12/static"
	quality.ClearingCondition = "the obligation quality for ac-12/static is elaborated"
	in := baseInput(t)
	// ac-1 is a prefix of ac-12 and oq-1 a prefix of oq-10: a prefix match
	// would name the shorter id.
	in.Shape.DeclaredObjectIDs = []string{"ac-1", "ac-12", "co-1", "dc-1", "oq-1", "oq-10"}
	in.Shape.OpenQuestionIDs = []string{"oq-1", "oq-10"}
	in.Shape.ClaimedQuestions = []ClaimedQuestion{{QuestionID: "oq-10", StubSlugs: []string{"probe"}}}
	in.Success = SuccessFacts{CriterionIDs: []string{"ac-1", "ac-12"}, UncoveredCriteria: []string{"ac-12"}}
	in.Journey.Blockers.Current = []journey.Blocker{quality}
	in.Journey.Blockers.Eventual.Items = []journey.Blocker{
		{ID: "outcome-floor/ac-12", Reason: journey.ReasonOutcomeFloorUnsatisfied, Class: journey.ClassJudgmental,
			Witnesses: []string{"AC ac-12: outcome floor unsatisfied"}, Owner: owner,
			ClearingCondition: "author an attestation for ac-12", Transition: "close"},
		{ID: "question-claimed/oq-10", Reason: journey.ReasonQuestionClaimedBySpike, Class: journey.ClassMechanical,
			Witnesses: []string{"oq-10 is claimed"}, Owner: owner,
			ClearingCondition: "probe resolves oq-10", Transition: "close"},
		{ID: "stub-unreconciled/probe", Reason: journey.ReasonStubUnreconciled, Class: journey.ClassMechanical,
			Witnesses: []string{"stub probe"}, Owner: owner,
			ClearingCondition: "reconcile stub probe", Transition: "close"},
	}
	if err := in.Journey.Validate(); err != nil {
		t.Fatalf("journey fixture Validate() = %v", err)
	}
	snapshot := mustDerive(t, in)

	tests := []struct {
		id   string
		want string
	}{
		{id: "shape/question/oq-1", want: "oq-1"},
		{id: "shape/question/oq-10", want: "oq-10"},
		{id: "success/coverage/ac-12", want: "ac-12"},
		{id: "success/blocker/obligation-quality/ac-12/static", want: "ac-12"},
		{id: "review/blocker/outcome-floor/ac-12", want: "ac-12"},
		{id: "review/blocker/question-claimed/oq-10", want: "oq-10"},
		{id: "review/blocker/stub-unreconciled/probe", want: ""},
		{id: "shape/problem", want: ""},
		{id: "shape/provenance", want: ""},
		{id: "success/criteria", want: ""},
		{id: "success/contributor/static", want: ""},
		{id: "context/verdict", want: ""},
		{id: "review/action", want: ""},
		{id: "review/eventual-derivation", want: ""},
	}
	for _, tt := range tests {
		if got := mustConcern(t, snapshot, tt.id).Object; got != tt.want {
			t.Errorf("concern %q Object = %q, want %q", tt.id, got, tt.want)
		}
	}
	for _, c := range snapshot.Attention {
		if c.Object != mustConcern(t, snapshot, c.ID).Object {
			t.Fatalf("attention concern %q Object differs from all concerns", c.ID)
		}
	}
}

func TestObjectOf(t *testing.T) {
	t.Parallel()

	declared := map[string]bool{"ac-1": true, "ac-12": true, "oq-1": true}
	tests := []struct {
		id   string
		want string
	}{
		{id: "success/coverage/ac-12", want: "ac-12"},
		{id: "success/coverage/ac-1", want: "ac-1"},
		{id: "success/coverage/ac-123", want: ""},
		{id: "shape/question/oq-1", want: "oq-1"},
		{id: "shape/question/oq-1x", want: ""},
		{id: "success/blocker/obligation-quality/ac-12/static", want: "ac-12"},
		// The family segments themselves never name an object.
		{id: "shape/problem", want: ""},
		{id: "review/role/close/attestation/countersign", want: ""},
	}
	for _, tt := range tests {
		if got := objectOf(tt.id, declared); got != tt.want {
			t.Errorf("objectOf(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
	if got := objectOf("success/coverage/ac-1", nil); got != "" {
		t.Fatalf("objectOf with no declared ids = %q, want empty", got)
	}
}

func TestValidateObjectNamesOneSegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		object  string
		wantErr string
	}{
		{name: "object names no segment", object: "oq-9", wantErr: "object"},
		{name: "object names a family segment", object: "shape", wantErr: "object"},
		{name: "object is a prefix of a segment", object: "oq", wantErr: "object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := validUnresolvedSnapshot()
			mutateConcern(&snapshot, "shape/problem", func(c *Concern) {
				c.ID = "shape/question/oq-1"
				c.Object = tt.object
			})
			if err := snapshot.Validate(); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
	snapshot := validUnresolvedSnapshot()
	mutateConcern(&snapshot, "shape/problem", func(c *Concern) {
		c.ID = "shape/question/oq-1"
		c.Object = "oq-1"
	})
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("object naming the question segment: Validate() = %v", err)
	}
}

// TestDeriveCarriesBoardPath is G3: the snapshot carries the target's board
// path the loader already computes, so the readiness page can link to the
// wall; an absent board path stays empty.
func TestDeriveCarriesBoardPath(t *testing.T) {
	t.Parallel()

	in := baseInput(t)
	if got := mustDerive(t, in).BoardPath; got != in.Target.BoardPath {
		t.Fatalf("snapshot BoardPath = %q, want %q", got, in.Target.BoardPath)
	}
	in.Target.BoardPath = ""
	if got := mustDerive(t, in).BoardPath; got != "" {
		t.Fatalf("snapshot BoardPath = %q, want empty", got)
	}

	for _, bad := range []string{"b/design/board", "/b/design\n/board"} {
		snapshot := validSnapshot()
		snapshot.BoardPath = bad
		if err := snapshot.Validate(); err == nil || !strings.Contains(err.Error(), "board path") {
			t.Fatalf("Validate(BoardPath %q) error = %v, want a board path refusal", bad, err)
		}
	}
}
