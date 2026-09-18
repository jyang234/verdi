package specdoc

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
)

func readinessFixture() readinesspilot.Snapshot {
	return readinesspilot.Snapshot{
		TargetRef: "spec/lockbox", TargetTitle: "Lockbox", TargetClass: "feature", Branch: "main",
		Head: strings.Repeat("a", 40), RequestDigest: "sha256:" + strings.Repeat("0", 64),
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateUnproven},
			{ID: readinesspilot.AreaSuccess, Label: "Define success", State: readinesspilot.StateProven},
		},
		CurrentFocus: readinesspilot.AreaShape,
		Attention: []readinesspilot.Concern{
			{ID: "shape/question/oq-2", Area: readinesspilot.AreaShape, State: readinesspilot.StateUnproven, Blocking: true, Timing: readinesspilot.TimingCurrent, Summary: "Declared open question remains unresolved", Witnesses: []string{"oq-2"}},
			{ID: "shape/question/oq-1", Area: readinesspilot.AreaShape, State: readinesspilot.StateUnproven, Blocking: false, Timing: readinesspilot.TimingEventual, Summary: "Declared open question is claimed by spike stubs and remains unresolved", Witnesses: []string{"oq-1", "audit-probe"}},
		},
		StaleNotice: "Startup snapshot; restart verdi serve after an edit.",
	}
}

func TestWithReadinessGatesOnTargetRef(t *testing.T) {
	snap := readinessFixture()
	got := WithReadiness(Facts{}, snap, "spec/lockbox")
	if got.Readiness == nil {
		t.Fatal("matching ref must supply readiness")
	}
	if got.Readiness.TargetRef != "spec/lockbox" || got.Readiness.CurrentFocus != "shape-proposal" || len(got.Readiness.Areas) != 2 || len(got.Readiness.Attention) != 2 {
		t.Fatalf("readiness facts = %+v", got.Readiness)
	}
	if got.Readiness.Attention[0].ID != "shape/question/oq-2" || !got.Readiness.Attention[0].Blocking || got.Readiness.Attention[1].Timing != "eventual" {
		t.Fatalf("attention order/fields = %+v", got.Readiness.Attention)
	}
	other := WithReadiness(Facts{}, snap, "spec/other")
	if other.Readiness != nil {
		t.Fatal("a snapshot for another spec must not be supplied")
	}
}

func TestWithReadinessDoesNotTouchOtherFacts(t *testing.T) {
	base := Facts{Coverage: map[string][]string{"ac-1": {"x"}}}
	got := WithReadiness(base, readinessFixture(), "spec/lockbox")
	if len(got.Coverage) != 1 || got.Coverage["ac-1"][0] != "x" {
		t.Fatal("WithReadiness must copy the other facts unchanged")
	}
}

func TestKindSectionsIncludeReadiness(t *testing.T) {
	spec := KindSpec.Sections()
	if spec[len(spec)-1] != SectionReadiness || spec[len(spec)-2] != SectionEvidence {
		t.Fatalf("spec sections = %v", spec)
	}
	tasks := KindTasks.Sections()
	if tasks[len(tasks)-1] != SectionReadiness {
		t.Fatalf("tasks sections = %v", tasks)
	}
	for _, s := range KindPlan.Sections() {
		if s == SectionReadiness {
			t.Fatal("plan kind must not carry readiness")
		}
	}
}

func TestBuildAndRenderReadiness(t *testing.T) {
	fm, body := loadFixture(t)
	commit := strings.Repeat("0", 39) + "1"
	with := WithReadiness(FactsFromSpec(fm), readinessFixture(), "spec/lockbox")
	doc, err := Build(Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: with, Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if !doc.ReadinessKnown || doc.Readiness == nil {
		t.Fatal("Build must carry supplied readiness")
	}
	md := RenderMarkdown(doc)
	for _, want := range []string{
		"## Readiness",
		"Source: readiness snapshot for `spec/lockbox` at `aaaaaaaaaaaa`. Current focus: Define the work.",
		"| Define the work | unproven |",
		"| Define success | proven |",
		"1. Declared open question remains unresolved — Define the work; blocking; current; unproven; witnesses: oq-2",
		"2. Declared open question is claimed by spike stubs and remains unresolved — Define the work; advisory; eventual; unproven; witnesses: oq-1, audit-probe",
		"Startup snapshot; restart verdi serve after an edit.",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	without, _ := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: FactsFromSpec(fm), Kind: KindSpec})
	if !strings.Contains(RenderMarkdown(without), "## Readiness\n\nReadiness was not supplied for this render.") {
		t.Errorf("unsupplied readiness must be stated:\n%s", RenderMarkdown(without))
	}
}

func TestEngineDigestChangedForVersion2(t *testing.T) {
	if engineVersion != 2 {
		t.Fatalf("engineVersion = %d, want 2 (readiness section added)", engineVersion)
	}
}
