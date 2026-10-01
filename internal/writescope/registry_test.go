package writescope_test

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/policyadopt"
	ws "github.com/jyang234/verdi/internal/writescope"
)

func TestRegistry_Validates(t *testing.T) {
	if err := ws.ValidateRegistry(ws.Registry(), ws.AwaitingFixes()); err != nil {
		t.Fatalf("ValidateRegistry(Registry(), AwaitingFixes()) = %v", err)
	}
}

func TestRegistry_OnlyTheBoardsCommitAndPushIsCarried(t *testing.T) {
	var carried []string
	for _, d := range ws.Registry() {
		if d.IndexCarry == ws.CarryCarried {
			carried = append(carried, d.Ritual)
		}
	}
	if strings.Join(carried, ",") != ws.RitualBoardCommitPush {
		t.Fatalf("carried declarations = %v, want exactly %s (parent dc-3, dc-11)", carried, ws.RitualBoardCommitPush)
	}
}

func TestRegistry_ReturnsAFreshValueEachCall(t *testing.T) {
	first := ws.Registry()
	first[0].Verbs[0] = ws.CLI("mutated")
	first[0].Ritual = "mutated"
	second := ws.Registry()
	if second[0].Ritual == "mutated" || second[0].Verbs[0].Name == "mutated" {
		t.Fatal("Registry() shares state between calls; a fixed table must be rebuilt per call")
	}
	fixes := ws.AwaitingFixes()
	fixes[0].Ritual = "mutated"
	if ws.AwaitingFixes()[0].Ritual == "mutated" {
		t.Fatal("AwaitingFixes() shares state between calls")
	}
}

// The board's Commit and push, at the root and under the /b/{branch}
// mount: the only verbs a carried declaration may name.
const (
	rootBoardCommit = "/board/spec/{name}/api/git-commit"
	bBoardCommit    = "/b/{branch}/board/spec/{name}/api/git-commit"
)

