package splice

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designprovenance"
)

func draftFixture() []byte {
	src := strings.Replace(sampleSpecWithStubs,
		"owners: [platform-team]\n",
		"owners: [platform-team]\nlinks: [ { type: depends-on, ref: spec/base } ]\ncontext: [spec/base@abcdef0, adr/choice@abcdef1]\n", 1)
	src = strings.Replace(src,
		"decisions:\n",
		"open_questions:\n  - { id: oq-1, text: \"which retry signal is stable?\", anchor: \"#oq-1\" }\ndecisions:\n", 1)
	src = strings.Replace(src,
		"  - { slug: existing-stub, acceptance_criteria: [ac-1] }\n",
		"  - { slug: existing-stub, acceptance_criteria: [ac-1] }\n  - { slug: second-stub, acceptance_criteria: [ac-2] }\n", 1)
	src += "\n## oq-1\n\nQuestion prose.\n"
	return []byte(src)
}

func TestApplyDraftMutationsEveryOperation(t *testing.T) {
	spike := true
	tests := []struct {
		name string
		op   designprovenance.Operation
		want string
	}{
		{"set problem", designprovenance.Operation{Op: designprovenance.OpSetProblem, Text: "new problem", Anchor: "#problem"}, `problem: { text: "new problem", anchor: "#problem" }`},
		{"set outcome", designprovenance.Operation{Op: designprovenance.OpSetOutcome, Text: "new outcome", Anchor: "#outcome"}, `outcome: { text: "new outcome", anchor: "#outcome" }`},
		{"add ac", designprovenance.Operation{Op: designprovenance.OpAddAC, ID: "ac-new", Text: "new criterion", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}, Anchor: "#ac-new"}, `id: ac-new, text: "new criterion"`},
		{"edit ac", designprovenance.Operation{Op: designprovenance.OpEditAC, ID: "ac-2", Text: "edited criterion", Evidence: []artifact.EvidenceKind{artifact.EvidenceBehavioral}, Anchor: "#ac-2"}, `id: ac-2, text: "edited criterion"`},
		{"remove ac", designprovenance.Operation{Op: designprovenance.OpRemoveAC, ID: "ac-3"}, "## ac-3"},
		{"reorder ac", designprovenance.Operation{Op: designprovenance.OpReorderAC, ID: "ac-3", AfterID: "ac-1"}, "acceptance_criteria:"},
		{"set ac evidence", designprovenance.Operation{Op: designprovenance.OpSetACEvidence, ID: "ac-2", Evidence: []artifact.EvidenceKind{artifact.EvidenceRuntime}}, "evidence: [runtime]"},
		{"add constraint", designprovenance.Operation{Op: designprovenance.OpAddConstraint, ID: "co-new", Text: "must be bounded", Anchor: "#co-new"}, `id: co-new, text: "must be bounded"`},
		{"edit constraint", designprovenance.Operation{Op: designprovenance.OpEditConstraint, ID: "co-1", Text: "retries stop at three", Anchor: "#co-1"}, `id: co-1, text: "retries stop at three"`},
		{"remove constraint", designprovenance.Operation{Op: designprovenance.OpRemoveConstraint, ID: "co-1"}, "## co-1"},
		{"add decision", designprovenance.Operation{Op: designprovenance.OpAddDecision, ID: "dc-new", Text: "use leases", Anchor: "#dc-new"}, `id: dc-new, text: "use leases"`},
		{"edit decision", designprovenance.Operation{Op: designprovenance.OpEditDecision, ID: "dc-2", Text: "use the durable channel", Anchor: "#dc-2"}, `id: dc-2, text: "use the durable channel"`},
		{"remove decision", designprovenance.Operation{Op: designprovenance.OpRemoveDecision, ID: "dc-2"}, "## dc-2"},
		{"add question", designprovenance.Operation{Op: designprovenance.OpAddQuestion, ID: "oq-new", Text: "what is stable?", Anchor: "#oq-new"}, `id: oq-new, text: "what is stable?"`},
		{"edit question", designprovenance.Operation{Op: designprovenance.OpEditQuestion, ID: "oq-1", Text: "which signal survives retry?", Anchor: "#oq-1"}, `id: oq-1, text: "which signal survives retry?"`},
		{"remove question", designprovenance.Operation{Op: designprovenance.OpRemoveQuestion, ID: "oq-1"}, "## oq-1"},
		{"add link", designprovenance.Operation{Op: designprovenance.OpAddLink, Source: "spec", Type: artifact.LinkImpacts, Ref: "spec/other", Note: "bounded"}, `type: impacts, ref: "spec/other", note: "bounded"`},
		{"remove link", designprovenance.Operation{Op: designprovenance.OpRemoveLink, Source: "spec", Type: artifact.LinkDependsOn, Ref: "spec/base"}, "links:"},
		{"add stub", designprovenance.Operation{Op: designprovenance.OpAddStub, Slug: "plain-new", AcceptanceCriteria: []string{"ac-1"}}, "slug: plain-new"},
		{"edit stub", designprovenance.Operation{Op: designprovenance.OpEditStub, Slug: "existing-stub", Spike: &spike, Resolves: []string{"oq-1"}}, "slug: existing-stub, spike: true"},
		{"remove stub", designprovenance.Operation{Op: designprovenance.OpRemoveStub, Slug: "existing-stub"}, "second-stub"},
		{"reorder stub", designprovenance.Operation{Op: designprovenance.OpReorderStub, Slug: "second-stub"}, "stubs:"},
		{"add context", designprovenance.Operation{Op: designprovenance.OpAddContextRef, Ref: "spec/other@abcdef2"}, "spec/other@abcdef2"},
		{"remove context", designprovenance.Operation{Op: designprovenance.OpRemoveContextRef, Ref: "spec/base@abcdef0"}, "adr/choice@abcdef1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyDraftMutations(draftFixture(), []designprovenance.Operation{tt.op})
			if err != nil {
				t.Fatalf("ApplyDraftMutations: %v", err)
			}
			if !bytes.Contains(got, []byte(tt.want)) {
				t.Fatalf("result does not contain %q:\n%s", tt.want, got)
			}
			if err := Validate(got); err != nil {
				t.Fatalf("Validate result: %v", err)
			}
		})
	}
}

