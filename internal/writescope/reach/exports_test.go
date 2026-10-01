package reach_test

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/writescope/reach"
)

func TestExportedFuncs_ListsFunctionsAndExportedMethods(t *testing.T) {
	prog := loadSynth(t)
	got, err := reach.ExportedFuncs(prog, "example.com/synth/gitx")
	if err != nil {
		t.Fatalf("ExportedFuncs: %v", err)
	}
	want := []string{
		"(gitx.Location).String", // exported method of an exported type
		"gitx.CommonDir",
		"gitx.Mutate",
		"gitx.Other",
		"gitx.Prune",
		"gitx.Publish",
		"gitx.Read",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("ExportedFuncs = %q, want %q (an exported method of an unexported type is not API)", got, want)
	}
}

func TestExportedFuncs_UnknownPackage(t *testing.T) {
	prog := loadSynth(t)
	if _, err := reach.ExportedFuncs(prog, "example.com/synth/nope"); err == nil {
		t.Fatal("ExportedFuncs accepted a package the module does not have")
	}
}

func TestFuncByName_ResolvesEveryNameForm(t *testing.T) {
	prog := loadSynth(t)
	tests := []struct {
		name string
		ok   bool
	}{
		{"gitx.Mutate", true},
		{"(gitx.Location).String", true},
		{"(*app.reader).Do", true},
		{"(app.mutator).Do", true},
		{"gitx.Nope", false},
		{"nope.Mutate", false},
		{"(*gitx.Location).Nope", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := prog.FuncByName(tt.name)
			if (fn != nil) != tt.ok {
				t.Fatalf("FuncByName(%q) = %v, want found=%v", tt.name, fn, tt.ok)
			}
			if fn != nil && prog.FuncName(fn) != tt.name {
				t.Fatalf("FuncName(FuncByName(%q)) = %q, want the same name back", tt.name, prog.FuncName(fn))
			}
		})
	}
}
