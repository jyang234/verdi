package residue

import (
	"testing"

	"github.com/jyang234/verdi/internal/specstate"
)

func TestFindPatternA_Happy(t *testing.T) {
	closeBranches := []CloseBranch{
		{Name: "widget", Branch: "close/widget", Tip: "sha-widget", ArchivedOnOwnTip: true, Class: RitualIncomplete},
		{Name: "already-done", Branch: "close/already-done", Tip: "sha-already-done", ArchivedOnOwnTip: true, Class: SupersededElsewhere},
		{Name: "fresh", Branch: "close/fresh", Tip: "sha-fresh", ArchivedOnOwnTip: false, Class: RitualIncomplete},
	}
	effective := map[string]specstate.Result{
		"widget":       {State: specstate.AcceptedPendingBuild},
		"already-done": {State: specstate.AcceptedPendingBuild}, // still active per THIS lookup, but archived-elsewhere per Class
		"fresh":        {State: specstate.AcceptedPendingBuild},
	}
	activeClass := map[string]string{
		"widget":       "story",
		"already-done": "feature",
		"fresh":        "story",
	}

	got := findPatternA(closeBranches, effective, activeClass)
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
	effective := map[string]specstate.Result{"fresh": {State: specstate.AcceptedPendingBuild}}

	got := findPatternA(closeBranches, effective, nil)
	if len(got) != 0 {
		t.Fatalf("findPatternA = %+v, want empty (never archived on its own tip)", got)
	}
}

// TestFindPatternA_Negative_SpecNotActivePendingBuild proves every OTHER
// specstate.State (never a raw legacy status string) excludes pattern (a),
// including the zero value (a name absent from effective entirely — fail
// closed, never assumed accepted).
func TestFindPatternA_Negative_SpecNotActivePendingBuild(t *testing.T) {
	cases := []specstate.State{"", specstate.Proposed, specstate.Closed, specstate.Superseded, specstate.Unproven}
	for _, state := range cases {
		t.Run(string(state), func(t *testing.T) {
			closeBranches := []CloseBranch{
				{Name: "widget", Branch: "close/widget", Tip: "sha", ArchivedOnOwnTip: true, Class: RitualIncomplete},
			}
			effective := map[string]specstate.Result{}
			if state != "" {
				effective["widget"] = specstate.Result{State: state}
			}
			got := findPatternA(closeBranches, effective, nil)
			if len(got) != 0 {
				t.Fatalf("findPatternA(state=%q) = %+v, want empty (not accepted-pending-build)", state, got)
			}
		})
	}
}