func TestApplyDraftMutationsEveryOperationNegative(t *testing.T) {
	tests := []struct {
		name string
		op   designprovenance.Operation
		want string
	}{
		{"set-problem invalid", designprovenance.Operation{Op: designprovenance.OpSetProblem, Anchor: "#problem"}, "text"},
		{"set-outcome invalid", designprovenance.Operation{Op: designprovenance.OpSetOutcome, Text: "outcome"}, "anchor"},
		{"add-ac duplicate", designprovenance.Operation{Op: designprovenance.OpAddAC, ID: "ac-1", Text: "duplicate", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}, Anchor: "#ac-1"}, "duplicate"},
		{"edit-ac missing", designprovenance.Operation{Op: designprovenance.OpEditAC, ID: "ac-missing", Text: "missing", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}, Anchor: "#ac-missing"}, "no object"},
		{"remove-ac missing", designprovenance.Operation{Op: designprovenance.OpRemoveAC, ID: "ac-missing"}, "no object"},
		{"reorder-ac invalid anchor", designprovenance.Operation{Op: designprovenance.OpReorderAC, ID: "ac-2", AfterID: "ac-missing"}, "after target"},
		{"set-ac-evidence missing", designprovenance.Operation{Op: designprovenance.OpSetACEvidence, ID: "ac-missing", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}}, "no object"},
		{"add-constraint duplicate", designprovenance.Operation{Op: designprovenance.OpAddConstraint, ID: "co-1", Text: "duplicate", Anchor: "#co-1"}, "duplicate"},
		{"edit-constraint missing", designprovenance.Operation{Op: designprovenance.OpEditConstraint, ID: "co-missing", Text: "missing", Anchor: "#co-missing"}, "no object"},
		{"remove-constraint missing", designprovenance.Operation{Op: designprovenance.OpRemoveConstraint, ID: "co-missing"}, "no object"},
		{"add-decision duplicate", designprovenance.Operation{Op: designprovenance.OpAddDecision, ID: "dc-1", Text: "duplicate", Anchor: "#dc-1"}, "duplicate"},
		{"edit-decision missing", designprovenance.Operation{Op: designprovenance.OpEditDecision, ID: "dc-missing", Text: "missing", Anchor: "#dc-missing"}, "no object"},
		{"remove-decision missing", designprovenance.Operation{Op: designprovenance.OpRemoveDecision, ID: "dc-missing"}, "no object"},
		{"add-question duplicate", designprovenance.Operation{Op: designprovenance.OpAddQuestion, ID: "oq-1", Text: "duplicate", Anchor: "#oq-1"}, "duplicate"},
		{"edit-question missing", designprovenance.Operation{Op: designprovenance.OpEditQuestion, ID: "oq-missing", Text: "missing", Anchor: "#oq-missing"}, "no object"},
		{"remove-question missing", designprovenance.Operation{Op: designprovenance.OpRemoveQuestion, ID: "oq-missing"}, "no object"},
		{"add-link duplicate", designprovenance.Operation{Op: designprovenance.OpAddLink, Source: "spec", Type: artifact.LinkDependsOn, Ref: "spec/base"}, "duplicate"},
		{"remove-link missing exact tuple", designprovenance.Operation{Op: designprovenance.OpRemoveLink, Source: "spec", Type: artifact.LinkImpacts, Ref: "spec/missing"}, "exact"},
		{"add-stub duplicate", designprovenance.Operation{Op: designprovenance.OpAddStub, Slug: "existing-stub", AcceptanceCriteria: []string{"ac-1"}}, "duplicate"},
		{"edit-stub missing", designprovenance.Operation{Op: designprovenance.OpEditStub, Slug: "missing-stub", AcceptanceCriteria: []string{"ac-1"}}, "no stub"},
		{"remove-stub missing", designprovenance.Operation{Op: designprovenance.OpRemoveStub, Slug: "missing-stub"}, "no stubs target"},
		{"reorder-stub invalid anchor", designprovenance.Operation{Op: designprovenance.OpReorderStub, Slug: "existing-stub", AfterSlug: "missing-stub"}, "after target"},
		{"add-context-ref duplicate", designprovenance.Operation{Op: designprovenance.OpAddContextRef, Ref: "spec/base@abcdef0"}, "duplicate"},
		{"remove-context-ref missing", designprovenance.Operation{Op: designprovenance.OpRemoveContextRef, Ref: "spec/missing@abcdef2"}, "no context target"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ApplyDraftMutations(draftFixture(), []designprovenance.Operation{tt.op}); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ApplyDraftMutations error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestApplyDraftMutationsDecisionLinksAndExactRemoval(t *testing.T) {
	add := designprovenance.Operation{Op: designprovenance.OpAddLink, Source: "dc-2", Type: artifact.LinkDependsOn, Ref: "spec/other", Note: "why"}
	withLink, err := ApplyDraftMutations(draftFixture(), []designprovenance.Operation{add})
	if err != nil {
		t.Fatalf("add decision link: %v", err)
	}
	if _, err := ApplyDraftMutations(withLink, []designprovenance.Operation{{Op: designprovenance.OpRemoveLink, Source: "dc-2", Type: artifact.LinkDependsOn, Ref: "spec/other"}}); err == nil || !strings.Contains(err.Error(), "exact") {
		t.Fatalf("inexact removal error = %v", err)
	}
	remove := add
	remove.Op = designprovenance.OpRemoveLink
	withoutLink, err := ApplyDraftMutations(withLink, []designprovenance.Operation{remove})
	if err != nil {
		t.Fatalf("exact removal: %v", err)
	}
	if bytes.Contains(withoutLink, []byte(`ref: "spec/other"`)) {
		t.Fatal("exactly removed decision link remains")
	}
}

func TestApplyDraftMutationsNegativeAndOrdered(t *testing.T) {
	tests := []struct {
		name string
		op   designprovenance.Operation
		want string
	}{
		{"missing target", designprovenance.Operation{Op: designprovenance.OpRemoveAC, ID: "ac-missing"}, "no object"},
		{"duplicate context", designprovenance.Operation{Op: designprovenance.OpAddContextRef, Ref: "spec/base@abcdef0"}, "duplicate"},
		{"duplicate link", designprovenance.Operation{Op: designprovenance.OpAddLink, Source: "spec", Type: artifact.LinkDependsOn, Ref: "spec/base"}, "duplicate"},
		{"duplicate object", designprovenance.Operation{Op: designprovenance.OpAddAC, ID: "ac-1", Text: "duplicate", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}, Anchor: "#ac-1"}, "duplicate"},
		{"duplicate stub", designprovenance.Operation{Op: designprovenance.OpAddStub, Slug: "existing-stub", AcceptanceCriteria: []string{"ac-1"}}, "duplicate"},
		{"illegal link source", designprovenance.Operation{Op: designprovenance.OpAddLink, Source: "ac-1", Type: artifact.LinkDependsOn, Ref: "spec/base"}, "source"},
		{"reorder after self", designprovenance.Operation{Op: designprovenance.OpReorderAC, ID: "ac-2", AfterID: "ac-2"}, "itself"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ApplyDraftMutations(draftFixture(), []designprovenance.Operation{tt.op}); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}

	ordered, err := ApplyDraftMutations(draftFixture(), []designprovenance.Operation{
		{Op: designprovenance.OpAddConstraint, ID: "co-ordered", Text: "first", Anchor: "#co-ordered"},
		{Op: designprovenance.OpEditConstraint, ID: "co-ordered", Text: "second", Anchor: "#co-ordered"},
	})
	if err != nil {
		t.Fatalf("ordered batch: %v", err)
	}
	if bytes.Contains(ordered, []byte(`text: "first"`)) || !bytes.Contains(ordered, []byte(`text: "second"`)) {
		t.Fatalf("ordered batch did not apply add then edit:\n%s", ordered)
	}

	reordered, err := ApplyDraftMutations(draftFixture(), []designprovenance.Operation{
		{Op: designprovenance.OpReorderAC, ID: "ac-3", AfterID: "ac-1"},
		{Op: designprovenance.OpReorderStub, Slug: "second-stub"},
	})
	if err != nil {
		t.Fatalf("reorder batch: %v", err)
	}
	fmBytes, _, _ := artifact.SplitFrontmatter(reordered)
	fm, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		t.Fatalf("DecodeSpec reordered: %v", err)
	}
	if got := []string{fm.AcceptanceCriteria[0].ID, fm.AcceptanceCriteria[1].ID, fm.AcceptanceCriteria[2].ID}; strings.Join(got, ",") != "ac-1,ac-3,ac-2" {
		t.Fatalf("AC order = %v", got)
	}
	if got := []string{fm.Stubs[0].Slug, fm.Stubs[1].Slug}; strings.Join(got, ",") != "second-stub,existing-stub" {
		t.Fatalf("stub order = %v", got)
	}
}

