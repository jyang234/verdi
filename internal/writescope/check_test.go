package writescope_test

import (
	"strings"
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// checkFixture is a small consistent world: one mutating gitx function,
// one read-only one, one git-directory writer, and three verbs, two of
// which reach a mutating function and are declared.
func checkFixture() ([]ws.Declaration, []ws.Classified, ws.Facts) {
	decls := []ws.Declaration{
		{
			Ritual: "sample_commit", Verbs: []ws.Verb{ws.CLI("sample"), ws.MCP("sample_tool")},
			RefsMove: []ws.RefPattern{ws.RefCheckedOut}, StagePaths: []ws.PathPattern{".verdi/x/"},
			IndexCarry: ws.CarryScoped,
		},
	}
	classes := []ws.Classified{
		{Func: "internal/gitx.Commit", Effect: ws.Mutating},
		{Func: "internal/gitx.Read", Effect: ws.ReadOnly},
		{Func: "(*internal/other.R).Write", Effect: ws.Mutating},
	}
	facts := ws.Facts{
		GitxPackage:   "internal/gitx",
		GitxExports:   []string{"internal/gitx.Commit", "internal/gitx.Read"},
		GitDirWriters: []ws.Writer{{Func: "(*internal/other.R).Write", At: "other.go:9"}},
		Functions:     map[string]bool{"(*internal/other.R).Write": true, "cmd/verdi.run": true},
		Verbs: map[ws.Verb][]ws.Hit{
			ws.CLI("sample"):      {{Func: "internal/gitx.Commit", Path: []string{"cmd/verdi.sample", "internal/gitx.Commit"}}},
			ws.MCP("sample_tool"): {{Func: "(*internal/other.R).Write", Path: []string{"tool", "(*internal/other.R).Write"}}},
			ws.CLI("lint"):        nil,
		},
		PreDispatchRoots: 3,
	}
	return decls, classes, facts
}

func TestCheck_ConsistentWorldHasNoFindings(t *testing.T) {
	decls, classes, facts := checkFixture()
	if got := ws.Check(decls, nil, classes, facts); len(got) != 0 {
		t.Fatalf("Check = %v, want no findings", got)
	}
}

func TestCheck_FindsEveryFalsifier(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*[]ws.Declaration, *[]ws.Classified, *ws.Facts)
		wantErr string
	}{
		{"declaration dropped: a verb reaches a mutating function with no declaration", func(d *[]ws.Declaration, _ *[]ws.Classified, _ *ws.Facts) {
			*d = nil
		}, "cli:sample reaches internal/gitx.Commit"},
		{"a new verb reaches a mutating function", func(_ *[]ws.Declaration, _ *[]ws.Classified, f *ws.Facts) {
			f.Verbs[ws.CLI("zap")] = []ws.Hit{{Func: "internal/gitx.Commit", Path: []string{"zap", "internal/gitx.Commit"}}}
		}, "cli:zap reaches internal/gitx.Commit"},
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
			f.GitxExports = []string{"internal/gitx.Commit"}
		}, "internal/gitx.Read no longer exists"},
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
			f.PreDispatch = []ws.Hit{{Func: "internal/gitx.Commit", Path: []string{"cmd/verdi.run", "cmd/verdi.preflight", "internal/gitx.Commit"}}}
		}, "pre-dispatch code reaches internal/gitx.Commit (cmd/verdi.run -> cmd/verdi.preflight -> internal/gitx.Commit)"},
		{"pre-dispatch code was not analyzed", func(_ *[]ws.Declaration, _ *[]ws.Classified, f *ws.Facts) {
			f.PreDispatchRoots = 0
		}, "pre-dispatch"},
		{"no facts at all", func(_ *[]ws.Declaration, _ *[]ws.Classified, f *ws.Facts) {
			*f = ws.Facts{}
		}, "no facts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decls, classes, facts := checkFixture()
			tt.mutate(&decls, &classes, &facts)
			got := ws.Check(decls, nil, classes, facts)
			if !strings.Contains(strings.Join(got, "\n"), tt.wantErr) {
				t.Fatalf("Check findings =\n%s\nwant one mentioning %q", strings.Join(got, "\n"), tt.wantErr)
			}
		})
	}
}
