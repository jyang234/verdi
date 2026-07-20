package residue

import "testing"

func TestFindPatternA_Happy(t *testing.T) {
	closeBranches := []CloseBranch{
		{Name: "widget", Branch: "close/widget", Tip: "sha-widget", ArchivedOnOwnTip: true, Class: RitualIncomplete},
		{Name: "already-done", Branch: "close/already-done", Tip: "sha-already-done", ArchivedOnOwnTip: true, Class: SupersededElsewhere},
		{Name: "fresh", Branch: "close/fresh", Tip: "sha-fresh", ArchivedOnOwnTip: false, Class: RitualIncomplete},
	}
	activeStatus := map[string]string{
		"widget":       "accepted-pending-build",
		"already-done": "accepted-pending-build", // still active per THIS lookup, but archived-elsewhere per Class
		"fresh":        "accepted-pending-build",
	}
	activeClass := map[string]string{
		"widget":       "story",
		"already-done": "feature",
		"fresh":        "story",
	}

	got := findPatternA(closeBranches, activeStatus, activeClass)
	if len(got) != 2 {
		t.Fatalf("findPatternA = %+v, want exactly 2 (widget and already-done — both ArchivedOnOwnTip)", got)
	}
	if got[0].SpecName != "already-done" || got[1].SpecName != "widget" {
		t.Fatalf("findPatternA names = [%s, %s], want sorted [already-done, widget]", got[0].SpecName, got[1].SpecName)
	}
	if got[1].Branch != "close/widget" || got[1].Tip != "sha-widget" {
		t.Fatalf("findPatternA widget entry = %+v, want Branch=close/widget Tip=sha-widget", got[1])
	}
	if got[1].Class != "story" {
		t.Errorf("findPatternA widget entry Class = %q, want story", got[1].Class)
	}
	if got[0].Class != "feature" {
		t.Errorf("findPatternA already-done entry Class = %q, want feature", got[0].Class)
	}
}

func TestFindPatternA_Negative_NotArchivedOnOwnTip(t *testing.T) {
	closeBranches := []CloseBranch{
		{Name: "fresh", Branch: "close/fresh", Tip: "sha", ArchivedOnOwnTip: false, Class: RitualIncomplete},
	}
	activeStatus := map[string]string{"fresh": "accepted-pending-build"}

	got := findPatternA(closeBranches, activeStatus, nil)
	if len(got) != 0 {
		t.Fatalf("findPatternA = %+v, want empty (never archived on its own tip)", got)
	}
}

func TestFindPatternA_Negative_SpecNotActivePendingBuild(t *testing.T) {
	cases := []string{"", "draft", "closed", "superseded"}
	for _, status := range cases {
		t.Run(status, func(t *testing.T) {
			closeBranches := []CloseBranch{
				{Name: "widget", Branch: "close/widget", Tip: "sha", ArchivedOnOwnTip: true, Class: RitualIncomplete},
			}
			activeStatus := map[string]string{}
			if status != "" {
				activeStatus["widget"] = status
			}
			got := findPatternA(closeBranches, activeStatus, nil)
			if len(got) != 0 {
				t.Fatalf("findPatternA(status=%q) = %+v, want empty (not accepted-pending-build)", status, got)
			}
		})
	}
}