func TestValidateRegistry_Rejects(t *testing.T) {
	scoped := func(ritual string, verbs ...ws.Verb) ws.Declaration {
		return ws.Declaration{
			Ritual: ritual, Verbs: verbs, RefsMove: []ws.RefPattern{ws.RefCheckedOut},
			StagePaths: []ws.PathPattern{".verdi/x/"}, IndexCarry: ws.CarryScoped,
		}
	}
	carried := func(ritual string, verbs ...ws.Verb) ws.Declaration {
		d := scoped(ritual, verbs...)
		d.IndexCarry = ws.CarryCarried
		d.StagePaths = []ws.PathPattern{ws.PathWholeTree}
		return d
	}
	tests := []struct {
		name     string
		decls    []ws.Declaration
		awaiting []ws.AwaitingFix
		wantErr  string
	}{
		{"an invalid declaration", []ws.Declaration{{Ritual: "bad"}}, nil, "no verb"},
		{"duplicate ritual", []ws.Declaration{scoped("a", ws.CLI("x")), scoped("a", ws.CLI("y"))}, nil, "twice"},
		{"a verb in two declarations", []ws.Declaration{scoped("a", ws.CLI("x")), scoped("b", ws.CLI("x"))}, nil, "cli:x"},
		{"carried on a ritual other than the board's commit and push", []ws.Declaration{carried("accept_diagram", ws.CLI("accept"))}, nil, "carried"},
		{"carried on a second declaration", []ws.Declaration{
			carried(ws.RitualBoardCommitPush, ws.Workbench(rootBoardCommit)),
			carried("accept_diagram", ws.CLI("accept")),
		}, nil, "carried"},
		// R1-B2 / mutant BM9: accept folded into the carried declaration.
		{"a carried declaration naming a verb beyond the board's Commit and push", []ws.Declaration{
			carried(ws.RitualBoardCommitPush, ws.Workbench(rootBoardCommit), ws.Workbench(bBoardCommit), ws.CLI("accept")),
		}, nil, "cli:accept"},
		{"a carried declaration naming another workbench action", []ws.Declaration{
			carried(ws.RitualBoardCommitPush, ws.Workbench("/board/spec/{name}/api/create")),
		}, nil, "carried"},
		{"awaiting fix on an unknown ritual", []ws.Declaration{scoped("a", ws.CLI("x"))}, []ws.AwaitingFix{{Ritual: "b", Path: "verdi x", Defect: "d"}}, "unknown"},
		{"awaiting fix on a declaration that is not scoped", []ws.Declaration{carried(ws.RitualBoardCommitPush, ws.Workbench(rootBoardCommit))}, []ws.AwaitingFix{{Ritual: ws.RitualBoardCommitPush, Path: "p", Defect: "d"}}, "scoped"},
		{"awaiting fix without a path", []ws.Declaration{scoped("a", ws.CLI("x"))}, []ws.AwaitingFix{{Ritual: "a", Defect: "d"}}, "path"},
		{"awaiting fix without a defect", []ws.Declaration{scoped("a", ws.CLI("x"))}, []ws.AwaitingFix{{Ritual: "a", Path: "p"}}, "defect"},
		{"the same awaiting path twice", []ws.Declaration{scoped("a", ws.CLI("x"))}, []ws.AwaitingFix{{Ritual: "a", Path: "p", Defect: "d"}, {Ritual: "a", Path: "p", Defect: "d"}}, "twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ws.ValidateRegistry(tt.decls, tt.awaiting)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateRegistry = %v, want an error mentioning %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRegistry_AcceptsTheCarriedBoardCommitAlone(t *testing.T) {
	d := ws.Declaration{
		Ritual: ws.RitualBoardCommitPush, Verbs: []ws.Verb{ws.Workbench(bBoardCommit), ws.Workbench(rootBoardCommit)},
		RefsMove: []ws.RefPattern{ws.RefCheckedOut}, StagePaths: []ws.PathPattern{ws.PathWholeTree},
		IndexCarry: ws.CarryCarried, UntrackedMayEnter: true, MayPush: true,
	}
	if err := ws.ValidateRegistry([]ws.Declaration{d}, nil); err != nil {
		t.Fatalf("ValidateRegistry = %v, want nil", err)
	}
}

func TestCarriedVerbs_AreTheBoardsCommitAndPush(t *testing.T) {
	var got []string
	for _, v := range ws.CarriedVerbs() {
		got = append(got, v.String())
	}
	want := "workbench:" + bBoardCommit + ",workbench:" + rootBoardCommit
	if strings.Join(got, ",") != want {
		t.Fatalf("CarriedVerbs() = %v, want %s", got, want)
	}
	first := ws.CarriedVerbs()
	first[0] = ws.CLI("mutated")
	if ws.CarriedVerbs()[0] == ws.CLI("mutated") {
		t.Fatal("CarriedVerbs() shares state between calls")
	}
}

// TestRegistry_PolicyAdoptStagesOnlyTheStarterPolicy pins R1-B8: policy
// adopt writes one policy file, the starter, so its declaration names that
// file rather than every policy (internal/policyadopt is the source).
func TestRegistry_PolicyAdoptStagesOnlyTheStarterPolicy(t *testing.T) {
	want := ws.PathPattern(".verdi/policy/policies/" + policyadopt.PolicyName + ".md")
	for _, d := range ws.Registry() {
		if d.Ritual != "policy_adopt" {
			continue
		}
		var policies []ws.PathPattern
		for _, p := range d.StagePaths {
			if strings.HasPrefix(string(p), ".verdi/policy/policies/") {
				policies = append(policies, p)
			}
		}
		if len(policies) != 1 || policies[0] != want {
			t.Fatalf("policy_adopt stages policies %v, want exactly [%s]", policies, want)
		}
		return
	}
	t.Fatal("no policy_adopt declaration")
}

func TestHostVerbs_AreServeMcpAndContextMcp(t *testing.T) {
	var got []string
	for _, v := range ws.HostVerbs() {
		got = append(got, v.String())
	}
	if strings.Join(got, ",") != "cli:context mcp,cli:mcp,cli:serve" {
		t.Fatalf("HostVerbs() = %v, want exactly serve, mcp, and context mcp (ledger SI-317 (1))", got)
	}
	first := ws.HostVerbs()
	first[0] = ws.CLI("mutated")
	if ws.HostVerbs()[0] == ws.CLI("mutated") {
		t.Fatal("HostVerbs() shares state between calls")
	}
}

// TestAwaitingFixes_PinStoryDc3 pins R1-RR-B2: the awaiting-fix list is
// exactly those of story dc-3's five ritual paths (design start, design
// start --supersedes, the commit-to-design ritual, accept diagram,
// constitution propose) whose fix has not landed, and each sits in a
// declaration that states the scoped fix. The story's intent is that the
// list only shrinks: spec/ritual-effect-witness removes each path in the
// same change as the fix that makes its ritual conform (dc-2), until the
// list is empty, and no path is ever added back. Fixed so far: design
// start, with its --supersedes path, the commit-to-design ritual, and
// accept diagram.
func TestAwaitingFixes_PinStoryDc3(t *testing.T) {
	want := []string{
		"constitution_propose|verdi context constitution propose",
	}
	var got []string
	for _, a := range ws.AwaitingFixes() {
		got = append(got, a.Ritual+"|"+a.Path)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("AwaitingFixes() =\n%s\nwant story dc-3's unfixed paths\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	carry := map[string]ws.IndexCarry{}
	for _, d := range ws.Registry() {
		carry[d.Ritual] = d.IndexCarry
	}
	for _, a := range ws.AwaitingFixes() {
		if carry[a.Ritual] != ws.CarryScoped {
			t.Errorf("%s (%s) is declared %q; story dc-3 declares it scoped", a.Ritual, a.Path, carry[a.Ritual])
		}
	}
}
