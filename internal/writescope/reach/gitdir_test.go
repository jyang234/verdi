package reach_test

import (
	"go/types"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/writescope/reach"
)

func TestGitDirWriters_FindsEveryWriteUnderTheGitDirectory(t *testing.T) {
	prog := loadSynth(t)
	locator := lookupFunc(t, prog, "example.com/synth/gitx", "CommonDir")
	writers, err := reach.GitDirWriters(prog, []*types.Func{locator}, []string{"example.com/synth/gitx"})
	if err != nil {
		t.Fatalf("GitDirWriters: %v", err)
	}
	got := map[string]bool{}
	for _, w := range writers {
		got[prog.FuncName(w.Func)] = true
		if !strings.Contains(w.At, "gitdir.go:") {
			t.Errorf("writer %s reports its write at %q, want a position in gitdir.go", prog.FuncName(w.Func), w.At)
		}
	}
	tests := []struct {
		fn   string
		want bool
	}{
		{"gitdir.Reconcile", true},       // locator result reaches os.RemoveAll through three helpers
		{"gitdir.WriteHook", true},       // a ".git" path literal reaches os.WriteFile
		{"gitdir.ViaOtherPackage", true}, // locator result handed to another package's writer
		{"gitdir.CacheKey", false},       // a git-directory value that is never written
		{"gitdir.WriteElsewhere", false}, // a write that never touches the git directory
		{"gitdir.removeOne", false},      // writes only what it is handed: its caller is the writer
		{"other.Write", false},           // generic writer: the caller handing it a git path is flagged
		{"gitdir.removeEntries", false},  // forwards its parameter: not a writer on its own
	}
	for _, tt := range tests {
		t.Run(tt.fn, func(t *testing.T) {
			if got[tt.fn] != tt.want {
				t.Fatalf("writer(%s) = %v, want %v; all writers: %v", tt.fn, got[tt.fn], tt.want, keys(got))
			}
		})
	}
}

func TestGitDirWriters_Errors(t *testing.T) {
	prog := loadSynth(t)
	foreign := types.NewFunc(0, types.NewPackage("example.org/elsewhere", "elsewhere"), "CommonDir", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	tests := []struct {
		name     string
		locators []*types.Func
	}{
		{"no locators", nil},
		{"locator outside the module", []*types.Func{foreign}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reach.GitDirWriters(prog, tt.locators, nil); err == nil {
				t.Fatal("GitDirWriters accepted locators that name no module function")
			}
		})
	}
}

func keys(m map[string]bool) string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return strings.Join(out, ",")
}
