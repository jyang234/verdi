package upstream

import "testing"

// TestRequest_BuildArgv_FlagsBeforePositional proves buildArgv always
// emits subcommand, then flags, then positional args — the flags-before-
// positional ordering spike S1 found upstream requires (PLAN.md §3), no
// matter what order a caller supplies Flags/Positional in Request.
func TestRequest_BuildArgv_FlagsBeforePositional(t *testing.T) {
	req := Request{
		Bin:        "flowmap",
		Subcommand: "graph",
		Flags:      []string{"-stamp", "deadbeef"},
		Positional: []string{"testdata/svcfix"},
	}
	got := req.buildArgv()
	want := []string{"graph", "-stamp", "deadbeef", "testdata/svcfix"}
	if len(got) != len(want) {
		t.Fatalf("buildArgv = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buildArgv = %v, want %v", got, want)
		}
	}
}

func TestRequest_BuildArgv_NoFlags(t *testing.T) {
	req := Request{Bin: "flowmap", Subcommand: "version"}
	got := req.buildArgv()
	if len(got) != 1 || got[0] != "version" {
		t.Fatalf("buildArgv = %v, want [version]", got)
	}
}
