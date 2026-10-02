package readinesspilot

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// TestGuidance_SharedSentences pins the moved wall sentences byte for byte
// (SI-338 (1)): the wall shell (internal/workbench's deriveASDShell) and
// this derivation both render them through Guidance, so a change here
// changes both surfaces together and neither keeps a copy.
func TestGuidance_SharedSentences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		family GuidanceFamily
		facts  GuidanceFacts
		want   string
	}{
		{name: "problem", family: GuidanceProblem, want: "State the problem (typed operation set-problem) — the case file opens with it."},
		{name: "outcome", family: GuidanceOutcome, want: "State the intended outcome (typed operation set-outcome)."},
		{
			name: "unclaimed question", family: GuidanceQuestion, facts: GuidanceFacts{Object: "oq-3"},
			want: "Resolve it on the wall: edit or remove oq-3, or graduate a decision that answers it.",
		},
		{
			name: "question claimed by one stub", family: GuidanceQuestion,
			facts: GuidanceFacts{Object: "oq-3", SpikeWord: "probe", ClaimingStubs: 1},
			want:  "No wall edit is required to accept: the claiming probe stub answers it after acceptance.",
		},
		{
			name: "question claimed by two stubs", family: GuidanceQuestion,
			facts: GuidanceFacts{Object: "oq-3", SpikeWord: "probe", ClaimingStubs: 2},
			want:  "No wall edit is required to accept: the claiming probe stubs answer it after acceptance.",
		},
		{name: "scratch", family: GuidanceScratch, want: "Graduate each sticky into the spec, or delete it — scratch never enters the record by itself."},
		{name: "criteria", family: GuidanceCriteria, want: "Declare what must be true when this lands (typed operation add-ac)."},
		{
			name: "coverage", family: GuidanceCoverage, facts: GuidanceFacts{Object: "ac-2"},
			want: "Plan the delivery: graduate a story sticky into a stub claiming ac-2.",
		},
		{name: "unknown family", family: GuidanceFamily(0), want: ""},
		{name: "family past the closed set", family: GuidanceCoverage + 1, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Guidance(tt.family, tt.facts); got != tt.want {
				t.Fatalf("Guidance(%v, %+v) = %q, want %q", tt.family, tt.facts, got, tt.want)
			}
		})
	}
}

// guidanceFamilyRow is one row of the closed vocabulary's guidance table:
// a representative unresolved concern of the family, the derivation input
// it is read against, and the exact guidance the derivation attaches.
type guidanceFamilyRow struct {
	family  concernFamily
	concern Concern
	input   func(*Input)
	want    string
}