func TestApplyDraftMutationsRejectsDuplicateBaseSets(t *testing.T) {
	tests := []struct {
		name string
		src  []byte
		op   designprovenance.Operation
	}{
		{
			name: "context refs",
			src:  bytes.Replace(draftFixture(), []byte("context: [spec/base@abcdef0, adr/choice@abcdef1]"), []byte("context: [spec/base@abcdef0, spec/base@abcdef0]"), 1),
			op:   designprovenance.Operation{Op: designprovenance.OpRemoveContextRef, Ref: "spec/base@abcdef0"},
		},
		{
			name: "exact links",
			src:  bytes.Replace(draftFixture(), []byte("links: [ { type: depends-on, ref: spec/base } ]"), []byte("links: [ { type: depends-on, ref: spec/base }, { type: depends-on, ref: spec/base } ]"), 1),
			op:   designprovenance.Operation{Op: designprovenance.OpRemoveLink, Source: "spec", Type: artifact.LinkDependsOn, Ref: "spec/base"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ApplyDraftMutations(tt.src, []designprovenance.Operation{tt.op}); err == nil || !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("error = %v, want duplicate refusal", err)
			}
		})
	}
}

func TestApplyDraftMutationsPreservesUntouchedBytes(t *testing.T) {
	before := draftFixture()
	_, bodyBefore, err := artifact.SplitFrontmatter(before)
	if err != nil {
		t.Fatal(err)
	}
	after, err := ApplyDraftMutations(before, []designprovenance.Operation{{
		Op: designprovenance.OpEditAC, ID: "ac-2", Text: "changed", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}, Anchor: "#ac-2",
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, bodyAfter, _ := artifact.SplitFrontmatter(after)
	if !bytes.Equal(bodyBefore, bodyAfter) {
		t.Fatal("frontmatter-only edit changed body bytes")
	}
	if !bytes.Equal(objectSpanBytes(t, before, "dc-1"), objectSpanBytes(t, after, "dc-1")) {
		t.Fatal("edit changed an untouched decision row")
	}
}
