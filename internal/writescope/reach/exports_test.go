package reach_test

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/writescope/reach"
)

// testExportedFuncsListsEveryCallableExport pins R1-A4: the export census
// lists every shape a caller outside gitx can run code through, not only
// declared functions and the methods of exported types.
func testExportedFuncsListsEveryCallableExport(t *testing.T, prog *reach.Program) {
	got, err := reach.ExportedFuncs(prog, "example.com/synth/gitx")
	if err != nil {
		t.Fatalf("ExportedFuncs: %v", err)
	}
	want := []string{
		"(gitx.Location).String", // exported method of an exported type (and of its alias Loc)
		"(gitx.Stager).Stage",    // method of an exported interface
		"(gitx.hidden).Visible",  // exported method of an unexported type
		"(gitx.stager).Stage",    // exported method of an unexported type behind an interface
		"gitx.CommonDir",
		"gitx.Mutate",
		"gitx.NewStager",
		"gitx.Ops", // R1-RR4: exported struct variable with a function field
		"gitx.Other",
		"gitx.Prune",
		"gitx.Publish",
		"gitx.Read",
		"gitx.StageAll", // exported variable of function type
		"gitx.Steps",    // R1-RR4: exported map of functions
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("ExportedFuncs =\n%q\nwant\n%q", got, want)
	}
}

func testExportedFuncsErrors(t *testing.T, prog *reach.Program) {
	for _, tt := range []struct{ name, pkg string }{
		{"unknown package", "example.com/synth/nope"},
		{"an exported alias of another package's type", "example.com/synth/aliasgitx"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reach.ExportedFuncs(prog, tt.pkg); err == nil {
				t.Fatalf("ExportedFuncs(%s) succeeded, want an error", tt.pkg)
			}
		})
	}
}

func testObjectByNameResolvesEveryNameForm(t *testing.T, prog *reach.Program) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"gitx.Mutate", true},
		{"(gitx.Location).String", true},
		{"(*app.reader).Do", true},
		{"(app.mutator).Do", true},
		{"gitx.StageAll", true},       // a package-level variable
		{"(gitx.Stager).Stage", true}, // an interface method
		{"(gitx.stager).Stage", true}, // a method of an unexported type
		{"gitx.Nope", false},
		{"nope.Mutate", false},
		{"(*gitx.Location).Nope", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := prog.ObjectByName(tt.name)
			if (obj != nil) != tt.ok {
				t.Fatalf("ObjectByName(%q) = %v, want found=%v", tt.name, obj, tt.ok)
			}
			if obj != nil && prog.ObjectName(obj) != tt.name {
				t.Fatalf("ObjectName(ObjectByName(%q)) = %q, want the same name back", tt.name, prog.ObjectName(obj))
			}
		})
	}
}

func testFuncByNameFindsOnlyDeclaredFunctions(t *testing.T, prog *reach.Program) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"gitx.Mutate", true},
		{"(gitx.stager).Stage", true},
		{"gitx.StageAll", false},       // a variable
		{"(gitx.Stager).Stage", false}, // an interface method
		{"gitx.Nope", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if fn := prog.FuncByName(tt.name); (fn != nil) != tt.ok {
				t.Fatalf("FuncByName(%q) = %v, want found=%v", tt.name, fn, tt.ok)
			}
		})
	}
}