// guidanceFamilyTable lists EVERY family of the closed concern-identity
// vocabulary (classifyConcern) with its corrective guidance (SI-338 (1);
// Wave 6 §3.1 "source-derived corrective guidance for every violated or
// unproven row"). TestGuidanceFor_EveryFamilyOfTheClosedVocabulary fails
// for a family this table omits, so a new family cannot ship without one.
func guidanceFamilyTable() []guidanceFamilyRow {
	unresolved := func(id string, state State) Concern {
		return Concern{ID: id, State: state, Summary: "source-derived readiness fact"}
	}
	blocker := func(id string) Concern {
		c := unresolved(id, StateViolated)
		c.Summary = "the clearing condition the journey states for this blocker"
		return c
	}
	return []guidanceFamilyRow{
		{family: familyShapeProblem, concern: unresolved("shape/problem", StateViolated),
			want: "State the problem (typed operation set-problem) — the case file opens with it."},
		{family: familyShapeOutcome, concern: unresolved("shape/outcome", StateViolated),
			want: "State the intended outcome (typed operation set-outcome)."},
		{family: familyShapeProvenance, concern: unresolved("shape/provenance", StateUnproven),
			want: "Make further spec edits through the wall's typed operations, so the design-provenance chain records each one; a gap or an unavailable context it already records stays disclosed."},
		{family: familyShapeMutation, concern: unresolved("shape/mutation", StateUnproven),
			want: "Make the next typed edit on the wall: it first completes or rolls back the interrupted mutation that left this residue, or refuses and names why."},
		{family: familyShapeBoard, concern: unresolved("shape/board", StateUnproven),
			want: "Repair the scratch board's annotation records the witness names, so the wall can enumerate this spec's open stickies again."},
		{family: familyShapeQuestion, concern: unresolved("shape/question/oq-1", StateUnproven),
			want: "Resolve it on the wall: edit or remove oq-1, or graduate a decision that answers it."},
		{family: familyShapeQuestion, concern: unresolved("shape/question/oq-2", StateUnproven),
			input: func(in *Input) {
				in.Shape.OpenQuestionIDs = []string{"oq-2"}
				in.Shape.ClaimedQuestions = []ClaimedQuestion{{QuestionID: "oq-2", StubSlugs: []string{"alpha-spike", "beta-spike"}}}
			},
			want: "No wall edit is required to accept: the claiming spike stubs answer it after acceptance."},
		{family: familyShapeBoardItem, concern: unresolved("shape/board/question/a-01ARZ3NDEKTSV4RRFFQ69G5FAV", StateUnproven),
			want: "Graduate each sticky into the spec, or delete it — scratch never enters the record by itself."},
		{family: familyShapeBoardItem, concern: unresolved("shape/board/agent-task/a-01ARZ3NDEKTSV4RRFFQ69G5FAW", StateUnproven),
			want: "Graduate each sticky into the spec, or delete it — scratch never enters the record by itself."},
		{family: familySuccessCriteria, concern: unresolved("success/criteria", StateViolated),
			want: "Declare what must be true when this lands (typed operation add-ac)."},
		{family: familySuccessCoverage, concern: unresolved("success/coverage/ac-2", StateUnproven),
			want: "Plan the delivery: graduate a story sticky into a stub claiming ac-2."},
		{family: familySuccessContributor, concern: unresolved("success/contributor/behavioral", StateUnproven),
			want: "Produce the behavioral evidence the acceptance criteria on the wall declare; this row stays unproven while no evidence source for it is wired to readiness."},
		{family: familySuccessBlocker, concern: blocker("success/blocker/obligation-quality/ac-1/static"),
			want: "the clearing condition the journey states for this blocker"},
		{family: familyContextVerdict, concern: unresolved("context/verdict", StateUnproven),
			want: "Run the context-conflict verb with this spec's context request, then resolve what it reports."},
		{family: familyContextMechanical, concern: unresolved("context/mechanical/configuration:go-version#complete", StateViolated),
			want: "Resolve this mechanical policy-conflict row's reasons, then re-run the context-conflict verb with this spec's context request."},
		{family: familyContextSemantic, concern: unresolved("context/semantic/sha256-abc", StateUnproven),
			want: "Resolve this semantic policy-conflict row's reasons or record a disposition, then re-run the context-conflict verb with this spec's context request."},
		{family: familyContextDisclosure, concern: unresolved("context/disclosure/repository-remote-unknown", StateUnproven),
			want: "Address the condition this policy-conflict disclosure names, then re-run the context-conflict verb with this spec's context request."},
		{family: familyReviewBlocker, concern: blocker("review/blocker/principal-resolution-unproven/close"),
			want: "the clearing condition the journey states for this blocker"},
		{family: familyReviewRole, concern: unresolved("review/role/close/attestation/countersign", StateUnproven),
			want: "Before close, a principal entitled to give attestation/countersign must provide it; verdi journey shows the requirement."},
		{family: familyReviewAction, concern: unresolved("review/action", StateUnproven),
			want: "Establish the facts the witnesses name, so verdi journey can offer a safe review action."},
		{family: familyReviewEventualDerivation, concern: unresolved("review/eventual-derivation", StateUnproven),
			want: "Restore the sources the witnesses name, so verdi journey derives every eventual closure blocker."},
	}
}

// TestGuidanceFor_EveryFamilyOfTheClosedVocabulary is the closed
// vocabulary's guidance table (SI-338 (1)): every family classifyConcern
// knows carries non-empty, control-free corrective guidance when the row
// is unresolved, and none when it is proven. A family missing from the
// table fails here before Validate's own guidance rule could turn it into
// an operational error on every readiness surface.
func TestGuidanceFor_EveryFamilyOfTheClosedVocabulary(t *testing.T) {
	t.Parallel()

	covered := map[concernFamily]bool{}
	for _, row := range guidanceFamilyTable() {
		covered[row.family] = true
		t.Run(row.concern.ID, func(t *testing.T) {
			if got := classifyConcern(row.concern.ID); got != row.family {
				t.Fatalf("classifyConcern(%q) = %v, want %v", row.concern.ID, got, row.family)
			}
			in := baseInput(t)
			if row.input != nil {
				row.input(&in)
			}
			got := guidanceFor(row.concern, in)
			if got != row.want {
				t.Fatalf("guidanceFor(%q) = %q, want %q", row.concern.ID, got, row.want)
			}
			if got == "" || containsControl(got) {
				t.Fatalf("guidanceFor(%q) = %q, want non-empty and control-free", row.concern.ID, got)
			}
			proven := row.concern
			proven.State = StateProven
			if got := guidanceFor(proven, in); got != "" {
				t.Fatalf("guidanceFor(proven %q) = %q, want empty", row.concern.ID, got)
			}
		})
	}
	for family := familyUnknown + 1; family < familyEnd; family++ {
		if !covered[family] {
			t.Errorf("concern family %v has no guidance row: every family of the closed vocabulary needs corrective guidance", family)
		}
	}
	if got := guidanceFor(Concern{ID: "shape/invented", State: StateUnproven}, baseInput(t)); got != "" {
		t.Fatalf("guidanceFor(unknown family) = %q, want empty", got)
	}
}

