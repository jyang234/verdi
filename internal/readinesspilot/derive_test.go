package readinesspilot

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/policyconflict"
)

func TestDeriveHappyPath(t *testing.T) {
	t.Parallel()

	snapshot, err := Derive(baseInput(t))
	if err != nil {
		t.Fatalf("Derive() error = %v", err)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("derived snapshot Validate() error = %v", err)
	}
	if snapshot.CurrentFocus != "" || len(snapshot.Attention) != 0 {
		t.Fatalf("all-proven snapshot focus/attention = %q/%+v, want empty", snapshot.CurrentFocus, snapshot.Attention)
	}
	if snapshot.TargetRef != "spec/example" || snapshot.TargetTitle != "Exact source title" || snapshot.Branch != "design/example" || snapshot.Head != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("snapshot identity = %+v, want exact input identity", snapshot)
	}
	wantLabels := []string{"Define the work", "Define success", "Check constraints", "Get approval"}
	for i, area := range snapshot.Areas {
		if area.State != StateProven {
			t.Fatalf("area %q state = %q, want %q", area.ID, area.State, StateProven)
		}
		if area.Label != wantLabels[i] {
			t.Fatalf("area[%d] label = %q, want %q", i, area.Label, wantLabels[i])
		}
	}
}

func TestDeriveShapeRequiredContentAndQuestions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*Input)
		concernID string
		wantState State
	}{
		{
			name: "problem present",
			mutate: func(in *Input) {
				in.Shape.ProblemPresent = true
			},
			concernID: "shape/problem",
			wantState: StateProven,
		},
		{
			name: "problem missing",
			mutate: func(in *Input) {
				in.Shape.ProblemPresent = false
			},
			concernID: "shape/problem",
			wantState: StateViolated,
		},
		{
			name: "outcome present",
			mutate: func(in *Input) {
				in.Shape.OutcomePresent = true
			},
			concernID: "shape/outcome",
			wantState: StateProven,
		},
		{
			name: "outcome missing",
			mutate: func(in *Input) {
				in.Shape.OutcomePresent = false
			},
			concernID: "shape/outcome",
			wantState: StateViolated,
		},
		{
			name: "declared open question",
			mutate: func(in *Input) {
				in.Shape.OpenQuestionIDs = []string{"oq-1"}
			},
			concernID: "shape/question/oq-1",
			wantState: StateUnproven,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput(t)
			tt.mutate(&in)
			snapshot := mustDerive(t, in)
			concern := mustConcern(t, snapshot, tt.concernID)
			if concern.State != tt.wantState {
				t.Fatalf("concern %q state = %q, want %q", tt.concernID, concern.State, tt.wantState)
			}
			if tt.wantState != StateProven && (!concern.Blocking || concern.Destination.BoardPath != in.Target.BoardPath) {
				t.Fatalf("unresolved required-shape concern = %+v, want blocking board destination", concern)
			}
		})
	}
}

