package skillpack

import (
	"testing"
)

func TestParseSequence(t *testing.T) {
	for _, s := range Skills() {
		b, _ := Template(s)
		seq, err := ParseSequence(b)
		if err != nil {
			t.Fatalf("ParseSequence(%s): %v", s, err)
		}
		if len(seq) == 0 || seq[0].Kind != "call" {
			t.Fatalf("sequence for %s must open with a call: %+v", s, seq)
		}
	}
	got, err := ParseSequence([]byte("---\nname: x\n---\n\n```verdi-sequence\ncall get_document kind=plan\nloop\nshow\nconfirm\ncall mutate_draft operations=1\nend\n```\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := Sequence{
		{Kind: "call", Tool: "get_document", Args: map[string]string{"kind": "plan"}},
		{Kind: "loop"}, {Kind: "show"}, {Kind: "confirm"},
		{Kind: "call", Tool: "mutate_draft", Args: map[string]string{"operations": "1"}},
		{Kind: "end"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Tool != want[i].Tool || len(got[i].Args) != len(want[i].Args) {
			t.Fatalf("step %d = %+v, want %+v", i, got[i], want[i])
		}
		for k, v := range want[i].Args {
			if got[i].Args[k] != v {
				t.Fatalf("step %d arg %s = %q, want %q", i, k, got[i].Args[k], v)
			}
		}
	}
	for _, bad := range []string{
		"no block at all",
		"```verdi-sequence\n```\n",                                         // empty
		"```verdi-sequence\nfrobnicate x\n```\n",                           // unknown kind
		"```verdi-sequence\ncall\n```\n",                                   // call without tool
		"```verdi-sequence\nloop\nshow\n```\n",                             // unclosed loop
		"```verdi-sequence\ncall a\n```\n```verdi-sequence\ncall b\n```\n", // two blocks
	} {
		if _, err := ParseSequence([]byte(bad)); err == nil {
			t.Fatalf("ParseSequence must refuse %q", bad)
		}
	}
}

func TestEveryWriteIsShownAndConfirmedFirst(t *testing.T) {
	// Static witness for ac-8's "shows the human every proposal before it
	// is written": in every template, each call to a write tool is
	// preceded, within the same loop body or at top level, by show then
	// confirm.
	writeTools := map[string]bool{"mutate_draft": true, "import_apply": true, "add_annotation": true}
	for _, s := range Skills() {
		b, _ := Template(s)
		seq, _ := ParseSequence(b)
		shown, confirmed := false, false
		for _, st := range seq {
			switch st.Kind {
			case "loop", "end":
				shown, confirmed = false, false
			case "show":
				shown = true
			case "confirm":
				confirmed = shown
			case "call":
				if writeTools[st.Tool] && !confirmed {
					t.Fatalf("skill %s calls %s without show+confirm before it", s, st.Tool)
				}
				if writeTools[st.Tool] && s == "tasks" {
					t.Fatalf("verdi-tasks must not call a write tool")
				}
			}
		}
	}
}