// TestDeriveAttachesGuidanceToEveryUnresolvedConcern derives a snapshot in
// which every family the derivation emits is unresolved at once and proves
// each row carries exactly guidanceFor's sentence, while every proven row
// carries none.
func TestDeriveAttachesGuidanceToEveryUnresolvedConcern(t *testing.T) {
	t.Parallel()

	in := baseInput(t)
	in.Shape.ProblemPresent = false
	in.Shape.OutcomePresent = false
	in.Shape.DeclaredObjectIDs = []string{"ac-1", "ac-2", "oq-1", "oq-2"}
	in.Shape.OpenQuestionIDs = []string{"oq-1", "oq-2"}
	in.Shape.ClaimedQuestions = []ClaimedQuestion{{QuestionID: "oq-2", StubSlugs: []string{"retry-spike"}}}
	in.Provenance.ChainState = StateUnproven
	in.Provenance.MutationState = StateUnproven
	in.Provenance.MutationWitnesses = []string{"draft mutation journal is present"}
	in.Board.OpenItems = []BoardItem{{ID: "a-01ARZ3NDEKTSV4RRFFQ69G5FAV", Kind: "question"}}
	in.Success = SuccessFacts{CriterionIDs: []string{"ac-1", "ac-2"}, UncoveredCriteria: []string{"ac-2"}}
	in.Journey.Evidence.Contributors = []journey.EvidenceContributor{{ID: "static", Kind: "static", Resolution: "unproven", Witness: "no source"}}
	in.Conflict = conflictReport(t, policyconflict.VerdictBlockedViolated)

	snapshot := mustDerive(t, in)
	unresolved := 0
	for _, concern := range snapshot.AllConcerns {
		want := guidanceFor(concern, in)
		if concern.Guidance != want {
			t.Fatalf("concern %q guidance = %q, want guidanceFor's %q", concern.ID, concern.Guidance, want)
		}
		if concern.State == StateProven {
			if concern.Guidance != "" {
				t.Fatalf("proven concern %q carries guidance %q", concern.ID, concern.Guidance)
			}
			continue
		}
		unresolved++
		if concern.Guidance == "" {
			t.Fatalf("unresolved concern %q carries no guidance", concern.ID)
		}
	}
	if unresolved < 10 {
		t.Fatalf("fixture derived only %d unresolved concerns; the guidance proof needs every derivable family unresolved", unresolved)
	}
	for _, concern := range snapshot.Attention {
		if concern.Guidance == "" {
			t.Fatalf("attention concern %q carries no guidance", concern.ID)
		}
	}
}

// TestValidate_GuidanceExactlyWhenUnresolved is SI-338 (1)'s schema rule:
// Guidance is non-empty and control-free exactly when the concern is not
// proven, and empty when it is.
func TestValidate_GuidanceExactlyWhenUnresolved(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Snapshot)
		wantErr string
	}{
		{
			name:    "unresolved concern without guidance",
			mutate:  func(s *Snapshot) { mutateConcern(s, "shape/problem", func(c *Concern) { c.Guidance = "" }) },
			wantErr: "guidance must be non-empty",
		},
		{
			name: "unresolved concern with control-bearing guidance",
			mutate: func(s *Snapshot) {
				mutateConcern(s, "shape/problem", func(c *Concern) { c.Guidance = "State it\nnow." })
			},
			wantErr: "guidance must be non-empty and control-free",
		},
		{
			name: "proven concern with guidance",
			mutate: func(s *Snapshot) {
				mutateConcern(s, "context/verdict", func(c *Concern) { c.Guidance = "Nothing to do." })
			},
			wantErr: "proven concern must carry no guidance",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := validUnresolvedSnapshot()
			tt.mutate(&snapshot)
			if err := snapshot.Validate(); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
	if err := validUnresolvedSnapshot().Validate(); err != nil {
		t.Fatalf("valid unresolved snapshot Validate() = %v", err)
	}
}
