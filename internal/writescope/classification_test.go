package writescope_test

import (
	"strings"
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

func TestClassification_Validates(t *testing.T) {
	if err := ws.ValidateClassification(ws.Classification()); err != nil {
		t.Fatalf("ValidateClassification(Classification()) = %v", err)
	}
}

func TestClassification_ReturnsAFreshValueEachCall(t *testing.T) {
	first := ws.Classification()
	first[0].Effect = "mutated"
	if ws.Classification()[0].Effect == "mutated" {
		t.Fatal("Classification() shares state between calls")
	}
}

func TestMutatingFuncs(t *testing.T) {
	list := []ws.Classified{
		{Func: "internal/gitx.B", Effect: ws.Mutating},
		{Func: "internal/gitx.A", Effect: ws.ReadOnly},
		{Func: "internal/gitx.C", Effect: ws.Mutating},
	}
	if got := strings.Join(ws.MutatingFuncs(list), ","); got != "internal/gitx.B,internal/gitx.C" {
		t.Fatalf("MutatingFuncs = %q, want the two mutating names sorted", got)
	}
	if got := ws.MutatingFuncs(nil); len(got) != 0 {
		t.Fatalf("MutatingFuncs(nil) = %v, want none", got)
	}
}

func TestValidateClassification_Rejects(t *testing.T) {
	tests := []struct {
		name    string
		list    []ws.Classified
		wantErr string
	}{
		{"empty list", nil, "empty"},
		{"empty name", []ws.Classified{{Func: "", Effect: ws.Mutating}}, "name"},
		{"name with spaces", []ws.Classified{{Func: "internal/gitx. AddAll", Effect: ws.Mutating}}, "name"},
		{"name without a package", []ws.Classified{{Func: "AddAll", Effect: ws.Mutating}}, "name"},
		{"unknown effect", []ws.Classified{{Func: "internal/gitx.AddAll", Effect: "writes"}}, "effect"},
		{"empty effect", []ws.Classified{{Func: "internal/gitx.AddAll"}}, "effect"},
		{"duplicate", []ws.Classified{{Func: "internal/gitx.AddAll", Effect: ws.Mutating}, {Func: "internal/gitx.AddAll", Effect: ws.ReadOnly}}, "twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ws.ValidateClassification(tt.list)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateClassification = %v, want an error mentioning %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateClassification_AcceptsEveryNameForm(t *testing.T) {
	list := []ws.Classified{
		{Func: "internal/gitx.AddAll", Effect: ws.Mutating},
		{Func: "(internal/gitx.DiffEntry).Pure", Effect: ws.ReadOnly},
		{Func: "(*internal/execworkspace.GitReconciler).ReconcileUnit", Effect: ws.Mutating},
	}
	if err := ws.ValidateClassification(list); err != nil {
		t.Fatalf("ValidateClassification = %v, want nil", err)
	}
}

// TestClassification_PinsTheCensusMutatingSet pins R1-B4: the gitx
// functions classified mutating are exactly the census's twenty, so a
// reclassification either way is a deliberate change to both lists.
func TestClassification_PinsTheCensusMutatingSet(t *testing.T) {
	var gitx []string
	for _, name := range ws.MutatingFuncs(ws.Classification()) {
		if strings.HasPrefix(name, "internal/gitx.") {
			gitx = append(gitx, name)
		}
	}
	census := ws.CensusMutatingGitx()
	if len(census) != 20 {
		t.Fatalf("the census names %d mutating gitx functions, want 20 (RWS fact pack 2026-09-30, section 1a)", len(census))
	}
	if strings.Join(gitx, ",") != strings.Join(census, ",") {
		t.Fatalf("gitx functions classified mutating =\n%v\nwant the census's\n%v", gitx, census)
	}
}