func TestDeriveProvenanceMutationAndBoardPostures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*Input)
		concernID string
		wantState State
		wantCLI   []string
	}{
		{
			name: "proven provenance",
			mutate: func(in *Input) {
				in.Provenance.ChainState = StateProven
			},
			concernID: "shape/provenance",
			wantState: StateProven,
		},
		{
			name: "unclassified direct markdown gap",
			mutate: func(in *Input) {
				in.Provenance.ChainState = StateUnproven
				in.Provenance.ChainWitnesses = []string{"direct Markdown change has unclassified origin"}
			},
			concernID: "shape/provenance",
			wantState: StateUnproven,
			wantCLI:   []string{"verdi", "design", "provenance", "spec/example"},
		},
		{
			name: "missing provenance",
			mutate: func(in *Input) {
				in.Provenance.ChainState = StateUnproven
				in.Provenance.ChainWitnesses = []string{"design-provenance sidecar is absent"}
			},
			concernID: "shape/provenance",
			wantState: StateUnproven,
			wantCLI:   []string{"verdi", "design", "provenance", "spec/example"},
		},
		{
			name: "violated provenance",
			mutate: func(in *Input) {
				in.Provenance.ChainState = StateViolated
				in.Provenance.ChainWitnesses = []string{"provenance chain mismatch"}
			},
			concernID: "shape/provenance",
			wantState: StateViolated,
			wantCLI:   []string{"verdi", "design", "provenance", "spec/example"},
		},
		{
			name: "mutation residue",
			mutate: func(in *Input) {
				in.Provenance.MutationState = StateUnproven
				in.Provenance.MutationWitnesses = []string{"draft mutation journal is present"}
			},
			concernID: "shape/mutation",
			wantState: StateUnproven,
			wantCLI:   []string{"verdi", "design", "provenance", "spec/example"},
		},
		{
			name: "board unavailable",
			mutate: func(in *Input) {
				in.Board.State = StateUnproven
				in.Board.Witnesses = []string{"scratch board enumeration unavailable"}
			},
			concernID: "shape/board",
			wantState: StateUnproven,
			wantCLI:   []string{"verdi", "design", "provenance", "spec/example"},
		},
		{
			name: "open board question",
			mutate: func(in *Input) {
				in.Board.OpenItems = []BoardItem{{ID: "oq-2", Kind: "question"}}
			},
			concernID: "shape/board/question/oq-2",
			wantState: StateUnproven,
		},
		{
			name: "open board agent task",
			mutate: func(in *Input) {
				in.Board.OpenItems = []BoardItem{{ID: "task-2", Kind: "agent-task"}}
			},
			concernID: "shape/board/agent-task/task-2",
			wantState: StateUnproven,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput(t)
			tt.mutate(&in)
			concern := mustConcern(t, mustDerive(t, in), tt.concernID)
			if concern.State != tt.wantState || concern.Blocking {
				t.Fatalf("concern = %+v, want state %q and nonblocking", concern, tt.wantState)
			}
			if tt.wantCLI != nil && !reflect.DeepEqual(concern.Destination.CLI, tt.wantCLI) {
				t.Fatalf("concern CLI = %q, want %q", concern.Destination.CLI, tt.wantCLI)
			}
			if strings.HasPrefix(tt.concernID, "shape/board/") && concern.Destination.BoardPath != in.Target.BoardPath {
				t.Fatalf("open board item destination = %+v, want board path %q", concern.Destination, in.Target.BoardPath)
			}
		})
	}
}

func TestDeriveCarriesDeclaredObjectIDsInShapeProvenance(t *testing.T) {
	t.Parallel()

	in := baseInput(t)
	in.Shape.DeclaredObjectIDs = []string{"decision-2", "constraint-1"}
	concern := mustConcern(t, mustDerive(t, in), "shape/provenance")
	for _, id := range in.Shape.DeclaredObjectIDs {
		if !contains(concern.Witnesses, id) {
			t.Fatalf("shape provenance witnesses = %q, want declared object %q retained", concern.Witnesses, id)
		}
	}
}

func TestDeriveJourneyEvidenceResolutions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		resolution string
		want       State
	}{
		{resolution: "proven", want: StateProven},
		{resolution: "violated-with-witness", want: StateViolated},
		{resolution: "unproven", want: StateUnproven},
	}

	for _, tt := range tests {
		t.Run(tt.resolution, func(t *testing.T) {
			in := baseInput(t)
			in.Journey.Evidence.Contributors[0].Resolution = tt.resolution
			in.Journey.Evidence.Contributors[0].Witness = "static evidence witness"
			concern := mustConcern(t, mustDerive(t, in), "success/contributor/static")
			if concern.State != tt.want || concern.Blocking || concern.WorkClass != "" {
				t.Fatalf("evidence concern = %+v, want state %q, nonblocking, unclassified", concern, tt.want)
			}
			if tt.want != StateProven && concern.Destination.BoardPath != in.Target.BoardPath {
				t.Fatalf("evidence destination = %+v, want board path", concern.Destination)
			}
		})
	}
}

