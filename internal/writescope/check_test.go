package writescope_test

import (
	"strings"
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// checkFixture is a small consistent world: one mutating gitx function
// (the whole-index commit), one read-only one, one git-directory writer,
// three verbs, two of which reach a mutating function and are declared,
// and the scoped declaration's awaiting-fix entry (it reaches the
// whole-index commit).
func checkFixture() ([]ws.Declaration, []ws.AwaitingFix, []ws.Classified, ws.Facts) {
	decls := []ws.Declaration{
		{
			Ritual: "sample_commit", Verbs: []ws.Verb{ws.CLI("sample"), ws.MCP("sample_tool")},
			RefsMove: []ws.RefPattern{ws.RefCheckedOut}, StagePaths: []ws.PathPattern{".verdi/x/"},
			IndexCarry: ws.CarryScoped,
		},
	}
	awaiting := []ws.AwaitingFix{{Ritual: "sample_commit", Path: "verdi sample", Defect: "commits every pre-staged entry"}}
	classes := []ws.Classified{
		{Func: "internal/gitx.CreateCommit", Effect: ws.Mutating},
		{Func: "internal/gitx.Push", Effect: ws.Mutating},
		{Func: "internal/gitx.Read", Effect: ws.ReadOnly},
		{Func: "(*internal/other.R).Write", Effect: ws.Mutating},
	}
	facts := ws.Facts{
		GitxPackage:   "internal/gitx",
		GitxExports:   []string{"internal/gitx.CreateCommit", "internal/gitx.Push", "internal/gitx.Read"},
		GitDirWriters: []ws.Writer{{Func: "(*internal/other.R).Write", At: "other.go:9"}},
		Functions:     map[string]bool{"(*internal/other.R).Write": true, "cmd/verdi.run": true},
		Verbs: map[ws.Verb][]ws.Hit{
			ws.CLI("sample"):      {{Func: "internal/gitx.CreateCommit", Path: []string{"cmd/verdi.sample", "internal/gitx.CreateCommit"}}},
			ws.MCP("sample_tool"): {{Func: "(*internal/other.R).Write", Path: []string{"tool", "(*internal/other.R).Write"}}},
			ws.CLI("lint"):        nil,
		},
		PreDispatchRoots: 3,
	}
	return decls, awaiting, classes, facts
}

func TestCheck_ConsistentWorldHasNoFindings(t *testing.T) {
	decls, awaiting, classes, facts := checkFixture()
	if got := ws.Check(decls, awaiting, classes, facts); len(got) != 0 {
		t.Fatalf("Check = %v, want no findings", got)
	}
}

func TestCheck_FindsEveryFalsifier(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*[]ws.Declaration, *[]ws.Classified, *ws.Facts)
		wantErr string
	}{
		// R1-B3 / mutant BM10: the list is checked against what reaches
		// the whole-index commit, in both directions.
		{"a scoped declaration reaching the whole-index commit with no awaiting-fix entry", func(d *[]ws.Declaration, _ *[]ws.Classified, _ *ws.Facts) {
			(*d)[0].Ritual = "renamed_commit"
		}, "renamed_commit is scoped, but cli:sample reaches internal/gitx.CreateCommit"},
		{"an awaiting-fix entry whose ritual reaches no whole-index commit", func(_ *[]ws.Declaration, c *[]ws.Classified, f *ws.Facts) {
			*c = append(*c, ws.Classified{Func: "internal/gitx.CreateCommitPaths", Effect: ws.Mutating})
			f.GitxExports = append(f.GitxExports, "internal/gitx.CreateCommitPaths")
			f.Verbs[ws.CLI("sample")] = []ws.Hit{{Func: "internal/gitx.CreateCommitPaths", Path: []string{"cmd/verdi.sample", "internal/gitx.CreateCommitPaths"}}}
		}, "no verb of sample_commit reaches internal/gitx.CreateCommit"},
		{"declaration dropped: a verb reaches a mutating function with no declaration", func(d *[]ws.Declaration, _ *[]ws.Classified, _ *ws.Facts) {
			*d = nil
		}, "cli:sample reaches internal/gitx.CreateCommit"},
		{"a new verb reaches a mutating function", func(_ *[]ws.Declaration, _ *[]ws.Classified, f *ws.Facts) {
			f.Verbs[ws.CLI("zap")] = []ws.Hit{{Func: "internal/gitx.CreateCommit", Path: []string{"zap", "internal/gitx.CreateCommit"}}}
		}, "cli:zap reaches internal/gitx.CreateCommit"},
		{"a declaration names a verb that reaches none", func(d *[]ws.Declaration, _ *[]ws.Classified, _ *ws.Facts) {
			(*d)[0].Verbs = append((*d)[0].Verbs, ws.CLI("lint"))
		}, "cli:lint, which reaches no mutating function"},
		{"a declaration names a verb no inventory defines", func(d *[]ws.Declaration, _ *[]ws.Classified, _ *ws.Facts) {
			(*d)[0].Verbs = append((*d)[0].Verbs, ws.CLI("ghost"))
		}, "cli:ghost, which no inventory defines"},
		{"an unclassified exported gitx function", func(_ *[]ws.Declaration, _ *[]ws.Classified, f *ws.Facts) {
			f.GitxExports = append(f.GitxExports, "internal/gitx.Synthetic")
		}, "unclassified exported gitx function internal/gitx.Synthetic"},
		{"an unclassified git-directory writer", func(_ *[]ws.Declaration, _ *[]ws.Classified, f *ws.Facts) {
			f.GitDirWriters = append(f.GitDirWriters, ws.Writer{Func: "internal/x.Hook", At: "x.go:3"})
		}, "unclassified git-directory writer internal/x.Hook"},
		{"a stale gitx name", func(_ *[]ws.Declaration, _ *[]ws.Classified, f *ws.Facts) {
			f.GitxExports = []string{"internal/gitx.CreateCommit", "internal/gitx.Push"}
		}, "internal/gitx.Read no longer exists"},
		// R1-B4 / mutants BM11 and BM12.
		{"a census mutating gitx function classified read-only", func(_ *[]ws.Declaration, c *[]ws.Classified, _ *ws.Facts) {
			setEffect(*c, "internal/gitx.Push", ws.ReadOnly)
		}, "internal/gitx.Push is classified read_only, but the census of 2026-09-30 found it mutating"},
		{"a detected git-directory writer classified read-only", func(_ *[]ws.Declaration, c *[]ws.Classified, _ *ws.Facts) {
			setEffect(*c, "(*internal/other.R).Write", ws.ReadOnly)
		}, "git-directory writer (*internal/other.R).Write (writes at other.go:9) is classified read_only"},
		{"a stale non-gitx name", func(_ *[]ws.Declaration, _ *[]ws.Classified, f *ws.Facts) {
			delete(f.Functions, "(*internal/other.R).Write")
			f.GitDirWriters = nil
		}, "(*internal/other.R).Write no longer exists"},
		{"an invalid registry: carried on a second declaration", func(d *[]ws.Declaration, _ *[]ws.Classified, _ *ws.Facts) {
			(*d)[0].IndexCarry = ws.CarryCarried
			(*d)[0].StagePaths = []ws.PathPattern{ws.PathWholeTree}
		}, "carried"},
		{"an invalid classification", func(_ *[]ws.Declaration, c *[]ws.Classified, _ *ws.Facts) {
			*c = append(*c, ws.Classified{Func: "internal/gitx.Read", Effect: ws.ReadOnly})
		}, "twice"},
		{"pre-dispatch code reaches a mutating function (R1-A2)", func(_ *[]ws.Declaration, _ *[]ws.Classified, f *ws.Facts) {
			f.PreDispatch = []ws.Hit{{Func: "internal/gitx.CreateCommit", Path: []string{"cmd/verdi.run", "cmd/verdi.preflight", "internal/gitx.CreateCommit"}}}
		}, "pre-dispatch code reaches internal/gitx.CreateCommit (cmd/verdi.run -> cmd/verdi.preflight -> internal/gitx.CreateCommit)"},
		{"pre-dispatch code was not analyzed", func(_ *[]ws.Declaration, _ *[]ws.Classified, f *ws.Facts) {
			f.PreDispatchRoots = 0
		}, "pre-dispatch"},
		{"no facts at all", func(_ *[]ws.Declaration, _ *[]ws.Classified, f *ws.Facts) {
			*f = ws.Facts{}
		}, "no facts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decls, awaiting, classes, facts := checkFixture()
			tt.mutate(&decls, &classes, &facts)
			got := ws.Check(decls, awaiting, classes, facts)
			if !strings.Contains(strings.Join(got, "\n"), tt.wantErr) {
				t.Fatalf("Check findings =\n%s\nwant one mentioning %q", strings.Join(got, "\n"), tt.wantErr)
			}
		})
	}
}

// setEffect reclassifies name in list.
func setEffect(list []ws.Classified, name string, e ws.Effect) {
	for i := range list {
		if list[i].Func == name {
			list[i].Effect = e
		}
	}
}
