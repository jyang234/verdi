package specdoc

import "testing"

func TestParseKind(t *testing.T) {
	cases := []struct {
		in      string
		want    Kind
		wantErr bool
	}{
		{"spec", KindSpec, false},
		{"plan", KindPlan, false},
		{"tasks", KindTasks, false},
		{"", "", true},
		{"SPEC", "", true},
		{"task", "", true},
	}
	for _, c := range cases {
		got, err := ParseKind(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseKind(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("ParseKind(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestKindSections(t *testing.T) {
	all := KindSpec.Sections()
	wantAll := []SectionID{SectionIdentity, SectionProblem, SectionOutcome, SectionDecisions, SectionConstraints, SectionCriteria, SectionQuestions, SectionPlan, SectionEvidence}
	if len(all) != len(wantAll) {
		t.Fatalf("spec sections = %v, want %v", all, wantAll)
	}
	for i := range all {
		if all[i] != wantAll[i] {
			t.Fatalf("spec sections = %v, want %v", all, wantAll)
		}
	}
	plan := KindPlan.Sections()
	wantPlan := []SectionID{SectionIdentity, SectionDecisions, SectionConstraints, SectionPlan}
	if len(plan) != len(wantPlan) {
		t.Fatalf("plan sections = %v, want %v", plan, wantPlan)
	}
	tasks := KindTasks.Sections()
	wantTasks := []SectionID{SectionIdentity, SectionPlan, SectionEvidence}
	if len(tasks) != len(wantTasks) {
		t.Fatalf("tasks sections = %v, want %v", tasks, wantTasks)
	}
}