func TestDeriveJourneyBlockerRoutingAndTiming(t *testing.T) {
	t.Parallel()

	fixture := mustJourney(t)
	nonQuality := fixture.Blockers.Current[0]
	quality := fixture.Blockers.Current[1]
	tests := []struct {
		name      string
		blocker   journey.Blocker
		eventual  bool
		concernID string
		area      AreaID
		fallback  []string
	}{
		{
			name:      "current obligation quality",
			blocker:   quality,
			concernID: "success/blocker/obligation-quality/ac-2/runtime",
			area:      AreaSuccess,
			fallback:  []string{"verdi", "journey", "spec/example", "--success"},
		},
		{
			name:      "eventual obligation quality",
			blocker:   quality,
			eventual:  true,
			concernID: "success/blocker/obligation-quality/ac-2/runtime",
			area:      AreaSuccess,
			fallback:  []string{"verdi", "journey", "spec/example", "--success"},
		},
		{
			name:      "current non quality",
			blocker:   nonQuality,
			concernID: "review/blocker/forge-facts-unavailable/close",
			area:      AreaReview,
			fallback:  []string{"verdi", "journey", "spec/example", "--review"},
		},
		{
			name:      "eventual non quality",
			blocker:   nonQuality,
			eventual:  true,
			concernID: "review/blocker/forge-facts-unavailable/close",
			area:      AreaReview,
			fallback:  []string{"verdi", "journey", "spec/example", "--review"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput(t)
			if tt.eventual {
				in.Journey.Blockers.Eventual.Items = []journey.Blocker{tt.blocker}
			} else {
				in.Journey.Blockers.Current = []journey.Blocker{tt.blocker}
			}
			concern := mustConcern(t, mustDerive(t, in), tt.concernID)
			wantTiming := TimingCurrent
			wantBlocking := true
			if tt.eventual {
				wantTiming = TimingEventual
				wantBlocking = false
			}
			if concern.Area != tt.area || concern.State != StateViolated || concern.Timing != wantTiming || concern.Blocking != wantBlocking {
				t.Fatalf("blocker concern = %+v, want area=%q state=%q timing=%q blocking=%v", concern, tt.area, StateViolated, wantTiming, wantBlocking)
			}
			if concern.WorkClass != tt.blocker.Class || !reflect.DeepEqual(concern.Destination.CLI, tt.fallback) {
				t.Fatalf("blocker class/destination = %q/%q, want %q/%q", concern.WorkClass, concern.Destination.CLI, tt.blocker.Class, tt.fallback)
			}
		})
	}
}

func TestDeriveConflictVerdictsAndRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		verdict policyconflict.Verdict
		want    State
	}{
		{verdict: policyconflict.VerdictPass, want: StateProven},
		{verdict: policyconflict.VerdictBlockedViolated, want: StateViolated},
		{verdict: policyconflict.VerdictBlockedUnproven, want: StateUnproven},
	}

	for _, tt := range tests {
		t.Run(string(tt.verdict), func(t *testing.T) {
			in := baseInput(t)
			in.Conflict = conflictReport(t, tt.verdict)
			snapshot := mustDerive(t, in)
			verdict := mustConcern(t, snapshot, "context/verdict")
			if verdict.State != tt.want || !verdict.Blocking {
				t.Fatalf("verdict concern = %+v, want state %q and blocking", verdict, tt.want)
			}
			if tt.want != StateProven && !reflect.DeepEqual(verdict.Destination.CLI, in.Fallbacks.Context) {
				t.Fatalf("verdict destination = %+v, want context fallback", verdict.Destination)
			}
		})
	}

	pass := mustDerive(t, baseInput(t))
	mechanical := mustConcern(t, pass, "context/mechanical/mechanical/go-toolchain")
	if mechanical.State != StateProven || mechanical.Blocking {
		t.Fatalf("mechanical detail = %+v, want proven nonblocking", mechanical)
	}

	unprovenInput := baseInput(t)
	unprovenInput.Conflict = conflictReport(t, policyconflict.VerdictBlockedUnproven)
	semantic := mustConcern(t, mustDerive(t, unprovenInput), "context/semantic/semantic/example-conflict")
	if semantic.State != StateUnproven || semantic.Blocking {
		t.Fatalf("semantic detail = %+v, want unproven nonblocking", semantic)
	}

	disclosureInput := baseInput(t)
	disclosureInput.Conflict.Disclosures = []policyconflict.Disclosure{{
		Code:      contextcompile.DisclosureRepositoryRemoteUnknown,
		Witnesses: []string{"remote identity unavailable"},
	}}
	if err := disclosureInput.Conflict.Validate(); err != nil {
		t.Fatalf("test report Validate() error = %v", err)
	}
	disclosure := mustConcern(t, mustDerive(t, disclosureInput), "context/disclosure/repository-remote-unknown")
	if disclosure.State != StateUnproven || disclosure.Blocking {
		t.Fatalf("disclosure detail = %+v, want unproven nonblocking", disclosure)
	}
}

func TestDeriveReviewPrincipalAndActionFacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		resolution string
		want       State
	}{
		{resolution: "authenticated", want: StateProven},
		{resolution: "violated-with-witness", want: StateViolated},
		{resolution: "unproven", want: StateUnproven},
	}

	for _, tt := range tests {
		t.Run("role "+tt.resolution, func(t *testing.T) {
			in := baseInput(t)
			in.Journey.Principals.Required = []journey.RequiredRole{{
				Transition: "review", Obligation: "attestation/countersign", Count: 1, Resolution: tt.resolution,
			}}
			in.Journey.Principals.Disclosures = []string{"principal requirement witness"}
			concern := mustConcern(t, mustDerive(t, in), "review/role/review/attestation/countersign")
			if concern.State != tt.want || concern.Blocking || concern.WorkClass != "" {
				t.Fatalf("role concern = %+v, want state %q, nonblocking, unclassified", concern, tt.want)
			}
		})
	}

	t.Run("missing principal facts", func(t *testing.T) {
		in := baseInput(t)
		in.Journey.Principals.ProfileAdopted = false
		in.Journey.Principals.SelectedProfileID = ""
		in.Journey.Principals.SelectedProfileDigest = ""
		in.Journey.Principals.Required = []journey.RequiredRole{}
		in.Journey.Principals.Disclosures = []string{"principal profile unavailable"}
		concern := mustConcern(t, mustDerive(t, in), "review/action")
		if concern.State != StateUnproven || !concern.Blocking || !contains(concern.Witnesses, "principal profile unavailable") {
			t.Fatalf("review action = %+v, want blocking unproven principal witness", concern)
		}
	})

	t.Run("missing action facts", func(t *testing.T) {
		in := baseInput(t)
		in.Journey.Actions.Safe = []journey.Action{}
		in.Journey.Actions.NeededFacts = []string{"review-transition"}
		concern := mustConcern(t, mustDerive(t, in), "review/action")
		if concern.State != StateUnproven || !concern.Blocking || !contains(concern.Witnesses, "review-transition") {
			t.Fatalf("review action = %+v, want blocking unproven action witness", concern)
		}
		if !reflect.DeepEqual(concern.Destination.CLI, in.Fallbacks.Review) {
			t.Fatalf("review destination = %+v, want review fallback", concern.Destination)
		}
	})

	t.Run("unproven lifecycle posture", func(t *testing.T) {
		in := baseInput(t)
		in.Journey.Lifecycle.State = "unproven"
		in.Journey.Lifecycle.Relation = "unproven"
		in.Journey.Lifecycle.Posture = "unknown"
		in.Journey.Lifecycle.Disclosures = []string{"lifecycle posture unavailable"}
		concern := mustConcern(t, mustDerive(t, in), "review/action")
		if concern.State != StateUnproven || !contains(concern.Witnesses, "lifecycle posture unavailable") {
			t.Fatalf("review action = %+v, want lifecycle disclosure", concern)
		}
	})

	t.Run("eventual blockers unavailable", func(t *testing.T) {
		in := baseInput(t)
		in.Journey.Blockers.Eventual.Derived = false
		in.Journey.Blockers.Eventual.Disclosures = []string{"eventual blockers unavailable"}
		concern := mustConcern(t, mustDerive(t, in), "review/action")
		if concern.State != StateUnproven || !contains(concern.Witnesses, "eventual blockers unavailable") {
			t.Fatalf("review action = %+v, want unavailable-eventual-blocker disclosure", concern)
		}
	})
}

func TestDeriveRetainsLaterViolationWhenEarlierAreaIsCurrent(t *testing.T) {
	t.Parallel()

	in := baseInput(t)
	in.Shape.OpenQuestionIDs = []string{"oq-1"}
	in.Conflict = conflictReport(t, policyconflict.VerdictBlockedViolated)
	snapshot := mustDerive(t, in)
	if snapshot.CurrentFocus != AreaShape {
		t.Fatalf("CurrentFocus = %q, want %q", snapshot.CurrentFocus, AreaShape)
	}
	if concern := mustConcern(t, snapshot, "context/verdict"); concern.State != StateViolated {
		t.Fatalf("later context concern = %+v, want retained violation", concern)
	}
	if !attentionContains(snapshot.Attention, "context/verdict") {
		t.Fatalf("attention omitted later violation: %+v", snapshot.Attention)
	}
}

