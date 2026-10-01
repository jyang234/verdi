package writescope_test

import (
	"strings"
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// baseDecl is a valid scoped declaration every table row starts from.
func baseDecl() ws.Declaration {
	return ws.Declaration{
		Ritual:     "sample_ritual",
		Verbs:      []ws.Verb{ws.CLI("sample start")},
		RefsCreate: []ws.RefPattern{"refs/heads/design/*"},
		HeadSwitch: true,
		StagePaths: []ws.PathPattern{".verdi/specs/active/*/"},
		IndexCarry: ws.CarryScoped,
	}
}

func TestDeclarationValidate_AcceptsEveryWellFormedValue(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ws.Declaration)
	}{
		{"base scoped declaration", func(*ws.Declaration) {}},
		{"workbench verb", func(d *ws.Declaration) { d.Verbs = []ws.Verb{ws.Workbench("/board/spec/{name}/api/git-switch")} }},
		{"mcp verb", func(d *ws.Declaration) { d.Verbs = []ws.Verb{ws.MCP("import_apply")} }},
		{"cli verb with a flag word", func(d *ws.Declaration) { d.Verbs = []ws.Verb{ws.CLI("design start --from-stub")} }},
		{"several verbs across surfaces", func(d *ws.Declaration) {
			d.Verbs = []ws.Verb{ws.CLI("design import apply"), ws.Workbench("/design/import/apply"), ws.MCP("import_apply")}
		}},
		{"exact ref", func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{"refs/heads/policy/adopt"} }},
		{"any branch", func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{"refs/heads/*"} }},
		{"checked-out branch moves", func(d *ws.Declaration) {
			d.RefsCreate = nil
			d.RefsMove = []ws.RefPattern{ws.RefCheckedOut}
		}},
		{"ref deletion", func(d *ws.Declaration) { d.RefsDelete = []ws.RefPattern{"refs/heads/close/*"} }},
		{"refused with a commit", func(d *ws.Declaration) { d.IndexCarry = ws.CarryRefused }},
		{"refused without a commit", func(d *ws.Declaration) {
			*d = ws.Declaration{Ritual: "switch", Verbs: []ws.Verb{ws.CLI("switch")}, HeadSwitch: true, IndexCarry: ws.CarryRefused}
		}},
		{"carried whole tree with untracked files and push", func(d *ws.Declaration) {
			d.RefsCreate = nil
			d.RefsMove = []ws.RefPattern{ws.RefCheckedOut}
			d.HeadSwitch = false
			d.StagePaths = []ws.PathPattern{ws.PathWholeTree}
			d.IndexCarry = ws.CarryCarried
			d.UntrackedMayEnter = true
			d.MayPush = true
		}},
		{"no commit, worktrees only", func(d *ws.Declaration) {
			*d = ws.Declaration{Ritual: "wt", Verbs: []ws.Verb{ws.CLI("wt")}, Worktrees: []ws.WorktreePattern{".verdi/data/worktrees/*"}, IndexCarry: ws.CarryNoCommit}
		}},
		{"temporary and registered worktrees", func(d *ws.Declaration) {
			d.Worktrees = []ws.WorktreePattern{ws.WorktreeTemp, ws.WorktreeRegistered}
		}},
		{"file and single-segment stage paths", func(d *ws.Declaration) {
			d.StagePaths = []ws.PathPattern{".verdi/policy/constitution.md", ".verdi/diagrams/*"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := baseDecl()
			tt.mutate(&d)
			if err := d.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestDeclarationValidate_RejectsUnknownAndInconsistentValues(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ws.Declaration)
		wantErr string
	}{
		{"empty ritual", func(d *ws.Declaration) { d.Ritual = "" }, "ritual"},
		{"ritual not snake case", func(d *ws.Declaration) { d.Ritual = "Design-Start" }, "ritual"},
		{"no verbs", func(d *ws.Declaration) { d.Verbs = nil }, "no verb"},
		{"unknown surface", func(d *ws.Declaration) { d.Verbs = []ws.Verb{{Surface: "rest", Name: "x"}} }, "surface"},
		{"empty verb name", func(d *ws.Declaration) { d.Verbs = []ws.Verb{ws.CLI("")} }, "verb"},
		{"cli verb with a doubled space", func(d *ws.Declaration) { d.Verbs = []ws.Verb{ws.CLI("design  start")} }, "verb"},
		{"cli verb with a leading space", func(d *ws.Declaration) { d.Verbs = []ws.Verb{ws.CLI(" design")} }, "verb"},
		{"workbench verb without a leading slash", func(d *ws.Declaration) { d.Verbs = []ws.Verb{ws.Workbench("board/{key}")} }, "verb"},
		{"workbench verb with a space", func(d *ws.Declaration) { d.Verbs = []ws.Verb{ws.Workbench("/board /x")} }, "verb"},
		{"mcp verb not snake case", func(d *ws.Declaration) { d.Verbs = []ws.Verb{ws.MCP("Import-Apply")} }, "verb"},
		{"duplicate verb", func(d *ws.Declaration) { d.Verbs = []ws.Verb{ws.CLI("x"), ws.CLI("x")} }, "twice"},
		{"unknown index carry", func(d *ws.Declaration) { d.IndexCarry = "partial" }, "index carry"},
		{"empty index carry", func(d *ws.Declaration) { d.IndexCarry = "" }, "index carry"},
		{"empty ref", func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{""} }, "ref"},
		{"ref outside refs/heads", func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{"refs/tags/v1"} }, "ref"},
		{"bare branch name", func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{"design/*"} }, "ref"},
		{"ref namespace only", func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{"refs/heads/"} }, "ref"},
		{"wildcard inside a ref", func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{"refs/heads/*/x"} }, "ref"},
		{"partial-segment wildcard", func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{"refs/heads/design-*"} }, "ref"},
		{"dot-dot in a ref", func(d *ws.Declaration) { d.RefsDelete = []ws.RefPattern{"refs/heads/a..b"} }, "ref"},
		{"space in a ref", func(d *ws.Declaration) { d.RefsMove = []ws.RefPattern{"refs/heads/a b"} }, "ref"},
		{"unknown ref role", func(d *ws.Declaration) { d.RefsMove = []ws.RefPattern{"@head"} }, "ref"},
		{"checked-out branch created", func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{ws.RefCheckedOut} }, "checked-out"},
		{"checked-out branch deleted", func(d *ws.Declaration) { d.RefsDelete = []ws.RefPattern{ws.RefCheckedOut} }, "checked-out"},
		{"duplicate ref", func(d *ws.Declaration) {
			d.RefsCreate = []ws.RefPattern{"refs/heads/design/*", "refs/heads/design/*"}
		}, "twice"},
		{"empty worktree", func(d *ws.Declaration) { d.Worktrees = []ws.WorktreePattern{""} }, "worktree"},
		{"absolute worktree", func(d *ws.Declaration) { d.Worktrees = []ws.WorktreePattern{"/tmp/*"} }, "worktree"},
		{"worktree without trailing /*", func(d *ws.Declaration) { d.Worktrees = []ws.WorktreePattern{".verdi/data/worktrees"} }, "worktree"},
		{"worktree escaping the store", func(d *ws.Declaration) { d.Worktrees = []ws.WorktreePattern{"../x/*"} }, "worktree"},
		{"unknown worktree role", func(d *ws.Declaration) { d.Worktrees = []ws.WorktreePattern{"@tmp"} }, "worktree"},
		{"duplicate worktree", func(d *ws.Declaration) { d.Worktrees = []ws.WorktreePattern{ws.WorktreeTemp, ws.WorktreeTemp} }, "twice"},
		{"empty stage path", func(d *ws.Declaration) { d.StagePaths = []ws.PathPattern{""} }, "path"},
		{"absolute stage path", func(d *ws.Declaration) { d.StagePaths = []ws.PathPattern{"/etc/x"} }, "path"},
		{"dot-dot stage path", func(d *ws.Declaration) { d.StagePaths = []ws.PathPattern{"a/../b"} }, "path"},
		{"dot stage path", func(d *ws.Declaration) { d.StagePaths = []ws.PathPattern{"./a"} }, "path"},
		{"backslash stage path", func(d *ws.Declaration) { d.StagePaths = []ws.PathPattern{`a\b`} }, "path"},
		{"empty segment", func(d *ws.Declaration) { d.StagePaths = []ws.PathPattern{"a//b"} }, "path"},
		{"partial-segment wildcard path", func(d *ws.Declaration) { d.StagePaths = []ws.PathPattern{"a/*.md"} }, "path"},
		{"deep wildcard inside a path", func(d *ws.Declaration) { d.StagePaths = []ws.PathPattern{"a/**/b"} }, "path"},
		{"duplicate stage path", func(d *ws.Declaration) { d.StagePaths = []ws.PathPattern{"a", "a"} }, "twice"},
		{"no_commit with stage paths", func(d *ws.Declaration) { d.IndexCarry = ws.CarryNoCommit }, "no_commit"},
		{"no_commit with untracked files", func(d *ws.Declaration) {
			d.IndexCarry = ws.CarryNoCommit
			d.StagePaths = nil
			d.UntrackedMayEnter = true
		}, "no_commit"},
		{"scoped with untracked files", func(d *ws.Declaration) { d.UntrackedMayEnter = true }, "scoped"},
		{"scoped over the whole tree", func(d *ws.Declaration) { d.StagePaths = []ws.PathPattern{ws.PathWholeTree} }, "whole tree"},
		{"refused over the whole tree", func(d *ws.Declaration) {
			d.IndexCarry = ws.CarryRefused
			d.StagePaths = []ws.PathPattern{ws.PathWholeTree}
		}, "whole tree"},
		{"scoped commit with no paths", func(d *ws.Declaration) { d.StagePaths = nil }, "stage"},
		{"carried commit with no paths", func(d *ws.Declaration) {
			d.IndexCarry = ws.CarryCarried
			d.StagePaths = nil
		}, "stage"},
		{"scoped commit landing on no branch", func(d *ws.Declaration) { d.RefsCreate = nil }, "branch"},
		{"declares no effect", func(d *ws.Declaration) {
			*d = ws.Declaration{Ritual: "noop", Verbs: []ws.Verb{ws.CLI("noop")}, IndexCarry: ws.CarryNoCommit}
		}, "no effect"},
		{"refused and nothing else", func(d *ws.Declaration) {
			*d = ws.Declaration{Ritual: "noop", Verbs: []ws.Verb{ws.CLI("noop")}, IndexCarry: ws.CarryRefused}
		}, "no effect"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := baseDecl()
			tt.mutate(&d)
			err := d.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error mentioning %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestVerbString(t *testing.T) {
	tests := []struct {
		verb ws.Verb
		want string
	}{
		{ws.CLI("design start"), "cli:design start"},
		{ws.Workbench("/b/{branch}/board/spec/{name}"), "workbench:/b/{branch}/board/spec/{name}"},
		{ws.MCP("import_apply"), "mcp:import_apply"},
		{ws.Verb{}, ":"},
	}
	for _, tt := range tests {
		if got := tt.verb.String(); got != tt.want {
			t.Errorf("%#v.String() = %q, want %q", tt.verb, got, tt.want)
		}
	}
}