func TestDeriveGroupsAttentionByCurrentFocusThenExistingComparator(t *testing.T) {
	t.Parallel()

	in := baseInput(t)
	in.Shape.ProblemPresent = false
	in.Shape.OpenQuestionIDs = []string{"oq-1"}
	in.Provenance.ChainState = StateUnproven
	in.Provenance.ChainWitnesses = []string{"design provenance is unavailable"}
	in.Journey.Blockers.Current = mustJourney(t).Blockers.Current
	in.Conflict = conflictReport(t, policyconflict.VerdictBlockedViolated)

	snapshot := mustDerive(t, in)
	if snapshot.CurrentFocus != AreaShape {
		t.Fatalf("CurrentFocus = %q, want %q", snapshot.CurrentFocus, AreaShape)
	}
	wantIDs := []string{
		"shape/problem",
		"shape/question/oq-1",
		"shape/provenance",
		"success/blocker/obligation-quality/ac-2/runtime",
		"context/verdict",
		"review/blocker/forge-facts-unavailable/close",
		"context/semantic/semantic/example-conflict",
	}
	gotIDs := make([]string, len(snapshot.Attention))
	for i, concern := range snapshot.Attention {
		gotIDs[i] = concern.ID
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("attention ids = %q, want current-focus group then unchanged comparator order %q", gotIDs, wantIDs)
	}

	currentCount := 0
	remainderCount := 0
	seenRemainder := false
	positions := make(map[string]int, len(snapshot.Attention))
	for i, concern := range snapshot.Attention {
		positions[concern.ID] = i
		isCurrent := concern.Area == snapshot.CurrentFocus
		if isCurrent {
			currentCount++
			if seenRemainder {
				t.Fatalf("current-focus concern %q follows remainder: %+v", concern.ID, snapshot.Attention)
			}
		} else {
			remainderCount++
			seenRemainder = true
		}
		if i > 0 {
			previous := snapshot.Attention[i-1]
			if (previous.Area == snapshot.CurrentFocus) == isCurrent && !attentionLess(previous, concern) {
				t.Fatalf("existing comparator changed within attention group at %q then %q", previous.ID, concern.ID)
			}
		}
	}
	if currentCount < 2 || remainderCount < 2 {
		t.Fatalf("test requires at least two concerns per group, got current=%d remainder=%d", currentCount, remainderCount)
	}
	if positions["shape/question/oq-1"] >= positions["context/verdict"] {
		t.Fatalf("current-area unproven concern did not precede downstream blocking violation: %+v", snapshot.Attention)
	}
}

func TestDeriveInputOrderDeterminism(t *testing.T) {
	t.Parallel()

	first := baseInput(t)
	first.Shape.DeclaredObjectIDs = []string{"decision-b", "constraint-a"}
	first.Shape.OpenQuestionIDs = []string{"oq-b", "oq-a"}
	first.Provenance.ChainWitnesses = []string{"z-chain", "a-chain"}
	first.Provenance.MutationWitnesses = []string{"z-mutation", "a-mutation"}
	first.Board.OpenItems = []BoardItem{{ID: "z-task", Kind: "agent-task"}, {ID: "a-question", Kind: "question"}}
	first.Board.Witnesses = []string{"z-board", "a-board"}

	second := baseInput(t)
	second.Shape.DeclaredObjectIDs = []string{"constraint-a", "decision-b"}
	second.Shape.OpenQuestionIDs = []string{"oq-a", "oq-b"}
	second.Provenance.ChainWitnesses = []string{"a-chain", "z-chain"}
	second.Provenance.MutationWitnesses = []string{"a-mutation", "z-mutation"}
	second.Board.OpenItems = []BoardItem{{ID: "a-question", Kind: "question"}, {ID: "z-task", Kind: "agent-task"}}
	second.Board.Witnesses = []string{"a-board", "z-board"}

	a := mustDerive(t, first)
	b := mustDerive(t, second)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("permuted inputs derived different snapshots:\nfirst:  %+v\nsecond: %+v", a, b)
	}
}

func TestValidateInputRejectsInvalidPosturesAndSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Input)
		wantErr string
	}{
		{
			name: "missing target title",
			mutate: func(in *Input) {
				in.Target.Title = ""
			},
			wantErr: "target title",
		},
		{
			name: "control-bearing target title",
			mutate: func(in *Input) {
				in.Target.Title = "Unsafe\ntitle"
			},
			wantErr: "target title",
		},
		{
			name: "invalid chain enum",
			mutate: func(in *Input) {
				in.Provenance.ChainState = State("unknown")
			},
			wantErr: "chain state",
		},
		{
			name: "violated mutation posture",
			mutate: func(in *Input) {
				in.Provenance.MutationState = StateViolated
			},
			wantErr: "mutation state",
		},
		{
			name: "violated board posture",
			mutate: func(in *Input) {
				in.Board.State = StateViolated
			},
			wantErr: "board state",
		},
		{
			name: "invalid board item kind",
			mutate: func(in *Input) {
				in.Board.OpenItems = []BoardItem{{ID: "task-1", Kind: "todo"}}
			},
			wantErr: "board item kind",
		},
		{
			name: "invalid board item id",
			mutate: func(in *Input) {
				in.Board.OpenItems = []BoardItem{{ID: "bad\nid", Kind: "question"}}
			},
			wantErr: "board item id",
		},
		{
			name: "accepts nil declared object ids",
			mutate: func(in *Input) {
				in.Shape.DeclaredObjectIDs = nil
			},
			wantErr: "declared object ids must be non-nil",
		},
		{
			name: "accepts nil open question ids",
			mutate: func(in *Input) {
				in.Shape.OpenQuestionIDs = nil
			},
			wantErr: "open question ids must be non-nil",
		},
		{
			name: "accepts nil chain witnesses",
			mutate: func(in *Input) {
				in.Provenance.ChainWitnesses = nil
			},
			wantErr: "chain witnesses must be non-nil",
		},
		{
			name: "accepts empty chain witness element",
			mutate: func(in *Input) {
				in.Provenance.ChainState = StateUnproven
				in.Provenance.ChainWitnesses = []string{""}
			},
			wantErr: "chain witnesses contains an empty witness",
		},
		{
			name: "accepts nil mutation witnesses",
			mutate: func(in *Input) {
				in.Provenance.MutationWitnesses = nil
			},
			wantErr: "mutation witnesses must be non-nil",
		},
		{
			name: "accepts empty mutation witness element",
			mutate: func(in *Input) {
				in.Provenance.MutationState = StateUnproven
				in.Provenance.MutationWitnesses = []string{""}
			},
			wantErr: "mutation witnesses contains an empty witness",
		},
		{
			name: "accepts nil board open items",
			mutate: func(in *Input) {
				in.Board.OpenItems = nil
			},
			wantErr: "open items must be non-nil",
		},
		{
			name: "accepts nil board witnesses",
			mutate: func(in *Input) {
				in.Board.Witnesses = nil
			},
			wantErr: "board witnesses must be non-nil",
		},
		{
			name: "accepts empty board witness element",
			mutate: func(in *Input) {
				in.Board.State = StateUnproven
				in.Board.Witnesses = []string{""}
			},
			wantErr: "board witnesses contains an empty witness",
		},
		{
			name: "accepts nil shape fallback",
			mutate: func(in *Input) {
				in.Fallbacks.Shape = nil
			},
			wantErr: "shape fallback must be non-nil",
		},
		{
			name: "accepts nil success fallback",
			mutate: func(in *Input) {
				in.Fallbacks.Success = nil
			},
			wantErr: "success fallback must be non-nil",
		},
		{
			name: "accepts nil context fallback",
			mutate: func(in *Input) {
				in.Fallbacks.Context = nil
			},
			wantErr: "context fallback must be non-nil",
		},
		{
			name: "accepts nil review fallback",
			mutate: func(in *Input) {
				in.Fallbacks.Review = nil
			},
			wantErr: "review fallback must be non-nil",
		},
		{
			name: "journey validator reused",
			mutate: func(in *Input) {
				in.Journey.Schema = "invented"
			},
			wantErr: "journey:",
		},
		{
			name: "policy conflict validator reused",
			mutate: func(in *Input) {
				in.Conflict.Schema = "invented"
			},
			wantErr: "policyconflict:",
		},
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

func baseInput(t *testing.T) Input {
	t.Helper()
	record := mustJourney(t)
	record.Blockers.Current = []journey.Blocker{}
	record.Blockers.Eventual.Derived = true
	record.Blockers.Eventual.Items = []journey.Blocker{}
	record.Blockers.Eventual.Disclosures = []string{}
	record.Principals.Required = []journey.RequiredRole{}
	record.Principals.Disclosures = []string{}
	record.Actions.NeededFacts = []string{}
	record.Lifecycle.State = "proposed"
	record.Lifecycle.Relation = "new"
	record.Lifecycle.Posture = "advisory"
	record.Lifecycle.AcceptedBaseline = nil
	record.Lifecycle.Disclosures = []string{}
	if err := record.Validate(); err != nil {
		t.Fatalf("base journey Validate() error = %v", err)
	}

	return Input{
		Target: TargetFacts{
			Ref:       "spec/example",
			Title:     "Exact source title",
			Class:     "feature",
			Branch:    "design/example",
			Head:      "0123456789abcdef0123456789abcdef01234567",
			BoardPath: "/b/design%2Fexample/board/spec/example",
		},
		Shape: ShapeFacts{
			ProblemPresent:    true,
			OutcomePresent:    true,
			DeclaredObjectIDs: []string{"ac-1"},
			OpenQuestionIDs:   []string{},
		},
		Provenance: ProvenanceFacts{
			ChainState:        StateProven,
			ChainWitnesses:    []string{"design-provenance chain classified"},
			MutationState:     StateProven,
			MutationWitnesses: []string{"no mutation residue"},
		},
		Board: BoardFacts{
			State:     StateProven,
			OpenItems: []BoardItem{},
			Witnesses: []string{"scratch board enumerated"},
		},
		Journey:  record,
		Conflict: conflictReport(t, policyconflict.VerdictPass),
		Fallbacks: Fallbacks{
			Shape:   []string{"verdi", "design", "provenance", "spec/example"},
			Success: []string{"verdi", "journey", "spec/example", "--success"},
			Context: []string{"verdi", "context", "conflict", "--request", ".verdi/context-request.json"},
			Review:  []string{"verdi", "journey", "spec/example", "--review"},
		},
		RequestDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
}

func mustJourney(t *testing.T) journey.Record {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "journey", "testdata", "canonical-record.json"))
	if err != nil {
		t.Fatalf("read journey fixture: %v", err)
	}
	record, err := journey.Decode(data)
	if err != nil {
		t.Fatalf("decode journey fixture: %v", err)
	}
	record.Digest = ""
	return record
}

func conflictReport(t *testing.T, verdict policyconflict.Verdict) policyconflict.Report {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "policyconflict", "testdata", "report.json"))
	if err != nil {
		t.Fatalf("read policy conflict fixture: %v", err)
	}
	report, err := policyconflict.DecodeReport(data)
	if err != nil {
		t.Fatalf("decode policy conflict fixture: %v", err)
	}
	report.Digest = ""
	switch verdict {
	case policyconflict.VerdictPass:
		report.Semantic = []policyconflict.SemanticEvaluation{}
		report.Verdict = verdict
	case policyconflict.VerdictBlockedUnproven:
		report.Verdict = verdict
	case policyconflict.VerdictBlockedViolated:
		report.Semantic[0].State = policyconflict.ProofViolatedWithWitness
		report.Semantic[0].Reasons = []policyconflict.ReasonCode{policyconflict.ReasonDispositionEffectiveConflict}
		report.Verdict = verdict
	default:
		t.Fatalf("unsupported test verdict %q", verdict)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("test report Validate() error = %v", err)
	}
	return report
}

func mustDerive(t *testing.T, in Input) Snapshot {
	t.Helper()
	snapshot, err := Derive(in)
	if err != nil {
		t.Fatalf("Derive() error = %v", err)
	}
	return snapshot
}

func mustConcern(t *testing.T, snapshot Snapshot, id string) Concern {
	t.Helper()
	for _, concern := range snapshot.AllConcerns {
		if concern.ID == id {
			return concern
		}
	}
	t.Fatalf("concern %q not found in %+v", id, snapshot.AllConcerns)
	return Concern{}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func attentionContains(attention []Concern, id string) bool {
	for _, concern := range attention {
		if concern.ID == id {
			return true
		}
	}
	return false
}
